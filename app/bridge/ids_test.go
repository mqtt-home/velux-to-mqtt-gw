package bridge

import "testing"

// TestGenerateID checks umlaut transliteration, space->hyphen, and lowercasing,
// matching the Python generate_id order (replace spaces, lower, transliterate).
func TestGenerateID(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Dachfenster Büro", "vlx-dachfenster-buero"},
		{"Küche", "vlx-kueche"},
		{"Wohnzimmer", "vlx-wohnzimmer"},
		{"Ölheizung", "vlx-oelheizung"},
		{"Straße", "vlx-strasse"},
		{"Ärger Über Ähnliches", "vlx-aerger-ueber-aehnliches"},
		{"MixedCASE Name", "vlx-mixedcase-name"},
		{"multiple   spaces", "vlx-multiple---spaces"},
		{"Rollladen Süd", "vlx-rollladen-sued"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := GenerateID(c.name); got != c.want {
				t.Fatalf("GenerateID(%q)=%q want %q", c.name, got, c.want)
			}
		})
	}
}

// TestNewTopicsWithPrefix verifies every topic is built as {prefix}{id}/... and
// the keep-open switch hangs off {prefix}{id}-keepopen.
func TestNewTopicsWithPrefix(t *testing.T) {
	tp := NewTopics("DEV-", "vlx-kueche")

	checks := map[string]string{
		"Base":              "DEV-vlx-kueche",
		"KeepOpenBase":      "DEV-vlx-kueche-keepopen",
		"State":             "DEV-vlx-kueche/state",
		"Position":          "DEV-vlx-kueche/position",
		"Available":         "DEV-vlx-kueche/available",
		"Set":               "DEV-vlx-kueche/set",
		"KeepOpenState":     "DEV-vlx-kueche-keepopen/state",
		"KeepOpenAvailable": "DEV-vlx-kueche-keepopen/available",
		"KeepOpenSet":       "DEV-vlx-kueche-keepopen/set",
	}
	got := map[string]string{
		"Base":              tp.Base,
		"KeepOpenBase":      tp.KeepOpenBase,
		"State":             tp.State,
		"Position":          tp.Position,
		"Available":         tp.Available,
		"Set":               tp.Set,
		"KeepOpenState":     tp.KeepOpenState,
		"KeepOpenAvailable": tp.KeepOpenAvailable,
		"KeepOpenSet":       tp.KeepOpenSet,
	}
	for k, want := range checks {
		if got[k] != want {
			t.Errorf("Topics.%s = %q, want %q", k, got[k], want)
		}
	}
}

// TestNewTopicsNoPrefix verifies the layout with an empty prefix (id used raw).
func TestNewTopicsNoPrefix(t *testing.T) {
	tp := NewTopics("", "vlx-buero")
	if tp.Base != "vlx-buero" {
		t.Errorf("Base = %q, want %q", tp.Base, "vlx-buero")
	}
	if tp.State != "vlx-buero/state" {
		t.Errorf("State = %q, want %q", tp.State, "vlx-buero/state")
	}
	if tp.KeepOpenSet != "vlx-buero-keepopen/set" {
		t.Errorf("KeepOpenSet = %q, want %q", tp.KeepOpenSet, "vlx-buero-keepopen/set")
	}
}
