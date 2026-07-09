package protocol

import (
	"encoding/binary"
	"fmt"
)

// ---------------------------------------------------------------------------
// AliasArray helper (ported from alias_array.py)
// ---------------------------------------------------------------------------

// aliasEntry holds one alias: a 2-byte type code and a 2-byte value.
type aliasEntry struct {
	typ [2]byte
	val [2]byte
}

// AliasArray stores up to 5 node aliases. Ported from AliasArray.
type AliasArray struct {
	entries []aliasEntry
}

// aliasArrayFromBytes parses an AliasArray from exactly 21 raw bytes.
// Ported from AliasArray.parse_raw.
func aliasArrayFromBytes(raw []byte) (AliasArray, error) {
	if len(raw) != 21 {
		return AliasArray{}, fmt.Errorf("AliasArray: invalid size %d, want 21", len(raw))
	}
	nbrOfAlias := int(raw[0])
	if nbrOfAlias > 5 {
		return AliasArray{}, fmt.Errorf("AliasArray: invalid nbr_of_alias %d", nbrOfAlias)
	}
	a := AliasArray{entries: make([]aliasEntry, nbrOfAlias)}
	for i := 0; i < nbrOfAlias; i++ {
		base := i*4 + 1
		copy(a.entries[i].typ[:], raw[base:base+2])
		copy(a.entries[i].val[:], raw[base+2:base+4])
	}
	return a, nil
}

// Bytes serialises the AliasArray into exactly 21 bytes.
// Ported from AliasArray.__bytes__:
//
//	ret = bytes([len(self.alias_array_)])
//	for alias in self.alias_array_: ret += alias[0] + alias[1]
//	ret += bytes((5 - len(self.alias_array_)) * 4)
func (a AliasArray) Bytes() []byte {
	out := make([]byte, 21)
	out[0] = byte(len(a.entries))
	for i, e := range a.entries {
		base := i*4 + 1
		copy(out[base:base+2], e.typ[:])
		copy(out[base+2:base+4], e.val[:])
	}
	// remaining bytes are already zero (padding)
	return out
}

// String returns a human-readable representation. Ported from AliasArray.__str__.
func (a AliasArray) String() string {
	if len(a.entries) == 0 {
		return ""
	}
	s := ""
	for i, e := range a.entries {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%02x%02x=%02x%02x", e.typ[0], e.typ[1], e.val[0], e.val[1])
	}
	return s
}

// ---------------------------------------------------------------------------
// Shared node-info payload marshal/unmarshal (124 bytes)
// Used by both FrameGetAllNodesInformationNotification and
// FrameGetNodeInformationNotification which share the identical wire layout.
// ---------------------------------------------------------------------------

type nodeInfo struct {
	NodeID             uint8
	Order              uint16
	Placement          uint8
	Name               string
	Velocity           Velocity
	NodeType           NodeTypeWithSubtype
	ProductGroup       uint8
	ProductType        uint8
	NodeVariation      NodeVariation
	PowerMode          uint8
	BuildNumber        uint8
	SerialNumber       [8]byte // all-zero means "no serial"
	State              uint8
	CurrentPosition    Parameter
	Target             Parameter
	CurrentPositionFP1 Parameter
	CurrentPositionFP2 Parameter
	CurrentPositionFP3 Parameter
	CurrentPositionFP4 Parameter
	RemainingTime      uint16
	Timestamp          uint32
	AliasArray         AliasArray
}

const nodeInfoPayloadLen = 124

