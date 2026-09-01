package graph

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

var (
	// ErrInvalidDefinition reports a nil, zero-value, or malformed definition.
	ErrInvalidDefinition = errors.New("invalid graph definition")
	// ErrInvalidQueueOps reports a custom queue missing a required callback.
	ErrInvalidQueueOps       = errors.New("invalid graph queue operations")
	errInvalidClosureReplica = errors.New("invalid closure node replica")
)

// Executor owns the workers and resources materialized for one Start. Drain
// and Close are sequential lifecycle operations; concurrent calls are unsupported.
type Executor struct {
	workers     []*executorWorker
	runners     []runner
	drains      []func()
	queuesEmpty []func() bool
	statuses    []AffinityStatus
	sources     atomic.Bool
	stopped     bool
	stopErr     error
}

type workerCommand uint8

const (
	workerPause workerCommand = iota + 1
	workerResume
	workerStop
	workerStart
	workerCall
)

type workerControl struct {
	command  workerCommand
	callback func() error
	response chan error
}

type executorWorker struct {
	group    GroupID
	cpu      int
	runners  []runner
	commands chan workerControl
	ready    chan AffinityStatus
	done     chan struct{}
}

type runner struct {
	group   GroupID
	source  bool
	run     func() bool
	discard func()
	pending func() bool
	init    func() error
	close   func() error
}

// AffinityOutcome identifies the worker's CPU-affinity startup result.
type AffinityOutcome uint8

const (
	AffinityOutcomeApplied AffinityOutcome = iota + 1
	AffinityOutcomeLockedThreadFallback
)

// AffinityStatus is one immutable per-group CPU-affinity startup result.
type AffinityStatus struct {
	GroupID GroupID
	CPU     int
	Outcome AffinityOutcome
	Warning error
}

// AffinityStatus returns an independent ordered snapshot of startup outcomes.
func (e *Executor) AffinityStatus() []AffinityStatus {
	if e == nil || len(e.statuses) == 0 {
		return nil
	}
	return append([]AffinityStatus(nil), e.statuses...)
}

type edgeBindingTarget struct {
	outputs     []edgeOutputBinding
	inputs      []edgeInputBinding
	drains      []func()
	queuesEmpty []func() bool
}

type edgeOutputBinding struct {
	endpoint  endpointID
	configure func(unsafe.Pointer) func(bool) bool
	isError   bool
}

type edgeInputBinding struct {
	endpoint endpointID
	input    func(unsafe.Pointer)
}

type runtimeInput[T any] struct {
	ready func() bool
	pop   func(*batch.Batch[T]) bool
}

type localRing[T any] struct {
	values []batch.Batch[T]
	head   uint32
	tail   uint32
	count  uint32
}

func newLocalRing[T any](capacity uint32) *localRing[T] {
	return &localRing[T]{values: make([]batch.Batch[T], capacity)}
}

func (q *localRing[T]) tryPush(value batch.Batch[T]) bool {
	if q.count == uint32(len(q.values)) {
		return false
	}
	q.values[q.tail] = value
	q.tail = (q.tail + 1) % uint32(len(q.values))
	q.count++
	return true
}

func (q *localRing[T]) tryPop(value *batch.Batch[T]) bool {
	if q.count == 0 || value == nil {
		return false
	}
	*value = q.values[q.head].Move()
	q.head = (q.head + 1) % uint32(len(q.values))
	q.count--
	return true
}

func (q *localRing[T]) len() uint32 { return q.count }

// remoteRing is a bounded typed SPSC queue. Tail publication transfers a
// fully-written slot to the consumer; head publication returns that slot.
type remoteRing[T any] struct {
	values []batch.Batch[T]
	head   atomic.Uint32
	tail   atomic.Uint32
}

func newRemoteRing[T any](capacity uint32) *remoteRing[T] {
	return &remoteRing[T]{values: make([]batch.Batch[T], capacity)}
}

func (q *remoteRing[T]) tryPush(value batch.Batch[T]) bool {
	tail := q.tail.Load()
	if tail-q.head.Load() == uint32(len(q.values)) {
		return false
	}
	q.values[tail%uint32(len(q.values))] = value
	q.tail.Store(tail + 1)
	return true
}

