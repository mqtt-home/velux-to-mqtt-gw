package klf200

import (
	"context"
	"testing"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// acceptCommand replays the confirmation (and, when waiting for completion, the
// session-finished notification) for a GW_COMMAND_SEND_REQ so CommandSend
// returns success. It reads the session id off the captured request.
func acceptCommand(waitForCompletion bool) func(protocol.Frame) []protocol.Frame {
	return func(request protocol.Frame) []protocol.Frame {
		req, ok := request.(*protocol.FrameCommandSendRequest)
		if !ok {
			return nil
		}
		frames := []protocol.Frame{
			&protocol.FrameCommandSendConfirmation{
				SessionID: req.SessionID,
				Status:    protocol.CommandSendConfirmationStatusAccepted,
			},
		}
		if waitForCompletion {
			frames = append(frames, &protocol.FrameSessionFinishedNotification{SessionID: req.SessionID})
		}
		return frames
	}
}

// newWindow builds a Window node wired to the given client, at 0% position.
func newWindow(t *testing.T, client *Client) *Window {
	t.Helper()
	pos, _ := protocol.NewPosition(nil, nil, intp(0))
	info := NodeInfo{NodeType: protocol.NodeTypeWithSubtypeWindowOpener, CurrentPosition: pos.Parameter, Target: pos.Parameter}
	node := NodeFromInfo(client, 5, "win", [8]byte{}, info)
	w, ok := node.(*Window)
	if !ok {
		t.Fatalf("expected *Window, got %T", node)
	}
	return w
}

func TestOpeningDevice_SetPosition_BuildsCommandSend(t *testing.T) {
	fake := &fakeAPI{responder: acceptCommand(false)}
	client := newTestClient(fake)
	w := newWindow(t, client)

	target, _ := protocol.NewPosition(nil, nil, intp(40))
	if err := w.SetPosition(context.Background(), target, false); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}

	req, ok := fake.last().(*protocol.FrameCommandSendRequest)
	if !ok {
		t.Fatalf("last request = %T, want *FrameCommandSendRequest", fake.last())
	}
	if got := req.NodeIDs; len(got) != 1 || got[0] != 5 {
		t.Fatalf("NodeIDs = %v, want [5]", got)
	}
	if req.Parameter.Raw != target.Raw {
		t.Fatalf("Parameter.Raw = %v, want %v (40%%)", req.Parameter.Raw, target.Raw)
	}
	if req.Originator != protocol.OriginatorUser {
		t.Fatalf("Originator = %v, want OriginatorUser", req.Originator)
	}
	if req.Priority != protocol.PriorityUserLevel2 {
		t.Fatalf("Priority = %v, want PriorityUserLevel2", req.Priority)
	}
	// No functional parameters for a plain opening device -> FPI both zero.
	if req.FPI1 != 0 || req.FPI2 != 0 {
		t.Fatalf("FPI1/FPI2 = %d/%d, want 0/0", req.FPI1, req.FPI2)
	}
}

func TestOpeningDevice_Open_BuildsZeroPercent(t *testing.T) {
	fake := &fakeAPI{responder: acceptCommand(false)}
	client := newTestClient(fake)
	w := newWindow(t, client)

	if err := w.Open(context.Background(), false); err != nil {
		t.Fatalf("Open: %v", err)
	}
	req := fake.last().(*protocol.FrameCommandSendRequest)
	want, _ := protocol.NewPosition(nil, nil, intp(0))
	if req.Parameter.Raw != want.Raw {
		t.Fatalf("Open Parameter.Raw = %v, want 0%% (%v)", req.Parameter.Raw, want.Raw)
	}
}

func TestOpeningDevice_Close_BuildsHundredPercent(t *testing.T) {
	fake := &fakeAPI{responder: acceptCommand(false)}
	client := newTestClient(fake)
	w := newWindow(t, client)

	if err := w.Close(context.Background(), false); err != nil {
		t.Fatalf("Close: %v", err)
	}
	req := fake.last().(*protocol.FrameCommandSendRequest)
	want, _ := protocol.NewPosition(nil, nil, intp(100))
	if req.Parameter.Raw != want.Raw {
		t.Fatalf("Close Parameter.Raw = %v, want 100%% (%v)", req.Parameter.Raw, want.Raw)
	}
}

