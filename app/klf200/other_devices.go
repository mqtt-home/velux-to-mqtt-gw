package klf200

// other_devices.go ports the non-opening node types:
//   - on_off_switch.py  → OnOffSwitch
//   - lightening_device.py → LighteningDevice (base), Light (concrete)
//
// Each type self-registers its KLF200 NodeTypeWithSubtype code in init().
// Command issuing (set_state, set_intensity) is stubbed and returns a
// not-implemented error; the commands fan-out phase fills them in once the
// command-send API call helpers exist.

import (
	"fmt"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// ============================================================
// OnOffSwitch
// ============================================================

// OnOffSwitch represents a KLF200 on/off switch node.
// Ported from on_off_switch.py OnOffSwitch.
type OnOffSwitch struct {
	BaseNode
	parameter protocol.SwitchParameter
}

// AfterUpdate satisfies the Node interface: notifies observers with this node.
func (n *OnOffSwitch) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// String returns a readable representation of this node.
func (n *OnOffSwitch) String() string {
	state := "off"
	if n.parameter.IsOn() {
		state = "on"
	}
	return fmt.Sprintf("<OnOffSwitch name=%q node_id=%d state=%s>",
		n.Name(), n.NodeID(), state)
}

// IsOn reports whether the switch is on.
func (n *OnOffSwitch) IsOn() bool { return n.parameter.IsOn() }

// IsOff reports whether the switch is off.
func (n *OnOffSwitch) IsOff() bool { return n.parameter.IsOff() }

// Parameter returns the current SwitchParameter value.
func (n *OnOffSwitch) Parameter() protocol.SwitchParameter { return n.parameter }

var _ Node = (*OnOffSwitch)(nil)

func init() {
	RegisterNodeType(
		protocol.NodeTypeWithSubtypeOnOffSwitch,
		func(base BaseNode, info NodeInfo) Node {
			return &OnOffSwitch{
				BaseNode:  base,
				parameter: protocol.NewSwitchParameter(nil),
			}
		},
	)
}

// ============================================================
// LighteningDevice (base for dimmable lights)
// ============================================================

// LighteningDevice is the base type for dimmable light nodes. It holds an
// Intensity parameter and satisfies the Node interface via embedding BaseNode.
// Ported from lightening_device.py LighteningDevice.
//
// Note: the pyvlx class does NOT have its own node-type registration — only
// the concrete Light subclass registers a type code. LighteningDevice is kept
// as an embeddable base so the pattern matches the Python inheritance.
type LighteningDevice struct {
	BaseNode
	intensity protocol.Intensity
}

// AfterUpdate satisfies the Node interface.
func (n *LighteningDevice) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// String returns a readable representation.
func (n *LighteningDevice) String() string {
	return fmt.Sprintf("<LighteningDevice name=%q node_id=%d>", n.Name(), n.NodeID())
}

// Intensity returns the current intensity value.
func (n *LighteningDevice) Intensity() protocol.Intensity { return n.intensity }

// SetIntensityState stores a new intensity without issuing a command. It is
// called by the node updater when a house-monitor notification reports a new
// main-parameter value, mirroring pyvlx's node.intensity = intensity
// assignment. It does not fire AfterUpdate; the updater does that afterwards.
func (n *LighteningDevice) SetIntensityState(intensity protocol.Position) {
	n.intensity = protocol.Intensity(intensity)
}

var _ Node = (*LighteningDevice)(nil)

// ============================================================
// Light (concrete dimmable light)
// ============================================================

// Light is a dimmable light node. It embeds LighteningDevice and registers
// the NodeTypeWithSubtypeLight type code.
// Ported from lightening_device.py Light.
type Light struct {
	LighteningDevice
}

// AfterUpdate satisfies the Node interface, notifying observers with this node.
func (n *Light) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// String returns a readable representation. Ported from Light.__str__.
func (n *Light) String() string {
	return fmt.Sprintf("<Light name=%q node_id=%d serial_number=%v>",
		n.Name(), n.NodeID(), n.SerialNumber())
}

var _ Node = (*Light)(nil)

func init() {
	RegisterNodeType(
		protocol.NodeTypeWithSubtypeLight,
		func(base BaseNode, info NodeInfo) Node {
			intensity, _ := protocol.NewIntensity(nil, nil, nil)
			return &Light{
				LighteningDevice: LighteningDevice{
					BaseNode:  base,
					intensity: intensity,
				},
			}
		},
	)
	// NodeTypeWithSubtypeLightOnOff maps to an on/off-only light — use OnOffSwitch
	// semantics but under the Light type hierarchy so both type codes are covered.
	// pyvlx does not define a separate LightOnOff class; it uses the OnOffSwitch
	// registration path for 0x0038 via the same on_off_switch module.
	// We register LightOnOff → OnOffSwitch so the behaviour matches.
	RegisterNodeType(
		protocol.NodeTypeWithSubtypeLightOnOff,
		func(base BaseNode, info NodeInfo) Node {
			return &OnOffSwitch{
				BaseNode:  base,
				parameter: protocol.NewSwitchParameter(nil),
			}
		},
	)
}
