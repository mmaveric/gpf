package batch

// InputScope tracks whether a handler has transferred or explicitly released
// its by-value copy of a runner-owned input. A scope is owned by one worker and
// must not be shared between concurrent invocations.
type InputScope[T any] struct {
	consumed bool
}

// Begin associates the runner's input handle with this invocation and returns
// a scoped copy for a by-value handler. Begin must not be called again until
// End has completed.
func (s *InputScope[T]) Begin(input *Batch[T]) Batch[T] {
	if s == nil || input == nil || input.slot == nil {
		return Batch[T]{}
	}
	s.consumed = false
	input.scope = s
	return *input
}

// End releases the runner's input if the handler did not move or explicitly
// release its copy, then clears the runner handle and invocation association.
func (s *InputScope[T]) End(input *Batch[T]) {
	if s == nil || input == nil {
		return
	}
	if !s.consumed {
		input.Release()
	}
	*input = Batch[T]{}
	s.consumed = false
}