func (q *remoteRing[T]) tryPop(value *batch.Batch[T]) bool {
	if value == nil {
		return false
	}
	head := q.head.Load()
	if q.tail.Load() == head {
		return false
	}
	*value = q.values[head%uint32(len(q.values))].Move()
	q.head.Store(head + 1)
	return true
}

func (q *remoteRing[T]) len() uint32 { return q.tail.Load() - q.head.Load() }

// Start materializes fixed-plan edge resources and node factories before
// publishing exactly one worker per planned group.
func (d *Definition) Start() (*Executor, error) {
	if d == nil || d.owner == nil || len(d.nodes) == 0 || len(d.edgeBindings) != len(d.edges) || len(d.edgeRemote) != len(d.edges) || len(d.placements) != len(d.nodes)+1 || len(d.groups) == 0 {
		return nil, ErrInvalidDefinition
	}
	for index, group := range d.groups {
		if group.id == InvalidGroupID || group.cpu < 0 || (index > 0 && d.groups[index-1].id >= group.id) {
			return nil, ErrInvalidDefinition
		}
	}

	edges := &edgeBindingTarget{}
	for index, bind := range d.edgeBindings {
		if err := bind(edges, d.edgeRemote[index]); err != nil {
			return nil, rollback(nil, edges, nil, err)
		}
	}
	target := normalizationTarget{edges: edges}
	for _, nodeID := range d.order {
		index := int(nodeID) - 1
		if index < 0 || index >= len(d.nodes) || d.nodes[index].nodeID != nodeID || d.placements[nodeID] == InvalidGroupID {
			return nil, rollback(&target, edges, nil, ErrInvalidDefinition)
		}
		target.currentNode = nodeID
		target.lifecycle = lifecycle{}
		target.committed = false
		if err := d.nodes[index].normalizer(&target); err != nil {
			return nil, rollback(&target, edges, nil, err)
		}
		if len(target.runners) == 0 {
			return nil, rollback(&target, edges, nil, ErrInvalidDefinition)
		}
		target.runners[len(target.runners)-1].group = d.placements[nodeID]
		target.committed = true
	}

	e := &Executor{
		drains:      edges.drains,
		queuesEmpty: edges.queuesEmpty,
		runners:     target.runners,
	}
	for _, group := range d.groups {
		worker := &executorWorker{
			group:    group.id,
			cpu:      group.cpu,
			commands: make(chan workerControl),
			ready:    make(chan AffinityStatus, 1),
			done:     make(chan struct{}),
		}
		for _, item := range target.runners {
			if item.group == group.id {
				worker.runners = append(worker.runners, item)
			}
		}
		if len(worker.runners) != len(group.nodes) {
			return nil, rollback(&target, edges, e, ErrInvalidDefinition)
		}
		e.workers = append(e.workers, worker)
		go worker.loop(&e.sources)
		e.statuses = append(e.statuses, <-worker.ready)
	}
	for _, item := range e.runners {
		if item.init == nil {
			continue
		}
		if err := e.call(item.group, item.init); err != nil {
			return nil, rollback(&target, edges, e, err)
		}
	}
	e.sources.Store(true)
	for _, worker := range e.workers {
		e.control(worker, workerStart, nil)
	}
	return e, nil
}

func rollback(target *normalizationTarget, edges *edgeBindingTarget, executor *Executor, trigger error) error {
	if target != nil {
		for _, runner := range target.runners {
			if runner.discard != nil {
				runner.discard()
			}
		}
	}
	for _, drain := range edges.drains {
		drain()
	}
	var cleanup error
	if target != nil {
		if executor != nil {
			cleanup = errors.Join(cleanup, executor.closeLifecycles())
		} else {
			for index := len(target.runners) - 1; index >= 0; index-- {
				if close := target.runners[index].close; close != nil {
					cleanup = errors.Join(cleanup, close())
				}
			}
		}
		if !target.committed && target.lifecycle.close != nil {
			cleanup = errors.Join(cleanup, target.lifecycle.close())
		}
	}
	if executor != nil {
		executor.stopWorkers()
	}
	return errors.Join(trigger, cleanup)
}

// Drain disables sources, drains accepted work through a stable paused snapshot,
// and closes owned lifecycles.
func (e *Executor) Drain() error { return e.stop(true) }