func marshalNodeInfo(n *nodeInfo) ([]byte, error) {
	payload := make([]byte, 0, nodeInfoPayloadLen)
	payload = append(payload, n.NodeID)
	payload = append(payload, byte(n.Order>>8&0xFF), byte(n.Order&0xFF))
	payload = append(payload, n.Placement)
	nameBytes, err := stringToBytes(n.Name, 64)
	if err != nil {
		return nil, err
	}
	payload = append(payload, nameBytes...)
	payload = append(payload, byte(n.Velocity))
	payload = append(payload, byte(uint16(n.NodeType)>>8&0xFF), byte(uint16(n.NodeType)&0xFF))
	payload = append(payload, n.ProductGroup)
	payload = append(payload, n.ProductType)
	payload = append(payload, byte(n.NodeVariation))
	payload = append(payload, n.PowerMode)
	payload = append(payload, n.BuildNumber)
	payload = append(payload, n.SerialNumber[:]...)
	payload = append(payload, n.State)
	payload = append(payload, n.CurrentPosition.Bytes()...)
	payload = append(payload, n.Target.Bytes()...)
	payload = append(payload, n.CurrentPositionFP1.Bytes()...)
	payload = append(payload, n.CurrentPositionFP2.Bytes()...)
	payload = append(payload, n.CurrentPositionFP3.Bytes()...)
	payload = append(payload, n.CurrentPositionFP4.Bytes()...)
	payload = append(payload, byte(n.RemainingTime>>8&0xFF), byte(n.RemainingTime&0xFF))
	var ts [4]byte
	binary.BigEndian.PutUint32(ts[:], n.Timestamp)
	payload = append(payload, ts[:]...)
	payload = append(payload, n.AliasArray.Bytes()...)
	return payload, nil
}

func unmarshalNodeInfo(payload []byte, n *nodeInfo) error {
	if len(payload) != nodeInfoPayloadLen {
		return fmt.Errorf("nodeInfo: invalid payload len %d, want %d", len(payload), nodeInfoPayloadLen)
	}
	n.NodeID = payload[0]
	n.Order = uint16(payload[1])*256 + uint16(payload[2])
	n.Placement = payload[3]
	n.Name = bytesToString(payload[4:68])
	n.Velocity = Velocity(payload[68])
	n.NodeType = NodeTypeWithSubtype(uint16(payload[69])*256 + uint16(payload[70]))
	n.ProductGroup = payload[71]
	n.ProductType = payload[72]
	n.NodeVariation = NodeVariation(payload[73])
	n.PowerMode = payload[74]
	n.BuildNumber = payload[75] // note: VELUX documentation is wrong here (per pyvlx comment)
	copy(n.SerialNumber[:], payload[76:84])
	n.State = payload[84]
	p, err := NewParameter(&[2]byte{payload[85], payload[86]})
	if err != nil {
		return err
	}
	n.CurrentPosition = p
	p, err = NewParameter(&[2]byte{payload[87], payload[88]})
	if err != nil {
		return err
	}
	n.Target = p
	p, err = NewParameter(&[2]byte{payload[89], payload[90]})
	if err != nil {
		return err
	}
	n.CurrentPositionFP1 = p
	p, err = NewParameter(&[2]byte{payload[91], payload[92]})
	if err != nil {
		return err
	}
	n.CurrentPositionFP2 = p
	p, err = NewParameter(&[2]byte{payload[93], payload[94]})
	if err != nil {
		return err
	}
	n.CurrentPositionFP3 = p
	p, err = NewParameter(&[2]byte{payload[95], payload[96]})
	if err != nil {
		return err
	}
	n.CurrentPositionFP4 = p
	n.RemainingTime = uint16(payload[97])*256 + uint16(payload[98])
	n.Timestamp = binary.BigEndian.Uint32(payload[99:103])
	// Alias array is the final 21 bytes (indices 103..123). pyvlx writes
	// payload[103:125]; Python clamps that to the 124-byte payload (21 bytes),
	// but Go reslices into the buffer capacity and would hand 22 bytes (incl. the
	// trailing CRC) to the 21-byte alias parser — so slice to the exact end.
	aa, err := aliasArrayFromBytes(payload[103:124])
	if err != nil {
		return err
	}
	n.AliasArray = aa
	return nil
}

// ---------------------------------------------------------------------------
// GW_GET_ALL_NODES_INFORMATION_REQ (0x0202)
// Ported from FrameGetAllNodesInformationRequest (PAYLOAD_LEN=0)
// ---------------------------------------------------------------------------

// FrameGetAllNodesInformationRequest implements Frame for GW_GET_ALL_NODES_INFORMATION_REQ.
type FrameGetAllNodesInformationRequest struct{}

