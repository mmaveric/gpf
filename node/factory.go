package node

// StateFactory creates one explicit state value for one physical node instance.
// A nil error transfers lifecycle ownership of the returned state to that
// future runtime; an error transfers no ownership.
type StateFactory[S any] func() (S, error)

// InitFunc initializes one replica's explicit state. It is optional.
type InitFunc[S any] func(*S) error

// CloseFunc releases one replica's explicit state. It is optional. A future
// runtime must call a provided CloseFunc exactly once after successful factory
// construction, including rollback after a partial InitFunc failure.
type CloseFunc[S any] func(*S) error

// Stateful describes explicit state and handlers for future replica creation.
// Factory is required; Init and Close are optional. Every successful Factory
// call must create fresh mutable state. Captured configuration must remain
// immutable because the framework cannot deep-copy or freeze it.
type Stateful[S, H any] struct {
	Factory  StateFactory[S]
	Handlers H
	Init     InitFunc[S]
	Close    CloseFunc[S]
}

// ClosureReplica is one physical node instance's closure-bound handler set and optional
// lifecycle callbacks. A successful ClosureFactory result transfers exactly one
// lifecycle to the future runtime.
type ClosureReplica[H any] struct {
	Handlers H
	Init     func() error
	Close    func() error
}

// ClosureFactory creates a fresh closure-bound physical instance. A nil error transfers
// lifecycle ownership; a non-nil error transfers none. A future runtime must
// call a provided Close exactly once after every successful result, including
// rollback after a partial Init failure.
type ClosureFactory[H any] func() (ClosureReplica[H], error)
