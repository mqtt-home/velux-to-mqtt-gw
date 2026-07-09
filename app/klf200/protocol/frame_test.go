package protocol

import (
	"bytes"
	"reflect"
	"testing"
	"time"
)

// marshalRoundTrip marshals a frame to wire bytes, validates the command code and
// framing, then parses it back via FrameFromRaw and returns the recovered frame.
func marshalRoundTrip(t *testing.T, f Frame, wantCmd Command) Frame {
	t.Helper()
	if f.Command() != wantCmd {
		t.Fatalf("Command() = %#x, want %#x", f.Command(), wantCmd)
	}
	raw, err := MarshalFrame(f)
	if err != nil {
		t.Fatalf("MarshalFrame: %v", err)
	}
	// Verify framing: protocol id 0, length byte, big-endian command, CRC.
	if len(raw) < 5 {
		t.Fatalf("frame too short: %v", raw)
	}
	if raw[0] != 0x00 {
		t.Errorf("protocol id = %#x, want 0x00", raw[0])
	}
	if int(raw[1]) != len(raw)-2 {
		t.Errorf("length byte = %d, want %d", raw[1], len(raw)-2)
	}
	gotCmd := Command(int(raw[2])*256 + int(raw[3]))
	if gotCmd != wantCmd {
		t.Errorf("wire command = %#x, want %#x", gotCmd, wantCmd)
	}
	if CalcCRC(raw[:len(raw)-1]) != raw[len(raw)-1] {
		t.Errorf("CRC mismatch")
	}

	parsed, err := FrameFromRaw(raw)
	if err != nil {
		t.Fatalf("FrameFromRaw: %v", err)
	}
	if parsed == nil {
		t.Fatalf("FrameFromRaw returned nil (command %#x not registered)", wantCmd)
	}
	if parsed.Command() != wantCmd {
		t.Errorf("parsed Command() = %#x, want %#x", parsed.Command(), wantCmd)
	}

	// Re-marshal the parsed frame; wire bytes must be identical (byte-exact).
	raw2, err := MarshalFrame(parsed)
	if err != nil {
		t.Fatalf("re-MarshalFrame: %v", err)
	}
	if !bytes.Equal(raw, raw2) {
		t.Errorf("re-marshal mismatch:\n orig = %v\n back = %v", raw, raw2)
	}
	return parsed
}

func TestFrameCommandSendRequestRoundTrip(t *testing.T) {
	pos, err := NewParameter(&[2]byte{0x00, 0x00}) // fully open
	if err != nil {
		t.Fatal(err)
	}
	f := &FrameCommandSendRequest{
		SessionID:  0x1234,
		Originator: OriginatorUser,
		Priority:   PriorityUserLevel2,
		Parameter:  pos,
		NodeIDs:    []uint8{5, 6, 7},
	}
	parsed := marshalRoundTrip(t, f, GW_COMMAND_SEND_REQ).(*FrameCommandSendRequest)
	if parsed.SessionID != 0x1234 {
		t.Errorf("SessionID = %#x", parsed.SessionID)
	}
	if parsed.Originator != OriginatorUser {
		t.Errorf("Originator = %v", parsed.Originator)
	}
	// from_payload reconstructs node_ids as a contiguous run starting at node[0].
	if !reflect.DeepEqual(parsed.NodeIDs, []uint8{5, 6, 7}) {
		t.Errorf("NodeIDs = %v, want [5 6 7]", parsed.NodeIDs)
	}
	if parsed.Parameter.Raw != [2]byte{0x00, 0x00} {
		t.Errorf("Parameter.Raw = %v", parsed.Parameter.Raw)
	}
	// Payload must be exactly 66 bytes (PAYLOAD_LEN).
	pl, _ := f.MarshalPayload()
	if len(pl) != 66 {
		t.Errorf("payload len = %d, want 66", len(pl))
	}
}

