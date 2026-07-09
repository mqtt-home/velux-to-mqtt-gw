package bridge

import (
	"context"
	"testing"
)

// newTestCover wires a fakeOpener + fake MQTT into a real Cover with the given
// inversion flag, returning all three for assertions.
func newTestCover(t *testing.T, inverted bool) (*Cover, *fakeOpener, *mqttRecorder) {
	t.Helper()
	node := newFakeOpener("Küche")
	m, rec := newFakeMQTT(true)
	c := NewCover(node, m, "", inverted)
	return c, node, rec
}

// TestCoverCommandNormal checks OPEN/CLOSE/STOP and a numeric position map to
// the expected node methods on a non-inverted cover.
func TestCoverCommandNormal(t *testing.T) {
	ctx := context.Background()

	t.Run("OPEN -> Open", func(t *testing.T) {
		c, node, _ := newTestCover(t, false)
		c.handleCoverCommand(ctx, "OPEN")
		if node.openCalls != 1 || node.closeCalls != 0 {
			t.Fatalf("open=%d close=%d, want open=1 close=0", node.openCalls, node.closeCalls)
		}
	})

	t.Run("CLOSE -> Close", func(t *testing.T) {
		c, node, _ := newTestCover(t, false)
		c.handleCoverCommand(ctx, "CLOSE")
		if node.closeCalls != 1 || node.openCalls != 0 {
			t.Fatalf("open=%d close=%d, want open=0 close=1", node.openCalls, node.closeCalls)
		}
	})

	t.Run("STOP -> Stop", func(t *testing.T) {
		c, node, _ := newTestCover(t, false)
		c.handleCoverCommand(ctx, "STOP")
		if node.stopCalls != 1 {
			t.Fatalf("stop=%d, want 1", node.stopCalls)
		}
	})

	t.Run("numeric -> SetPosition", func(t *testing.T) {
		c, node, _ := newTestCover(t, false)
		c.handleCoverCommand(ctx, "42")
		if len(node.setPositions) != 1 || node.setPositions[0] != 42 {
			t.Fatalf("setPositions=%v, want [42]", node.setPositions)
		}
	})

	t.Run("boundary positions 0 and 100", func(t *testing.T) {
		c, node, _ := newTestCover(t, false)
		c.handleCoverCommand(ctx, "0")
		c.handleCoverCommand(ctx, "100")
		if len(node.setPositions) != 2 || node.setPositions[0] != 0 || node.setPositions[1] != 100 {
			t.Fatalf("setPositions=%v, want [0 100]", node.setPositions)
		}
	})
}

// TestCoverCommandInverted checks OPEN/CLOSE swap to the opposite node method on
// an inverted (awning) cover, while STOP and numeric position are unaffected
// (the numeric position is left to HA's position_open/closed mapping).
func TestCoverCommandInverted(t *testing.T) {
	ctx := context.Background()

	t.Run("OPEN -> Close (inverted)", func(t *testing.T) {
		c, node, _ := newTestCover(t, true)
		c.handleCoverCommand(ctx, "OPEN")
		if node.closeCalls != 1 || node.openCalls != 0 {
			t.Fatalf("open=%d close=%d, want open=0 close=1", node.openCalls, node.closeCalls)
		}
	})

	t.Run("CLOSE -> Open (inverted)", func(t *testing.T) {
		c, node, _ := newTestCover(t, true)
		c.handleCoverCommand(ctx, "CLOSE")
		if node.openCalls != 1 || node.closeCalls != 0 {
			t.Fatalf("open=%d close=%d, want open=1 close=0", node.openCalls, node.closeCalls)
		}
	})

	t.Run("STOP unaffected by inversion", func(t *testing.T) {
		c, node, _ := newTestCover(t, true)
		c.handleCoverCommand(ctx, "STOP")
		if node.stopCalls != 1 {
			t.Fatalf("stop=%d, want 1", node.stopCalls)
		}
	})

	t.Run("numeric position not inverted", func(t *testing.T) {
		c, node, _ := newTestCover(t, true)
		c.handleCoverCommand(ctx, "30")
		if len(node.setPositions) != 1 || node.setPositions[0] != 30 {
			t.Fatalf("setPositions=%v, want [30]", node.setPositions)
		}
	})
}