func (f *FrameGetAllNodesInformationRequest) Command() Command {
	return GW_GET_ALL_NODES_INFORMATION_REQ
}

func (f *FrameGetAllNodesInformationRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGetAllNodesInformationRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGetAllNodesInformationRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGetAllNodesInformationRequest)(nil)

// ---------------------------------------------------------------------------
// GW_GET_ALL_NODES_INFORMATION_CFM (0x0203)
// Ported from FrameGetAllNodesInformationConfirmation (PAYLOAD_LEN=2)
// ---------------------------------------------------------------------------

// AllNodesInformationStatus is the status code in GW_GET_ALL_NODES_INFORMATION_CFM.
// Ported from AllNodesInformationStatus.
type AllNodesInformationStatus uint8

const (
	AllNodesInformationStatusOK                    AllNodesInformationStatus = 0
	AllNodesInformationStatusErrorSystemTableEmpty AllNodesInformationStatus = 1
)

// FrameGetAllNodesInformationConfirmation implements Frame for GW_GET_ALL_NODES_INFORMATION_CFM.
type FrameGetAllNodesInformationConfirmation struct {
	Status        AllNodesInformationStatus
	NumberOfNodes uint8
}

func (f *FrameGetAllNodesInformationConfirmation) Command() Command {
	return GW_GET_ALL_NODES_INFORMATION_CFM
}

func (f *FrameGetAllNodesInformationConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{byte(f.Status), f.NumberOfNodes}, nil
}

func (f *FrameGetAllNodesInformationConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 2 {
		return fmt.Errorf("FrameGetAllNodesInformationConfirmation: invalid payload len %d, want 2", len(payload))
	}
	f.Status = AllNodesInformationStatus(payload[0])
	f.NumberOfNodes = payload[1]
	return nil
}

var _ Frame = (*FrameGetAllNodesInformationConfirmation)(nil)

// ---------------------------------------------------------------------------
// GW_GET_ALL_NODES_INFORMATION_NTF (0x0204)
// Ported from FrameGetAllNodesInformationNotification (PAYLOAD_LEN=124)
// ---------------------------------------------------------------------------

// FrameGetAllNodesInformationNotification implements Frame for GW_GET_ALL_NODES_INFORMATION_NTF.
type FrameGetAllNodesInformationNotification struct {
	nodeInfo
}

func (f *FrameGetAllNodesInformationNotification) Command() Command {
	return GW_GET_ALL_NODES_INFORMATION_NTF
}

func (f *FrameGetAllNodesInformationNotification) MarshalPayload() ([]byte, error) {
	return marshalNodeInfo(&f.nodeInfo)
}

func (f *FrameGetAllNodesInformationNotification) UnmarshalPayload(payload []byte) error {
	return unmarshalNodeInfo(payload, &f.nodeInfo)
}

var _ Frame = (*FrameGetAllNodesInformationNotification)(nil)

// ---------------------------------------------------------------------------
// GW_GET_ALL_NODES_INFORMATION_FINISHED_NTF (0x0205)
// Ported from FrameGetAllNodesInformationFinishedNotification (PAYLOAD_LEN=0)
// ---------------------------------------------------------------------------

// FrameGetAllNodesInformationFinishedNotification implements Frame for GW_GET_ALL_NODES_INFORMATION_FINISHED_NTF.
type FrameGetAllNodesInformationFinishedNotification struct{}

func (f *FrameGetAllNodesInformationFinishedNotification) Command() Command {
	return GW_GET_ALL_NODES_INFORMATION_FINISHED_NTF
}

func (f *FrameGetAllNodesInformationFinishedNotification) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGetAllNodesInformationFinishedNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGetAllNodesInformationFinishedNotification: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGetAllNodesInformationFinishedNotification)(nil)

// ---------------------------------------------------------------------------
// GW_GET_NODE_INFORMATION_REQ (0x0200)
// Ported from FrameGetNodeInformationRequest (PAYLOAD_LEN=1)
// ---------------------------------------------------------------------------

// FrameGetNodeInformationRequest implements Frame for GW_GET_NODE_INFORMATION_REQ.
type FrameGetNodeInformationRequest struct {
	NodeID uint8
}

