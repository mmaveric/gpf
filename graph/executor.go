package graph

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
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
	closing      atomic.Bool
	owner        *graphOwner
	ingresses    map[NodeID]unsafe.Pointer
	logger       *runtimeLogger
	participants []stopParticipant
	metrics      *metricStore
	edges        *edgeBindingTarget
	workers      []*executorWorker
	runners      []runner
	drains       []func()
	queuesEmpty  []func() bool
	statuses     []AffinityStatus
	sources      atomic.Bool
	stopped      bool
	stopErr      error
}

// stopParticipant connects runtime-local admission and asynchronous components
// to the executor's single stop budget. Callbacks must return cooperatively.
type stopParticipant struct {
	closeAdmission func(context.Context) error
	drain          func(context.Context) error
	discard        func()
	join           func() error
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
	paused        atomic.Bool
	closing       *atomic.Bool
	logicalGroup  GroupID
	replica       uint32
	logicalNode   NodeID
	nodeWorker    uint32
	isNodeWorker  bool
	metrics       *groupCounters
	timingEnabled bool
	passMetrics   workerPassMetrics
	group         GroupID
	cpu           int
	runners       []runner
	commands      chan workerControl
	ready         chan AffinityStatus
	done          chan struct{}
}

type workerPassMetrics struct {
	passes, idle, measured uint64
	idleNanos, workNanos   uint64
	flushEvery             uint64
	passesSinceFlush       uint64
	idleDirty              bool
	timingDirty            bool
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

// AffinityStatus is one immutable per-physical-group CPU-affinity startup result.
type AffinityStatus struct {
	Replica     uint32
	GroupID     GroupID
	NodeID      NodeID
	NodeReplica uint32
	CPU         int
	Outcome     AffinityOutcome
	Warning     error
}

// AffinityStatus returns an independent physical-worker ordered snapshot of
// startup outcomes, including logical group and zero-based replica identity.
func (e *Executor) AffinityStatus() []AffinityStatus {
	if e == nil || len(e.statuses) == 0 {
		return nil
	}
	return append([]AffinityStatus(nil), e.statuses...)
}

type edgeBindingTarget struct {
	metrics     *metricStore
	discarding  bool
	outputs     []edgeOutputBinding
	inputs      []edgeInputBinding
	drains      []func()
	queuesEmpty []func() bool
	onStop      []func()
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
	values                []batch.Batch[T]
	head                  atomic.Uint64
	tail                  atomic.Uint64
	readIndex, writeIndex uint32
}

func newRemoteRing[T any](capacity uint32) *remoteRing[T] {
	return &remoteRing[T]{values: make([]batch.Batch[T], capacity)}
}

func (q *remoteRing[T]) tryPush(value batch.Batch[T]) bool {
	tail := q.tail.Load()
	if tail-q.head.Load() == uint64(len(q.values)) {
		return false
	}
	q.values[q.writeIndex] = value
	q.writeIndex++
	if q.writeIndex == uint32(len(q.values)) {
		q.writeIndex = 0
	}
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
	*value = q.values[q.readIndex].Move()
	q.readIndex++
	if q.readIndex == uint32(len(q.values)) {
		q.readIndex = 0
	}
	q.head.Store(head + 1)
	return true
}

func (q *remoteRing[T]) len() uint32 {
	used := q.tail.Load() - q.head.Load()
	if used > uint64(len(q.values)) {
		return uint32(len(q.values))
	}
	return uint32(used)
}

func queueOperationsForModel[T any](model QueueModel, config queue.Config, factory queue.Factory[T], descriptors ...queue.Allocator[T]) (queue.QueueOps[T], error) {
	if len(descriptors) != 0 {
		runtime.KeepAlive(descriptors[0])
	}
	if config.Capacity == 0 {
		return queue.QueueOps[T]{}, queue.ErrInvalidCapacity
	}
	if model != QueueModelLocal && factory != nil {
		return queue.QueueOps[T]{}, ErrIncompatibleQueueFactory
	}
	switch model {
	case QueueModelLocal:
		if factory != nil {
			ops, err := factory(config)
			if err != nil {
				return queue.QueueOps[T]{}, err
			}
			if ops.TryPush == nil || ops.TryPop == nil || ops.Len == nil {
				return queue.QueueOps[T]{}, ErrInvalidQueueOps
			}
			return ops, nil
		}
		ring := newLocalRing[T](config.Capacity)
		return queue.QueueOps[T]{TryPush: ring.tryPush, TryPop: ring.tryPop, Len: ring.len}, nil
	case QueueModelSPSC:
		ring := newRemoteRing[T](config.Capacity)
		return queue.QueueOps[T]{TryPush: ring.tryPush, TryPop: ring.tryPop, Len: ring.len}, nil
	case QueueModelMPSC:
		ring, err := queue.NewMPSC[T](config.Capacity)
		if err != nil {
			return queue.QueueOps[T]{}, err
		}
		return ring.Ops(), nil
	case QueueModelSPMC:
		ring, err := queue.NewSPMC[T](config.Capacity)
		if err != nil {
			return queue.QueueOps[T]{}, err
		}
		return ring.Ops(), nil
	case QueueModelMPMC:
		ring, err := queue.NewMPMC[T](config.Capacity)
		if err != nil {
			return queue.QueueOps[T]{}, err
		}
		return ring.Ops(), nil
	default:
		return queue.QueueOps[T]{}, ErrInvalidDefinition
	}
}

// Start materializes fixed-plan edge resources and node factories before
// publishing exactly one worker per planned group.
func (d *Definition) Start() (*Executor, error) {
	if d != nil && d.execution != nil {
		return d.execution.Start()
	}
	if d == nil || d.owner == nil || len(d.nodes) == 0 || len(d.edgeBindings) != len(d.edges) || len(d.edgeRemote) != len(d.edges) || len(d.edgeQueuePlans) != len(d.edges) || len(d.placements) != len(d.nodes)+1 || len(d.groups) == 0 {
		return nil, ErrInvalidDefinition
	}
	for index, group := range d.groups {
		if group.id == InvalidGroupID || group.cpu < 0 || (index > 0 && d.groups[index-1].id >= group.id) {
			return nil, ErrInvalidDefinition
		}
	}

	for index, item := range d.nodes {
		if item.nodeID != NodeID(index+1) {
			return nil, ErrInvalidDefinition
		}
	}
	for index, edge := range d.edges {
		if edge.source.nodeID == 0 || int(edge.source.nodeID) > len(d.nodes) || edge.target.nodeID == 0 || int(edge.target.nodeID) > len(d.nodes) {
			return nil, ErrInvalidDefinition
		}
		plan := d.edgeQueuePlans[index]
		model, err := ResolveQueueModel(plan.ProducerWorkers, plan.ConsumerWorkers, !plan.Remote)
		if err != nil || plan.Model != model || plan.EdgeID != edge.id || edge.id != EdgeID(index+1) || plan.Remote != d.edgeRemote[index] {
			return nil, ErrInvalidDefinition
		}
		if edge.hasFactory && plan.Model != QueueModelLocal {
			return nil, ErrIncompatibleQueueFactory
		}
	}
	metrics := newMetricStore(d)
	edges := &edgeBindingTarget{metrics: metrics}
	for index, bind := range d.edgeBindings {
		plan := d.edgeQueuePlans[index]
		if err := bind(edges, plan); err != nil {
			return nil, rollback(nil, edges, nil, err)
		}
	}
	e := &Executor{owner: d.owner, metrics: metrics, edges: edges, drains: edges.drains, queuesEmpty: edges.queuesEmpty}
	target := normalizationTarget{executor: e, edges: edges, metrics: metrics, sourceAllowed: e.sources.Load}
	if d.logger != nil {
		e.logger = newRuntimeLogger(d.groups, *d.logger, metrics)
		target.eventSink = e.logger.sink
		e.participants = append(e.participants, stopParticipant{drain: e.logger.drain, discard: e.logger.discard, join: e.logger.join})
	}
	for _, nodeID := range d.order {
		index := int(nodeID) - 1
		if index < 0 || index >= len(d.nodes) || d.nodes[index].nodeID != nodeID || d.placements[nodeID] == InvalidGroupID {
			return nil, rollback(&target, edges, nil, ErrInvalidDefinition)
		}
		target.currentNode = nodeID
		target.currentLogicalNode = nodeID
		target.currentGroup = d.placements[nodeID]
		if len(d.nodeInstances) != 0 {
			target.currentReplica = d.nodeInstances[index].replica
			target.currentLogicalNode = d.nodeInstances[index].logical
		}
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

	e.runners = target.runners
	for groupIndex, group := range d.groups {
		worker := &executorWorker{
			metrics:       metrics.groups[groupIndex],
			timingEnabled: metrics.groups[groupIndex].clock != nil && metrics.groups[groupIndex].timingEvery != 0,
			passMetrics: workerPassMetrics{
				flushEvery: metrics.groups[groupIndex].flushEvery,
			},
			group:    group.id,
			cpu:      group.cpu,
			commands: make(chan workerControl),
			ready:    make(chan AffinityStatus, 1),
			done:     make(chan struct{}),
		}
		worker.logicalGroup = group.id
		worker.closing = &e.closing
		if len(d.groupInstances) != 0 {
			worker.logicalGroup = d.groupInstances[groupIndex].logical
			worker.replica = d.groupInstances[groupIndex].replica
		}
		if group.nodePoolWorker && len(group.nodes) == 1 {
			instance := d.nodeInstances[group.nodes[0]-1]
			worker.logicalNode, worker.nodeWorker, worker.isNodeWorker = instance.logical, instance.replica, true
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
	if e.logger != nil {
		go e.logger.loop()
	}
	e.sources.Store(true)
	for _, worker := range e.workers {
		e.control(worker, workerStart, nil)
	}
	return e, nil
}

func rollback(target *normalizationTarget, edges *edgeBindingTarget, executor *Executor, trigger error) error {
	if edges.metrics != nil {
		edges.metrics.live.Store(false)
	}
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
	for _, release := range edges.onStop {
		release()
	}
	edges.onStop = nil
	return errors.Join(trigger, cleanup)
}

// Drain disables sources, drains accepted work through a stable paused snapshot,
// and closes owned lifecycles.
func (e *Executor) Drain() error { return e.DrainContext(context.Background()) }

// DrainContext drains accepted work within one cooperative shutdown budget.
// Cancellation requests forced cleanup at a safe worker barrier; it cannot
// interrupt a user handler, queue callback, or lifecycle callback that does not
// return. ctx must not be nil. Repeated stop calls retain the final error.
func (e *Executor) DrainContext(ctx context.Context) error {
	if ctx == nil {
		panic("graph: nil drain context")
	}
	return e.stop(ctx, true)
}

// Close discards queued and pending work from a paused snapshot before closing
// owned lifecycles.
func (e *Executor) Close() error { return e.stop(context.Background(), false) }

func (e *Executor) stop(ctx context.Context, drain bool) error {
	if e == nil || e.stopped {
		if e == nil {
			return nil
		}
		return e.stopErr
	}
	var stopErr error
	for _, participant := range e.participants {
		if participant.closeAdmission != nil {
			stopErr = errors.Join(stopErr, participant.closeAdmission(ctx))
		}
	}
	e.sources.Store(false)
	if !drain {
		e.closing.Store(true)
	}
	e.pauseWorkers()
	forced := !drain || stopErr != nil
	if drain {
		if err := drainCancellation(ctx); err != nil {
			stopErr = errors.Join(stopErr, err)
			forced = true
		}
		for !forced && !e.empty() {
			e.resumeWorkers()
			e.pauseWorkers()
			if err := drainCancellation(ctx); err != nil {
				stopErr = errors.Join(stopErr, err)
				forced = true
			}
		}
	}
	if forced {
		if e.edges != nil {
			e.edges.discarding = true
		}
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
	if e.metrics != nil {
		for _, edge := range e.metrics.edges {
			edge.depth.Store(0)
		}
	}
	for _, participant := range e.participants {
		if drain && !forced {
			if err := drainCancellation(ctx); err != nil {
				stopErr = errors.Join(stopErr, err)
				forced = true
			}
		}
		if !forced && participant.drain != nil {
			if err := participant.drain(ctx); err != nil {
				stopErr = errors.Join(stopErr, err, drainCancellation(ctx))
				forced = true
			}
			if !forced {
				if err := drainCancellation(ctx); err != nil {
					stopErr = errors.Join(stopErr, err)
					forced = true
				}
			}
		}
		if forced && participant.discard != nil {
			participant.discard()
		}
	}
	stopErr = errors.Join(stopErr, e.closeLifecycles())
	e.stopWorkers()
	if e.edges != nil {
		for _, release := range e.edges.onStop {
			release()
		}
		e.edges.onStop = nil
	}
	for _, participant := range e.participants {
		if participant.join != nil {
			stopErr = errors.Join(stopErr, participant.join())
		}
	}
	e.stopErr = stopErr
	if e.metrics != nil {
		e.metrics.live.Store(false)
	}
	e.stopped = true
	return e.stopErr
}

func drainCancellation(ctx context.Context) error {
	if ctx.Err() == nil {
		return nil
	}
	return errors.Join(ctx.Err(), context.Cause(ctx))
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
		if worker.paused.Load() {
			continue
		}
		e.control(worker, workerPause, nil)
	}
}

func (e *Executor) resumeWorkers() {
	for _, worker := range e.workers {
		if worker.paused.Load() {
			e.control(worker, workerResume, nil)
		}
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
	status := AffinityStatus{GroupID: w.logicalGroup, Replica: w.replica, NodeID: w.logicalNode, NodeReplica: w.nodeWorker, CPU: w.cpu, Outcome: AffinityOutcomeApplied}
	if status.GroupID == 0 {
		status.GroupID = w.group
	}
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
		w.flushPassMetrics()
		w.paused.Store(true)
		w.respond(control, nil)
		for {
			control := <-w.commands
			switch control.command {
			case workerResume:
				w.paused.Store(false)
				w.respond(control, nil)
				return false
			case workerStop:
				w.flushPassMetrics()
				w.respond(control, nil)
				return true
			case workerCall:
				w.respond(control, callWorker(control.callback))
			}
		}
	case workerCall:
		w.respond(control, callWorker(control.callback))
	case workerStop:
		w.flushPassMetrics()
		w.respond(control, nil)
		return true
	}
	return false
}

func (w *executorWorker) pass(sources *atomic.Bool) bool {
	var started uint64
	var measured bool
	if w.timingEnabled {
		started, measured = w.beginPassTiming()
	}
	progress := false
	for i := range w.runners {
		runner := &w.runners[i]
		if runner.source && w.closing != nil && w.closing.Load() {
			continue
		}
		if runner.source && !sources.Load() && (runner.pending == nil || !runner.pending()) {
			continue
		}
		if runner.run() {
			progress = true
		}
	}
	if w.metrics != nil {
		w.recordPassMetrics(progress, started, measured)
	}
	return progress
}

func (w *executorWorker) beginPassTiming() (uint64, bool) {
	m := w.metrics
	if m.clock == nil || m.timingEvery == 0 || w.passMetrics.passes%m.timingEvery != 0 {
		return 0, false
	}
	return m.clock(), true
}

func (w *executorWorker) recordPassMetrics(progress bool, started uint64, measured bool) {
	metrics := w.metrics
	local := &w.passMetrics
	if measured {
		elapsed := metrics.clock() - started
		local.measured++
		local.timingDirty = true
		if progress {
			local.workNanos += elapsed
		} else {
			local.idleNanos += elapsed
		}
	}
	local.passes++
	if !progress {
		local.idle++
		local.idleDirty = true
	}
	local.passesSinceFlush++
	if local.flushEvery == 0 {
		local.flushEvery = metrics.flushEvery
		if local.flushEvery == 0 {
			local.flushEvery = 1
		}
	}
	if local.passesSinceFlush >= local.flushEvery {
		w.flushPassMetrics()
	}
}

func (w *executorWorker) flushPassMetrics() {
	metrics := w.metrics
	local := &w.passMetrics
	if metrics == nil || local.passesSinceFlush == 0 {
		return
	}
	metrics.passes.Store(local.passes)
	if local.idleDirty {
		metrics.idle.Store(local.idle)
		local.idleDirty = false
	}
	if local.timingDirty {
		metrics.measured.Store(local.measured)
		metrics.idleNanos.Store(local.idleNanos)
		metrics.workNanos.Store(local.workNanos)
		local.timingDirty = false
	}
	local.passesSinceFlush = 0
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

func (t *normalizationTarget) errorConnected(nodeID NodeID) bool {
	for _, binding := range t.edges.outputs {
		if binding.isError && binding.endpoint.nodeID == nodeID {
			return true
		}
	}
	return false
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

func futureBinding[T any](id EdgeID, source, target endpointID, config queue.Config, factory queue.Factory[T], isError bool, descriptors ...AllocatorDescriptors[T]) edgeBinding {
	var allocator AllocatorDescriptors[T]
	if len(descriptors) != 0 {
		allocator = descriptors[0]
	}
	return func(bindings *edgeBindingTarget, plan EdgeQueuePlan) error {
		model, err := ResolveQueueModel(plan.ProducerWorkers, plan.ConsumerWorkers, !plan.Remote)
		bindingID := plan.bindingID
		if bindingID == 0 {
			bindingID = plan.EdgeID
		}
		if err != nil || bindingID != id || model != plan.Model {
			return ErrInvalidDefinition
		}
		id := plan.EdgeID
		if factory != nil && plan.Model != QueueModelLocal {
			return ErrIncompatibleQueueFactory
		}
		metric := &edgeCounters{identity: EdgeMetrics{EdgeID: id, QueueCapacity: config.Capacity, ErrorEdge: isError, Remote: plan.Remote, NUMA: config.NUMA}}
		if bindings.metrics != nil {
			metric = bindings.metrics.edges[id-1]
		}
		if config.Capacity == math.MaxUint32 {
			return fmt.Errorf("edge %d: %w", id, ErrInvalidDefinition)
		}
		ops, err := queueOperationsForModel[T](plan.Model, config, factory, allocator.Queue)
		if err != nil {
			return fmt.Errorf("edge %d queue model %s: %w", id, plan.Model, err)
		}
		sources, targets := []endpointID{source}, []endpointID{target}
		if plan.endpoints != nil {
			sources, targets = plan.endpoints.sources, plan.endpoints.targets
		}
		pools := make([]*batch.Pool[T], len(sources))
		for index, source := range sources {
			pool, err := batch.NewPoolWithOptions[T](batch.PoolOptions[T]{Config: batch.PoolConfig{Capacity: config.Capacity + 1, BatchCapacity: config.BatchCapacity, NUMA: config.NUMA, OwnerID: uint32(id)}, Allocator: allocator.Pool})
			if err != nil {
				return fmt.Errorf("edge %d source %d pool: %w", id, source.nodeID, err)
			}
			pools[index] = pool
			if bindings.metrics != nil {
				identity := metric.identity
				identity.SourceNodeID = source.nodeID
				if source.nodeID < NodeID(len(bindings.metrics.nodes)+1) {
					identity.SourceGroupID = bindings.metrics.nodes[source.nodeID-1].identity.GroupID
				}
				sourceReplica := uint32(0)
				if int(source.nodeID) <= len(bindings.metrics.nodes) {
					sourceReplica = bindings.metrics.nodes[source.nodeID-1].identity.Replica
				}
				bindings.metrics.pools = append(bindings.metrics.pools, poolMetricsBinding{identity: PoolMetrics{EdgeID: identity.EdgeID, Replica: identity.Replica, SourceReplica: sourceReplica, SourceNodeID: identity.SourceNodeID, SourceGroupID: identity.SourceGroupID, NUMA: config.NUMA}, read: pool.Metrics})
			}
		}
		for sourceIndex, source := range sources {
			pool := pools[sourceIndex]
			bindings.outputs = append(bindings.outputs, edgeOutputBinding{
				endpoint: source,
				isError:  isError,
				configure: func(pointer unsafe.Pointer) func(bool) bool {
					output := (*node.Output[T])(pointer)
					node.InstallOutput(output, func() (batch.Batch[T], bool) {
						value, ok := pool.Acquire()
						if !ok {
							metric.producer.exhausted.Add(1)
						}
						return value, ok
					})
					pending := false
					publish := func(value batch.Batch[T]) bool {
						length := uint64(value.Len())
						if pending {
							metric.producer.retries.Add(1)
						}
						if ops.TryPush(value) {
							metric.producer.accepted.Add(1)
							metric.producer.elements.Add(length)
							metric.sample(ops.Len())
							pending = false
							return true
						}
						if config.Policy == queue.Drop {
							metric.producer.dropped.Add(1)
							metric.producer.droppedElements.Add(length)
							metric.sample(ops.Len())
							value.Release()
							return true
						}
						pending = true
						metric.sample(ops.Len())
						return false
					}
					return func(failed bool) bool {
						return node.FinalizeOutputObserved(output, failed, publish, func(value batch.Batch[T]) {
							if bindings.discarding {
								metric.discard(value.Len())
							} else {
								metric.producer.failed.Add(1)
								metric.producer.failedElements.Add(uint64(value.Len()))
							}
							pending = false
						})
					}
				},
			})
		}
		for _, target := range targets {
			bindings.inputs = append(bindings.inputs, edgeInputBinding{
				endpoint: target,
				input: func(pointer unsafe.Pointer) {
					*(*runtimeInput[T])(pointer) = runtimeInput[T]{ready: func() bool { return ops.Len() != 0 }, pop: func(value *batch.Batch[T]) bool {
						if !ops.TryPop(value) {
							return false
						}
						metric.consumer.dequeued.Add(1)
						metric.consumer.elements.Add(uint64(value.Len()))
						metric.sample(ops.Len())
						return true
					}}
				},
			})
		}
		bindings.drains = append(bindings.drains, func() {
			var value batch.Batch[T]
			for ops.TryPop(&value) {
				if bindings.discarding {
					metric.discard(value.Len())
				}
				value.Release()
			}
		})
		bindings.queuesEmpty = append(bindings.queuesEmpty, func() bool { return ops.Len() == 0 })
		return nil
	}
}