func TestFrameGetAllNodesInformationRequestRoundTrip(t *testing.T) {
	f := &FrameGetAllNodesInformationRequest{}
	parsed := marshalRoundTrip(t, f, GW_GET_ALL_NODES_INFORMATION_REQ)
	if _, ok := parsed.(*FrameGetAllNodesInformationRequest); !ok {
		t.Errorf("parsed type = %T", parsed)
	}
	// Empty payload.
	pl, _ := f.MarshalPayload()
	if len(pl) != 0 {
		t.Errorf("payload len = %d, want 0", len(pl))
	}
}

func TestFramePasswordEnterRequestRoundTrip(t *testing.T) {
	f := &FramePasswordEnterRequest{Password: "velux123"}
	parsed := marshalRoundTrip(t, f, GW_PASSWORD_ENTER_REQ).(*FramePasswordEnterRequest)
	if parsed.Password != "velux123" {
		t.Errorf("Password = %q, want %q", parsed.Password, "velux123")
	}
	// Payload must be NUL-padded to 32 bytes.
	pl, err := f.MarshalPayload()
	if err != nil {
		t.Fatal(err)
	}
	if len(pl) != 32 {
		t.Errorf("payload len = %d, want 32", len(pl))
	}
	if !bytes.Equal(pl[:8], []byte("velux123")) {
		t.Errorf("payload prefix = %v", pl[:8])
	}
	for _, b := range pl[8:] {
		if b != 0 {
			t.Errorf("expected NUL padding, got %v", pl[8:])
			break
		}
	}
}

func TestFrameSetLimitationRequestRoundTrip(t *testing.T) {
	f := &FrameSetLimitationRequest{
		SessionID:          0x00AB,
		Originator:         OriginatorUser,
		Priority:           PriorityUserLevel2,
		NodeIDs:            []uint8{1, 2},
		ParameterID:        0,
		LimitationValueMin: [2]byte{0x00, 0x00},
		LimitationValueMax: [2]byte{0xC8, 0x00},
		LimitationTime:     NewLimitationTimeClearMaster(),
	}
	parsed := marshalRoundTrip(t, f, GW_SET_LIMITATION_REQ).(*FrameSetLimitationRequest)
	if parsed.SessionID != 0x00AB {
		t.Errorf("SessionID = %#x", parsed.SessionID)
	}
	if !reflect.DeepEqual(parsed.NodeIDs, []uint8{1, 2}) {
		t.Errorf("NodeIDs = %v", parsed.NodeIDs)
	}
	if parsed.LimitationValueMax != [2]byte{0xC8, 0x00} {
		t.Errorf("LimitationValueMax = %v", parsed.LimitationValueMax)
	}
	if !parsed.LimitationTime.Equal(NewLimitationTimeClearMaster()) {
		t.Errorf("LimitationTime = %v", parsed.LimitationTime.Raw)
	}
	// Payload is exactly 31 bytes.
	pl, _ := f.MarshalPayload()
	if len(pl) != 31 {
		t.Errorf("payload len = %d, want 31", len(pl))
	}
}

func TestFrameNodeStatePositionChangedNotificationRoundTrip(t *testing.T) {
	cp, _ := NewParameter(&[2]byte{0x00, 0x00})
	tp, _ := NewParameter(&[2]byte{0xC8, 0x00})
	fpUnknown, _ := NewParameter(&[2]byte{0xF7, 0xFF})
	f := &FrameNodeStatePositionChangedNotification{
		NodeID:             3,
		State:              5,
		CurrentPosition:    cp,
		Target:             tp,
		CurrentPositionFP1: fpUnknown,
		CurrentPositionFP2: fpUnknown,
		CurrentPositionFP3: fpUnknown,
		CurrentPositionFP4: fpUnknown,
		RemainingTime:      0x0102,
		Timestamp:          0xDEADBEEF,
	}
	parsed := marshalRoundTrip(t, f, GW_NODE_STATE_POSITION_CHANGED_NTF).(*FrameNodeStatePositionChangedNotification)
	if parsed.NodeID != 3 || parsed.State != 5 {
		t.Errorf("NodeID=%d State=%d", parsed.NodeID, parsed.State)
	}
	if parsed.CurrentPosition.Raw != [2]byte{0x00, 0x00} {
		t.Errorf("CurrentPosition = %v", parsed.CurrentPosition.Raw)
	}
	if parsed.Target.Raw != [2]byte{0xC8, 0x00} {
		t.Errorf("Target = %v", parsed.Target.Raw)
	}
	if parsed.RemainingTime != 0x0102 {
		t.Errorf("RemainingTime = %#x", parsed.RemainingTime)
	}
	if parsed.Timestamp != 0xDEADBEEF {
		t.Errorf("Timestamp = %#x", parsed.Timestamp)
	}
	// Payload is exactly 20 bytes.
	pl, _ := f.MarshalPayload()
	if len(pl) != 20 {
		t.Errorf("payload len = %d, want 20", len(pl))
	}
}

