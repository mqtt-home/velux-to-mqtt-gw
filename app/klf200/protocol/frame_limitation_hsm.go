package protocol

import (
	"encoding/binary"
	"fmt"
)

// SetLimitationRequestStatus mirrors the Python SetLimitationRequestStatus enum.
type SetLimitationRequestStatus uint8

const (
	SetLimitationRequestStatusRejected SetLimitationRequestStatus = 0
	SetLimitationRequestStatusAccepted SetLimitationRequestStatus = 1
)

// FrameGetLimitationStatusRequest implements Frame for GW_GET_LIMITATION_STATUS_REQ.
// Ported from FrameGetLimitationStatus in frame_get_limitation.py.
type FrameGetLimitationStatusRequest struct {
	SessionID       uint16
	NodeIDs         []uint8
	ParameterID     uint8
	LimitationsType LimitationType
}

func (f *FrameGetLimitationStatusRequest) Command() Command {
	return GW_GET_LIMITATION_STATUS_REQ
}

// MarshalPayload returns a 25-byte payload matching pyvlx get_payload.
// Layout: session_id(2) + count(1) + node_ids[20] + parameter_id(1) + limitations_type(1)
func (f *FrameGetLimitationStatusRequest) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 25)
	binary.BigEndian.PutUint16(payload[0:2], f.SessionID)
	nodeCount := len(f.NodeIDs)
	if nodeCount > 20 {
		return nil, fmt.Errorf("FrameGetLimitationStatusRequest: node_ids length %d exceeds max 20", nodeCount)
	}
	payload[2] = uint8(nodeCount)
	copy(payload[3:3+nodeCount], f.NodeIDs)
	// bytes 3+nodeCount .. 22 are zero (20 - len(node_ids) padding)
	payload[23] = f.ParameterID
	payload[24] = uint8(f.LimitationsType)
	return payload, nil
}

// UnmarshalPayload is not implemented for request-only frames (gateway never sends this).
func (f *FrameGetLimitationStatusRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 25 {
		return fmt.Errorf("FrameGetLimitationStatusRequest: invalid payload len %d, want 25", len(payload))
	}
	f.SessionID = binary.BigEndian.Uint16(payload[0:2])
	nodeCount := int(payload[2])
	if nodeCount > 20 {
		return fmt.Errorf("FrameGetLimitationStatusRequest: node count %d exceeds 20", nodeCount)
	}
	f.NodeIDs = make([]uint8, nodeCount)
	copy(f.NodeIDs, payload[3:3+nodeCount])
	f.ParameterID = payload[23]
	f.LimitationsType = LimitationType(payload[24])
	return nil
}

var _ Frame = (*FrameGetLimitationStatusRequest)(nil)

func init() {
	RegisterFrame(GW_GET_LIMITATION_STATUS_REQ, func() Frame { return &FrameGetLimitationStatusRequest{} })
}

// FrameGetLimitationStatusConfirmation implements Frame for GW_GET_LIMITATION_STATUS_CFM.
// Ported from FrameGetLimitationStatusConfirmation in frame_get_limitation.py.
type FrameGetLimitationStatusConfirmation struct {
	SessionID uint16
	Data      uint8
}

func (f *FrameGetLimitationStatusConfirmation) Command() Command {
	return GW_GET_LIMITATION_STATUS_CFM
}

// MarshalPayload returns a 3-byte payload matching pyvlx get_payload.
func (f *FrameGetLimitationStatusConfirmation) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 3)
	binary.BigEndian.PutUint16(payload[0:2], f.SessionID)
	payload[2] = f.Data
	return payload, nil
}

// UnmarshalPayload populates the frame from a 3-byte payload (== from_payload).
func (f *FrameGetLimitationStatusConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 3 {
		return fmt.Errorf("FrameGetLimitationStatusConfirmation: invalid payload len %d, want 3", len(payload))
	}
	f.SessionID = binary.BigEndian.Uint16(payload[0:2])
	f.Data = payload[2]
	return nil
}

var _ Frame = (*FrameGetLimitationStatusConfirmation)(nil)

func init() {
	RegisterFrame(GW_GET_LIMITATION_STATUS_CFM, func() Frame { return &FrameGetLimitationStatusConfirmation{} })
}

// FrameLimitationStatusNotification implements Frame for GW_LIMITATION_STATUS_NTF.
// Ported from FrameGetLimitationStatusNotification in frame_get_limitation.py.
type FrameLimitationStatusNotification struct {
	SessionID       uint16
	NodeID          uint8
	ParameterID     uint8
	MinValue        uint16
	MaxValue        uint16
	LimitOriginator Originator
	LimitTime       uint8
}

func (f *FrameLimitationStatusNotification) Command() Command {
	return GW_LIMITATION_STATUS_NTF
}

