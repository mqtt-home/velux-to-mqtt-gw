package bridge

import (
	"strconv"
	"testing"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
)

// TestUpdateNodePublishesState checks that UpdateNode publishes the derived
// position + state (normal and inverted) and marks the cover online.
func TestUpdateNodePublishesState(t *testing.T) {
	cases := []struct {
		name        string
		inverted    bool
		posPercent  int
		targPercent int
		wantPos     int
		wantState   string
	}{
		{"normal closing", false, 20, 80, 20, StateClosing},
		{"normal closed at rest", false, 100, 100, 100, StateClosed},
		{"inverted opening", true, 20, 80, 20, StateOpening},
		{"inverted closed at rest", true, 0, 0, 0, StateClosed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := newFakeOpener("Küche")
			node.position = mustPercentPosition(c.posPercent)
			node.target = mustPercentPosition(c.targPercent)
			m, rec := newFakeMQTT(true)
			cover := NewCover(node, m, "", c.inverted)

			cover.UpdateNode()

			if pos, ok := rec.last(cover.topics.Position); !ok || pos != strconv.Itoa(c.wantPos) {
				t.Errorf("position published=%q (ok=%v), want %d", pos, ok, c.wantPos)
			}
			if state, ok := rec.last(cover.topics.State); !ok || state != c.wantState {
				t.Errorf("state published=%q (ok=%v), want %q", state, ok, c.wantState)
			}
			if avail, ok := rec.last(cover.topics.Available); !ok || avail != availableOnline {
				t.Errorf("availability=%q (ok=%v), want %q", avail, ok, availableOnline)
			}
		})
	}
}

// TestUpdateLimitSwitchState checks the keep-open switch state: 'on' when the
// max limitation is below fully-open (< 100%), else 'off', with an unknown
// limitation falling back to 'off'.
func TestUpdateLimitSwitchState(t *testing.T) {
	cases := []struct {
		name     string
		limitMax protocol.Position
		want     string
	}{
		{"unknown limitation -> off", protocol.NewUnknownPosition(), keepOpenOff},
		{"fully open (100%) -> off", mustPercentPosition(100), keepOpenOff},
		{"limited to 0% -> on", mustPercentPosition(0), keepOpenOn},
		{"limited to 50% -> on", mustPercentPosition(50), keepOpenOn},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := newFakeOpener("Küche")
			node.limitMax = c.limitMax
			m, rec := newFakeMQTT(true)
			cover := NewCover(node, m, "", false)

			cover.updateLimitSwitch()

			if got, ok := rec.last(cover.topics.KeepOpenState); !ok || got != c.want {
				t.Fatalf("keep-open state=%q (ok=%v), want %q", got, ok, c.want)
			}
		})
	}
}

// TestPublishAvailability checks online/offline mapping to both the cover and
// keep-open availability topics.
func TestPublishAvailability(t *testing.T) {
	node := newFakeOpener("Küche")
	m, rec := newFakeMQTT(true)
	cover := NewCover(node, m, "", false)

	cover.PublishAvailability(false)
	if got, _ := rec.last(cover.topics.Available); got != availableOffline {
		t.Errorf("cover availability=%q, want %q", got, availableOffline)
	}
	if got, _ := rec.last(cover.topics.KeepOpenAvailable); got != availableOffline {
		t.Errorf("keep-open availability=%q, want %q", got, availableOffline)
	}

	cover.PublishAvailability(true)
	if got, _ := rec.last(cover.topics.Available); got != availableOnline {
		t.Errorf("cover availability=%q, want %q", got, availableOnline)
	}
}