// TestFrameStatusRequestNotificationMainInfoAddress verifies the 4-byte
// LastMasterExecutionAddress (spec Table 189, Data 14-17, offset 13-16) and the
// LastCommandOriginator at offset 17 round-trip byte-exactly.
func TestFrameStatusRequestNotificationMainInfoAddress(t *testing.T) {
	tp, _ := NewParameter(&[2]byte{0xC8, 0x00})
	cp, _ := NewParameter(&[2]byte{0x00, 0x00})
	f := &FrameStatusRequestNotification{
		SessionID:                  0x0102,
		StatusID:                   3,
		NodeID:                     4,
		RunStatus:                  RunStatusExecutionCompleted,
		StatusReply:                StatusReplyUnknownStatusReply,
		StatusType:                 StatusTypeRequestMainInfo,
		TargetPosition:             tp,
		CurrentPosition:            cp,
		RemainingTime:              0x0203,
		LastMasterExecutionAddress: 0x00AABBCC,
		LastCommandOriginator:      9,
	}
	pl, err := f.MarshalPayload()
	if err != nil {
		t.Fatal(err)
	}
	// Main-Info payload must be exactly 18 bytes.
	if len(pl) != 18 {
		t.Fatalf("payload len = %d, want 18", len(pl))
	}
	// Address bytes at offset 13-16 must be 00 AA BB CC (big-endian).
	if !bytes.Equal(pl[13:17], []byte{0x00, 0xAA, 0xBB, 0xCC}) {
		t.Errorf("address bytes = % X, want 00 AA BB CC", pl[13:17])
	}
	// Originator at offset 17.
	if pl[17] != 9 {
		t.Errorf("originator byte = %d, want 9", pl[17])
	}
	parsed := marshalRoundTrip(t, f, GW_STATUS_REQUEST_NTF).(*FrameStatusRequestNotification)
	if parsed.LastMasterExecutionAddress != 0x00AABBCC {
		t.Errorf("LastMasterExecutionAddress = %#x, want 0xAABBCC", parsed.LastMasterExecutionAddress)
	}
	if parsed.LastCommandOriginator != 9 {
		t.Errorf("LastCommandOriginator = %d, want 9", parsed.LastCommandOriginator)
	}
}

// TestFrameStatusRequestNotificationParameterDataRegion verifies the
// ParameterData region is a constant 51 bytes (spec Table 188, Data 9-59)
// regardless of how many entries are used.
func TestFrameStatusRequestNotificationParameterDataRegion(t *testing.T) {
	p, _ := NewParameter(&[2]byte{0x12, 0x34})
	f := &FrameStatusRequestNotification{
		SessionID:     0x0001,
		StatusType:    StatusTypeRequestTargetPosition,
		StatusCount:   1,
		ParameterData: map[NodeParameter]Parameter{NodeParameter(0): p},
	}
	pl, err := f.MarshalPayload()
	if err != nil {
		t.Fatal(err)
	}
	// 7 header + 1 StatusCount + 51 fixed ParameterData region = 59 bytes.
	if len(pl) != 59 {
		t.Fatalf("payload len = %d, want 59 (7 header + 1 count + 51 region)", len(pl))
	}
}

