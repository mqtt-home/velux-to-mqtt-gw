package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ============================================================
// GW_GET_SCENE_LIST_REQ (0x040C)
// ============================================================

// FrameGetSceneListRequest implements Frame for GW_GET_SCENE_LIST_REQ.
// Ported from frame_get_scene_list.py FrameGetSceneListRequest.
type FrameGetSceneListRequest struct{}

func (f *FrameGetSceneListRequest) Command() Command { return GW_GET_SCENE_LIST_REQ }

func (f *FrameGetSceneListRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGetSceneListRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGetSceneListRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGetSceneListRequest)(nil)

func init() {
	RegisterFrame(GW_GET_SCENE_LIST_REQ, func() Frame { return &FrameGetSceneListRequest{} })
}

// ============================================================
// GW_GET_SCENE_LIST_CFM (0x040D)
// ============================================================

// FrameGetSceneListConfirmation implements Frame for GW_GET_SCENE_LIST_CFM.
// Ported from frame_get_scene_list.py FrameGetSceneListConfirmation.
type FrameGetSceneListConfirmation struct {
	CountScenes uint8
}

func (f *FrameGetSceneListConfirmation) Command() Command { return GW_GET_SCENE_LIST_CFM }

func (f *FrameGetSceneListConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{f.CountScenes}, nil
}

func (f *FrameGetSceneListConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 1 {
		return fmt.Errorf("FrameGetSceneListConfirmation: invalid payload len %d, want 1", len(payload))
	}
	f.CountScenes = payload[0]
	return nil
}

var _ Frame = (*FrameGetSceneListConfirmation)(nil)

func init() {
	RegisterFrame(GW_GET_SCENE_LIST_CFM, func() Frame { return &FrameGetSceneListConfirmation{} })
}

// ============================================================
// GW_GET_SCENE_LIST_NTF (0x040E)
// ============================================================

// SceneEntry holds a scene number and name pair, as used in the scene list notification.
type SceneEntry struct {
	Number uint8
	Name   string
}

// FrameGetSceneListNotification implements Frame for GW_GET_SCENE_LIST_NTF.
// Ported from frame_get_scene_list.py FrameGetSceneListNotification.
//
// Wire layout per scene entry (65 bytes total):
//   - 1 byte:  scene number
//   - 64 bytes: scene name (UTF-8, zero-padded; bytes_to_string stops at first 0x00)
type FrameGetSceneListNotification struct {
	Scenes          []SceneEntry
	RemainingScenes uint8
}

func (f *FrameGetSceneListNotification) Command() Command { return GW_GET_SCENE_LIST_NTF }

// sceneNameToBytes encodes a name to exactly 64 bytes (UTF-8, zero-padded).
// Mirrors pyvlx string_helper.string_to_bytes(name, 64).
func sceneNameToBytes(name string) ([]byte, error) {
	encoded := []byte(name)
	if len(encoded) > 64 {
		return nil, fmt.Errorf("scene name too long: %d bytes (max 64)", len(encoded))
	}
	buf := make([]byte, 64)
	copy(buf, encoded)
	return buf, nil
}

// sceneNameFromBytes decodes a zero-terminated UTF-8 name from exactly 64 bytes.
// Mirrors pyvlx string_helper.bytes_to_string.
func sceneNameFromBytes(raw []byte) string {
	idx := bytes.IndexByte(raw, 0x00)
	if idx == -1 {
		return string(raw)
	}
	return string(raw[:idx])
}

func (f *FrameGetSceneListNotification) MarshalPayload() ([]byte, error) {
	var buf []byte
	buf = append(buf, byte(len(f.Scenes)))
	for _, s := range f.Scenes {
		nameBytes, err := sceneNameToBytes(s.Name)
		if err != nil {
			return nil, err
		}
		buf = append(buf, s.Number)
		buf = append(buf, nameBytes...)
	}
	buf = append(buf, f.RemainingScenes)
	return buf, nil
}

func (f *FrameGetSceneListNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) < 2 {
		return fmt.Errorf("FrameGetSceneListNotification: payload too short (%d bytes)", len(payload))
	}
	numberOfObjects := int(payload[0])
	// pyvlx: predicted_len = number_of_objects * 65 + 2
	predicted := numberOfObjects*65 + 2
	if len(payload) != predicted {
		return fmt.Errorf("FrameGetSceneListNotification: wrong length %d, want %d (scene_list_notification_wrong_length)", len(payload), predicted)
	}
	f.RemainingScenes = payload[len(payload)-1]
	f.Scenes = make([]SceneEntry, numberOfObjects)
	for i := 0; i < numberOfObjects; i++ {
		// pyvlx: scene = payload[(i * 65 + 1) : (i * 65 + 66)]
		start := i*65 + 1
		scene := payload[start : start+65]
		f.Scenes[i] = SceneEntry{
			Number: scene[0],
			Name:   sceneNameFromBytes(scene[1:]),
		}
	}
	return nil
}

