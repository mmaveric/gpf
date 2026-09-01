// Package node defines generated fixed-arity typed node contracts.
//
// Outputs0 through Outputs8 are the sealed ordinary output shapes. Outputs1
// through Outputs8 contain typed Out1 through Out8 Output handles. A handler
// acquires and commits at most one batch through each Output; uncommitted
// acquisitions are released after the handler returns. Node0
// through Node8 use explicit state, while ClosureNode0 through ClosureNode8 use
// callbacks that may capture state. Every input has a separate typed handler.
// Multi-input selectors address zero-based positions in ReadyMask and must
// choose a ready position when invoked.
//
// Ordinary output arity is limited to eight. Result retains a separate error
// type that graph exposes as an optional ErrorPort. A failed result causes graph
// to discard ordinary output and route its typed Value only through ConnectError.
//
// Stateful and ClosureFactory describe future replica-local construction. Their
// factories must create fresh mutable state for each start, while captured
// configuration remains immutable. Graph registration records only static
// factory and handler facts; Build does not invoke factories. Fixed manual
// placement can run handlers in one group worker and move ordinary batches to
// another through graph's typed SPSC boundary; replicas remain unavailable.
package node
