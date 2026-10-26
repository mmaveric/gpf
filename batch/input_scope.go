package batch

// InputScope tracks whether a handler transferred or explicitly released its
// by-value copy of a runner-owned input. A scope belongs to one worker and must
// not be shared between concurrent invocations. Begin must complete before End,
// and the next Begin must wait until End returns.
type InputScope[T any] struct {
	consumed bool
}

// Begin associates input with this invocation and returns a scoped copy for a
// by-value handler. A nil scope, nil input, or empty input produces an empty
// batch. Begin must not be called again until End has completed.
func (s *InputScope[T]) Begin(input *Batch[T]) Batch[T] {
	if s == nil || input == nil || input.slot == nil {
		return Batch[T]{}
	}
	s.consumed = false
	input.scope = s
	return *input
}

// End releases the runner's input if the handler did not move or explicitly
// release its scoped copy. It then clears the runner handle and resets the
// invocation state. A nil scope or input is a no-op.
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