func (f *FrameGetNodeInformationRequest) Command() Command { return GW_GET_NODE_INFORMATION_REQ }

func (f *FrameGetNodeInformationRequest) MarshalPayload() ([]byte, error) {
	return []byte{f.NodeID}, nil
}

func (f *FrameGetNodeInformationRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 1 {
		return fmt.Errorf("FrameGetNodeInformationRequest: invalid payload len %d, want 1", len(payload))
	}
	f.NodeID = payload[0]
	return nil
}

var _ Frame = (*FrameGetNodeInformationRequest)(nil)

// ---------------------------------------------------------------------------
// GW_GET_NODE_INFORMATION_CFM (0x0201)
// Ported from FrameGetNodeInformationConfirmation (PAYLOAD_LEN=2)
// ---------------------------------------------------------------------------

// NodeInformationStatus is the status code in GW_GET_NODE_INFORMATION_CFM.
// Ported from NodeInformationStatus.
type NodeInformationStatus uint8

const (
	NodeInformationStatusOK                    NodeInformationStatus = 0
	NodeInformationStatusErrorRequestRejected  NodeInformationStatus = 1
	NodeInformationStatusErrorInvalidNodeIndex NodeInformationStatus = 2
)

// FrameGetNodeInformationConfirmation implements Frame for GW_GET_NODE_INFORMATION_CFM.
type FrameGetNodeInformationConfirmation struct {
	Status NodeInformationStatus
	NodeID uint8
}

func (f *FrameGetNodeInformationConfirmation) Command() Command { return GW_GET_NODE_INFORMATION_CFM }

func (f *FrameGetNodeInformationConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{byte(f.Status), f.NodeID}, nil
}

func (f *FrameGetNodeInformationConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 2 {
		return fmt.Errorf("FrameGetNodeInformationConfirmation: invalid payload len %d, want 2", len(payload))
	}
	f.Status = NodeInformationStatus(payload[0])
	f.NodeID = payload[1]
	return nil
}

var _ Frame = (*FrameGetNodeInformationConfirmation)(nil)

// ---------------------------------------------------------------------------
// GW_GET_NODE_INFORMATION_NTF (0x0210)
// Ported from FrameGetNodeInformationNotification (PAYLOAD_LEN=124)
// ---------------------------------------------------------------------------

// FrameGetNodeInformationNotification implements Frame for GW_GET_NODE_INFORMATION_NTF.
type FrameGetNodeInformationNotification struct {
	nodeInfo
}

func (f *FrameGetNodeInformationNotification) Command() Command { return GW_GET_NODE_INFORMATION_NTF }

func (f *FrameGetNodeInformationNotification) MarshalPayload() ([]byte, error) {
	return marshalNodeInfo(&f.nodeInfo)
}

func (f *FrameGetNodeInformationNotification) UnmarshalPayload(payload []byte) error {
	return unmarshalNodeInfo(payload, &f.nodeInfo)
}

var _ Frame = (*FrameGetNodeInformationNotification)(nil)

// ---------------------------------------------------------------------------
// GW_NODE_INFORMATION_CHANGED_NTF (0x020C)
// Ported from FrameNodeInformationChangedNotification (PAYLOAD_LEN=69)
// ---------------------------------------------------------------------------

// FrameNodeInformationChangedNotification implements Frame for GW_NODE_INFORMATION_CHANGED_NTF.
type FrameNodeInformationChangedNotification struct {
	NodeID        uint8
	Name          string
	Order         uint16
	Placement     uint8
	NodeVariation NodeVariation
}

func (f *FrameNodeInformationChangedNotification) Command() Command {
	return GW_NODE_INFORMATION_CHANGED_NTF
}

func (f *FrameNodeInformationChangedNotification) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 0, 69)
	payload = append(payload, f.NodeID)
	nameBytes, err := stringToBytes(f.Name, 64)
	if err != nil {
		return nil, err
	}
	payload = append(payload, nameBytes...)
	payload = append(payload, byte(f.Order>>8&0xFF), byte(f.Order&0xFF))
	payload = append(payload, f.Placement)
	payload = append(payload, byte(f.NodeVariation))
	return payload, nil
}

func (f *FrameNodeInformationChangedNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 69 {
		return fmt.Errorf("FrameNodeInformationChangedNotification: invalid payload len %d, want 69", len(payload))
	}
	f.NodeID = payload[0]
	f.Name = bytesToString(payload[1:65])
	f.Order = uint16(payload[65])*256 + uint16(payload[66])
	f.Placement = payload[67]
	f.NodeVariation = NodeVariation(payload[68])
	return nil
}

var _ Frame = (*FrameNodeInformationChangedNotification)(nil)

// ---------------------------------------------------------------------------
// GW_CS_DISCOVER_NODES_REQ (0x0103)
// Ported from FrameDiscoverNodesRequest (PAYLOAD_LEN=1)
// ---------------------------------------------------------------------------

// FrameDiscoverNodesRequest implements Frame for GW_CS_DISCOVER_NODES_REQ.
type FrameDiscoverNodesRequest struct {
	NodeType NodeType
}

func (f *FrameDiscoverNodesRequest) Command() Command { return GW_CS_DISCOVER_NODES_REQ }

func (f *FrameDiscoverNodesRequest) MarshalPayload() ([]byte, error) {
	return []byte{byte(f.NodeType)}, nil
}

func (f *FrameDiscoverNodesRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 1 {
		return fmt.Errorf("FrameDiscoverNodesRequest: invalid payload len %d, want 1", len(payload))
	}
	f.NodeType = NodeType(payload[0])
	return nil
}

var _ Frame = (*FrameDiscoverNodesRequest)(nil)

// ---------------------------------------------------------------------------
// GW_CS_DISCOVER_NODES_CFM (0x0104)
// Ported from FrameDiscoverNodesConfirmation (PAYLOAD_LEN=0)
// ---------------------------------------------------------------------------

// FrameDiscoverNodesConfirmation implements Frame for GW_CS_DISCOVER_NODES_CFM.
type FrameDiscoverNodesConfirmation struct{}

func (f *FrameDiscoverNodesConfirmation) Command() Command { return GW_CS_DISCOVER_NODES_CFM }

func (f *FrameDiscoverNodesConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameDiscoverNodesConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameDiscoverNodesConfirmation: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameDiscoverNodesConfirmation)(nil)

// ---------------------------------------------------------------------------
// GW_CS_DISCOVER_NODES_NTF (0x0105)
// Ported from FrameDiscoverNodesNotification (PAYLOAD_LEN=131)
// ---------------------------------------------------------------------------

// FrameDiscoverNodesNotification implements Frame for GW_CS_DISCOVER_NODES_NTF.
// pyvlx treats this as an opaque 131-byte payload.
type FrameDiscoverNodesNotification struct {
	Payload [131]byte
}

func (f *FrameDiscoverNodesNotification) Command() Command { return GW_CS_DISCOVER_NODES_NTF }

func (f *FrameDiscoverNodesNotification) MarshalPayload() ([]byte, error) {
	out := make([]byte, 131)
	copy(out, f.Payload[:])
	return out, nil
}

func (f *FrameDiscoverNodesNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 131 {
		return fmt.Errorf("FrameDiscoverNodesNotification: invalid payload len %d, want 131", len(payload))
	}
	copy(f.Payload[:], payload)
	return nil
}

var _ Frame = (*FrameDiscoverNodesNotification)(nil)

// ---------------------------------------------------------------------------
// GW_SET_NODE_NAME_REQ (0x0208)
// Ported from FrameSetNodeNameRequest (PAYLOAD_LEN=65)
// ---------------------------------------------------------------------------

// FrameSetNodeNameRequest implements Frame for GW_SET_NODE_NAME_REQ.
type FrameSetNodeNameRequest struct {
	NodeID uint8
	Name   string
}

func (f *FrameSetNodeNameRequest) Command() Command { return GW_SET_NODE_NAME_REQ }

func (f *FrameSetNodeNameRequest) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 0, 65)
	payload = append(payload, f.NodeID)
	nameBytes, err := stringToBytes(f.Name, 64)
	if err != nil {
		return nil, err
	}
	payload = append(payload, nameBytes...)
	return payload, nil
}

