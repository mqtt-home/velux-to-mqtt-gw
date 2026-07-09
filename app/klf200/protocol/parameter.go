package protocol

import (
	"errors"
	"fmt"
	"math"
)

// Sentinel parameter values (raw uint16). Ported from parameter.Parameter.
const (
	ParameterUnknownValue = 0xF7FF
	ParameterCurrent      = 0xD200
	ParameterMax          = 0xC800
	ParameterMin          = 0x0000
	ParameterOn           = 0x0000
	ParameterOff          = 0xC800
	ParameterTarget       = 0xD100
	ParameterIgnore       = 0xD400
)

// Parameter is a 2-byte parameter value (main or functional parameter of a
// node). It stores the raw big-endian bytes, mirroring parameter.Parameter.
type Parameter struct {
	// Raw holds the two on-wire bytes (big-endian). Always length 2.
	Raw [2]byte
}

// NewParameter creates a Parameter defaulting to the UNKNOWN sentinel; if raw is
// supplied it is validated/normalized via from_raw. Mirrors Parameter.__init__.
func NewParameter(raw *[2]byte) (Parameter, error) {
	p := Parameter{Raw: parameterFromInt(ParameterUnknownValue)}
	if raw != nil {
		norm, err := parameterFromRaw(*raw)
		if err != nil {
			return Parameter{}, err
		}
		p.Raw = norm
	}
	return p, nil
}

// FromParameter copies the raw state from another Parameter. Mirrors from_parameter.
func (p *Parameter) FromParameter(other Parameter) {
	p.Raw = other.Raw
}

// Bytes returns the 2 raw bytes.
func (p Parameter) Bytes() []byte {
	return []byte{p.Raw[0], p.Raw[1]}
}

// String returns "0xHHHH". Mirrors Parameter.__str__.
func (p Parameter) String() string {
	return fmt.Sprintf("0x%02X%02X", p.Raw[0], p.Raw[1])
}

// parameterFromInt renders an int value into 2 big-endian bytes.
// Mirrors Parameter.from_int (validity must be checked by the caller when needed).
func parameterFromInt(value int) [2]byte {
	return [2]byte{byte(value >> 8 & 255), byte(value & 255)}
}

// ParameterIsValidInt reports whether value can be rendered.
// Mirrors Parameter.is_valid_int.
func ParameterIsValidInt(value int) bool {
	if value >= 0 && value <= ParameterMax { // includes ON and OFF
		return true
	}
	switch value {
	case ParameterUnknownValue, ParameterIgnore, ParameterCurrent, ParameterTarget:
		return true
	}
	return false
}

// ParameterFromInt validates and renders value to raw bytes.
// Mirrors Parameter.from_int including its range check.
func ParameterFromInt(value int) ([2]byte, error) {
	if !ParameterIsValidInt(value) {
		return [2]byte{}, errors.New("value out of range")
	}
	return parameterFromInt(value), nil
}

// parameterToInt converts raw bytes to int. Mirrors Position.to_int / Parameter.to_int.
func parameterToInt(raw [2]byte) int {
	return int(raw[0])*256 + int(raw[1])
}

// parameterFromRaw normalizes raw bytes: any value greater than MAX that is not
// one of the CURRENT/IGNORE/TARGET/UNKNOWN sentinels collapses to UNKNOWN.
// Mirrors Position.from_raw.
func parameterFromRaw(raw [2]byte) ([2]byte, error) {
	if raw != parameterFromInt(ParameterCurrent) &&
		raw != parameterFromInt(ParameterIgnore) &&
		raw != parameterFromInt(ParameterTarget) &&
		raw != parameterFromInt(ParameterUnknownValue) &&
		parameterToInt(raw) > ParameterMax {
		return parameterFromInt(ParameterUnknownValue), nil
	}
	return raw, nil
}

// SwitchParameter stores an On/Off value. Mirrors parameter.SwitchParameter.
type SwitchParameter struct {
	Parameter
}

// NewSwitchParameter creates a SwitchParameter, optionally copying from another
// Parameter. Mirrors SwitchParameter.__init__.
func NewSwitchParameter(from *Parameter) SwitchParameter {
	sp := SwitchParameter{Parameter: Parameter{Raw: parameterFromInt(ParameterUnknownValue)}}
	if from != nil {
		sp.FromParameter(*from)
	}
	return sp
}

