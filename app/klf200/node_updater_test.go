package klf200

import (
	"sync/atomic"
	"testing"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// TestNodeUpdater_PositionChangedNotification_UpdatesNodeAndFiresCallback feeds a
// GW_NODE_STATE_POSITION_CHANGED_NTF through the updater and asserts the matching
// node's position/target are updated and its device-updated callback fires.
// Ported from NodeUpdater.process_frame.
func TestNodeUpdater_PositionChangedNotification_UpdatesNodeAndFiresCallback(t *testing.T) {
	client := newTestClient(&fakeAPI{})

	// Register a roller shutter starting at 0%.
	zero, _ := protocol.NewPosition(nil, nil, intp(0))
	info := NodeInfo{NodeType: protocol.NodeTypeWithSubtypeRollerShutter, CurrentPosition: zero.Parameter, Target: zero.Parameter}
	rs := NodeFromInfo(client, 4, "rs", [8]byte{}, info).(*RollerShutter)
	client.Nodes().Add(rs)

	var fired int32
	var gotNode Node
	rs.RegisterDeviceUpdatedCB(func(n Node) {
		atomic.AddInt32(&fired, 1)
		gotNode = n
	})

	updater := NewNodeUpdater(client)

	newPos, _ := protocol.NewPosition(nil, nil, intp(60))
	newTarget, _ := protocol.NewPosition(nil, nil, intp(80))
	ntf := &protocol.FrameNodeStatePositionChangedNotification{
		NodeID:          4,
		CurrentPosition: newPos.Parameter,
		Target:          newTarget.Parameter,
	}
	updater.processFrame(ntf)

	if got := atomic.LoadInt32(&fired); got != 1 {
		t.Fatalf("callback fired %d times, want 1", got)
	}
	if gotNode == nil || gotNode.NodeID() != 4 {
		t.Fatalf("callback node = %v, want node 4", gotNode)
	}
	if rs.Position().Raw != newPos.Raw {
		t.Fatalf("position = %v, want %v (60%%)", rs.Position().Raw, newPos.Raw)
	}
	if rs.TargetPosition().Raw != newTarget.Raw {
		t.Fatalf("target = %v, want %v (80%%)", rs.TargetPosition().Raw, newTarget.Raw)
	}
}

// TestNodeUpdater_UnknownNodeIgnored verifies a notification for a node that is
// not in the collection is silently ignored (no panic). Ported from the
// "node_id not in nodes" guard.
func TestNodeUpdater_UnknownNodeIgnored(t *testing.T) {
	client := newTestClient(&fakeAPI{})
	updater := NewNodeUpdater(client)

	pos, _ := protocol.NewPosition(nil, nil, intp(50))
	ntf := &protocol.FrameNodeStatePositionChangedNotification{
		NodeID:          99,
		CurrentPosition: pos.Parameter,
		Target:          pos.Parameter,
	}
	updater.processFrame(ntf) // must not panic
}

// TestNodeUpdater_BlindDoesNotUpdateOrientationFromPositionFrame verifies the
// House-Monitor FP3 workaround: a position-changed notification updates a Blind's
// position but NOT its orientation. Ported from the commented-out orientation
// branch in NodeUpdater.process_frame.
func TestNodeUpdater_BlindDoesNotUpdateOrientationFromPositionFrame(t *testing.T) {
	client := newTestClient(&fakeAPI{})

	zero, _ := protocol.NewPosition(nil, nil, intp(0))
	info := NodeInfo{NodeType: protocol.NodeTypeWithSubtypeExteriorVenetianBlind, CurrentPosition: zero.Parameter, Target: zero.Parameter}
	blind := NodeFromInfo(client, 6, "b", [8]byte{}, info).(*Blind)
	client.Nodes().Add(blind)

	origOrientation := blind.Orientation().Raw

	updater := NewNodeUpdater(client)
	newPos, _ := protocol.NewPosition(nil, nil, intp(45))
	ntf := &protocol.FrameNodeStatePositionChangedNotification{
		NodeID:          6,
		CurrentPosition: newPos.Parameter,
		Target:          newPos.Parameter,
	}
	updater.processFrame(ntf)

	if blind.Position().Raw != newPos.Raw {
		t.Fatalf("blind position = %v, want %v", blind.Position().Raw, newPos.Raw)
	}
	if blind.Orientation().Raw != origOrientation {
		t.Fatalf("blind orientation changed to %v; must be untouched by a position frame", blind.Orientation().Raw)
	}
}

// TestNodeUpdater_StatusRequestNotification_UpdatesBlindOrientation feeds a
// GW_STATUS_REQUEST_NTF with MP and FP3 and asserts the Blind's position and
// orientation are updated (the heartbeat FP3 refresh path). Ported from
// NodeUpdater.process_frame_status_request_notification.
func TestNodeUpdater_StatusRequestNotification_UpdatesBlindOrientation(t *testing.T) {
	client := newTestClient(&fakeAPI{})

	zero, _ := protocol.NewPosition(nil, nil, intp(0))
	info := NodeInfo{NodeType: protocol.NodeTypeWithSubtypeExteriorVenetianBlind, CurrentPosition: zero.Parameter, Target: zero.Parameter}
	blind := NodeFromInfo(client, 8, "b", [8]byte{}, info).(*Blind)
	client.Nodes().Add(blind)

	var fired int32
	blind.RegisterDeviceUpdatedCB(func(Node) { atomic.AddInt32(&fired, 1) })

	updater := NewNodeUpdater(client)

	mp, _ := protocol.NewPosition(nil, nil, intp(35))
	fp3, _ := protocol.NewPosition(nil, nil, intp(70))
	ntf := &protocol.FrameStatusRequestNotification{
		NodeID: 8,
		ParameterData: map[protocol.NodeParameter]protocol.Parameter{
			protocol.NodeParameterMP:  mp.Parameter,
			protocol.NodeParameterFP3: fp3.Parameter,
		},
	}
	updater.processFrame(ntf)

	if atomic.LoadInt32(&fired) != 1 {
		t.Fatalf("callback fired %d times, want 1", atomic.LoadInt32(&fired))
	}
	if blind.Position().Raw != mp.Raw {
		t.Fatalf("blind position = %v, want %v (MP)", blind.Position().Raw, mp.Raw)
	}
	if blind.Orientation().Raw != fp3.Raw {
		t.Fatalf("blind orientation = %v, want %v (FP3)", blind.Orientation().Raw, fp3.Raw)
	}
}