// MarshalPayload returns a 10-byte payload matching pyvlx get_payload.
func (f *FrameLimitationStatusNotification) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 10)
	binary.BigEndian.PutUint16(payload[0:2], f.SessionID)
	payload[2] = f.NodeID
	payload[3] = f.ParameterID
	binary.BigEndian.PutUint16(payload[4:6], f.MinValue)
	binary.BigEndian.PutUint16(payload[6:8], f.MaxValue)
	payload[8] = uint8(f.LimitOriginator)
	payload[9] = f.LimitTime
	return payload, nil
}

// UnmarshalPayload populates the frame from a 10-byte payload (== from_payload).
func (f *FrameLimitationStatusNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 10 {
		return fmt.Errorf("FrameLimitationStatusNotification: invalid payload len %d, want 10", len(payload))
	}
	f.SessionID = binary.BigEndian.Uint16(payload[0:2])
	f.NodeID = payload[2]
	f.ParameterID = payload[3]
	f.MinValue = binary.BigEndian.Uint16(payload[4:6])
	f.MaxValue = binary.BigEndian.Uint16(payload[6:8])
	f.LimitOriginator = Originator(payload[8])
	f.LimitTime = payload[9]
	return nil
}

var _ Frame = (*FrameLimitationStatusNotification)(nil)

func init() {
	RegisterFrame(GW_LIMITATION_STATUS_NTF, func() Frame { return &FrameLimitationStatusNotification{} })
}

// FrameSetLimitationRequest implements Frame for GW_SET_LIMITATION_REQ.
// Ported from FrameSetLimitationRequest in frame_set_limitation.py.
// limitation_value_min and limitation_value_max are 2-byte Parameter raw values (big-endian).
// limitation_time is a LimitationTime (1 byte).
type FrameSetLimitationRequest struct {
	SessionID          uint16
	Originator         Originator
	Priority           Priority
	NodeIDs            []uint8
	ParameterID        uint8
	LimitationValueMin [2]byte
	LimitationValueMax [2]byte
	LimitationTime     LimitationTime
}

func (f *FrameSetLimitationRequest) Command() Command {
	return GW_SET_LIMITATION_REQ
}

// MarshalPayload returns a 31-byte payload matching pyvlx get_payload.
// Layout: session_id(2) + originator(1) + priority(1) + count(1) + node_ids[20] + parameter_id(1) + min(2) + max(2) + time(1)
func (f *FrameSetLimitationRequest) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 31)
	binary.BigEndian.PutUint16(payload[0:2], f.SessionID)
	payload[2] = uint8(f.Originator)
	payload[3] = uint8(f.Priority)
	nodeCount := len(f.NodeIDs)
	if nodeCount > 20 {
		return nil, fmt.Errorf("FrameSetLimitationRequest: node_ids length %d exceeds max 20", nodeCount)
	}
	payload[4] = uint8(nodeCount)
	copy(payload[5:5+nodeCount], f.NodeIDs)
	// bytes 5+nodeCount .. 24 are zero padding (20 - len(node_ids))
	payload[25] = f.ParameterID
	copy(payload[26:28], f.LimitationValueMin[:])
	copy(payload[28:30], f.LimitationValueMax[:])
	copy(payload[30:31], f.LimitationTime.Bytes())
	return payload, nil
}

// UnmarshalPayload is not implemented for request-only frames (gateway never sends this).
func (f *FrameSetLimitationRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 31 {
		return fmt.Errorf("FrameSetLimitationRequest: invalid payload len %d, want 31", len(payload))
	}
	f.SessionID = binary.BigEndian.Uint16(payload[0:2])
	f.Originator = Originator(payload[2])
	f.Priority = Priority(payload[3])
	nodeCount := int(payload[4])
	if nodeCount > 20 {
		return fmt.Errorf("FrameSetLimitationRequest: node count %d exceeds 20", nodeCount)
	}
	f.NodeIDs = make([]uint8, nodeCount)
	copy(f.NodeIDs, payload[5:5+nodeCount])
	f.ParameterID = payload[25]
	copy(f.LimitationValueMin[:], payload[26:28])
	copy(f.LimitationValueMax[:], payload[28:30])
	f.LimitationTime = LimitationTime{Raw: payload[30]}
	return nil
}

var _ Frame = (*FrameSetLimitationRequest)(nil)

func init() {
	RegisterFrame(GW_SET_LIMITATION_REQ, func() Frame { return &FrameSetLimitationRequest{} })
}

// FrameSetLimitationConfirmation implements Frame for GW_SET_LIMITATION_CFM.
// Ported from FrameSetLimitationConfirmation in frame_set_limitation.py.
type FrameSetLimitationConfirmation struct {
	SessionID uint16
	Status    SetLimitationRequestStatus
}

func (f *FrameSetLimitationConfirmation) Command() Command {
	return GW_SET_LIMITATION_CFM
}