var _ Frame = (*FrameGetSceneListNotification)(nil)

func init() {
	RegisterFrame(GW_GET_SCENE_LIST_NTF, func() Frame { return &FrameGetSceneListNotification{} })
}

// ============================================================
// GW_ACTIVATE_SCENE_REQ (0x0412)
// ============================================================

// FrameActivateSceneRequest implements Frame for GW_ACTIVATE_SCENE_REQ.
// Ported from frame_activate_scene.py FrameActivateSceneRequest.
//
// Wire layout (6 bytes payload):
//
//	[0-1] SessionID big-endian
//	[2]   Originator
//	[3]   Priority
//	[4]   SceneID
//	[5]   Velocity
type FrameActivateSceneRequest struct {
	SceneID    uint8
	SessionID  uint16
	Originator Originator
	Priority   Priority
	Velocity   Velocity
}

func (f *FrameActivateSceneRequest) Command() Command { return GW_ACTIVATE_SCENE_REQ }

func (f *FrameActivateSceneRequest) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 6)
	binary.BigEndian.PutUint16(payload[0:2], f.SessionID)
	payload[2] = uint8(f.Originator)
	payload[3] = uint8(f.Priority)
	payload[4] = f.SceneID
	payload[5] = uint8(f.Velocity)
	return payload, nil
}

func (f *FrameActivateSceneRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 6 {
		return fmt.Errorf("FrameActivateSceneRequest: invalid payload len %d, want 6", len(payload))
	}
	f.SessionID = binary.BigEndian.Uint16(payload[0:2])
	f.Originator = Originator(payload[2])
	f.Priority = Priority(payload[3])
	f.SceneID = payload[4]
	f.Velocity = Velocity(payload[5])
	return nil
}

var _ Frame = (*FrameActivateSceneRequest)(nil)

func init() {
	RegisterFrame(GW_ACTIVATE_SCENE_REQ, func() Frame {
		return &FrameActivateSceneRequest{
			Originator: OriginatorUser,
			Priority:   PriorityUserLevel2,
			Velocity:   VelocityDefault,
		}
	})
}

// ============================================================
// GW_ACTIVATE_SCENE_CFM (0x0413)
// ============================================================

// ActivateSceneConfirmationStatus is the status code in GW_ACTIVATE_SCENE_CFM.
// Ported from frame_activate_scene.py ActivateSceneConfirmationStatus.
type ActivateSceneConfirmationStatus uint8

const (
	ActivateSceneConfirmationStatusAccepted              ActivateSceneConfirmationStatus = 0
	ActivateSceneConfirmationStatusErrorInvalidParameter ActivateSceneConfirmationStatus = 1
	ActivateSceneConfirmationStatusErrorRequestRejected  ActivateSceneConfirmationStatus = 2
)

// FrameActivateSceneConfirmation implements Frame for GW_ACTIVATE_SCENE_CFM.
// Ported from frame_activate_scene.py FrameActivateSceneConfirmation.
//
// Wire layout (3 bytes payload):
//
//	[0]   Status
//	[1-2] SessionID big-endian
type FrameActivateSceneConfirmation struct {
	SessionID uint16
	Status    ActivateSceneConfirmationStatus
}

func (f *FrameActivateSceneConfirmation) Command() Command { return GW_ACTIVATE_SCENE_CFM }

func (f *FrameActivateSceneConfirmation) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 3)
	payload[0] = uint8(f.Status)
	binary.BigEndian.PutUint16(payload[1:3], f.SessionID)
	return payload, nil
}

func (f *FrameActivateSceneConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 3 {
		return fmt.Errorf("FrameActivateSceneConfirmation: invalid payload len %d, want 3", len(payload))
	}
	f.Status = ActivateSceneConfirmationStatus(payload[0])
	f.SessionID = binary.BigEndian.Uint16(payload[1:3])
	return nil
}

var _ Frame = (*FrameActivateSceneConfirmation)(nil)

func init() {
	RegisterFrame(GW_ACTIVATE_SCENE_CFM, func() Frame { return &FrameActivateSceneConfirmation{} })
}

// ============================================================
// GW_WINK_SEND_REQ (0x0308)
// ============================================================

// FrameWinkSendRequest implements Frame for GW_WINK_SEND_REQ.
// Ported from the VELUX KLF200 API specification (pyvlx defines the command codes
// in const.py but provides no frame implementation file for wink_send).
//
// Wire layout (spec Table 192, GW_WINK_SEND_REQ):
//
//	[0-1]   SessionID big-endian    (Data 1-2)
//	[2]     Originator              (Data 3)
//	[3]     Priority                (Data 4)
//	[4]     WinkState (0=disable, 1=enable)  (Data 5)
//	[5]     WinkTime                (Data 6)
//	[6]     IndexArrayCount (len of NodeIDs, 0..20)  (Data 7)
//	[7-26]  IndexArray: NodeIDs zero-padded to 20 bytes  (Data 8-27)
//
// Total payload length: 27 bytes.
type FrameWinkSendRequest struct {
	SessionID  uint16
	Originator Originator
	Priority   Priority
	NodeIDs    []uint8 // up to 20 node IDs
	WinkState  uint8   // 0=disable, 1=enable
	WinkTime   WinkTime
}

