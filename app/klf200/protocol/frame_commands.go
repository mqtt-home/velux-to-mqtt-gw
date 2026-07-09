package protocol

import (
	"encoding/binary"
	"fmt"
)

// ============================================================
// command_send frames
// (ported from pyvlx/api/frames/frame_command_send.py)
// ============================================================

// CommandSendConfirmationStatus mirrors pyvlx CommandSendConfirmationStatus.
type CommandSendConfirmationStatus uint8

const (
	CommandSendConfirmationStatusRejected CommandSendConfirmationStatus = 0
	CommandSendConfirmationStatusAccepted CommandSendConfirmationStatus = 1
)

// FrameCommandSendRequest implements Frame for GW_COMMAND_SEND_REQ.
// Ported from FrameCommandSendRequest (PAYLOAD_LEN = 66).
type FrameCommandSendRequest struct {
	SessionID           uint16
	Originator          Originator
	Priority            Priority
	ActiveParameter     uint8
	FPI1                uint8
	FPI2                uint8
	Parameter           Parameter
	FunctionalParameter [16][2]byte // fp1..fp16
	NodeIDs             []uint8
}

// Command returns the constant command code for this frame.
func (f *FrameCommandSendRequest) Command() Command { return GW_COMMAND_SEND_REQ }

// MarshalPayload returns the 66-byte payload (== pyvlx get_payload).
func (f *FrameCommandSendRequest) MarshalPayload() ([]byte, error) {
	if len(f.NodeIDs) > 20 {
		return nil, fmt.Errorf("FrameCommandSendRequest: too many node_ids (%d > 20)", len(f.NodeIDs))
	}
	payload := make([]byte, 0, 66)
	// Session id (2 bytes)
	payload = append(payload, byte(f.SessionID>>8), byte(f.SessionID&0xFF))
	// Originator
	payload = append(payload, byte(f.Originator))
	// Priority
	payload = append(payload, byte(f.Priority))
	// ParameterActive (active_parameter)
	payload = append(payload, f.ActiveParameter)
	// FPI 1+2
	payload = append(payload, f.FPI1, f.FPI2)
	// Main parameter (2 bytes)
	payload = append(payload, f.Parameter.Bytes()...)
	// fp1, fp2, fp3 (indices 0,1,2 in FunctionalParameter)
	payload = append(payload, f.FunctionalParameter[0][:]...)
	payload = append(payload, f.FunctionalParameter[1][:]...)
	payload = append(payload, f.FunctionalParameter[2][:]...)
	// fp4..fp16 — pyvlx emits 26 zero bytes for these
	payload = append(payload, make([]byte, 26)...)
	// Nodes array: count + node array (20 bytes) padded
	payload = append(payload, byte(len(f.NodeIDs)))
	nodeArr := make([]byte, 20)
	copy(nodeArr, f.NodeIDs)
	payload = append(payload, nodeArr...)
	// Priority Level Lock (1 byte)
	payload = append(payload, 0)
	// Priority Level information 1+2 (2 bytes)
	payload = append(payload, 0, 0)
	// Locktime (1 byte)
	payload = append(payload, 0)
	return payload, nil
}

// UnmarshalPayload populates the frame from a 66-byte payload (== pyvlx from_payload).
func (f *FrameCommandSendRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 66 {
		return fmt.Errorf("FrameCommandSendRequest: invalid payload len %d, want 66", len(payload))
	}
	f.SessionID = uint16(payload[0])*256 + uint16(payload[1])
	f.Originator = Originator(payload[2])
	f.Priority = Priority(payload[3])

	lenNodeIDs := int(payload[41])
	if lenNodeIDs > 20 {
		return fmt.Errorf("FrameCommandSendRequest: command_send_request_wrong_node_length (%d)", lenNodeIDs)
	}
	f.NodeIDs = make([]uint8, lenNodeIDs)
	for i := 0; i < lenNodeIDs; i++ {
		f.NodeIDs[i] = payload[42] + uint8(i)
	}

	p, err := NewParameter(&[2]byte{payload[7], payload[8]})
	if err != nil {
		return err
	}
	f.Parameter = p
	return nil
}