func (f *FrameSetNodeNameRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 65 {
		return fmt.Errorf("FrameSetNodeNameRequest: invalid payload len %d, want 65", len(payload))
	}
	f.NodeID = payload[0]
	f.Name = bytesToString(payload[1:65])
	return nil
}

var _ Frame = (*FrameSetNodeNameRequest)(nil)

// ---------------------------------------------------------------------------
// GW_SET_NODE_NAME_CFM (0x0209)
// Ported from FrameSetNodeNameConfirmation (PAYLOAD_LEN=2)
// ---------------------------------------------------------------------------

// SetNodeNameConfirmationStatus is the status code in GW_SET_NODE_NAME_CFM.
// Ported from SetNodeNameConfirmationStatus.
type SetNodeNameConfirmationStatus uint8

const (
	SetNodeNameConfirmationStatusOK                           SetNodeNameConfirmationStatus = 0
	SetNodeNameConfirmationStatusErrorRequestRejected         SetNodeNameConfirmationStatus = 1
	SetNodeNameConfirmationStatusErrorInvalidSystemTableIndex SetNodeNameConfirmationStatus = 2
)

// FrameSetNodeNameConfirmation implements Frame for GW_SET_NODE_NAME_CFM.
type FrameSetNodeNameConfirmation struct {
	Status SetNodeNameConfirmationStatus
	NodeID uint8
}

func (f *FrameSetNodeNameConfirmation) Command() Command { return GW_SET_NODE_NAME_CFM }

func (f *FrameSetNodeNameConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{byte(f.Status), f.NodeID}, nil
}

func (f *FrameSetNodeNameConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 2 {
		return fmt.Errorf("FrameSetNodeNameConfirmation: invalid payload len %d, want 2", len(payload))
	}
	f.Status = SetNodeNameConfirmationStatus(payload[0])
	f.NodeID = payload[1]
	return nil
}

var _ Frame = (*FrameSetNodeNameConfirmation)(nil)

// ---------------------------------------------------------------------------
// init: register all frames in this file
// ---------------------------------------------------------------------------

func init() {
	RegisterFrame(GW_GET_ALL_NODES_INFORMATION_REQ, func() Frame { return &FrameGetAllNodesInformationRequest{} })
	RegisterFrame(GW_GET_ALL_NODES_INFORMATION_CFM, func() Frame { return &FrameGetAllNodesInformationConfirmation{} })
	RegisterFrame(GW_GET_ALL_NODES_INFORMATION_NTF, func() Frame { return &FrameGetAllNodesInformationNotification{} })
	RegisterFrame(GW_GET_ALL_NODES_INFORMATION_FINISHED_NTF, func() Frame { return &FrameGetAllNodesInformationFinishedNotification{} })
	RegisterFrame(GW_GET_NODE_INFORMATION_REQ, func() Frame { return &FrameGetNodeInformationRequest{} })
	RegisterFrame(GW_GET_NODE_INFORMATION_CFM, func() Frame { return &FrameGetNodeInformationConfirmation{} })
	RegisterFrame(GW_GET_NODE_INFORMATION_NTF, func() Frame { return &FrameGetNodeInformationNotification{} })
	RegisterFrame(GW_NODE_INFORMATION_CHANGED_NTF, func() Frame { return &FrameNodeInformationChangedNotification{} })
	RegisterFrame(GW_CS_DISCOVER_NODES_REQ, func() Frame { return &FrameDiscoverNodesRequest{} })
	RegisterFrame(GW_CS_DISCOVER_NODES_CFM, func() Frame { return &FrameDiscoverNodesConfirmation{} })
	RegisterFrame(GW_CS_DISCOVER_NODES_NTF, func() Frame { return &FrameDiscoverNodesNotification{} })
	RegisterFrame(GW_SET_NODE_NAME_REQ, func() Frame { return &FrameSetNodeNameRequest{} })
	RegisterFrame(GW_SET_NODE_NAME_CFM, func() Frame { return &FrameSetNodeNameConfirmation{} })
}