// SetOn sets the parameter to the 'on' state.
func (sp *SwitchParameter) SetOn() { sp.Raw = parameterFromInt(ParameterOn) }

// SetOff sets the parameter to the 'off' state.
func (sp *SwitchParameter) SetOff() { sp.Raw = parameterFromInt(ParameterOff) }

// IsOn reports whether the parameter is 'on'.
func (sp SwitchParameter) IsOn() bool { return sp.Raw == parameterFromInt(ParameterOn) }

// IsOff reports whether the parameter is 'off'.
func (sp SwitchParameter) IsOff() bool { return sp.Raw == parameterFromInt(ParameterOff) }

// NewSwitchParameterOn returns a SwitchParameter in the 'on' state.
func NewSwitchParameterOn() SwitchParameter {
	sp := NewSwitchParameter(nil)
	sp.SetOn()
	return sp
}

// NewSwitchParameterOff returns a SwitchParameter in the 'off' state.
func NewSwitchParameterOff() SwitchParameter {
	sp := NewSwitchParameter(nil)
	sp.SetOff()
	return sp
}

// Position stores a node position. Mirrors parameter.Position.
type Position struct {
	Parameter
}

// NewPosition builds a Position. Exactly one of the option pointers should be
// non-nil, matching the mutually-exclusive kwargs of Position.__init__
// (parameter / position / positionPercent). If all are nil the position is
// UNKNOWN.
func NewPosition(from *Parameter, position *int, positionPercent *int) (Position, error) {
	p := Position{Parameter: Parameter{Raw: parameterFromInt(ParameterUnknownValue)}}
	switch {
	case from != nil:
		p.FromParameter(*from)
	case position != nil:
		if err := p.SetPosition(*position); err != nil {
			return Position{}, err
		}
	case positionPercent != nil:
		if err := p.SetPositionPercent(*positionPercent); err != nil {
			return Position{}, err
		}
	}
	return p, nil
}

// Known reports whether the position is not the UNKNOWN sentinel.
func (p Position) Known() bool {
	return p.Raw != parameterFromInt(ParameterUnknownValue)
}

// Open reports whether the position is fully open (MIN).
func (p Position) Open() bool {
	return p.Raw == parameterFromInt(ParameterMin)
}

// Closed reports whether the position is fully closed (100%).
// Mirrors Position.closed (percentage-based with device tolerance).
func (p Position) Closed() bool {
	return p.PositionPercent() == 100
}

// Position returns the raw position as an int (0..65535).
func (p Position) Position() int {
	return parameterToInt(p.Raw)
}

// SetPosition sets the raw position from an int, with validation.
func (p *Position) SetPosition(position int) error {
	raw, err := ParameterFromInt(position)
	if err != nil {
		return err
	}
	p.Raw = raw
	return nil
}

// PositionPercent returns the position as an integer percentage.
// Mirrors Position.position_percent (uses to_percent with +0.5 tolerance).
func (p Position) PositionPercent() int {
	return positionToPercent(p.Raw)
}

// SetPositionPercent sets the raw position from a percentage (0..100).
func (p *Position) SetPositionPercent(positionPercent int) error {
	raw, err := positionFromPercent(positionPercent)
	if err != nil {
		return err
	}
	p.Raw = raw
	return nil
}

// String mirrors Position.__str__.
func (p Position) String() string {
	if p.Raw == parameterFromInt(ParameterUnknownValue) {
		return "UNKNOWN"
	}
	return fmt.Sprintf("%d %%", p.PositionPercent())
}

// positionFromPercent renders a percent value to raw bytes.
// Mirrors Position.from_percent: bytes([percent*2, 0]).
func positionFromPercent(positionPercent int) ([2]byte, error) {
	if positionPercent < 0 {
		return [2]byte{}, errors.New("position_percent has to be positive")
	}
	if positionPercent > 100 {
		return [2]byte{}, errors.New("position_percent out of range")
	}
	return [2]byte{byte(positionPercent * 2), 0}, nil
}

