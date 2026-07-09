//go:build integration

package klf200_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mqtt-home/velux-mqtt-gw/klf200"
	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// mover is satisfied by the roller-shutter node type: read position + command it.
type mover interface {
	Position() protocol.Position
	TargetPosition() protocol.Position
	SetPosition(ctx context.Context, position protocol.Position, waitForCompletion bool) error
}

// TestLiveCommandKLF200 physically moves one cover and verifies the full
// command + live-update round-trip, then restores the cover to its original
// position (guaranteed via defer, even on failure).
//
//	VELUX_HOST=.. VELUX_PASSWORD=.. VELUX_LIVE_MOVE=1 [VELUX_LIVE_NODE=office] \
//	  go test -tags integration -run TestLiveCommandKLF200 -v -timeout 240s ./klf200/
func TestLiveCommandKLF200(t *testing.T) {
	host := os.Getenv("VELUX_HOST")
	password := os.Getenv("VELUX_PASSWORD")
	if host == "" || password == "" {
		t.Skip("set VELUX_HOST and VELUX_PASSWORD")
	}
	if os.Getenv("VELUX_LIVE_MOVE") == "" {
		t.Skip("set VELUX_LIVE_MOVE=1 to allow physically moving a cover")
	}
	nodeName := os.Getenv("VELUX_LIVE_NODE")
	if nodeName == "" {
		nodeName = "office"
	}

	pos := func(pct int) protocol.Position {
		p, err := protocol.NewPosition(nil, nil, &pct)
		if err != nil {
			t.Fatalf("build position %d%%: %v", pct, err)
		}
		return p
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Second)
	defer cancel()

	client := klf200.NewClient(host, password)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()
	if err := client.PasswordEnter(ctx, password); err != nil {
		t.Fatalf("auth: %v", err)
	}
	if err := klf200.HouseStatusMonitorEnable(ctx, client); err != nil {
		t.Logf("HSM enable (non-fatal): %v", err)
	}
	// Wire the mutating node updater exactly as the production app does, so live
	// GW_NODE_STATE_POSITION_CHANGED_NTF frames flow into the node model.
	klf200.NewNodeUpdater(client).Register()
	if err := client.LoadNodes(ctx); err != nil {
		t.Fatalf("load nodes: %v", err)
	}

	node, ok := client.Nodes().ByName(nodeName)
	if !ok {
		t.Fatalf("node %q not found", nodeName)
	}
	mv, ok := node.(mover)
	if !ok {
		t.Fatalf("node %q (%T) is not movable", nodeName, node)
	}

	start := mv.Position().PositionPercent()
	t.Logf("%q start position: %d%%", nodeName, start)

	// Count live position-change notifications.
	live := 0
	client.RegisterNodeUpdater(func(f protocol.Frame) {
		if n, ok := f.(*protocol.FrameNodeStatePositionChangedNotification); ok {
			live++
			cp, _ := protocol.NewPosition(&n.CurrentPosition, nil, nil)
			tp, _ := protocol.NewPosition(&n.Target, nil, nil)
			t.Logf("live NTF: node=%d state=%d curRaw=% x cur=%d%% known=%v tgtRaw=% x tgt=%d%%",
				n.NodeID, n.State, n.CurrentPosition.Bytes(), cp.PositionPercent(), cp.Known(),
				n.Target.Bytes(), tp.PositionPercent())
		}
	})

	// Choose a gentle, clearly-observable target and ALWAYS restore afterwards.
	target := start - 20
	if target < 0 {
		target = start + 20
	}

	// waitTarget mirrors the production bridge: fire the command asynchronously
	// (wait_for_completion=False, as the Python bridge does) and then observe the
	// node's position converge via live GW_NODE_STATE_POSITION_CHANGED_NTF frames.
	waitTarget := func(want int) (int, bool) {
		deadline := time.Now().Add(70 * time.Second)
		for time.Now().Before(deadline) {
			if p := mv.Position().PositionPercent(); p >= want-5 && p <= want+5 {
				return p, true
			}
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return mv.Position().PositionPercent(), false
			}
		}
		return mv.Position().PositionPercent(), false
	}

	// Guaranteed restore to the original position, even if an assertion fails.
	defer func() {
		rctx, rcancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer rcancel()
		t.Logf("restoring %q to %d%%", nodeName, start)
		if err := mv.SetPosition(rctx, pos(start), false); err != nil {
			t.Errorf("RESTORE command failed — %q may be left at ~%d%%: %v", nodeName, target, err)
			return
		}
		if got, ok := waitTarget(start); ok {
			t.Logf("restored %q to %d%%", nodeName, got)
		} else {
			t.Errorf("RESTORE did not converge — %q at %d%%, want %d%%", nodeName, got, start)
		}
	}()

	t.Logf("moving %q %d%% -> %d%% (async, observing live updates)...", nodeName, start, target)
	if err := mv.SetPosition(ctx, pos(target), false); err != nil {
		t.Fatalf("SetPosition %d%%: %v", target, err)
	}

	got, ok := waitTarget(target)
	t.Logf("%q converged to %d%% (reached=%v, live frames: %d)", nodeName, got, ok, live)

	if live == 0 {
		t.Errorf("no live GW_NODE_STATE_POSITION_CHANGED_NTF observed during the move")
	}
	if !ok {
		t.Errorf("position did not reach target via live updates: got %d%%, want ~%d%%", got, target)
	}
	t.Log("LIVE COMMAND TEST OK: async command + physical movement observed via live updates")
}