// Close discards queued and pending work from a paused snapshot before closing
// owned lifecycles.
func (e *Executor) Close() error { return e.stop(false) }

func (e *Executor) stop(drain bool) error {
	if e == nil || e.stopped {
		if e == nil {
			return nil
		}
		return e.stopErr
	}
	e.pauseWorkers()
	e.sources.Store(false)
	if drain {
		for !e.empty() {
			e.resumeWorkers()
			e.pauseWorkers()
		}
	} else {
		for _, worker := range e.workers {
			for _, runner := range worker.runners {
				if runner.discard != nil {
					runner.discard()
				}
			}
		}
	}
	for _, discardQueue := range e.drains {
		discardQueue()
	}
	e.stopErr = e.closeLifecycles()
	e.stopWorkers()
	e.stopped = true
	return e.stopErr
}

func (e *Executor) empty() bool {
	for _, worker := range e.workers {
		for _, runner := range worker.runners {
			if runner.pending != nil && runner.pending() {
				return false
			}
		}
	}
	for _, empty := range e.queuesEmpty {
		if !empty() {
			return false
		}
	}
	return true
}

func (e *Executor) pauseWorkers() {
	for _, worker := range e.workers {
		e.control(worker, workerPause, nil)
	}
}

func (e *Executor) resumeWorkers() {
	for _, worker := range e.workers {
		e.control(worker, workerResume, nil)
	}
}

func (e *Executor) stopWorkers() {
	for _, worker := range e.workers {
		e.control(worker, workerStop, nil)
	}
	for _, worker := range e.workers {
		<-worker.done
	}
}

func (w *executorWorker) loop(sources *atomic.Bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(w.done)
	status := AffinityStatus{GroupID: w.group, CPU: w.cpu, Outcome: AffinityOutcomeApplied}
	if err := newAffinityAdapter().bind(w.cpu); err != nil {
		status.Outcome = AffinityOutcomeLockedThreadFallback
		status.Warning = fmt.Errorf("%w: %w", ErrAffinityUnavailable, err)
	}
	w.ready <- status
	for {
		control := <-w.commands
		switch control.command {
		case workerStart:
			w.respond(control, nil)
			goto running
		case workerCall:
			w.respond(control, callWorker(control.callback))
		case workerStop:
			w.respond(control, nil)
			return
		}
	}

running:
	var idle uint32
	for {
		select {
		case control := <-w.commands:
			if w.handle(control) {
				return
			}
		default:
		}
		if w.pass(sources) {
			idle = 0
		} else {
			idle++
			if idle == 1<<20 {
				runtime.Gosched()
				idle = 0
			}
		}
	}
}

func (w *executorWorker) handle(control workerControl) bool {
	switch control.command {
	case workerPause:
		w.respond(control, nil)
		for {
			control := <-w.commands
			switch control.command {
			case workerResume:
				w.respond(control, nil)
				return false
			case workerStop:
				w.respond(control, nil)
				return true
			case workerCall:
				w.respond(control, callWorker(control.callback))
			}
		}
	case workerCall:
		w.respond(control, callWorker(control.callback))
	case workerStop:
		w.respond(control, nil)
		return true
	}
	return false
}

func (w *executorWorker) pass(sources *atomic.Bool) bool {
	progress := false
	for _, runner := range w.runners {
		if runner.source && !sources.Load() {
			continue
		}
		if runner.run() {
			progress = true
		}
	}
	return progress
}

func (e *Executor) closeLifecycles() error {
	var err error
	for index := len(e.runners) - 1; index >= 0; index-- {
		if close := e.runners[index].close; close != nil {
			err = errors.Join(err, e.call(e.runners[index].group, close))
		}
	}
	e.runners = nil
	return err
}

func (e *Executor) call(group GroupID, callback func() error) error {
	for _, worker := range e.workers {
		if worker.group == group {
			return e.control(worker, workerCall, callback)
		}
	}
	return ErrInvalidDefinition
}

func (e *Executor) control(worker *executorWorker, command workerCommand, callback func() error) error {
	response := make(chan error, 1)
	worker.commands <- workerControl{command: command, callback: callback, response: response}
	return <-response
}

func (w *executorWorker) respond(control workerControl, err error) {
	if control.response != nil {
		control.response <- err
	}
}

