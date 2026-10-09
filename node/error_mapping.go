package node

// ErrorEvent is a fixed numeric payload for asynchronous logging. Attributes
// carry application codes, not pointers or formatted strings. Runtime identity
// fields are overwritten by the producing executor before publication.
type ErrorEvent struct {
	NodeID, GroupID, Replica, Worker uint32
	Code                             uint32
	Severity                         uint8
	Attributes                       [4]uint64
}

// ErrorMapper converts a concrete failure value into a fixed logging event.
// When called by a worker it must not block or allocate. Captured configuration
// must remain immutable; the framework neither certifies callbacks nor recovers
// their panics.
type ErrorMapper[E any] func(E) ErrorEvent

// Map converts value without boxing it. A nil mapper returns a zero event.
// Runtime bindings keep mapping dormant until a logging destination is present.
func (m ErrorMapper[E]) Map(value E) ErrorEvent {
	if m == nil {
		return ErrorEvent{}
	}
	return m(value)
}
