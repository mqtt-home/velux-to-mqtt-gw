package bridge

import "testing"

// TestDeriveStateNormal covers every state case for a normal (non-inverted)
// cover, where KLF convention 100% == fully closed.
func TestDeriveStateNormal(t *testing.T) {
	cases := []struct {
		name     string
		position int
		target   int
		want     string
	}{
		{"open at rest (0/0)", 0, 0, StateOpen},
		{"partly open at rest (50/50)", 50, 50, StateOpen},
		{"closed at rest (100/100)", 100, 100, StateClosed},
		{"opening: target below position", 80, 20, StateOpening},
		{"opening: to fully open", 50, 0, StateOpening},
		{"closing: target above position", 20, 80, StateClosing},
		{"closing: to fully closed", 50, 100, StateClosing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DeriveState(c.position, c.target, false); got != c.want {
				t.Fatalf("DeriveState(%d,%d,false)=%q want %q", c.position, c.target, got, c.want)
			}
		})
	}
}

// TestDeriveStateInverted covers every state case for an inverted cover
// (awnings), where 0% == fully closed and moving directions swap.
func TestDeriveStateInverted(t *testing.T) {
	cases := []struct {
		name     string
		position int
		target   int
		want     string
	}{
		{"closed at rest (0/0)", 0, 0, StateClosed},
		{"partly open at rest (50/50)", 50, 50, StateOpen},
		{"open at rest (100/100)", 100, 100, StateOpen},
		{"closing: target below position", 80, 20, StateClosing},
		{"closing: to fully closed", 50, 0, StateClosing},
		{"opening: target above position", 20, 80, StateOpening},
		{"opening: to fully open", 50, 100, StateOpening},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DeriveState(c.position, c.target, true); got != c.want {
				t.Fatalf("DeriveState(%d,%d,true)=%q want %q", c.position, c.target, got, c.want)
			}
		})
	}
}

// TestValidatePosition checks in-range pass-through and out-of-range fallback to
// 0, matching the Python updateCover fallback.
func TestValidatePosition(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, 0},
		{50, 50},
		{100, 100},
		{-1, 0},
		{101, 0},
		{-100, 0},
		{999, 0},
	}
	for _, c := range cases {
		if got := ValidatePosition(c.in); got != c.want {
			t.Errorf("ValidatePosition(%d)=%d want %d", c.in, got, c.want)
		}
	}
}

// TestValidateTarget checks that an out-of-range target falls back to the valid
// position (treated as stopped) and reports wasInvalid, while an in-range target
// passes through unchanged.
func TestValidateTarget(t *testing.T) {
	cases := []struct {
		name        string
		target      int
		validPos    int
		wantResolve int
		wantInvalid bool
	}{
		{"valid target passes through", 30, 70, 30, false},
		{"valid zero target", 0, 50, 0, false},
		{"valid 100 target", 100, 50, 100, false},
		{"negative target -> position", -1, 70, 70, true},
		{"over-range target -> position", 200, 40, 40, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resolved, invalid := ValidateTarget(c.target, c.validPos)
			if resolved != c.wantResolve || invalid != c.wantInvalid {
				t.Fatalf("ValidateTarget(%d,%d)=(%d,%v) want (%d,%v)",
					c.target, c.validPos, resolved, invalid, c.wantResolve, c.wantInvalid)
			}
		})
	}
}

// TestCoverStateFallbacks exercises the composed CoverState with invalid inputs:
// an invalid position falls back to 0, and an invalid target is treated as
// stopped at the (validated) position, so no spurious moving state appears.
func TestCoverStateFallbacks(t *testing.T) {
	cases := []struct {
		name         string
		rawPosition  int
		rawTarget    int
		inverted     bool
		wantPosition int
		wantState    string
	}{
		// Invalid position -> 0; invalid target -> stopped at 0.
		// Normal: position 0 at rest -> open.
		{"invalid pos+target normal", -5, 999, false, 0, StateOpen},
		// Inverted: position 0 at rest -> closed.
		{"invalid pos+target inverted", -5, 999, true, 0, StateClosed},
		// Valid position, invalid target: stopped at position (open normal).
		{"valid pos, invalid target normal", 100, -1, false, 100, StateClosed},
		{"valid pos, invalid target inverted", 100, 500, true, 100, StateOpen},
		// Both valid, sanity through the composed path.
		{"both valid opening normal", 80, 20, false, 80, StateOpening},
		{"both valid opening inverted", 20, 80, true, 20, StateOpening},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pos, state := CoverState(c.rawPosition, c.rawTarget, c.inverted)
			if pos != c.wantPosition || state != c.wantState {
				t.Fatalf("CoverState(%d,%d,%v)=(%d,%q) want (%d,%q)",
					c.rawPosition, c.rawTarget, c.inverted, pos, state, c.wantPosition, c.wantState)
			}
		})
	}
}
