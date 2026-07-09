package klf200

import (
	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// NodeInfo is the type-specific node information handed to a NodeConstructor. It
// is extracted from a GW_GET_(ALL_)NODE(S)_INFORMATION_NTF frame and decouples
// concrete node constructors from the protocol frame layout. It carries exactly
// the fields pyvlx's node_helper.convert_frame_to_node reads off the frame.
type NodeInfo struct {
	NodeType        protocol.NodeTypeWithSubtype
	CurrentPosition protocol.Parameter
	Target          protocol.Parameter
}

// NodeFromInfo constructs the correct concrete Node for the given raw node
// information. It looks up the node-type registry and, if a constructor is
// registered, builds the typed node; for unknown types it falls back to a
// generic node so every reported node still appears in the collection. It is
// the Go port of node_helper.convert_frame_to_node (the big if/elif dispatch is
// replaced by the decentralised node-type registry).
func NodeFromInfo(client *Client, nodeID uint16, name string, serial [8]byte, info NodeInfo) Node {
	base := NewBaseNode(client, nodeID, name, serial)
	if constructor, ok := lookupNodeType(info.NodeType); ok {
		return constructor(base, info)
	}
	// Unknown node type: fall back to a generic node (pyvlx logs a warning and
	// returns None; here we keep the node so it is still tracked).
	return newGenericNode(base)
}

// ConvertAllNodesFrame builds a Node from a GW_GET_ALL_NODES_INFORMATION_NTF
// frame. Ported from node_helper.convert_frame_to_node (all-nodes variant).
func ConvertAllNodesFrame(client *Client, frame *protocol.FrameGetAllNodesInformationNotification) Node {
	return NodeFromInfo(
		client,
		uint16(frame.NodeID),
		frame.Name,
		frame.SerialNumber,
		NodeInfo{
			NodeType:        frame.NodeType,
			CurrentPosition: frame.CurrentPosition,
			Target:          frame.Target,
		},
	)
}

// ConvertNodeFrame builds a Node from a GW_GET_NODE_INFORMATION_NTF frame.
// Ported from node_helper.convert_frame_to_node (single-node variant).
func ConvertNodeFrame(client *Client, frame *protocol.FrameGetNodeInformationNotification) Node {
	return NodeFromInfo(
		client,
		uint16(frame.NodeID),
		frame.Name,
		frame.SerialNumber,
		NodeInfo{
			NodeType:        frame.NodeType,
			CurrentPosition: frame.CurrentPosition,
			Target:          frame.Target,
		},
	)
}

// genericNode is the fallback Node for node types without a registered
// constructor. It carries only the shared identity/observer behaviour of
// BaseNode. It corresponds to pyvlx returning None for unimplemented types,
// except we keep the node so callers still see it exists.
type genericNode struct {
	BaseNode
}

func newGenericNode(base BaseNode) Node {
	return &genericNode{BaseNode: base}
}

// AfterUpdate satisfies the Node interface, notifying observers with the
// concrete generic node.
func (n *genericNode) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

var _ Node = (*genericNode)(nil)
