package klf200

import (
	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// BlindNode is the interface implemented by Blind nodes. The node_updater uses
// it to distinguish blinds (which need special FP3 / orientation handling) from
// plain opening devices. Ported from isinstance(node, Blind) in node_updater.py.
type BlindNode interface {
	Node
	// SetPositionState stores the new main-parameter position.
	SetPositionState(pos protocol.Position)
	// SetOrientationState stores the FP3 orientation.
	SetOrientationState(orient protocol.Position)
}

// OpeningDeviceNode is the interface implemented by nodes that have a main
// position and a target position (windows, roller shutters, awnings, …).
// Ported from isinstance(node, OpeningDevice) in node_updater.py.
type OpeningDeviceNode interface {
	Node
	// SetPositionState stores the new main-parameter position.
	SetPositionState(pos protocol.Position)
	// SetTargetState stores the new target position.
	SetTargetState(target protocol.Position)
}

// LighteningDeviceNode is the interface implemented by dimmable light nodes.
// Ported from isinstance(node, LighteningDevice) in node_updater.py.
type LighteningDeviceNode interface {
	Node
	// SetIntensityState stores the new intensity (main parameter used as Intensity).
	SetIntensityState(intensity protocol.Position)
}

// Compile-time assertions that the concrete node types satisfy the updater
// interfaces. Without these, a signature drift on the state setters would
// silently make the node_updater's type assertions fail at runtime (the bug
// this guards against): a live-update notification would be received but no
// node would ever be mutated.
var (
	_ BlindNode                = (*Blind)(nil)
	_ OpeningDeviceNode        = (*Window)(nil)
	_ OpeningDeviceNode        = (*RollerShutter)(nil)
	_ OpeningDeviceNode        = (*Awning)(nil)
	_ OpeningDeviceNode        = (*GarageDoor)(nil)
	_ OpeningDeviceNode        = (*Gate)(nil)
	_ OpeningDeviceNode        = (*Blade)(nil)
	_ LighteningDeviceNode     = (*Light)(nil)
	_ LimitedOpeningDeviceNode = (*Window)(nil)
	_ LimitedOpeningDeviceNode = (*Blind)(nil)
)

// NodeUpdater processes inbound notification frames and pushes position/state
// changes into the matching Node in the client's Nodes collection. It is the
// Go port of pyvlx's NodeUpdater class.
//
// Wire it up by calling Register after Connect:
//
//	updater := NewNodeUpdater(client)
//	updater.Register()
type NodeUpdater struct {
	client *Client
}

// NewNodeUpdater constructs a NodeUpdater backed by client.
// Ported from NodeUpdater.__init__.
func NewNodeUpdater(client *Client) *NodeUpdater {
	return &NodeUpdater{client: client}
}

// Register installs the updater as a frame processor on the client.
// Call once after Connect. Corresponds to pyvlx registering
// node_updater.process_frame as a frame-received callback.
func (u *NodeUpdater) Register() {
	u.client.RegisterNodeUpdater(u.processFrame)
}

// processFrame is the FrameProcessor invoked for every inbound frame.
// It dispatches to the appropriate handler. Ported from NodeUpdater.process_frame.
func (u *NodeUpdater) processFrame(frame protocol.Frame) {
	switch f := frame.(type) {
	case *protocol.FrameGetAllNodesInformationNotification:
		u.processPositionFrame(uint16(f.NodeID), f.CurrentPosition, f.Target)
	case *protocol.FrameNodeStatePositionChangedNotification:
		u.processNodeStateFrame(f)
	case *protocol.FrameStatusRequestNotification:
		u.processStatusRequestNotification(f)
	case *protocol.FrameLimitationStatusNotification:
		u.processLimitationStatusNotification(f)
	}
}

// processPositionFrame handles FrameGetAllNodesInformationNotification and
// FrameNodeStatePositionChangedNotification for the position/target fields.
// Ported from NodeUpdater.process_frame (the isinstance(frame, (FrameGetAllNodes…, FrameNodeState…)) branch).
func (u *NodeUpdater) processPositionFrame(nodeID uint16, currentPosition, target protocol.Parameter) {
	node, ok := u.client.Nodes().ByID(nodeID)
	if !ok {
		return
	}

	position, err := protocol.NewPosition(&currentPosition, nil, nil)
	if err != nil {
		return
	}

	if blind, ok := node.(BlindNode); ok {
		// KLF transmits 0xF7FF for functional parameters when no value is known.
		// Guard against out-of-range values (pyvlx: position.position <= Parameter.MAX).
		if position.Known() && position.Position() <= protocol.ParameterMax {
			blind.SetPositionState(position)
		}
		// House Monitor delivers wrong values for FP3 / orientation — do NOT update
		// orientation here; it is refreshed by the heartbeat StatusRequest (FP3 workaround).
		blind.AfterUpdate()
		return
	}

	if od, ok := node.(OpeningDeviceNode); ok {
		if position.Known() && position.Position() <= protocol.ParameterMax {
			od.SetPositionState(position)
		}
		targetPos, err := protocol.NewPosition(&target, nil, nil)
		if err == nil && targetPos.Known() && targetPos.Position() <= protocol.ParameterMax {
			od.SetTargetState(targetPos)
		}
		od.AfterUpdate()
		return
	}

	if ld, ok := node.(LighteningDeviceNode); ok {
		intensity, err := protocol.NewPosition(&currentPosition, nil, nil)
		if err == nil && intensity.Known() && intensity.Position() <= protocol.ParameterMax {
			ld.SetIntensityState(intensity)
		}
		ld.AfterUpdate()
		return
	}
}

// processNodeStateFrame handles FrameNodeStatePositionChangedNotification.
// It extracts NodeID, CurrentPosition, and Target and delegates to processPositionFrame.
// Ported from the isinstance(frame, FrameNodeStatePositionChangedNotification) branch
// in NodeUpdater.process_frame.
func (u *NodeUpdater) processNodeStateFrame(f *protocol.FrameNodeStatePositionChangedNotification) {
	u.processPositionFrame(uint16(f.NodeID), f.CurrentPosition, f.Target)
}

// processStatusRequestNotification handles FrameStatusRequestNotification.
// For Blind nodes it updates position from MP (NodeParameterMP) and orientation
// from FP3 (NodeParameterFP3). Ported from
// NodeUpdater.process_frame_status_request_notification.
func (u *NodeUpdater) processStatusRequestNotification(f *protocol.FrameStatusRequestNotification) {
	node, ok := u.client.Nodes().ByID(uint16(f.NodeID))
	if !ok {
		return
	}

	blind, isBlind := node.(BlindNode)
	if !isBlind {
		return
	}

	mpRaw, hasMp := f.ParameterData[protocol.NodeParameterMP]
	fp3Raw, hasFp3 := f.ParameterData[protocol.NodeParameterFP3]
	if !hasMp || !hasFp3 {
		return
	}

	position, err := protocol.NewPosition(&mpRaw, nil, nil)
	if err == nil && position.Known() && position.Position() <= protocol.ParameterMax {
		blind.SetPositionState(position)
	}

	orientation, err := protocol.NewPosition(&fp3Raw, nil, nil)
	if err == nil && orientation.Known() && orientation.Position() <= protocol.ParameterMax {
		blind.SetOrientationState(orientation)
	}

	blind.AfterUpdate()
}

// processLimitationStatusNotification handles FrameLimitationStatusNotification.
// Ported from NodeUpdater.process_frame_limitation_status_notification.
// This implementation updates OpeningDeviceNode limitation fields if the concrete
// node type exposes them via the LimitedOpeningDeviceNode interface; otherwise it
// is a no-op (limitation fields belong to the commands/opening-device fan-out phase).
func (u *NodeUpdater) processLimitationStatusNotification(f *protocol.FrameLimitationStatusNotification) {
	node, ok := u.client.Nodes().ByID(uint16(f.NodeID))
	if !ok {
		return
	}
	if lim, ok := node.(LimitedOpeningDeviceNode); ok {
		minPos, errMin := protocol.NewPosition(&protocol.Parameter{Raw: [2]byte{byte(f.MinValue >> 8), byte(f.MinValue & 0xFF)}}, nil, nil)
		maxPos, errMax := protocol.NewPosition(&protocol.Parameter{Raw: [2]byte{byte(f.MaxValue >> 8), byte(f.MaxValue & 0xFF)}}, nil, nil)
		if errMin == nil && errMax == nil {
			lim.SetLimitationState(minPos, maxPos, f.LimitOriginator, f.LimitTime)
		}
		node.AfterUpdate()
	}
}

// LimitedOpeningDeviceNode is an optional interface for opening-device nodes
// that expose limitation state. The node_updater calls it when a
// GW_LIMITATION_STATUS_NTF arrives. The concrete opening-device fan-out phase
// implements this on the nodes that need it.
type LimitedOpeningDeviceNode interface {
	Node
	SetLimitationState(min, max protocol.Position, originator protocol.Originator, limitTime uint8)
}
