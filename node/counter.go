package node

// CounterID is an application-owned nonzero numeric counter identity.
type CounterID uint32

// CounterBinder installs one pre-registered update function into fresh state
// during normalization. It is a startup callback, not a data-plane lookup.
type CounterBinder[S any] func(*S, CounterID, func(uint64))

// ClosureCounterBinder installs one pre-registered update function into a fresh
// closure environment during normalization.
type ClosureCounterBinder func(CounterID, func(uint64))