// Compile-time assertion that the type satisfies Frame.
var _ Frame = (*FrameCommandSendRequest)(nil)

func init() {
	RegisterFrame(GW_COMMAND_SEND_REQ, func() Frame { return &FrameCommandSendRequest{} })
}

// FrameCommandSendConfirmation implements Frame for GW_COMMAND_SEND_CFM.
// Ported from FrameCommandSendConfirmation (PAYLOAD_LEN = 3).
type FrameCommandSendConfirmation struct {
	SessionID uint16
	Status    CommandSendConfirmationStatus
}

// Command returns the constant command code for this frame.
func (f *FrameCommandSendConfirmation) Command() Command { return GW_COMMAND_SEND_CFM }

// MarshalPayload returns the 3-byte payload (== pyvlx get_payload).
func (f *FrameCommandSendConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{
		byte(f.SessionID >> 8),
		byte(f.SessionID & 0xFF),
		byte(f.Status),
	}, nil
}

// UnmarshalPayload populates the frame from a 3-byte payload (== pyvlx from_payload).
func (f *FrameCommandSendConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 3 {
		return fmt.Errorf("FrameCommandSendConfirmation: invalid payload len %d, want 3", len(payload))
	}
	f.SessionID = uint16(payload[0])*256 + uint16(payload[1])
	f.Status = CommandSendConfirmationStatus(payload[2])
	return nil
}

var _ Frame = (*FrameCommandSendConfirmation)(nil)

func init() {
	RegisterFrame(GW_COMMAND_SEND_CFM, func() Frame { return &FrameCommandSendConfirmation{} })
}

// FrameCommandRunStatusNotification implements Frame for GW_COMMAND_RUN_STATUS_NTF.
// Ported from FrameCommandRunStatusNotification (PAYLOAD_LEN = 13).
type FrameCommandRunStatusNotification struct {
	SessionID      uint16
	StatusID       uint8
	IndexID        uint8
	NodeParameter  uint8
	ParameterValue uint16
}

// Command returns the constant command code for this frame.
func (f *FrameCommandRunStatusNotification) Command() Command { return GW_COMMAND_RUN_STATUS_NTF }

// MarshalPayload returns the 13-byte payload (== pyvlx get_payload).
// Note: pyvlx leaves run_status, status_reply, information_code as zeros (XXX comment).
func (f *FrameCommandRunStatusNotification) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 13)
	payload[0] = byte(f.SessionID >> 8)
	payload[1] = byte(f.SessionID & 0xFF)
	payload[2] = f.StatusID
	payload[3] = f.IndexID
	payload[4] = f.NodeParameter
	payload[5] = byte(f.ParameterValue >> 8)
	payload[6] = byte(f.ParameterValue & 0xFF)
	// bytes 7..12 remain zero (pyvlx: bytes(6) for run_status/status_reply/information_code)
	return payload, nil
}

// UnmarshalPayload populates the frame from a 13-byte payload (== pyvlx from_payload).
func (f *FrameCommandRunStatusNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 13 {
		return fmt.Errorf("FrameCommandRunStatusNotification: invalid payload len %d, want 13", len(payload))
	}
	f.SessionID = uint16(payload[0])*256 + uint16(payload[1])
	f.StatusID = payload[2]
	f.IndexID = payload[3]
	f.NodeParameter = payload[4]
	f.ParameterValue = uint16(payload[5])*256 + uint16(payload[6])
	return nil
}

var _ Frame = (*FrameCommandRunStatusNotification)(nil)

func init() {
	RegisterFrame(GW_COMMAND_RUN_STATUS_NTF, func() Frame { return &FrameCommandRunStatusNotification{} })
}

// FrameCommandRemainingTimeNotification implements Frame for GW_COMMAND_REMAINING_TIME_NTF.
// Ported from FrameCommandRemainingTimeNotification (PAYLOAD_LEN = 6).
type FrameCommandRemainingTimeNotification struct {
	SessionID     uint16
	IndexID       uint8
	NodeParameter uint8
	Seconds       uint16
}