// TestCoverCommandInvalid ensures malformed / out-of-range payloads are ignored
// (no node method called).
func TestCoverCommandInvalid(t *testing.T) {
	ctx := context.Background()
	invalid := []string{"", "foo", "OPENn", "-1", "101", "3.5", "12abc", "  "}
	for _, payload := range invalid {
		t.Run("payload="+payload, func(t *testing.T) {
			c, node, _ := newTestCover(t, false)
			c.handleCoverCommand(ctx, payload)
			total := node.openCalls + node.closeCalls + node.stopCalls + len(node.setPositions)
			if total != 0 {
				t.Fatalf("payload %q triggered %d node calls, want 0 (open=%d close=%d stop=%d set=%v)",
					payload, total, node.openCalls, node.closeCalls, node.stopCalls, node.setPositions)
			}
		})
	}
}

// TestCoverCommandWhitespaceTrimmed confirms surrounding whitespace on valid
// keyword commands is trimmed (matching TrimSpace in the handler).
func TestCoverCommandWhitespaceTrimmed(t *testing.T) {
	ctx := context.Background()
	c, node, _ := newTestCover(t, false)
	c.handleCoverCommand(ctx, "  OPEN  ")
	if node.openCalls != 1 {
		t.Fatalf("open=%d, want 1 (whitespace-padded OPEN should be trimmed)", node.openCalls)
	}
}

// TestKeepOpenCommand checks ON sets a (0,0) limitation and OFF clears it, and
// invalid payloads are ignored. Inversion does not affect the keep-open switch.
func TestKeepOpenCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("ON sets (0,0) limitation", func(t *testing.T) {
		c, node, _ := newTestCover(t, false)
		c.handleKeepOpenCommand(ctx, "ON")
		if len(node.setLimits) != 1 || node.setLimits[0] != [2]int{0, 0} {
			t.Fatalf("setLimits=%v, want [[0 0]]", node.setLimits)
		}
		if node.clearLimitCalls != 0 {
			t.Fatalf("clearLimitCalls=%d, want 0", node.clearLimitCalls)
		}
	})

	t.Run("OFF clears limitation", func(t *testing.T) {
		c, node, _ := newTestCover(t, false)
		c.handleKeepOpenCommand(ctx, "OFF")
		if node.clearLimitCalls != 1 {
			t.Fatalf("clearLimitCalls=%d, want 1", node.clearLimitCalls)
		}
		if len(node.setLimits) != 0 {
			t.Fatalf("setLimits=%v, want none", node.setLimits)
		}
	})

	t.Run("invalid keep-open payloads ignored", func(t *testing.T) {
		for _, payload := range []string{"", "on", "off", "1", "TRUE", "xyz"} {
			c, node, _ := newTestCover(t, false)
			c.handleKeepOpenCommand(ctx, payload)
			if len(node.setLimits) != 0 || node.clearLimitCalls != 0 {
				t.Fatalf("payload %q: setLimits=%v clear=%d, want none",
					payload, node.setLimits, node.clearLimitCalls)
			}
		}
	})
}

// TestSubscribeRoutesCommands verifies that a delivered MQTT message on the /set
// and keepopen/set topics reaches the right handler via the subscription seam.
func TestSubscribeRoutesCommands(t *testing.T) {
	ctx := context.Background()
	node := newFakeOpener("Küche")
	m, rec := newFakeMQTT(true)
	c := NewCover(node, m, "DEV-", false)

	// Manually register the subscriptions the way Start() does (without touching
	// the KLF200 callback / discovery publish paths).
	m.Subscribe(c.topics.Set, func(payload string) {
		c.handleCoverCommand(ctx, payload)
	})
	m.Subscribe(c.topics.KeepOpenSet, func(payload string) {
		c.handleKeepOpenCommand(ctx, payload)
	})

	if !rec.subscribed(c.topics.Set) || !rec.subscribed(c.topics.KeepOpenSet) {
		t.Fatalf("expected subscriptions for %q and %q", c.topics.Set, c.topics.KeepOpenSet)
	}

	rec.deliver(c.topics.Set, "OPEN")
	if node.openCalls != 1 {
		t.Fatalf("delivering OPEN to %q did not call Open (open=%d)", c.topics.Set, node.openCalls)
	}

	rec.deliver(c.topics.KeepOpenSet, "ON")
	if len(node.setLimits) != 1 {
		t.Fatalf("delivering ON to %q did not set limitation (setLimits=%v)", c.topics.KeepOpenSet, node.setLimits)
	}
}