func TestOpeningDevice_Stop_BuildsCurrentSentinel(t *testing.T) {
	fake := &fakeAPI{responder: acceptCommand(false)}
	client := newTestClient(fake)
	w := newWindow(t, client)

	if err := w.Stop(context.Background(), false); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	req := fake.last().(*protocol.FrameCommandSendRequest)
	want := protocol.NewCurrentPosition()
	if req.Parameter.Raw != want.Raw {
		t.Fatalf("Stop Parameter.Raw = %v, want CURRENT sentinel (%v)", req.Parameter.Raw, want.Raw)
	}
}

func TestOpeningDevice_SetPosition_WaitForCompletion(t *testing.T) {
	// With waitForCompletion true the helper must also await the
	// session-finished notification; the responder supplies it.
	fake := &fakeAPI{responder: acceptCommand(true)}
	client := newTestClient(fake)
	w := newWindow(t, client)

	target, _ := protocol.NewPosition(nil, nil, intp(50))
	if err := w.SetPosition(context.Background(), target, true); err != nil {
		t.Fatalf("SetPosition(wait): %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(fake.calls))
	}
}

func TestBlind_SetPosition_SetsFP3(t *testing.T) {
	fake := &fakeAPI{responder: acceptCommand(false)}
	client := newTestClient(fake)

	pos, _ := protocol.NewPosition(nil, nil, intp(0))
	info := NodeInfo{NodeType: protocol.NodeTypeWithSubtypeExteriorVenetianBlind, CurrentPosition: pos.Parameter, Target: pos.Parameter}
	blind, ok := NodeFromInfo(client, 9, "blind", [8]byte{}, info).(*Blind)
	if !ok {
		t.Fatal("expected *Blind")
	}

	// Non-zero target position with no explicit orientation -> fp3 = Ignore, and
	// fp3 present means FPI1 has bit for fp3 set (bit 8-3 = bit 5 => 0x20).
	target, _ := protocol.NewPosition(nil, nil, intp(30))
	if err := blind.SetPosition(context.Background(), target, false); err != nil {
		t.Fatalf("Blind.SetPosition: %v", err)
	}
	req := fake.last().(*protocol.FrameCommandSendRequest)
	if req.FPI1 != 0x20 {
		t.Fatalf("FPI1 = %#x, want 0x20 (fp3 present)", req.FPI1)
	}
	// fp3 slot (index 2) must carry the Ignore sentinel since target != 0%.
	ignore := protocol.NewIgnorePosition()
	if req.FunctionalParameter[2] != ignore.Raw {
		t.Fatalf("fp3 raw = %v, want IGNORE (%v)", req.FunctionalParameter[2], ignore.Raw)
	}
}

func TestOpeningDevice_SetPositionLimitations_BuildsSetLimitation(t *testing.T) {
	fake := &fakeAPI{responder: func(request protocol.Frame) []protocol.Frame {
		req := request.(*protocol.FrameSetLimitationRequest)
		return []protocol.Frame{&protocol.FrameSetLimitationConfirmation{
			SessionID: req.SessionID,
			Status:    protocol.SetLimitationRequestStatus(1), // != Rejected(0)
		}}
	}}
	client := newTestClient(fake)
	w := newWindow(t, client)

	min, _ := protocol.NewPosition(nil, nil, intp(10))
	max, _ := protocol.NewPosition(nil, nil, intp(90))
	if err := w.SetPositionLimitations(context.Background(), min, max); err != nil {
		t.Fatalf("SetPositionLimitations: %v", err)
	}
	req, ok := fake.last().(*protocol.FrameSetLimitationRequest)
	if !ok {
		t.Fatalf("last request = %T, want *FrameSetLimitationRequest", fake.last())
	}
	if len(req.NodeIDs) != 1 || req.NodeIDs[0] != 5 {
		t.Fatalf("NodeIDs = %v, want [5]", req.NodeIDs)
	}
	if req.LimitationValueMin != min.Raw {
		t.Fatalf("LimitationValueMin = %v, want %v", req.LimitationValueMin, min.Raw)
	}
	if req.LimitationValueMax != max.Raw {
		t.Fatalf("LimitationValueMax = %v, want %v", req.LimitationValueMax, max.Raw)
	}
	if !req.LimitationTime.IsUnlimited() {
		t.Fatalf("LimitationTime = %#x, want UNLIMITED", req.LimitationTime.Raw)
	}
	// Cached limits must reflect the request.
	if w.LimitationMin().Raw != min.Raw || w.LimitationMax().Raw != max.Raw {
		t.Fatalf("cached limits = %v/%v, want %v/%v", w.LimitationMin().Raw, w.LimitationMax().Raw, min.Raw, max.Raw)
	}
}

func TestOpeningDevice_ClearPositionLimitations_BuildsClearAll(t *testing.T) {
	fake := &fakeAPI{responder: func(request protocol.Frame) []protocol.Frame {
		req := request.(*protocol.FrameSetLimitationRequest)
		return []protocol.Frame{&protocol.FrameSetLimitationConfirmation{
			SessionID: req.SessionID,
			Status:    protocol.SetLimitationRequestStatus(1),
		}}
	}}
	client := newTestClient(fake)
	w := newWindow(t, client)

	if err := w.ClearPositionLimitations(context.Background()); err != nil {
		t.Fatalf("ClearPositionLimitations: %v", err)
	}
	req := fake.last().(*protocol.FrameSetLimitationRequest)
	if !req.LimitationTime.IsClearAll() {
		t.Fatalf("LimitationTime = %#x, want CLEAR_ALL", req.LimitationTime.Raw)
	}
	ignore := protocol.NewIgnorePosition()
	if req.LimitationValueMin != ignore.Raw || req.LimitationValueMax != ignore.Raw {
		t.Fatalf("min/max = %v/%v, want IGNORE/%v", req.LimitationValueMin, req.LimitationValueMax, ignore.Raw)
	}
	// Cached limits reset to Ignore.
	if w.LimitationMin().Raw != ignore.Raw || w.LimitationMax().Raw != ignore.Raw {
		t.Fatalf("cached limits = %v/%v, want IGNORE", w.LimitationMin().Raw, w.LimitationMax().Raw)
	}
}

func TestOpeningDevice_PositionAndTargetAccessors(t *testing.T) {
	client := newTestClient(&fakeAPI{})
	current, _ := protocol.NewPosition(nil, nil, intp(25))
	target, _ := protocol.NewPosition(nil, nil, intp(75))
	info := NodeInfo{
		NodeType:        protocol.NodeTypeWithSubtypeRollerShutter,
		CurrentPosition: current.Parameter,
		Target:          target.Parameter,
	}
	rs := NodeFromInfo(client, 2, "rs", [8]byte{}, info).(*RollerShutter)

	if rs.Position().Raw != current.Raw {
		t.Fatalf("Position = %v, want %v", rs.Position().Raw, current.Raw)
	}
	if rs.TargetPosition().Raw != target.Raw {
		t.Fatalf("TargetPosition = %v, want %v", rs.TargetPosition().Raw, target.Raw)
	}
}

func TestCommandSend_RejectedStatusFails(t *testing.T) {
	fake := &fakeAPI{responder: func(request protocol.Frame) []protocol.Frame {
		req := request.(*protocol.FrameCommandSendRequest)
		return []protocol.Frame{&protocol.FrameCommandSendConfirmation{
			SessionID: req.SessionID,
			Status:    protocol.CommandSendConfirmationStatusRejected,
		}}
	}}
	client := newTestClient(fake)
	w := newWindow(t, client)

	pos, _ := protocol.NewPosition(nil, nil, intp(10))
	if err := w.SetPosition(context.Background(), pos, false); err == nil {
		t.Fatal("expected error on rejected command, got nil")
	}
}
