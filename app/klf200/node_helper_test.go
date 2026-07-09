package klf200

import (
	"testing"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// TestNodeFromInfo_ConcreteTypes verifies that node_helper builds the correct
// concrete node type from a node-type code via the registry. Ported from the
// behaviour of node_helper.convert_frame_to_node.
func TestNodeFromInfo_ConcreteTypes(t *testing.T) {
	client := newTestClient(&fakeAPI{})
	pos, _ := protocol.NewPosition(nil, nil, intp(0))

	cases := []struct {
		name     string
		typeCode protocol.NodeTypeWithSubtype
		want     string // discriminated by a type switch below
	}{
		{"window opener", protocol.NodeTypeWithSubtypeWindowOpener, "Window"},
		{"window w/ rain sensor", protocol.NodeTypeWithSubtypeWindowOpenerWithRainSensor, "Window"},
		{"roller shutter", protocol.NodeTypeWithSubtypeRollerShutter, "RollerShutter"},
		{"dual roller shutter", protocol.NodeTypeWithSubtypeDualRollerShutter, "RollerShutter"},
		{"vertical exterior awning", protocol.NodeTypeWithSubtypeVerticalExteriorAwning, "Awning"},
		{"garage door", protocol.NodeTypeWithSubtypeGarageDoorOpener, "GarageDoor"},
		{"gate opener", protocol.NodeTypeWithSubtypeGateOpener, "Gate"},
		{"blade opener", protocol.NodeTypeWithSubtypeBladeOpener, "Blade"},
		{"exterior venetian blind", protocol.NodeTypeWithSubtypeExteriorVenetianBlind, "Blind"},
		{"louver blind", protocol.NodeTypeWithSubtypeLouverBlind, "Blind"},
		{"on/off switch", protocol.NodeTypeWithSubtypeOnOffSwitch, "OnOffSwitch"},
		{"light", protocol.NodeTypeWithSubtypeLight, "Light"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := NodeInfo{
				NodeType:        tc.typeCode,
				CurrentPosition: pos.Parameter,
				Target:          pos.Parameter,
			}
			node := NodeFromInfo(client, 7, "n", [8]byte{}, info)
			if node == nil {
				t.Fatalf("NodeFromInfo returned nil for %v", tc.typeCode)
			}
			got := concreteTypeName(node)
			if got != tc.want {
				t.Fatalf("type = %q, want %q", got, tc.want)
			}
			if node.NodeID() != 7 {
				t.Fatalf("NodeID = %d, want 7", node.NodeID())
			}
		})
	}
}

// TestNodeFromInfo_UnknownTypeFallback verifies the unknown-type fallback: an
// unregistered node-type code produces a generic node (kept, not dropped) so the
// node is still tracked. Ported from the else/None branch of
// convert_frame_to_node (we keep the node rather than dropping it).
func TestNodeFromInfo_UnknownTypeFallback(t *testing.T) {
	client := newTestClient(&fakeAPI{})

	// Use a node-type value that is (almost certainly) unregistered.
	unknown := protocol.NodeTypeWithSubtype(0xFFFF)
	if _, ok := lookupNodeType(unknown); ok {
		t.Skip("0xFFFF unexpectedly registered; cannot test fallback")
	}

	node := NodeFromInfo(client, 3, "unknown-node", [8]byte{}, NodeInfo{NodeType: unknown})
	if node == nil {
		t.Fatal("NodeFromInfo returned nil for unknown type; want generic fallback node")
	}
	if concreteTypeName(node) != "genericNode" {
		t.Fatalf("fallback type = %q, want genericNode", concreteTypeName(node))
	}
	if node.NodeID() != 3 || node.Name() != "unknown-node" {
		t.Fatalf("fallback node identity = (%d,%q), want (3,%q)", node.NodeID(), node.Name(), "unknown-node")
	}
}

// concreteTypeName returns a short name for the concrete node type behind a Node.
func concreteTypeName(n Node) string {
	switch n.(type) {
	case *Window:
		return "Window"
	case *RollerShutter:
		return "RollerShutter"
	case *Awning:
		return "Awning"
	case *GarageDoor:
		return "GarageDoor"
	case *Gate:
		return "Gate"
	case *Blade:
		return "Blade"
	case *Blind:
		return "Blind"
	case *OnOffSwitch:
		return "OnOffSwitch"
	case *Light:
		return "Light"
	case *genericNode:
		return "genericNode"
	default:
		return "unknown"
	}
}

func intp(v int) *int { return &v }
