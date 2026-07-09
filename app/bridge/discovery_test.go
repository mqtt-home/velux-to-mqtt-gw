package bridge

import (
	"encoding/json"
	"testing"

	"github.com/mqtt-home/velux-mqtt-gw/klf200"
)

// TestDeviceClassFor maps each concrete klf200 opening-device type to its HA
// cover device_class. The zero-value struct is sufficient because DeviceClassFor
// only performs a type switch (no method calls).
func TestDeviceClassFor(t *testing.T) {
	cases := []struct {
		name string
		node Opener
		want string
	}{
		{"Window", &klf200.Window{}, DeviceClassWindow},
		{"Blind", &klf200.Blind{}, DeviceClassBlind},
		{"Awning", &klf200.Awning{}, DeviceClassAwning},
		{"RollerShutter", &klf200.RollerShutter{}, DeviceClassShutter},
		{"GarageDoor", &klf200.GarageDoor{}, DeviceClassGarage},
		{"Gate", &klf200.Gate{}, DeviceClassGate},
		{"Blade", &klf200.Blade{}, DeviceClassShade},
		{"fake/unknown", newFakeOpener("x"), DeviceClassNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DeviceClassFor(c.node); got != c.want {
				t.Fatalf("DeviceClassFor(%s)=%q want %q", c.name, got, c.want)
			}
		})
	}
}

// unmarshalCover builds the cover discovery for a cover and returns the topic
// plus parsed payload.
func unmarshalCover(t *testing.T, c *Cover) (string, coverDiscoveryPayload) {
	t.Helper()
	topic, raw, err := buildCoverDiscovery(c)
	if err != nil {
		t.Fatalf("buildCoverDiscovery: %v", err)
	}
	var p coverDiscoveryPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal cover payload: %v", err)
	}
	return topic, p
}

// TestCoverDiscoveryDeviceClass checks the built cover payload carries the
// device_class for a concrete node type. A Window is used here because
// DeviceClassFor is exercised via NewCover.
func TestCoverDiscoveryDeviceClass(t *testing.T) {
	m, _ := newFakeMQTT(true)
	c := NewCover(&klf200.Window{}, m, "", false)
	_, p := unmarshalCover(t, c)
	if p.DeviceClass != DeviceClassWindow {
		t.Fatalf("device_class=%q want %q", p.DeviceClass, DeviceClassWindow)
	}
}

// TestCoverDiscoveryPositionNormal checks the normal position_open/closed
// mapping (open=0, closed=100).
func TestCoverDiscoveryPositionNormal(t *testing.T) {
	m, _ := newFakeMQTT(true)
	c := NewCover(newFakeOpener("Fenster"), m, "", false)
	_, p := unmarshalCover(t, c)
	if p.PositionOpen != 0 || p.PositionClosed != 100 {
		t.Fatalf("normal position_open=%d position_closed=%d, want 0/100", p.PositionOpen, p.PositionClosed)
	}
}

// TestCoverDiscoveryPositionInverted checks the inverted mapping swaps
// position_open/closed (open=100, closed=0).
func TestCoverDiscoveryPositionInverted(t *testing.T) {
	m, _ := newFakeMQTT(true)
	c := NewCover(&klf200.Awning{}, m, "", true)
	_, p := unmarshalCover(t, c)
	if p.PositionOpen != 100 || p.PositionClosed != 0 {
		t.Fatalf("inverted position_open=%d position_closed=%d, want 100/0", p.PositionOpen, p.PositionClosed)
	}
}

// TestSharedDeviceBlock checks the cover entity and the keep-open switch share
// the same device block (identifiers + name), so HA groups them under one tile.
func TestSharedDeviceBlock(t *testing.T) {
	m, _ := newFakeMQTT(true)
	c := NewCover(newFakeOpener("Dachfenster Büro"), m, "DEV-", false)

	_, coverP := unmarshalCover(t, c)

	switchTopic, switchRaw, err := buildSwitchDiscovery(c)
	if err != nil {
		t.Fatalf("buildSwitchDiscovery: %v", err)
	}
	var switchP switchDiscoveryPayload
	if err := json.Unmarshal(switchRaw, &switchP); err != nil {
		t.Fatalf("unmarshal switch payload: %v", err)
	}

	if len(coverP.Device.Identifiers) != 1 || len(switchP.Device.Identifiers) != 1 {
		t.Fatalf("expected 1 identifier each; cover=%v switch=%v",
			coverP.Device.Identifiers, switchP.Device.Identifiers)
	}
	if coverP.Device.Identifiers[0] != switchP.Device.Identifiers[0] {
		t.Fatalf("device identifiers differ: cover=%q switch=%q",
			coverP.Device.Identifiers[0], switchP.Device.Identifiers[0])
	}
	if coverP.Device.Name != switchP.Device.Name {
		t.Fatalf("device names differ: cover=%q switch=%q", coverP.Device.Name, switchP.Device.Name)
	}
	// Identifier is {prefix}{id} == topics.Base.
	if coverP.Device.Identifiers[0] != c.topics.Base {
		t.Fatalf("device identifier=%q want %q", coverP.Device.Identifiers[0], c.topics.Base)
	}

	// Discovery topics live under the correct HA component roots.
	coverTopic, _, _ := buildCoverDiscovery(c)
	wantCover := "homeassistant/cover/DEV-vlx-dachfenster-buero/config"
	wantSwitch := "homeassistant/switch/DEV-vlx-dachfenster-buero-keepopen/config"
	if coverTopic != wantCover {
		t.Errorf("cover discovery topic=%q want %q", coverTopic, wantCover)
	}
	if switchTopic != wantSwitch {
		t.Errorf("switch discovery topic=%q want %q", switchTopic, wantSwitch)
	}
}

// TestPublishDiscoveryTopicsAndCleanup verifies that publishing discovery
// records the two config topics retained, and that CleanupDiscovery clears
// exactly those tracked topics with an empty retained payload.
func TestPublishDiscoveryTopicsAndCleanup(t *testing.T) {
	// Reset the package-level tracked-topics set to isolate this test.
	discoveredMu.Lock()
	discoveredTopics = map[string]struct{}{}
	discoveredMu.Unlock()

	m, rec := newFakeMQTT(true)
	c := NewCover(newFakeOpener("Küche"), m, "", false)

	if err := c.PublishDiscovery(); err != nil {
		t.Fatalf("PublishDiscovery: %v", err)
	}

	coverTopic := "homeassistant/cover/vlx-kueche/config"
	switchTopic := "homeassistant/switch/vlx-kueche-keepopen/config"

	// Both config topics must have been published retained with non-empty JSON.
	for _, topic := range []string{coverTopic, switchTopic} {
		payload, ok := rec.last(topic)
		if !ok {
			t.Fatalf("no publish recorded for %q", topic)
		}
		if payload == "" {
			t.Fatalf("discovery payload for %q was empty at publish time", topic)
		}
	}

	// Cleanup clears exactly the tracked topics with an empty retained payload.
	CleanupDiscovery(m)

	for _, topic := range []string{coverTopic, switchTopic} {
		payload, ok := rec.last(topic)
		if !ok {
			t.Fatalf("no publish recorded for %q after cleanup", topic)
		}
		if payload != "" {
			t.Fatalf("cleanup for %q left payload %q, want empty", topic, payload)
		}
	}

	// The tracked set must be empty after cleanup so a second call is a no-op.
	discoveredMu.Lock()
	remaining := len(discoveredTopics)
	discoveredMu.Unlock()
	if remaining != 0 {
		t.Fatalf("discoveredTopics has %d entries after cleanup, want 0", remaining)
	}
}
