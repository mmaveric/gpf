// Package numa defines portable NUMA placement metadata.
//
// The graph, batch, and queue packages currently retain these values only as
// inactive annotations. They do not discover topology or allocate local memory.
package numa

import "errors"

// ErrInvalidPlacement indicates that placement metadata violates its
// representation constraints. It does not indicate an unavailable host NUMA
// node, because this package does not inspect host topology.
var ErrInvalidPlacement = errors.New("invalid NUMA placement metadata")

// NodeID is an application-declared NUMA node identifier used as metadata only.
// This package does not verify that an identifier exists on the host or apply
// memory or execution placement based on it.
type NodeID uint32

// Placement records an optional NUMA node annotation; it does not cause NUMA
// discovery, memory allocation, or execution placement. Its zero value means
// no node is specified. Validate requires Node to be zero when Present is
// false; when Present is true, any NodeID value, including zero, is accepted.
type Placement struct {
	// Node is the annotated node identifier. It is meaningful when Present is true.
	Node NodeID
	// Present distinguishes an explicit Node value (including zero) from no annotation.
	Present bool
}

// On returns metadata marking node as explicitly selected, including node zero.
// It does not check host topology or apply placement.
func On(node NodeID) Placement { return Placement{Node: node, Present: true} }

// Validate checks the representation constraint that an absent placement must
// have Node zero. It accepts any NodeID when Present is true and does not
// consult host topology or apply placement. An invalid value returns
// ErrInvalidPlacement.
func (p Placement) Validate() error {
	if !p.Present && p.Node != 0 {
		return ErrInvalidPlacement
	}
	return nil
}
