package klf200

// node_loading.go — ports api/get_all_nodes_information.py and
// api/get_node_information.py from pyvlx (tjaehnel/pyvlx@master_vlxmqttha).
//
// LoadAllNodes and LoadNode implement the two request/response sequences:
//
//   get_all_nodes_information.py:
//     send  GW_GET_ALL_NODES_INFORMATION_REQ
//     recv  GW_GET_ALL_NODES_INFORMATION_CFM   (records number_of_nodes)
//     recv* GW_GET_ALL_NODES_INFORMATION_NTF   (one per node)
//     recv  GW_GET_ALL_NODES_INFORMATION_FINISHED_NTF  (terminal)
//
//   get_node_information.py:
//     send  GW_GET_NODE_INFORMATION_REQ  (node_id)
//     recv  GW_GET_NODE_INFORMATION_CFM  (same node_id, ignored if not matching)
//     recv  GW_GET_NODE_INFORMATION_NTF  (same node_id, terminal)
//
// Client.LoadNodes (client.go) delegates to LoadAllNodes and rebuilds the node
// collection. Client.LoadNode loads or refreshes a single node by ID and
// upserts it into the collection — porting Nodes._load_node from pyvlx.

import (
	"context"
	"fmt"
	"log"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// LoadAllNodes issues GW_GET_ALL_NODES_INFORMATION_REQ, accumulates the
// per-node notification frames (after the confirmation), waits for the
// FINISHED_NTF, and returns the collected notification frames. Faithfully ports
// GetAllNodesInformation from api/get_all_nodes_information.py: the
// confirmation sets an expected count and a count mismatch is logged as a
// warning (matching pyvlx's PYVLXLOG.warning call).
//
// It is called by Client.LoadNodes which owns rebuilding the Nodes collection.
func LoadAllNodes(ctx context.Context, c *Client) ([]*protocol.FrameGetAllNodesInformationNotification, error) {
	req := &protocol.FrameGetAllNodesInformationRequest{}

	numberOfNodes := -1 // -1 = confirmation not yet received
	var collected []*protocol.FrameGetAllNodesInformationNotification

	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		switch f := frame.(type) {
		case *protocol.FrameGetAllNodesInformationConfirmation:
			// Confirmation arrives first; record expected count but keep waiting.
			// Mirrors: self.number_of_nodes = frame.number_of_nodes; return False
			numberOfNodes = int(f.NumberOfNodes)
			return false

		case *protocol.FrameGetAllNodesInformationNotification:
			// Accumulate per-node frames.
			// Mirrors: self.notification_frames.append(frame); return False
			collected = append(collected, f)
			return false

		case *protocol.FrameGetAllNodesInformationFinishedNotification:
			// Terminal frame. Mirrors: self.success = True; return True
			// Warn when the received count does not match what was promised.
			if numberOfNodes >= 0 && numberOfNodes != len(collected) {
				log.Printf("klf200: load all nodes: received %d nodes, expected %d",
					len(collected), numberOfNodes)
			}
			return true

		default:
			return false
		}
	})
	if err != nil {
		return nil, fmt.Errorf("klf200: load all nodes: %w", err)
	}
	return collected, nil
}

// LoadNode loads or refreshes a single node by nodeID and upserts it into the
// client's Nodes collection. It ports GetNodeInformation from
// api/get_node_information.py and Nodes._load_node from pyvlx:
//
//	send  GW_GET_NODE_INFORMATION_REQ (node_id)
//	recv  GW_GET_NODE_INFORMATION_CFM  — correlated by node_id, discarded
//	recv  GW_GET_NODE_INFORMATION_NTF  — correlated by node_id, terminal
//
// The node is built via ConvertNodeFrame (node_helper) and Add'd to
// Client.Nodes, replacing any existing entry for that nodeID.
func (c *Client) LoadNode(ctx context.Context, nodeID uint8) error {
	req := &protocol.FrameGetNodeInformationRequest{NodeID: nodeID}

	var ntf *protocol.FrameGetNodeInformationNotification

	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		switch f := frame.(type) {
		case *protocol.FrameGetNodeInformationConfirmation:
			// Only care if it matches our node; still waiting for the NTF.
			// Mirrors: isinstance(...Confirmation) and frame.node_id == self.node_id; return False
			if f.NodeID != nodeID {
				return false
			}
			return false

		case *protocol.FrameGetNodeInformationNotification:
			// Terminal frame when node_id matches.
			// Mirrors: isinstance(...Notification) and frame.node_id == self.node_id; return True
			if f.NodeID != nodeID {
				return false
			}
			ntf = f
			return true

		default:
			return false
		}
	})
	if err != nil {
		return fmt.Errorf("klf200: load node %d: %w", nodeID, err)
	}
	if ntf == nil {
		return fmt.Errorf("klf200: load node %d: no notification received", nodeID)
	}

	node := ConvertNodeFrame(c, ntf)
	if node != nil {
		c.nodes.Add(node)
	}
	return nil
}
