package bridge

// MQTT cover state strings, matching the Python bridge and the HA cover
// contract exactly.
const (
	StateOpen    = "open"
	StateClosed  = "closed"
	StateOpening = "opening"
	StateClosing = "closing"
)

// The KLF200 position convention (as reported by pyvlx / the klf200 package):
// 0% == fully open, 100% == fully closed.
const (
	positionFullyOpen   = 0
	positionFullyClosed = 100
)

// ValidatePosition clamps/falls back a raw position percentage the way the
// Python bridge's updateCover does: an out-of-range position (<0 or >100) is
// treated as invalid and replaced with 0 (the fallback used in Python).
// A valid position is returned unchanged.
func ValidatePosition(position int) int {
	if position < 0 || position > 100 {
		return 0
	}
	return position
}

// ValidateTarget resolves the target position against an already-validated
// current position. Mirroring Python: an out-of-range target is treated as
// "stopped at the current position" so no spurious moving state is produced.
// The returned bool reports whether the target was invalid (i.e. treated as
// stopped), for optional logging by the caller.
func ValidateTarget(target, validPosition int) (resolved int, wasInvalid bool) {
	if target < 0 || target > 100 {
		return validPosition, true
	}
	return target, false
}

// DeriveState computes the MQTT cover state from a position and target position
// (both integer percentages). It faithfully ports VeluxMqttCover.updateCover
// (normal) and VeluxMqttCoverInverted.updateCover (inverted).
//
// Callers should pass values already run through ValidatePosition /
// ValidateTarget. DeriveState itself does no clamping so the pure state logic
// stays isolated and testable.
//
// Normal (inverted == false), KLF convention 100% == closed:
//   - position == target: closed if position==100, else open
//   - target  <  position: opening (moving toward more-open / lower %)
//   - target  >  position: closing (moving toward more-closed / higher %)
//
// Inverted (inverted == true), e.g. awnings where 0% == closed:
//   - position == target: closed if position==0, else open
//   - target  <  position: closing
//   - target  >  position: opening
func DeriveState(position, target int, inverted bool) string {
	if inverted {
		return deriveStateInverted(position, target)
	}
	return deriveStateNormal(position, target)
}

func deriveStateNormal(position, target int) string {
	switch {
	case position == target:
		if position == positionFullyClosed {
			return StateClosed
		}
		return StateOpen
	case target < position:
		return StateOpening
	case target > position:
		return StateClosing
	default:
		return StateOpen
	}
}

func deriveStateInverted(position, target int) string {
	switch {
	case position == target:
		if position == positionFullyOpen {
			return StateClosed
		}
		return StateOpen
	case target < position:
		return StateClosing
	case target > position:
		return StateOpening
	default:
		return StateOpen
	}
}

// CoverState is a convenience that validates position/target and derives the
// state in one call, returning the values to publish. It composes
// ValidatePosition, ValidateTarget, and DeriveState so the fan-out cover code
// has a single seam to call and the same fallback semantics everywhere.
func CoverState(rawPosition, rawTarget int, inverted bool) (position int, state string) {
	position = ValidatePosition(rawPosition)
	target, _ := ValidateTarget(rawTarget, position)
	state = DeriveState(position, target, inverted)
	return position, state
}
