package klf200

import (
	"sync"
	"testing"
	"time"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
)

// heartbeatResponder answers GW_GET_STATE_REQ and GW_STATUS_REQUEST_REQ so a
// heartbeat pulse completes without a real gateway.
func heartbeatResponder(request protocol.Frame) []protocol.Frame {
	switch req := request.(type) {
	case *protocol.FrameGetStateRequest:
		return []protocol.Frame{&protocol.FrameGetStateConfirmation{}}
	case *protocol.FrameStatusRequestRequest:
		return []protocol.Frame{&protocol.FrameStatusRequestNotification{SessionID: req.SessionID}}
	default:
		return nil
	}
}

// TestHeartbeat_Pulse_SendsGetState verifies a single pulse issues a
// GW_GET_STATE_REQ. Ported from Heartbeat.pulse.
func TestHeartbeat_Pulse_SendsGetState(t *testing.T) {
	fake := &fakeAPI{responder: heartbeatResponder}
	client := newTestClient(fake)
	hb := NewHeartbeat(client)

	if err := hb.pulse(); err != nil {
		t.Fatalf("pulse: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls = %d, want 1 (get_state only, no nodes)", len(fake.calls))
	}
	if _, ok := fake.calls[0].request.(*protocol.FrameGetStateRequest); !ok {
		t.Fatalf("request = %T, want *FrameGetStateRequest", fake.calls[0].request)
	}
}

// TestHeartbeat_Pulse_RefreshesBlindFP3 verifies a pulse also issues a
// GW_STATUS_REQUEST_REQ for each Blind node (the FP3 orientation refresh).
// Ported from the Blind loop in Heartbeat.pulse.
func TestHeartbeat_Pulse_RefreshesBlindFP3(t *testing.T) {
	fake := &fakeAPI{responder: heartbeatResponder}
	client := newTestClient(fake)

	zero, _ := protocol.NewPosition(nil, nil, intp(0))
	info := NodeInfo{NodeType: protocol.NodeTypeWithSubtypeExteriorVenetianBlind, CurrentPosition: zero.Parameter, Target: zero.Parameter}
	blind := NodeFromInfo(client, 11, "b", [8]byte{}, info).(*Blind)
	client.Nodes().Add(blind)

	// A non-blind node must NOT trigger a status request.
	rsInfo := NodeInfo{NodeType: protocol.NodeTypeWithSubtypeRollerShutter, CurrentPosition: zero.Parameter, Target: zero.Parameter}
	client.Nodes().Add(NodeFromInfo(client, 12, "rs", [8]byte{}, rsInfo))

	hb := NewHeartbeat(client)
	if err := hb.pulse(); err != nil {
		t.Fatalf("pulse: %v", err)
	}

	// Expect: 1 get_state + 1 status_request (only for the blind).
	var getState, statusReq int
	var statusNodeID uint8
	for _, c := range fake.calls {
		switch req := c.request.(type) {
		case *protocol.FrameGetStateRequest:
			getState++
		case *protocol.FrameStatusRequestRequest:
			statusReq++
			if len(req.NodeIDs) == 1 {
				statusNodeID = req.NodeIDs[0]
			}
		}
	}
	if getState != 1 {
		t.Fatalf("get_state count = %d, want 1", getState)
	}
	if statusReq != 1 {
		t.Fatalf("status_request count = %d, want 1 (blind only)", statusReq)
	}
	if statusNodeID != 11 {
		t.Fatalf("status_request node = %d, want 11 (the blind)", statusNodeID)
	}
}

// TestHeartbeat_Timer_FiresPulse verifies the timer loop issues a get_state on
// its tick. Uses a short interval so the test is fast and deterministic.
// Ported from Heartbeat.loop.
func TestHeartbeat_Timer_FiresPulse(t *testing.T) {
	var mu sync.Mutex
	pulses := 0
	done := make(chan struct{})

	fake := &fakeAPI{responder: func(request protocol.Frame) []protocol.Frame {
		if _, ok := request.(*protocol.FrameGetStateRequest); ok {
			mu.Lock()
			pulses++
			if pulses == 1 {
				close(done)
			}
			mu.Unlock()
		}
		return heartbeatResponder(request)
	}}
	client := newTestClient(fake)

	hb := NewHeartbeat(client, WithHeartbeatInterval(5*time.Millisecond))
	hb.Start()
	defer hb.Stop()

	select {
	case <-done:
		// Got at least one timer-driven pulse.
	case <-time.After(2 * time.Second):
		t.Fatal("timer did not fire a get_state pulse within 2s")
	}
}