func callWorker(callback func() error) error {
	if callback == nil {
		return nil
	}
	return callback()
}

func runtimeInputFor[T any](t *normalizationTarget, endpoint endpointID) runtimeInput[T] {
	for _, binding := range t.edges.inputs {
		if binding.endpoint == endpoint {
			var input runtimeInput[T]
			binding.input(unsafe.Pointer(&input))
			return input
		}
	}
	return runtimeInput[T]{}
}

func bindOutputs[O node.OutputSet](t *normalizationTarget, nodeID NodeID, outputs *O) []func(bool) bool {
	if outputs == nil {
		return nil
	}
	finalizers := make([]func(bool) bool, 0, (*outputs).OutputCount())
	node.BindOutputs(outputs, func(portID uint8, output unsafe.Pointer) {
		endpoint := endpointID{nodeID: nodeID, portID: PortID(portID)}
		for _, binding := range t.edges.outputs {
			if binding.isError || binding.endpoint != endpoint {
				continue
			}
			finalizers = append(finalizers, binding.configure(output))
			break
		}
	})
	return finalizers
}

func (t *normalizationTarget) bindError(nodeID NodeID, output unsafe.Pointer) func(bool) bool {
	endpoint := endpointID{nodeID: nodeID, portID: errorPortID}
	for _, binding := range t.edges.outputs {
		if binding.isError && binding.endpoint == endpoint {
			return binding.configure(output)
		}
	}
	return func(bool) bool { return true }
}

func (t *normalizationTarget) finalize(finalizers []func(bool) bool, failed bool) bool {
	complete := true
	for _, finalize := range finalizers {
		if !finalize(failed) {
			complete = false
		}
	}
	return complete
}

func futureBinding[T any](id EdgeID, source, target endpointID, config queue.Config, factory queue.Factory[T], isError bool) edgeBinding {
	return func(bindings *edgeBindingTarget, remote bool) error {
		if config.Capacity == math.MaxUint32 {
			return fmt.Errorf("edge %d: %w", id, ErrInvalidDefinition)
		}
		pool, err := batch.NewPool[T](batch.PoolConfig{Capacity: config.Capacity + 1, BatchCapacity: config.BatchCapacity})
		if err != nil {
			return fmt.Errorf("edge %d pool: %w", id, err)
		}
		var ops queue.QueueOps[T]
		if remote {
			ring := newRemoteRing[T](config.Capacity)
			ops = queue.QueueOps[T]{TryPush: ring.tryPush, TryPop: ring.tryPop, Len: ring.len}
		} else if factory == nil {
			ring := newLocalRing[T](config.Capacity)
			ops = queue.QueueOps[T]{TryPush: ring.tryPush, TryPop: ring.tryPop, Len: ring.len}
		} else {
			ops, err = factory(config)
			if err != nil {
				return fmt.Errorf("edge %d queue factory: %w", id, err)
			}
			if ops.TryPush == nil || ops.TryPop == nil || ops.Len == nil {
				return fmt.Errorf("edge %d: %w", id, ErrInvalidQueueOps)
			}
		}
		bindings.outputs = append(bindings.outputs, edgeOutputBinding{
			endpoint: source,
			isError:  isError,
			configure: func(pointer unsafe.Pointer) func(bool) bool {
				output := (*node.Output[T])(pointer)
				node.InstallOutput(output, pool.Acquire)
				publish := func(value batch.Batch[T]) bool {
					if ops.TryPush(value) {
						return true
					}
					if config.Policy == queue.Drop {
						value.Release()
						return true
					}
					return false
				}
				return func(failed bool) bool {
					return node.FinalizeOutput(output, failed, publish)
				}
			},
		})
		bindings.inputs = append(bindings.inputs, edgeInputBinding{
			endpoint: target,
			input: func(pointer unsafe.Pointer) {
				*(*runtimeInput[T])(pointer) = runtimeInput[T]{ready: func() bool { return ops.Len() != 0 }, pop: ops.TryPop}
			},
		})
		bindings.drains = append(bindings.drains, func() {
			var value batch.Batch[T]
			for ops.TryPop(&value) {
				value.Release()
			}
		})
		bindings.queuesEmpty = append(bindings.queuesEmpty, func() bool { return ops.Len() == 0 })
		return nil
	}
}