// Command returns the constant command code for this frame.
func (f *FrameCommandRemainingTimeNotification) Command() Command {
	return GW_COMMAND_REMAINING_TIME_NTF
}

// MarshalPayload returns the 6-byte payload (== pyvlx get_payload).
func (f *FrameCommandRemainingTimeNotification) MarshalPayload() ([]byte, error) {
	return []byte{
		byte(f.SessionID >> 8),
		byte(f.SessionID & 0xFF),
		f.IndexID,
		f.NodeParameter,
		byte(f.Seconds >> 8),
		byte(f.Seconds & 0xFF),
	}, nil
}

// UnmarshalPayload populates the frame from a 6-byte payload (== pyvlx from_payload).
func (f *FrameCommandRemainingTimeNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 6 {
		return fmt.Errorf("FrameCommandRemainingTimeNotification: invalid payload len %d, want 6", len(payload))
	}
	f.SessionID = uint16(payload[0])*256 + uint16(payload[1])
	f.IndexID = payload[2]
	f.NodeParameter = payload[3]
	f.Seconds = uint16(payload[4])*256 + uint16(payload[5])
	return nil
}

var _ Frame = (*FrameCommandRemainingTimeNotification)(nil)

func init() {
	RegisterFrame(GW_COMMAND_REMAINING_TIME_NTF, func() Frame { return &FrameCommandRemainingTimeNotification{} })
}

// FrameSessionFinishedNotification implements Frame for GW_SESSION_FINISHED_NTF.
// Ported from FrameSessionFinishedNotification (PAYLOAD_LEN = 2).
type FrameSessionFinishedNotification struct {
	SessionID uint16
}

// Command returns the constant command code for this frame.
func (f *FrameSessionFinishedNotification) Command() Command { return GW_SESSION_FINISHED_NTF }

// MarshalPayload returns the 2-byte payload (== pyvlx get_payload).
func (f *FrameSessionFinishedNotification) MarshalPayload() ([]byte, error) {
	return []byte{
		byte(f.SessionID >> 8),
		byte(f.SessionID & 0xFF),
	}, nil
}

// UnmarshalPayload populates the frame from a 2-byte payload (== pyvlx from_payload).
func (f *FrameSessionFinishedNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 2 {
		return fmt.Errorf("FrameSessionFinishedNotification: invalid payload len %d, want 2", len(payload))
	}
	f.SessionID = uint16(payload[0])*256 + uint16(payload[1])
	return nil
}

var _ Frame = (*FrameSessionFinishedNotification)(nil)

func init() {
	RegisterFrame(GW_SESSION_FINISHED_NTF, func() Frame { return &FrameSessionFinishedNotification{} })
}

// ============================================================
// status_request frames
// (ported from pyvlx/api/frames/frame_status_request.py)
// ============================================================

// StatusRequestStatus mirrors pyvlx StatusRequestStatus.
type StatusRequestStatus uint8

const (
	StatusRequestStatusRejected StatusRequestStatus = 0
	StatusRequestStatusAccepted StatusRequestStatus = 1
)

// FrameStatusRequestRequest implements Frame for GW_STATUS_REQUEST_REQ.
// Ported from FrameStatusRequestRequest (PAYLOAD_LEN = 26).
type FrameStatusRequestRequest struct {
	SessionID  uint16
	NodeIDs    []uint8
	StatusType StatusType
	FPI1       uint8
	FPI2       uint8
}

// Command returns the constant command code for this frame.
func (f *FrameStatusRequestRequest) Command() Command { return GW_STATUS_REQUEST_REQ }

// MarshalPayload returns the 26-byte payload (== pyvlx get_payload).
func (f *FrameStatusRequestRequest) MarshalPayload() ([]byte, error) {
	if len(f.NodeIDs) > 20 {
		return nil, fmt.Errorf("FrameStatusRequestRequest: too many node_ids (%d > 20)", len(f.NodeIDs))
	}
	payload := make([]byte, 0, 26)
	payload = append(payload, byte(f.SessionID>>8), byte(f.SessionID&0xFF))
	payload = append(payload, byte(len(f.NodeIDs)))
	nodeArr := make([]byte, 20)
	copy(nodeArr, f.NodeIDs)
	payload = append(payload, nodeArr...)
	payload = append(payload, byte(f.StatusType))
	payload = append(payload, f.FPI1)
	payload = append(payload, f.FPI2)
	return payload, nil
}

