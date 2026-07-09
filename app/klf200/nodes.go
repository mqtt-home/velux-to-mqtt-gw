package klf200

import "sync"

// Nodes is the collection of known nodes, keyed by node id. It is the Go
// counterpart of pyvlx's Nodes container: add/replace by id, look up by id or
// name, iterate, clear. It is safe for concurrent use.
//
// pyvlx's Nodes also carries load()/_load_all_nodes() which issue the
// GetAllNodesInformation API call; that loading logic lives on the Client
// (LoadNodes) in this port, keeping the container itself a pure store.
type Nodes struct {
	mu    sync.RWMutex
	nodes []Node
}

// NewNodes returns an empty Nodes collection. Ported from Nodes.__init__.
func NewNodes() *Nodes {
	return &Nodes{}
}

// Add inserts a node, replacing an existing node with the same node id.
// Ported from Nodes.add.
func (ns *Nodes) Add(node Node) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	for i, existing := range ns.nodes {
		if existing.NodeID() == node.NodeID() {
			ns.nodes[i] = node
			return
		}
	}
	ns.nodes = append(ns.nodes, node)
}

// ByID returns the node with the given id, or (nil, false) if none exists.
// Ported from the integer branch of Nodes.__getitem__.
func (ns *Nodes) ByID(nodeID uint16) (Node, bool) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	for _, node := range ns.nodes {
		if node.NodeID() == nodeID {
			return node, true
		}
	}
	return nil, false
}

// ByName returns the node with the given name, or (nil, false) if none exists.
// Ported from the string branch of Nodes.__getitem__.
func (ns *Nodes) ByName(name string) (Node, bool) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	for _, node := range ns.nodes {
		if node.Name() == name {
			return node, true
		}
	}
	return nil, false
}

// Contains reports whether a node with the given id is present.
// Ported from the integer branch of Nodes.__contains__.
func (ns *Nodes) Contains(nodeID uint16) bool {
	_, ok := ns.ByID(nodeID)
	return ok
}

// Len returns the number of stored nodes. Ported from Nodes.__len__.
func (ns *Nodes) Len() int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return len(ns.nodes)
}

// Clear removes all nodes. Ported from Nodes.clear.
func (ns *Nodes) Clear() {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.nodes = nil
}

// All returns a snapshot slice of the stored nodes for iteration, mirroring
// Nodes.__iter__. The returned slice is a copy, so callers may range over it
// without holding the lock.
func (ns *Nodes) All() []Node {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	out := make([]Node, len(ns.nodes))
	copy(out, ns.nodes)
	return out
}
