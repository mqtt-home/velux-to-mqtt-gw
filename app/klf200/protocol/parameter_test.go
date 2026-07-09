package protocol

import "testing"

// intPtr is a small helper for the option pointers in the New* constructors.
func intPtr(v int) *int { return &v }

// TestPositionPercentRoundTrip verifies position_percent <-> raw against known
// pyvlx values. from_percent = bytes([percent*2, 0]); to_percent (Position) =
// int(raw[0]/2 + 0.5).
func TestPositionPercentRoundTrip(t *testing.T) {
	for pct := 0; pct <= 100; pct++ {
		p, err := NewPosition(nil, nil, intPtr(pct))
		if err != nil {
			t.Fatalf("NewPosition(percent=%d): %v", pct, err)
		}
		wantHi := byte(pct * 2)
		if p.Raw != [2]byte{wantHi, 0} {
			t.Errorf("percent %d: raw = %v, want [%d 0]", pct, p.Raw, wantHi)
		}
		if got := p.PositionPercent(); got != pct {
			t.Errorf("percent %d: PositionPercent round-trip = %d", pct, got)
		}
	}
}

// TestPositionToPercentTolerance pins the +0.5 rounding used by Position.
func TestPositionToPercentTolerance(t *testing.T) {
	// raw[0]=199 -> int(199/2 + 0.5) = int(99.5+0.5) = 100
	p, _ := NewPosition(&Parameter{Raw: [2]byte{199, 0}}, nil, nil)
	if got := p.PositionPercent(); got != 100 {
		t.Errorf("raw[0]=199: PositionPercent = %d, want 100", got)
	}
	if !p.Closed() {
		t.Errorf("raw[0]=199: expected Closed() true")
	}
	// raw[0]=1 -> int(0.5 + 0.5) = int(1.0) = 1
	p2, _ := NewPosition(&Parameter{Raw: [2]byte{1, 0}}, nil, nil)
	if got := p2.PositionPercent(); got != 1 {
		t.Errorf("raw[0]=1: PositionPercent = %d, want 1", got)
	}
}

// TestIntensityToPercentNoTolerance pins the truncation used by Intensity
// (int(raw[0]/2), no +0.5).
func TestIntensityToPercentNoTolerance(t *testing.T) {
	// raw[0]=199 -> int(99.5) = 99  (differs from Position which gives 100)
	i, _ := NewIntensity(&Parameter{Raw: [2]byte{199, 0}}, nil, nil)
	if got := i.IntensityPercent(); got != 99 {
		t.Errorf("raw[0]=199: IntensityPercent = %d, want 99", got)
	}
	// round-trip for exact even values
	for pct := 0; pct <= 100; pct++ {
		in, err := NewIntensity(nil, nil, intPtr(pct))
		if err != nil {
			t.Fatalf("NewIntensity(percent=%d): %v", pct, err)
		}
		if in.Raw != [2]byte{byte(pct * 2), 0} {
			t.Errorf("intensity percent %d: raw = %v", pct, in.Raw)
		}
		if got := in.IntensityPercent(); got != pct {
			t.Errorf("intensity percent %d: round-trip = %d", pct, got)
		}
	}
}

// TestParameterSentinels checks the special sentinel raw values against pyvlx.
func TestParameterSentinels(t *testing.T) {
	cases := []struct {
		name string
		val  int
		raw  [2]byte
	}{
		{"UNKNOWN", ParameterUnknownValue, [2]byte{0xF7, 0xFF}},
		{"CURRENT", ParameterCurrent, [2]byte{0xD2, 0x00}},
		{"MAX", ParameterMax, [2]byte{0xC8, 0x00}},
		{"MIN", ParameterMin, [2]byte{0x00, 0x00}},
		{"TARGET", ParameterTarget, [2]byte{0xD1, 0x00}},
		{"IGNORE", ParameterIgnore, [2]byte{0xD4, 0x00}},
	}
	for _, c := range cases {
		got, err := ParameterFromInt(c.val)
		if err != nil {
			t.Fatalf("%s: ParameterFromInt: %v", c.name, err)
		}
		if got != c.raw {
			t.Errorf("%s: raw = %v, want %v", c.name, got, c.raw)
		}
		if parameterToInt(got) != c.val {
			t.Errorf("%s: to_int(%v) = %d, want %d", c.name, got, parameterToInt(got), c.val)
		}
	}
}