// positionToPercent converts raw bytes to an int percentage.
// Mirrors Position.to_percent: int(raw[0]/2 + 0.5).
func positionToPercent(raw [2]byte) int {
	return int(float64(raw[0])/2 + 0.5)
}

// NewUnknownPosition returns a Position with the UNKNOWN sentinel.
func NewUnknownPosition() Position {
	v := ParameterUnknownValue
	p, _ := NewPosition(nil, &v, nil)
	return p
}

// NewCurrentPosition returns the CURRENT-position sentinel (used to stop devices).
func NewCurrentPosition() Position {
	v := ParameterCurrent
	p, _ := NewPosition(nil, &v, nil)
	return p
}

// NewTargetPosition returns the TARGET-position sentinel.
func NewTargetPosition() Position {
	v := ParameterTarget
	p, _ := NewPosition(nil, &v, nil)
	return p
}

// NewIgnorePosition returns the IGNORE sentinel (parameter to be ignored).
func NewIgnorePosition() Position {
	v := ParameterIgnore
	p, _ := NewPosition(nil, &v, nil)
	return p
}

// Intensity stores an intensity value. Mirrors parameter.Intensity.
type Intensity struct {
	Parameter
}

// NewIntensity builds an Intensity. Exactly one of the option pointers should be
// non-nil (parameter / intensity / intensityPercent). If all are nil it is UNKNOWN.
func NewIntensity(from *Parameter, intensity *int, intensityPercent *int) (Intensity, error) {
	i := Intensity{Parameter: Parameter{Raw: parameterFromInt(ParameterUnknownValue)}}
	switch {
	case from != nil:
		i.FromParameter(*from)
	case intensity != nil:
		if err := i.SetIntensity(*intensity); err != nil {
			return Intensity{}, err
		}
	case intensityPercent != nil:
		if err := i.SetIntensityPercent(*intensityPercent); err != nil {
			return Intensity{}, err
		}
	}
	return i, nil
}

// Known reports whether the intensity is not the UNKNOWN sentinel.
func (i Intensity) Known() bool {
	return i.Raw != parameterFromInt(ParameterUnknownValue)
}

// On reports whether intensity is fully on (MIN).
func (i Intensity) On() bool {
	return i.Raw == parameterFromInt(ParameterMin)
}

// Off reports whether intensity is fully off (MAX). Mirrors Intensity.off.
func (i Intensity) Off() bool {
	return i.Raw == [2]byte{byte(ParameterMax >> 8 & 255), byte(ParameterMax & 255)}
}

// Intensity returns the raw intensity as an int.
func (i Intensity) Intensity() int {
	return parameterToInt(i.Raw)
}

// SetIntensity sets the raw intensity from an int, with validation.
func (i *Intensity) SetIntensity(intensity int) error {
	raw, err := ParameterFromInt(intensity)
	if err != nil {
		return err
	}
	i.Raw = raw
	return nil
}

// IntensityPercent returns the intensity as an integer percentage.
// Mirrors Intensity.intensity_percent (uses to_percent WITHOUT the +0.5).
func (i Intensity) IntensityPercent() int {
	return intensityToPercent(i.Raw)
}

// SetIntensityPercent sets the raw intensity from a percentage (0..100).
func (i *Intensity) SetIntensityPercent(intensityPercent int) error {
	raw, err := intensityFromPercent(intensityPercent)
	if err != nil {
		return err
	}
	i.Raw = raw
	return nil
}

// String mirrors Intensity.__str__.
func (i Intensity) String() string {
	if i.Raw == parameterFromInt(ParameterUnknownValue) {
		return "UNKNOWN"
	}
	return fmt.Sprintf("%d %%", i.IntensityPercent())
}

// intensityFromPercent renders percent to raw. Mirrors Intensity.from_percent.
func intensityFromPercent(intensityPercent int) ([2]byte, error) {
	if intensityPercent < 0 {
		return [2]byte{}, errors.New("intensity_percent has to be positive")
	}
	if intensityPercent > 100 {
		return [2]byte{}, errors.New("intensity_percent out of range")
	}
	return [2]byte{byte(intensityPercent * 2), 0}, nil
}

// intensityToPercent converts raw to percent. Mirrors Intensity.to_percent:
// int(raw[0]/2) — note: NO +0.5 tolerance, unlike Position.
func intensityToPercent(raw [2]byte) int {
	return int(raw[0]) / 2
}

