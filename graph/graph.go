package graph

import "errors"

// NodeID identifies a node within a graph definition.
type NodeID uint32

// PortID identifies a port within a node definition.
type PortID uint8

// GroupID identifies an execution group in a future runtime.
type GroupID uint32

// Definition is an immutable graph produced by a future Builder.
type Definition struct{}

// Builder collects graph metadata before producing an immutable Definition.
// Its operations are not implemented in this contract-only stage.
type Builder struct{}

// InPort is a typed input-port handle. Its zero value is a declaration-only
// handle and cannot receive work until graph construction is implemented.
type InPort[T any] struct {
	_ [0]func(T)
}

// OutPort is a typed output-port handle. Its zero value is a declaration-only
// handle and cannot publish work until graph construction is implemented.
type OutPort[T any] struct {
	_ [0]func(T)
}

// EdgeConfig configures a future graph edge.
type EdgeConfig struct {
	QueueCapacity uint32
	WaitForSpace  bool
}

// ErrUnavailable reports a contract operation whose executable implementation
// belongs to a later framework stage.
var ErrUnavailable = errors.New("graph operation is not implemented")

// Connect declares a statically typed edge between matching ports.
//
// The type parameter prevents connecting incompatible port element types. This
// contract-only implementation returns ErrUnavailable and performs no graph
// mutation.
func Connect[T any](
	definition *Definition,
	output OutPort[T],
	input InPort[T],
	config EdgeConfig,
) error {
	return ErrUnavailable
}
