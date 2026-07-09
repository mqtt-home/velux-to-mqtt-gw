//go:build integration

// Package klf200_test contains a hardware smoke test (OpenSpec task 3.8) that
// runs ONLY with `-tags integration` against a real KLF200 gateway.
//
// It reads the gateway address and API password from the environment so no
// secret ever appears on a command line or in the repo:
//
//	VELUX_HOST=192.168.x.x VELUX_PASSWORD='...' \
//	  go test -tags integration -run TestSmokeKLF200 -v ./klf200/
//
// The KLF200 allows only TWO API sessions and does not free them on an unclean
// disconnect, so this test always disconnects cleanly (defer) and uses a single
// connection. Stop any production bridge first to avoid exhausting the slots.
package klf200_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mqtt-home/velux-mqtt-gw/klf200"
	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// positioner is satisfied by the opening-device node types; used only to print
// live position state without depending on concrete types.
type positioner interface {
	Position() protocol.Position
	TargetPosition() protocol.Position
	LimitationMax() protocol.Position
}

func TestSmokeKLF200(t *testing.T) {
	host := os.Getenv("VELUX_HOST")
	password := os.Getenv("VELUX_PASSWORD")
	if host == "" || password == "" {
		t.Skip("set VELUX_HOST and VELUX_PASSWORD to run the KLF200 hardware smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client := klf200.NewClient(host, password)

	// 1) transport connect
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect %s: %v", host, err)
	}
	// CLEAN disconnect no matter what happens below — releases the session slot.
	defer func() {
		if err := client.Disconnect(); err != nil {
			t.Logf("clean disconnect returned: %v", err)
		} else {
			t.Log("clean disconnect OK (session slot released)")
		}
	}()

	// 2) authenticate
	if err := client.PasswordEnter(ctx, password); err != nil {
		t.Fatalf("password_enter: %v", err)
	}
	t.Log("authenticated")

	// 3) gateway version (proves request/response round-trips over the wire)
	if v, err := client.GetVersion(ctx); err != nil {
		t.Logf("get_version failed (non-fatal): %v", err)
	} else {
		t.Logf("gateway version: %+v", v)
	}

	// 4) best-effort clock sync + house-status monitor for live updates
	if err := client.SetUTC(ctx, time.Now()); err != nil {
		t.Logf("set_utc failed (non-fatal): %v", err)
	}
	if err := klf200.HouseStatusMonitorEnable(ctx, client); err != nil {
		t.Logf("house_status_monitor enable failed (non-fatal): %v", err)
	}

	// 5) load nodes
	if err := client.LoadNodes(ctx); err != nil {
		t.Fatalf("load_nodes: %v", err)
	}
	nodes := client.Nodes().All()
	t.Logf("loaded %d node(s):", len(nodes))
	if len(nodes) == 0 {
		t.Fatal("no nodes returned — check pairing / gateway state")
	}
	for _, n := range nodes {
		line := fmt.Sprintf("  node #%d %q  (%T)", n.NodeID(), n.Name(), n)
		if p, ok := n.(positioner); ok {
			line += fmt.Sprintf("  pos=%d%% target=%d%% limitMax=%d%%",
				p.Position().PositionPercent(),
				p.TargetPosition().PositionPercent(),
				p.LimitationMax().PositionPercent())
		}
		t.Log(line)
	}

	// 6) observe live updates for a short window (move a cover by hand / remote)
	updates := 0
	client.RegisterNodeUpdater(func(f protocol.Frame) {
		updates++
		t.Logf("live frame: %s", f.Command())
	})
	t.Log("observing live updates for 15s — operate a cover to see notifications...")
	select {
	case <-time.After(15 * time.Second):
	case <-ctx.Done():
	}
	t.Logf("observed %d live frame(s)", updates)
	t.Log("SMOKE TEST OK: connect → auth → load nodes → observe → clean disconnect")
}