// NewUnknownIntensity returns an Intensity with the UNKNOWN sentinel.
func NewUnknownIntensity() Intensity {
	v := ParameterUnknownValue
	i, _ := NewIntensity(nil, &v, nil)
	return i
}

// NewCurrentIntensity returns the CURRENT-intensity sentinel.
func NewCurrentIntensity() Intensity {
	v := ParameterCurrent
	i, _ := NewIntensity(nil, &v, nil)
	return i
}

// LimitationTime sentinel values. Ported from parameter.LimitationTime.
const (
	LimitationTimeUnlimitedRaw   = 253
	LimitationTimeClearMasterRaw = 254
	LimitationTimeClearAllRaw    = 255
)

// LimitationTime stores a single-byte limitation time. Mirrors parameter.LimitationTime.
type LimitationTime struct {
	// Raw holds the single on-wire byte.
	Raw byte
}

// NewLimitationTime constructs a LimitationTime. Provide exactly one of:
//   - limitRaw: raw byte value
//   - limitationTime: a sentinel (UNLIMITED/CLEAR_MASTER/CLEAR_ALL) or raw bus value
//   - timeSeconds: a duration in seconds (rounded up to 30s steps, capped at 252)
//
// If none is provided it defaults to CLEAR_MASTER. This mirrors the precedence
// in LimitationTime.__init__ (limit_raw, then limitation_time, then time).
func NewLimitationTime(timeSeconds *int, limitationTime *int, limitRaw *int) LimitationTime {
	raw := LimitationTimeClearMasterRaw
	if limitRaw != nil {
		raw = *limitRaw
	}
	if limitationTime != nil {
		raw = *limitationTime
	} else if timeSeconds != nil {
		if *timeSeconds > 7590 {
			raw = 252
		} else {
			raw = int(math.Ceil(float64(*timeSeconds)/30)) - 1
		}
	}
	return LimitationTime{Raw: byte(raw)}
}

// Bytes returns the single raw byte.
func (lt LimitationTime) Bytes() []byte { return []byte{lt.Raw} }

// Equal reports byte equality. Mirrors LimitationTime.__eq__.
func (lt LimitationTime) Equal(other LimitationTime) bool { return lt.Raw == other.Raw }

// GetTimeSeconds returns the limitation time in seconds, or -1 for a sentinel
// (UNLIMITED / CLEAR_MASTER / CLEAR_ALL). Mirrors LimitationTime.get_time,
// where sentinels return dedicated subclass objects; here callers should check
// IsUnlimited / IsClearMaster / IsClearAll for those cases.
func (lt LimitationTime) GetTimeSeconds() int {
	switch lt.Raw {
	case LimitationTimeUnlimitedRaw, LimitationTimeClearMasterRaw, LimitationTimeClearAllRaw:
		return -1
	}
	return (int(lt.Raw) + 1) * 30
}

// IsUnlimited reports whether this is the UNLIMITED sentinel.
func (lt LimitationTime) IsUnlimited() bool { return lt.Raw == LimitationTimeUnlimitedRaw }

// IsClearMaster reports whether this is the CLEAR_MASTER sentinel.
func (lt LimitationTime) IsClearMaster() bool { return lt.Raw == LimitationTimeClearMasterRaw }

// IsClearAll reports whether this is the CLEAR_ALL sentinel.
func (lt LimitationTime) IsClearAll() bool { return lt.Raw == LimitationTimeClearAllRaw }

// NewLimitationTimeUnlimited returns the UNLIMITED sentinel.
func NewLimitationTimeUnlimited() LimitationTime {
	v := LimitationTimeUnlimitedRaw
	return NewLimitationTime(nil, &v, nil)
}

// NewLimitationTimeClearMaster returns the CLEAR_MASTER sentinel.
func NewLimitationTimeClearMaster() LimitationTime {
	v := LimitationTimeClearMasterRaw
	return NewLimitationTime(nil, &v, nil)
}

// NewLimitationTimeClearAll returns the CLEAR_ALL sentinel.
func NewLimitationTimeClearAll() LimitationTime {
	v := LimitationTimeClearAllRaw
	return NewLimitationTime(nil, &v, nil)
}
