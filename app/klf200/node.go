package klf200

import (
	"context"
	"fmt"
)

// DeviceUpdatedCallback is invoked after a node's internal state changes. It is
// the Go equivalent of the callables registered with pyvlx's
// Node.register_device_updated_cb (called from Node.after_update).
//
// The callback receives the node whose state changed. It is invoked from
// whatever goroutine performed the update (typically the live-update / read
// loop); callbacks should be quick and must not block the caller.
type DeviceUpdatedCallback func(node Node)

// Node is the interface implemented by every device abstraction (windows,
// roller shutters, blinds, lights, switches, ...). It mirrors pyvlx's Node base
// class: every node has an id, a name and a serial number, and can notify
// registered observers when its state changes.
//
// Concrete node types embed BaseNode, which supplies the identity accessors and
// the callback machinery, and add their own type-specific state and behaviour
// in the fan-out phases.
type Node interface {
	// NodeID returns the KLF200 system-table index of this node.
	NodeID() uint16
	// Name returns the node's human-readable name.
	Name() string
	// SerialNumber returns the node's serial number as an 8-byte value; an
	// all-zero value means "no serial".
	SerialNumber() [8]byte

	// RegisterDeviceUpdatedCB registers a callback invoked after the node's
	// state changes. Ported from Node.register_device_updated_cb.
	RegisterDeviceUpdatedCB(cb DeviceUpdatedCallback)

	// AfterUpdate notifies all registered callbacks that the node's state has
	// changed. Ported from Node.after_update.
	AfterUpdate()

	// String returns a readable representation of the node.
	String() string
}

// BaseNode holds the identity and observer machinery common to every node. It
// is embedded (by value) into every concrete node type and supplies the shared
// portion of the Node interface. It is the Go counterpart of pyvlx's Node base
// class.
//
// The zero value is not usable; construct one with NewBaseNode so the client
// reference is set.
type BaseNode struct {
	// client is the owning Client, needed by node methods (e.g. rename) that
	// issue API calls. It corresponds to pyvlx's Node.pyvlx back-reference.
	client *Client

	nodeID       uint16
	name         string
	serialNumber [8]byte

	deviceUpdatedCBs []DeviceUpdatedCallback
}

// NewBaseNode constructs a BaseNode. node_helper builds this from a node's raw
// information and passes it to the type-specific constructor registered in the
// node-type registry. Ported from Node.__init__.
func NewBaseNode(client *Client, nodeID uint16, name string, serialNumber [8]byte) BaseNode {
	return BaseNode{
		client:       client,
		nodeID:       nodeID,
		name:         name,
		serialNumber: serialNumber,
	}
}

// Client returns the owning client, for use by concrete node methods that need
// to issue API calls.
func (n *BaseNode) Client() *Client { return n.client }

// NodeID returns the node's system-table index.
func (n *BaseNode) NodeID() uint16 { return n.nodeID }

// Name returns the node's name.
func (n *BaseNode) Name() string { return n.name }

// SetName updates the node's cached name. Used after a successful rename.
func (n *BaseNode) SetName(name string) { n.name = name }

// SerialNumber returns the node's serial number.
func (n *BaseNode) SerialNumber() [8]byte { return n.serialNumber }

// RegisterDeviceUpdatedCB registers a state-change callback.
// Ported from Node.register_device_updated_cb.
func (n *BaseNode) RegisterDeviceUpdatedCB(cb DeviceUpdatedCallback) {
	n.deviceUpdatedCBs = append(n.deviceUpdatedCBs, cb)
}

// AfterUpdate invokes every registered callback with the given node. Concrete
// node types call n.AfterUpdate(self) from their update methods. It is the Go
// equivalent of Node.after_update.
//
// The self parameter is the fully-typed node value to hand to observers; a
// concrete type passes itself so callbacks receive the real node, not the
// embedded BaseNode.
func (n *BaseNode) AfterUpdate(self Node) {
	for _, cb := range n.deviceUpdatedCBs {
		cb(self)
	}
}

// String returns a readable representation. Ported from Node.__str__.
func (n *BaseNode) String() string {
	return fmt.Sprintf("<name=%q node_id=%d serial_number=%v>", n.name, n.nodeID, n.serialNumber)
}

// Rename changes the node's name on the gateway and updates the cached value.
// Ported from Node.rename. The concrete SetNodeName API call is issued in the
// commands fan-out phase; this seam is left for it.
//
// TODO(fan-out): issue GW_SET_NODE_NAME_REQ via the client and check the
// confirmation status before updating the cached name.
func (n *BaseNode) Rename(ctx context.Context, name string) error {
	_ = ctx
	_ = name
	return fmt.Errorf("klf200: Node.Rename not yet implemented")
}