// UnmarshalPayload populates the frame from a 26-byte payload (== pyvlx from_payload).
func (f *FrameStatusRequestRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 26 {
		return fmt.Errorf("FrameStatusRequestRequest: invalid payload len %d, want 26", len(payload))
	}
	f.SessionID = uint16(payload[0])*256 + uint16(payload[1])
	lenNodeIDs := int(payload[2])
	if lenNodeIDs > 20 {
		return fmt.Errorf("FrameStatusRequestRequest: command_send_request_wrong_node_length (%d)", lenNodeIDs)
	}
	f.NodeIDs = make([]uint8, lenNodeIDs)
	for i := 0; i < lenNodeIDs; i++ {
		f.NodeIDs[i] = payload[3] + uint8(i)
	}
	f.StatusType = StatusType(payload[23])
	f.FPI1 = payload[24]
	f.FPI2 = payload[25]
	return nil
}

var _ Frame = (*FrameStatusRequestRequest)(nil)

func init() {
	RegisterFrame(GW_STATUS_REQUEST_REQ, func() Frame {
		return &FrameStatusRequestRequest{
			StatusType: StatusTypeRequestCurrentPosition,
			FPI1:       254,
			FPI2:       0,
		}
	})
}

// FrameStatusRequestConfirmation implements Frame for GW_STATUS_REQUEST_CFM.
// Ported from FrameStatusRequestConfirmation (PAYLOAD_LEN = 3).
type FrameStatusRequestConfirmation struct {
	SessionID uint16
	Status    StatusRequestStatus
}

// Command returns the constant command code for this frame.
func (f *FrameStatusRequestConfirmation) Command() Command { return GW_STATUS_REQUEST_CFM }

// MarshalPayload returns the 3-byte payload (== pyvlx get_payload).
func (f *FrameStatusRequestConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{
		byte(f.SessionID >> 8),
		byte(f.SessionID & 0xFF),
		byte(f.Status),
	}, nil
}

// UnmarshalPayload populates the frame from a 3-byte payload (== pyvlx from_payload).
func (f *FrameStatusRequestConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 3 {
		return fmt.Errorf("FrameStatusRequestConfirmation: invalid payload len %d, want 3", len(payload))
	}
	f.SessionID = uint16(payload[0])*256 + uint16(payload[1])
	f.Status = StatusRequestStatus(payload[2])
	return nil
}

var _ Frame = (*FrameStatusRequestConfirmation)(nil)

func init() {
	RegisterFrame(GW_STATUS_REQUEST_CFM, func() Frame { return &FrameStatusRequestConfirmation{} })
}

// FrameStatusRequestNotification implements Frame for GW_STATUS_REQUEST_NTF.
// Ported from FrameStatusRequestNotification (variable-length payload).
//
// When StatusType == StatusTypeRequestMainInfo the trailing bytes encode
// target_position, current_position, remaining_time,
// last_master_execution_address (3 bytes), and last_command_originator.
// For all other StatusType values the trailing bytes encode a parameter_data map.
type FrameStatusRequestNotification struct {
	SessionID   uint16
	StatusID    uint8
	NodeID      uint8
	RunStatus   RunStatus
	StatusReply StatusReply
	StatusType  StatusType

	// REQUEST_MAIN_INFO branch
	TargetPosition             Parameter
	CurrentPosition            Parameter
	RemainingTime              uint16
	LastMasterExecutionAddress uint32 // only lower 3 bytes used
	LastCommandOriginator      uint8

	// All other StatusType branches
	StatusCount   uint8
	ParameterData map[NodeParameter]Parameter
}

// Command returns the constant command code for this frame.
func (f *FrameStatusRequestNotification) Command() Command { return GW_STATUS_REQUEST_NTF }

