package klf200

import (
	"sync"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
)

// NodeConstructor builds a concrete Node from a BaseNode plus the raw node
// information the gateway reported. It is the per-type factory registered in the
// node-type registry, mirroring how frame types register a constructor with the
// protocol frame registry.
//
// info carries the type-specific fields (current position, target, etc.) that a
// concrete node needs at construction; a constructor reads whatever it needs and
// ignores the rest.
type NodeConstructor func(base BaseNode, info NodeInfo) Node

// nodeTypeRegistry maps a combined node-type+subtype value to its constructor.
// Concrete node types register themselves here from init(), decentralised like
// the protocol frame registry (RegisterFrame). It mirrors the big
// type-dispatch in pyvlx's node_helper.convert_frame_to_node.
var (
	nodeTypeRegistryMu sync.RWMutex
	nodeTypeRegistry   = make(map[protocol.NodeTypeWithSubtype]NodeConstructor)
)

// RegisterNodeType registers a constructor for a node type+subtype. It is safe
// to call from init(). Registering the same type code twice panics, to catch
// duplicate registrations at startup (mirroring RegisterFrame).
func RegisterNodeType(typeCode protocol.NodeTypeWithSubtype, constructor NodeConstructor) {
	nodeTypeRegistryMu.Lock()
	defer nodeTypeRegistryMu.Unlock()
	if _, exists := nodeTypeRegistry[typeCode]; exists {
		panic("klf200: duplicate node-type registration for type code")
	}
	nodeTypeRegistry[typeCode] = constructor
}

// lookupNodeType returns the constructor registered for a node type code, or
// (nil, false) if none is registered. Used by node_helper to build the correct
// concrete node, falling back to a generic node for unknown types.
func lookupNodeType(typeCode protocol.NodeTypeWithSubtype) (NodeConstructor, bool) {
	nodeTypeRegistryMu.RLock()
	defer nodeTypeRegistryMu.RUnlock()
	c, ok := nodeTypeRegistry[typeCode]
	return c, ok
}
