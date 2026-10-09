// Package numa defines portable NUMA placement metadata.
//
// The graph, batch, and queue packages currently retain these values only as
// inactive annotations. They do not discover topology or allocate local memory.
package numa

import "errors"

var ErrInvalidPlacement = errors.New("invalid NUMA placement metadata")

// NodeID identifies an application-declared NUMA node. Zero is a valid node ID
// only when explicitly selected with Present=true.
type NodeID uint32

// Placement distinguishes unspecified zero values from an explicit node zero.
type Placement struct {
	Node    NodeID
	Present bool
}

// On returns an explicit NUMA node annotation, including node zero.
func On(node NodeID) Placement { return Placement{Node: node, Present: true} }

// Validate rejects a nonzero ID marked absent. It does not consult the host.
func (p Placement) Validate() error {
	if !p.Present && p.Node != 0 {
		return ErrInvalidPlacement
	}
	return nil
}