// MarshalPayload returns the variable-length payload (== pyvlx get_payload).
func (f *FrameStatusRequestNotification) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 0, 64)
	payload = append(payload,
		byte(f.SessionID>>8), byte(f.SessionID&0xFF),
		f.StatusID,
		f.NodeID,
		byte(f.RunStatus),
		byte(f.StatusReply),
		byte(f.StatusType),
	)
	if f.StatusType == StatusTypeRequestMainInfo {
		payload = append(payload, f.TargetPosition.Bytes()...)
		payload = append(payload, f.CurrentPosition.Bytes()...)
		payload = append(payload,
			byte(f.RemainingTime>>8), byte(f.RemainingTime&0xFF),
			// LastMasterExecutionAddress: 4-byte big-endian (spec Table 189,
			// Data 14-17, offset 13-16; §10.3.3.12 unsigned 32-bit 0..0x00FFFFFF).
			byte(f.LastMasterExecutionAddress>>24),
			byte(f.LastMasterExecutionAddress>>16),
			byte(f.LastMasterExecutionAddress>>8),
			byte(f.LastMasterExecutionAddress),
			f.LastCommandOriginator,
		)
	} else {
		payload = append(payload, f.StatusCount)
		count := int(f.StatusCount)
		for key, val := range f.ParameterData {
			if count <= 0 {
				break
			}
			payload = append(payload, byte(key))
			payload = append(payload, val.Bytes()...)
			count--
		}
		// ParameterData region is a fixed 51 bytes (spec Table 188, Data 9-59 =
		// 17 entries x 3). N used entries occupy 3*N bytes; pad the remainder.
		remaining := 51 - 3*len(f.ParameterData)
		payload = append(payload, make([]byte, remaining)...)
	}
	return payload, nil
}

// UnmarshalPayload populates the frame from a variable-length payload (== pyvlx from_payload).
func (f *FrameStatusRequestNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) < 7 {
		return fmt.Errorf("FrameStatusRequestNotification: payload too short (%d)", len(payload))
	}
	f.SessionID = uint16(payload[0])*256 + uint16(payload[1])
	f.StatusID = payload[2]
	f.NodeID = payload[3]
	f.RunStatus = RunStatus(payload[4])
	f.StatusReply = StatusReply(payload[5])
	f.StatusType = StatusType(payload[6])

	f.ParameterData = make(map[NodeParameter]Parameter)

	if f.StatusType == StatusTypeRequestMainInfo {
		if len(payload) < 18 {
			return fmt.Errorf("FrameStatusRequestNotification: REQUEST_MAIN_INFO payload too short (%d)", len(payload))
		}
		tp, err := NewParameter(&[2]byte{payload[7], payload[8]})
		if err != nil {
			return err
		}
		f.TargetPosition = tp
		// pyvlx reads payload[9:10] for current_position (single byte slice — replicating faithfully)
		cp, err := NewParameter(&[2]byte{payload[9], payload[10]})
		if err != nil {
			return err
		}
		f.CurrentPosition = cp
		f.RemainingTime = uint16(payload[11])*256 + uint16(payload[12])
		// LastMasterExecutionAddress: 4-byte big-endian (spec Table 189,
		// Data 14-17, offset 13-16). Originator follows at Data 18, offset 17.
		f.LastMasterExecutionAddress = binary.BigEndian.Uint32(payload[13:17])
		f.LastCommandOriginator = payload[17]
	} else {
		if len(payload) < 8 {
			return fmt.Errorf("FrameStatusRequestNotification: payload too short for status_count (%d)", len(payload))
		}
		f.StatusCount = payload[7]
		// pyvlx: for i in range(8, 8 + status_count*3, 3)
		end := 8 + int(f.StatusCount)*3
		if len(payload) < end {
			return fmt.Errorf("FrameStatusRequestNotification: payload too short for parameter_data (%d)", len(payload))
		}
		for i := 8; i < end; i += 3 {
			p, err := NewParameter(&[2]byte{payload[i+1], payload[i+2]})
			if err != nil {
				return err
			}
			f.ParameterData[NodeParameter(payload[i])] = p
		}
	}
	return nil
}