// TestParameterFromRawNormalization verifies that a raw value above MAX that is
// not a known sentinel collapses to UNKNOWN, while sentinels are preserved.
func TestParameterFromRawNormalization(t *testing.T) {
	// Above MAX, not a sentinel -> UNKNOWN.
	p, err := NewParameter(&[2]byte{0xC9, 0x00}) // 0xC900 > 0xC800
	if err != nil {
		t.Fatal(err)
	}
	if p.Raw != [2]byte{0xF7, 0xFF} {
		t.Errorf("0xC900 normalized to %v, want UNKNOWN 0xF7FF", p.Raw)
	}
	// Sentinels preserved.
	for _, raw := range [][2]byte{{0xD2, 0x00}, {0xD1, 0x00}, {0xD4, 0x00}, {0xF7, 0xFF}} {
		q, err := NewParameter(&raw)
		if err != nil {
			t.Fatal(err)
		}
		if q.Raw != raw {
			t.Errorf("sentinel %v was altered to %v", raw, q.Raw)
		}
	}
	// A value <= MAX is preserved as-is.
	r, err := NewParameter(&[2]byte{0xC8, 0x00}) // exactly MAX
	if err != nil {
		t.Fatal(err)
	}
	if r.Raw != [2]byte{0xC8, 0x00} {
		t.Errorf("MAX altered to %v", r.Raw)
	}
}

// TestParameterIsValidInt pins the accepted range and sentinels.
func TestParameterIsValidInt(t *testing.T) {
	valid := []int{0, 1, ParameterMax, ParameterUnknownValue, ParameterCurrent, ParameterTarget, ParameterIgnore}
	for _, v := range valid {
		if !ParameterIsValidInt(v) {
			t.Errorf("expected %#x valid", v)
		}
	}
	invalid := []int{-1, ParameterMax + 1, 0xC900, 0xD000}
	for _, v := range invalid {
		if ParameterIsValidInt(v) {
			t.Errorf("expected %#x invalid", v)
		}
	}
	// Out-of-range values must be rejected by ParameterFromInt.
	if _, err := ParameterFromInt(0xC900); err == nil {
		t.Errorf("ParameterFromInt(0xC900) expected error")
	}
}

// TestSwitchParameter checks on/off state helpers against ON/OFF raw values.
func TestSwitchParameter(t *testing.T) {
	on := NewSwitchParameterOn()
	if !on.IsOn() || on.IsOff() {
		t.Errorf("on: IsOn=%v IsOff=%v", on.IsOn(), on.IsOff())
	}
	if on.Raw != [2]byte{0x00, 0x00} {
		t.Errorf("on raw = %v, want 0x0000", on.Raw)
	}
	off := NewSwitchParameterOff()
	if !off.IsOff() || off.IsOn() {
		t.Errorf("off: IsOn=%v IsOff=%v", off.IsOn(), off.IsOff())
	}
	if off.Raw != [2]byte{0xC8, 0x00} {
		t.Errorf("off raw = %v, want 0xC800", off.Raw)
	}
}

// TestLimitationTime verifies raw computation and sentinels vs pyvlx.
func TestLimitationTime(t *testing.T) {
	// time in seconds -> ceil(time/30) - 1
	cases := []struct {
		seconds int
		raw     byte
		secs    int
	}{
		{30, 0, 30},                 // ceil(1)-1 = 0 ; get_time = (0+1)*30 = 30
		{60, 1, 60},                 // ceil(2)-1 = 1 ; (1+1)*30 = 60
		{45, 1, 60},                 // ceil(1.5)-1 = 1
		{7590, 252, (252 + 1) * 30}, // boundary, not >7590
	}
	for _, c := range cases {
		lt := NewLimitationTime(intPtr(c.seconds), nil, nil)
		if lt.Raw != c.raw {
			t.Errorf("time %ds: raw = %d, want %d", c.seconds, lt.Raw, c.raw)
		}
		if got := lt.GetTimeSeconds(); got != c.secs {
			t.Errorf("time %ds: GetTimeSeconds = %d, want %d", c.seconds, got, c.secs)
		}
	}
	// > 7590 clamps to 252.
	big := NewLimitationTime(intPtr(8000), nil, nil)
	if big.Raw != 252 {
		t.Errorf("time 8000s: raw = %d, want 252", big.Raw)
	}

	// Sentinels.
	if !NewLimitationTimeUnlimited().IsUnlimited() {
		t.Error("unlimited sentinel wrong")
	}
	if !NewLimitationTimeClearMaster().IsClearMaster() {
		t.Error("clear-master sentinel wrong")
	}
	if !NewLimitationTimeClearAll().IsClearAll() {
		t.Error("clear-all sentinel wrong")
	}
	if NewLimitationTimeUnlimited().Raw != 253 ||
		NewLimitationTimeClearMaster().Raw != 254 ||
		NewLimitationTimeClearAll().Raw != 255 {
		t.Error("sentinel raw values do not match pyvlx (253/254/255)")
	}
	// Sentinels report -1 seconds.
	if NewLimitationTimeUnlimited().GetTimeSeconds() != -1 {
		t.Error("unlimited GetTimeSeconds should be -1")
	}
	// Default (no args) is CLEAR_MASTER.
	if !NewLimitationTime(nil, nil, nil).IsClearMaster() {
		t.Error("default LimitationTime should be CLEAR_MASTER")
	}
}
