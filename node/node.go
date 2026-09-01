package node

import (
	"unsafe"

	"github.com/asharipov/gpf/batch"
)

// OutputSet is the sealed set of supported ordinary output shapes.
// Implementations are generated as Outputs0 through Outputs8.
type OutputSet interface {
	outputSet(unsafe.Pointer, OutputBinder)
	OutputCount() uint8
}

// OutputBinder receives an ordinary output's one-based port and address during
// runtime setup. It is a control-plane hook; batches remain concrete typed values.
type OutputBinder func(uint8, unsafe.Pointer)

// BindOutputs exposes generated ordinary output addresses to the graph runtime.
func BindOutputs[O OutputSet](outputs *O, bind OutputBinder) {
	if outputs == nil || bind == nil {
		return
	}
	var shape O
	shape.outputSet(unsafe.Pointer(outputs), bind)
}

// Result is the typed outcome of a node handler.
type Result[E any] struct {
	Value  E
	Failed bool
}

// Output is one runtime-installed typed ordinary output. Its zero value cannot
// acquire or commit a batch. One output can acquire and commit at most one
// batch during a handler invocation.
type Output[T any] struct {
	acquire   func() (batch.Batch[T], bool)
	acquired  batch.Batch[T]
	staged    batch.Batch[T]
	committed bool
}

// Acquire obtains one batch from this output's edge-owned pool.
func (o *Output[T]) Acquire() (batch.Batch[T], bool) {
	if o == nil || o.acquire == nil || o.acquired.Cap() != 0 || o.committed {
		return batch.Batch[T]{}, false
	}
	b, ok := o.acquire()
	if !ok {
		return batch.Batch[T]{}, false
	}
	o.acquired = b
	return b, true
}

// Commit moves one compatible batch to this output for post-handler
// publication. Invalid or duplicate commits leave the argument owned by the
// caller.
func (o *Output[T]) Commit(b *batch.Batch[T]) bool {
	if o == nil || o.acquire == nil || o.committed || b == nil || b.Cap() == 0 {
		return false
	}
	o.staged = b.Move()
	o.committed = true
	return true
}

// InstallOutput installs the private runtime acquisition hook. It is exported
// solely so graph can bind the monomorphized runtime without a package cycle.
func InstallOutput[T any](o *Output[T], acquire func() (batch.Batch[T], bool)) {
	if o == nil {
		return
	}
	*o = Output[T]{acquire: acquire}
}

// FinalizeOutput releases abandoned or failed batches, or calls publish for a
// successful committed batch. A false result retains a pending batch for retry.
func FinalizeOutput[T any](o *Output[T], failed bool, publish func(batch.Batch[T]) bool) bool {
	if o == nil || o.acquire == nil {
		return true
	}
	if !o.committed {
		o.acquired.Release()
		*o = Output[T]{acquire: o.acquire}
		return true
	}
	if failed {
		o.staged.Release()
		*o = Output[T]{acquire: o.acquire}
		return true
	}
	if !publish(o.staged) {
		return false
	}
	*o = Output[T]{acquire: o.acquire}
	return true
}

// SourceHandler processes a source-like invocation with explicit state.
type SourceHandler[S any, O OutputSet, E any] func(state *S, outputs *O) Result[E]

// InputHandler processes one owned input batch with explicit state. A future
// runtime releases input unless the handler moves it to a compatible output.
type InputHandler[S, I any, O OutputSet, E any] func(
	state *S,
	input batch.Batch[I],
	outputs *O,
) Result[E]

// ReadyMask identifies ready input positions. Bit 1<<PortN represents PortN.
type ReadyMask uint8

// PortIndex is a zero-based input position in a ReadyMask.
type PortIndex uint8

const (
	// Port1 is zero so its readiness maps directly to the least-significant bit.
	Port1 PortIndex = iota
	Port2
	Port3
	Port4
	Port5
	Port6
	Port7
	Port8
)

// Selector chooses one input when multiple positions are ready. It must return
// an in-range position whose bit is set in ready. A future runtime invokes and
// validates selectors.
type Selector[S any] func(state *S, ready ReadyMask) PortIndex

// ClosureSelector chooses one input using state captured by its closure. It has
// the same return obligations as Selector.
type ClosureSelector func(ready ReadyMask) PortIndex