// MarshalPayload returns a 3-byte payload matching pyvlx get_payload.
func (f *FrameSetLimitationConfirmation) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 3)
	binary.BigEndian.PutUint16(payload[0:2], f.SessionID)
	payload[2] = uint8(f.Status)
	return payload, nil
}

// UnmarshalPayload populates the frame from a 3-byte payload (== from_payload).
func (f *FrameSetLimitationConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 3 {
		return fmt.Errorf("FrameSetLimitationConfirmation: invalid payload len %d, want 3", len(payload))
	}
	f.SessionID = binary.BigEndian.Uint16(payload[0:2])
	f.Status = SetLimitationRequestStatus(payload[2])
	return nil
}

var _ Frame = (*FrameSetLimitationConfirmation)(nil)

func init() {
	RegisterFrame(GW_SET_LIMITATION_CFM, func() Frame { return &FrameSetLimitationConfirmation{} })
}

// FrameHouseStatusMonitorEnableRequest implements Frame for GW_HOUSE_STATUS_MONITOR_ENABLE_REQ.
// Ported from FrameHouseStatusMonitorEnableRequest in frame_house_status_monitor_enable_req.py.
// Empty payload (PAYLOAD_LEN = 0).
type FrameHouseStatusMonitorEnableRequest struct{}

func (f *FrameHouseStatusMonitorEnableRequest) Command() Command {
	return GW_HOUSE_STATUS_MONITOR_ENABLE_REQ
}

func (f *FrameHouseStatusMonitorEnableRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameHouseStatusMonitorEnableRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameHouseStatusMonitorEnableRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameHouseStatusMonitorEnableRequest)(nil)

func init() {
	RegisterFrame(GW_HOUSE_STATUS_MONITOR_ENABLE_REQ, func() Frame { return &FrameHouseStatusMonitorEnableRequest{} })
}

// FrameHouseStatusMonitorEnableConfirmation implements Frame for GW_HOUSE_STATUS_MONITOR_ENABLE_CFM.
// Ported from FrameHouseStatusMonitorEnableConfirmation in frame_house_status_monitor_enable_cfm.py.
// Empty payload (PAYLOAD_LEN = 0).
type FrameHouseStatusMonitorEnableConfirmation struct{}

func (f *FrameHouseStatusMonitorEnableConfirmation) Command() Command {
	return GW_HOUSE_STATUS_MONITOR_ENABLE_CFM
}

func (f *FrameHouseStatusMonitorEnableConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameHouseStatusMonitorEnableConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameHouseStatusMonitorEnableConfirmation: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameHouseStatusMonitorEnableConfirmation)(nil)

func init() {
	RegisterFrame(GW_HOUSE_STATUS_MONITOR_ENABLE_CFM, func() Frame { return &FrameHouseStatusMonitorEnableConfirmation{} })
}

// FrameHouseStatusMonitorDisableRequest implements Frame for GW_HOUSE_STATUS_MONITOR_DISABLE_REQ.
// Ported from FrameHouseStatusMonitorDisableRequest in frame_house_status_monitor_disable_req.py.
// Empty payload (PAYLOAD_LEN = 0).
type FrameHouseStatusMonitorDisableRequest struct{}

func (f *FrameHouseStatusMonitorDisableRequest) Command() Command {
	return GW_HOUSE_STATUS_MONITOR_DISABLE_REQ
}

func (f *FrameHouseStatusMonitorDisableRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameHouseStatusMonitorDisableRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameHouseStatusMonitorDisableRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameHouseStatusMonitorDisableRequest)(nil)

func init() {
	RegisterFrame(GW_HOUSE_STATUS_MONITOR_DISABLE_REQ, func() Frame { return &FrameHouseStatusMonitorDisableRequest{} })
}

// FrameHouseStatusMonitorDisableConfirmation implements Frame for GW_HOUSE_STATUS_MONITOR_DISABLE_CFM.
// Ported from FrameHouseStatusMonitorDisableConfirmation in frame_house_status_monitor_disable_cfm.py.
// Empty payload (PAYLOAD_LEN = 0).
type FrameHouseStatusMonitorDisableConfirmation struct{}

func (f *FrameHouseStatusMonitorDisableConfirmation) Command() Command {
	return GW_HOUSE_STATUS_MONITOR_DISABLE_CFM
}

func (f *FrameHouseStatusMonitorDisableConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameHouseStatusMonitorDisableConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameHouseStatusMonitorDisableConfirmation: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameHouseStatusMonitorDisableConfirmation)(nil)

func init() {
	RegisterFrame(GW_HOUSE_STATUS_MONITOR_DISABLE_CFM, func() Frame { return &FrameHouseStatusMonitorDisableConfirmation{} })
}