var _ Frame = (*FrameStatusRequestNotification)(nil)

func init() {
	RegisterFrame(GW_STATUS_REQUEST_NTF, func() Frame {
		return &FrameStatusRequestNotification{
			RunStatus:     RunStatusExecutionCompleted,
			StatusReply:   StatusReplyUnknownStatusReply,
			StatusType:    StatusTypeRequestTargetPosition,
			ParameterData: make(map[NodeParameter]Parameter),
		}
	})
}

// ============================================================
// node_state_position_changed_notification frame
// (ported from pyvlx/api/frames/frame_node_state_position_changed_notification.py)
// ============================================================

// FrameNodeStatePositionChangedNotification implements Frame for
// GW_NODE_STATE_POSITION_CHANGED_NTF.
// Ported from FrameNodeStatePositionChangedNotification (PAYLOAD_LEN = 20).
type FrameNodeStatePositionChangedNotification struct {
	NodeID             uint8
	State              uint8
	CurrentPosition    Parameter
	Target             Parameter
	CurrentPositionFP1 Parameter
	CurrentPositionFP2 Parameter
	CurrentPositionFP3 Parameter
	CurrentPositionFP4 Parameter
	RemainingTime      uint16
	Timestamp          uint32
}

// Command returns the constant command code for this frame.
func (f *FrameNodeStatePositionChangedNotification) Command() Command {
	return GW_NODE_STATE_POSITION_CHANGED_NTF
}

// MarshalPayload returns the 20-byte payload (== pyvlx get_payload).
func (f *FrameNodeStatePositionChangedNotification) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 0, 20)
	payload = append(payload, f.NodeID)
	payload = append(payload, f.State)
	payload = append(payload, f.CurrentPosition.Bytes()...)
	payload = append(payload, f.Target.Bytes()...)
	payload = append(payload, f.CurrentPositionFP1.Bytes()...)
	payload = append(payload, f.CurrentPositionFP2.Bytes()...)
	payload = append(payload, f.CurrentPositionFP3.Bytes()...)
	payload = append(payload, f.CurrentPositionFP4.Bytes()...)
	payload = append(payload, byte(f.RemainingTime>>8), byte(f.RemainingTime&0xFF))
	payload = binary.BigEndian.AppendUint32(payload, f.Timestamp)
	return payload, nil
}

// UnmarshalPayload populates the frame from a 20-byte payload (== pyvlx from_payload).
func (f *FrameNodeStatePositionChangedNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 20 {
		return fmt.Errorf("FrameNodeStatePositionChangedNotification: invalid payload len %d, want 20", len(payload))
	}
	f.NodeID = payload[0]
	f.State = payload[1]

	cp, err := NewParameter(&[2]byte{payload[2], payload[3]})
	if err != nil {
		return err
	}
	f.CurrentPosition = cp

	tp, err := NewParameter(&[2]byte{payload[4], payload[5]})
	if err != nil {
		return err
	}
	f.Target = tp

	fp1, err := NewParameter(&[2]byte{payload[6], payload[7]})
	if err != nil {
		return err
	}
	f.CurrentPositionFP1 = fp1

	fp2, err := NewParameter(&[2]byte{payload[8], payload[9]})
	if err != nil {
		return err
	}
	f.CurrentPositionFP2 = fp2

	fp3, err := NewParameter(&[2]byte{payload[10], payload[11]})
	if err != nil {
		return err
	}
	f.CurrentPositionFP3 = fp3

	fp4, err := NewParameter(&[2]byte{payload[12], payload[13]})
	if err != nil {
		return err
	}
	f.CurrentPositionFP4 = fp4

	f.RemainingTime = uint16(payload[14])*256 + uint16(payload[15])
	f.Timestamp = binary.BigEndian.Uint32(payload[16:20])
	return nil
}

var _ Frame = (*FrameNodeStatePositionChangedNotification)(nil)

func init() {
	RegisterFrame(GW_NODE_STATE_POSITION_CHANGED_NTF, func() Frame {
		return &FrameNodeStatePositionChangedNotification{}
	})
}