func (f *FrameWinkSendRequest) Command() Command { return GW_WINK_SEND_REQ }

func (f *FrameWinkSendRequest) MarshalPayload() ([]byte, error) {
	if len(f.NodeIDs) > 20 {
		return nil, fmt.Errorf("FrameWinkSendRequest: too many node IDs %d (max 20)", len(f.NodeIDs))
	}
	payload := make([]byte, 27)
	binary.BigEndian.PutUint16(payload[0:2], f.SessionID)
	payload[2] = uint8(f.Originator)
	payload[3] = uint8(f.Priority)
	payload[4] = f.WinkState
	payload[5] = uint8(f.WinkTime)
	payload[6] = uint8(len(f.NodeIDs))
	copy(payload[7:27], f.NodeIDs)
	// bytes 7+len..26 remain zero (IndexArray padding to 20 bytes)
	return payload, nil
}

func (f *FrameWinkSendRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 27 {
		return fmt.Errorf("FrameWinkSendRequest: invalid payload len %d, want 27", len(payload))
	}
	f.SessionID = binary.BigEndian.Uint16(payload[0:2])
	f.Originator = Originator(payload[2])
	f.Priority = Priority(payload[3])
	f.WinkState = payload[4]
	f.WinkTime = WinkTime(payload[5])
	numberOfNodes := int(payload[6])
	if numberOfNodes > 20 {
		return fmt.Errorf("FrameWinkSendRequest: node count %d exceeds maximum 20", numberOfNodes)
	}
	f.NodeIDs = make([]uint8, numberOfNodes)
	copy(f.NodeIDs, payload[7:7+numberOfNodes])
	return nil
}

var _ Frame = (*FrameWinkSendRequest)(nil)

func init() {
	RegisterFrame(GW_WINK_SEND_REQ, func() Frame {
		return &FrameWinkSendRequest{
			Originator: OriginatorUser,
			Priority:   PriorityUserLevel2,
		}
	})
}

// ============================================================
// GW_WINK_SEND_CFM (0x0309)
// ============================================================

// WinkSendConfirmationStatus is the status code in GW_WINK_SEND_CFM.
type WinkSendConfirmationStatus uint8

const (
	WinkSendConfirmationStatusAccepted WinkSendConfirmationStatus = 0
	WinkSendConfirmationStatusRejected WinkSendConfirmationStatus = 1
)

// FrameWinkSendConfirmation implements Frame for GW_WINK_SEND_CFM.
//
// Wire layout (3 bytes payload):
//
//	[0-1] SessionID big-endian
//	[2]   Status
type FrameWinkSendConfirmation struct {
	SessionID uint16
	Status    WinkSendConfirmationStatus
}

func (f *FrameWinkSendConfirmation) Command() Command { return GW_WINK_SEND_CFM }

func (f *FrameWinkSendConfirmation) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 3)
	binary.BigEndian.PutUint16(payload[0:2], f.SessionID)
	payload[2] = uint8(f.Status)
	return payload, nil
}

func (f *FrameWinkSendConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 3 {
		return fmt.Errorf("FrameWinkSendConfirmation: invalid payload len %d, want 3", len(payload))
	}
	f.SessionID = binary.BigEndian.Uint16(payload[0:2])
	f.Status = WinkSendConfirmationStatus(payload[2])
	return nil
}

var _ Frame = (*FrameWinkSendConfirmation)(nil)

func init() {
	RegisterFrame(GW_WINK_SEND_CFM, func() Frame { return &FrameWinkSendConfirmation{} })
}

// ============================================================
// GW_WINK_SEND_NTF (0x030A)
// ============================================================

// FrameWinkSendNotification implements Frame for GW_WINK_SEND_NTF.
//
// Wire layout (spec Table 197, 2 bytes payload):
//
//	[0-1] SessionID big-endian  (Data 1-2)
//
// GW_WINK_SEND_NTF is a single session-level notification carrying only the
// SessionID (Figure 19); there are no Status or NodeID fields.
type FrameWinkSendNotification struct {
	SessionID uint16
}

func (f *FrameWinkSendNotification) Command() Command { return GW_WINK_SEND_NTF }

func (f *FrameWinkSendNotification) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 2)
	binary.BigEndian.PutUint16(payload[0:2], f.SessionID)
	return payload, nil
}

func (f *FrameWinkSendNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 2 {
		return fmt.Errorf("FrameWinkSendNotification: invalid payload len %d, want 2", len(payload))
	}
	f.SessionID = binary.BigEndian.Uint16(payload[0:2])
	return nil
}

var _ Frame = (*FrameWinkSendNotification)(nil)

func init() {
	RegisterFrame(GW_WINK_SEND_NTF, func() Frame { return &FrameWinkSendNotification{} })
}