// TestFrameWinkSendRequestLayout verifies the spec Table 192 byte order.
func TestFrameWinkSendRequestLayout(t *testing.T) {
	f := &FrameWinkSendRequest{
		SessionID:  0x1122,
		Originator: OriginatorUser,
		Priority:   PriorityUserLevel2,
		NodeIDs:    []uint8{5, 6, 7},
		WinkState:  1,
		WinkTime:   WinkTime(254),
	}
	pl, err := f.MarshalPayload()
	if err != nil {
		t.Fatal(err)
	}
	if len(pl) != 27 {
		t.Fatalf("payload len = %d, want 27", len(pl))
	}
	if pl[4] != 1 {
		t.Errorf("WinkState @4 = %d, want 1", pl[4])
	}
	if pl[5] != 254 {
		t.Errorf("WinkTime @5 = %d, want 254", pl[5])
	}
	if pl[6] != 3 {
		t.Errorf("IndexArrayCount @6 = %d, want 3", pl[6])
	}
	if !bytes.Equal(pl[7:10], []byte{5, 6, 7}) {
		t.Errorf("IndexArray @7 = % X, want 05 06 07", pl[7:10])
	}
	parsed := marshalRoundTrip(t, f, GW_WINK_SEND_REQ).(*FrameWinkSendRequest)
	if parsed.WinkState != 1 || uint8(parsed.WinkTime) != 254 {
		t.Errorf("WinkState=%d WinkTime=%d", parsed.WinkState, parsed.WinkTime)
	}
	if !reflect.DeepEqual(parsed.NodeIDs, []uint8{5, 6, 7}) {
		t.Errorf("NodeIDs = %v", parsed.NodeIDs)
	}
}

// TestFrameWinkSendNotificationLayout verifies the 2-byte SessionID-only frame
// (spec Table 197).
func TestFrameWinkSendNotificationLayout(t *testing.T) {
	f := &FrameWinkSendNotification{SessionID: 0xBEEF}
	pl, err := f.MarshalPayload()
	if err != nil {
		t.Fatal(err)
	}
	if len(pl) != 2 {
		t.Fatalf("payload len = %d, want 2", len(pl))
	}
	parsed := marshalRoundTrip(t, f, GW_WINK_SEND_NTF).(*FrameWinkSendNotification)
	if parsed.SessionID != 0xBEEF {
		t.Errorf("SessionID = %#x, want 0xBEEF", parsed.SessionID)
	}
}

// TestFrameGetLocalTimeConfirmationMonth verifies the 0-based wire month
// (spec §6.4.6.6, "Months since January", range 0-11).
func TestFrameGetLocalTimeConfirmationMonth(t *testing.T) {
	// July local time; wire month byte must be 6 (July is month 7, 0-based = 6).
	july := time.Date(2026, time.July, 9, 12, 0, 0, 0, time.Local)
	f := &FrameGetLocalTimeConfirmation{
		Time: LocalTime{UTCTime: july.UTC(), LocalTime: july},
	}
	pl, err := f.MarshalPayload()
	if err != nil {
		t.Fatal(err)
	}
	if pl[8] != 6 {
		t.Errorf("wire month byte @8 = %d, want 6 (July, 0-based)", pl[8])
	}
	parsed := marshalRoundTrip(t, f, GW_GET_LOCAL_TIME_CFM).(*FrameGetLocalTimeConfirmation)
	if parsed.Time.LocalTime.Month() != time.July {
		t.Errorf("decoded month = %v, want July", parsed.Time.LocalTime.Month())
	}
}

// TestFrameFromRawUnknownCommand verifies an unknown but well-formed frame
// returns (nil, nil), matching frame_creation semantics.
func TestFrameFromRawUnknownCommand(t *testing.T) {
	raw := BuildFrame(Command(0xFFFE), []byte{0x01})
	f, err := FrameFromRaw(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != nil {
		t.Errorf("expected nil frame for unknown command, got %T", f)
	}
}

// TestExtractFromFrameBadCRC verifies CRC validation.
func TestExtractFromFrameBadCRC(t *testing.T) {
	raw := BuildFrame(GW_GET_ALL_NODES_INFORMATION_REQ, nil)
	raw[len(raw)-1] ^= 0xFF // corrupt CRC
	if _, _, err := ExtractFromFrame(raw); err == nil {
		t.Errorf("expected CRC error")
	}
}
