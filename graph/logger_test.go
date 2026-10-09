package graph

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func waitForLogger(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatal("logger condition did not become ready")
		}
		runtime.Gosched()
	}
}

func TestLoggerConfigurationValidation(t *testing.T) {
	if err := SetLogger(nil, LoggerConfig{}); !errors.Is(err, ErrInvalidBuilder) {
		t.Fatal(err)
	}
	b := NewBuilder()
	if err := SetLogger(b, LoggerConfig{Capacity: 1}); !errors.Is(err, ErrInvalidLogger) {
		t.Fatal(err)
	}
	if err := SetLogger(b, LoggerConfig{Handle: func(node.ErrorEvent) {}}); !errors.Is(err, ErrInvalidLogger) {
		t.Fatal(err)
	}
	if err := SetLogger(b, LoggerConfig{Capacity: 1, Handle: func(node.ErrorEvent) {}}); err != nil {
		t.Fatal(err)
	}
	if err := SetLogger(b, LoggerConfig{Capacity: 1, Handle: func(node.ErrorEvent) {}}); !errors.Is(err, ErrLoggerConfigured) {
		t.Fatal(err)
	}
}

func TestAsyncLoggerMapsUnconnectedFailuresAndDrains(t *testing.T) {
	b := NewBuilder()
	var mapped, delivered atomic.Uint64
	var code atomic.Uint32
	p, err := AddNode0(b, node.Stateful[bool, node.Node0[bool, node.Outputs0, int]]{
		Factory: func() (bool, error) { return false, nil },
		Handlers: node.Node0[bool, node.Outputs0, int]{
			MapError: func(value int) node.ErrorEvent { mapped.Add(1); return node.ErrorEvent{Code: uint32(value)} },
			Handle: func(emitted *bool, _ *node.Outputs0) node.Result[int] {
				if *emitted {
					return node.Result[int]{}
				}
				*emitted = true
				return node.Result[int]{Value: 37, Failed: true}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := SetLogger(b, LoggerConfig{Capacity: 2, Handle: func(event node.ErrorEvent) {
		if event.NodeID != uint32(p.NodeID()) || event.GroupID != uint32(DefaultGroupID) {
			t.Error("logger lost runtime identity")
		}
		code.Store(event.Code)
		delivered.Add(1)
	}}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if mapped.Load() != 0 || delivered.Load() != 0 {
		t.Fatal("Build called logging callbacks")
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	waitForLogger(t, func() bool { return delivered.Load() == 1 })
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	got := e.Metrics()
	if mapped.Load() != 1 || code.Load() != 37 || got.Nodes[0].UnconnectedErrors != 1 || len(got.Loggers) != 1 {
		t.Fatalf("mapping/counts = %d/%d %+v", mapped.Load(), code.Load(), got)
	}
	if got.Loggers[0].AttemptedEvents != 1 || got.Loggers[0].DeliveredEvents != 1 || got.Loggers[0].ShutdownDiscardedEvents != 0 {
		t.Fatalf("log counters = %+v", got.Loggers[0])
	}
	got.Loggers[0].AttemptedEvents = 99
	if e.Metrics().Loggers[0].AttemptedEvents != 1 {
		t.Fatal("logger snapshot aliases storage")
	}
}

func TestAsyncLoggerSaturationDoesNotBlockDataWorker(t *testing.T) {
	b := NewBuilder()
	started := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	_, err := AddNode0(b, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, int]]{
		Factory:  func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs0, int]{Handle: func(*struct{}, *node.Outputs0) node.Result[int] { return node.Result[int]{Value: 9, Failed: true} }},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := SetLogger(b, LoggerConfig{Capacity: 1, Handle: func(node.ErrorEvent) { first.Do(func() { close(started); <-release }) }}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); _ = e.Close() })
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("logger did not start")
	}
	waitForLogger(t, func() bool { return e.Metrics().Loggers[0].QueueDroppedEvents > 0 })
	done := make(chan error, 1)
	go func() { done <- e.Close() }()
	waitForLogger(t, func() bool { return len(e.logger.commands) == 1 })
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not join returning logger")
	}
	got := e.Metrics()
	logs := got.Loggers[0]
	if logs.AttemptedEvents != got.Nodes[0].FailedResults || logs.AcceptedEvents != logs.DeliveredEvents+logs.ShutdownDiscardedEvents || logs.QueueDroppedEvents == 0 {
		t.Fatalf("saturated logger counts = %+v node=%+v", logs, got.Nodes[0])
	}
	select {
	case <-e.logger.done:
	default:
		t.Fatal("logger was not joined")
	}
}

func TestAsyncLoggerCancellationDiscardsFlushRemainder(t *testing.T) {
	b := NewBuilder()
	var inCallback atomic.Bool
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	_, err := AddNode0(b, node.Stateful[int, node.Node0[int, node.Outputs0, int]]{
		Factory: func() (int, error) { return 0, nil },
		Handlers: node.Node0[int, node.Outputs0, int]{Handle: func(emitted *int, _ *node.Outputs0) node.Result[int] {
			if *emitted == 2 || (*emitted == 1 && !inCallback.Load()) {
				return node.Result[int]{}
			}
			*emitted++
			return node.Result[int]{Value: *emitted, Failed: true}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := SetLogger(b, LoggerConfig{Capacity: 1, Handle: func(node.ErrorEvent) {
		if inCallback.CompareAndSwap(false, true) {
			<-release
		}
	}}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); _ = e.Close() })
	waitForLogger(t, func() bool { return e.Metrics().Loggers[0].AcceptedEvents == 2 })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- e.DrainContext(ctx) }()
	waitForLogger(t, func() bool { return len(e.logger.commands) == 1 })
	cancel()
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("drain cancellation = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("logger flush did not finish")
	}
	got := e.Metrics().Loggers[0]
	if got.AcceptedEvents != 2 || got.DeliveredEvents != 1 || got.ShutdownDiscardedEvents != 1 {
		t.Fatalf("flush outcome = %+v", got)
	}
}

func TestEventRingAndMappedPublicationAllocs(t *testing.T) {
	metrics := &metricStore{}
	l := newRuntimeLogger([]groupPlan{{id: 1}}, LoggerConfig{Capacity: 1, Handle: func(node.ErrorEvent) {}}, metrics)
	target := &normalizationTarget{currentGroup: 1, eventSink: l.sink}
	mapError := bindErrorMapper(target, 1, node.ErrorMapper[int](func(value int) node.ErrorEvent { return node.ErrorEvent{Code: uint32(value)} }))
	var event node.ErrorEvent
	if allocations := testing.AllocsPerRun(100, func() {
		mapError(11)
		mapError(12) // full ring drops once without blocking
		if !l.rings[0].pop(&event) {
			panic("event missing")
		}
	}); allocations != 0 {
		t.Fatalf("allocations = %v", allocations)
	}
	if event.Code != 11 || metrics.loggers[0].dropped.Load() != 101 {
		t.Fatal("event/full-ring counts incorrect")
	}
}

func TestLoggerPublicationIndependentOfErrorDropAndExhaustion(t *testing.T) {
	store := &metricStore{}
	l := newRuntimeLogger([]groupPlan{{id: 1}}, LoggerConfig{Capacity: 8, Handle: func(node.ErrorEvent) {}}, store)
	edges := &edgeBindingTarget{}
	plan := EdgeQueuePlan{EdgeID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}
	if err := futureBinding[int](1, endpointID{1, errorPortID}, endpointID{2, 1}, queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}, nil, true)(edges, plan); err != nil {
		t.Fatal(err)
	}
	target := &normalizationTarget{edges: edges, currentGroup: 1, eventSink: l.sink}
	var mapped int
	definition := node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, int]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs0, int]{
			MapError: func(value int) node.ErrorEvent { mapped++; return node.ErrorEvent{Code: uint32(value)} },
			Handle:   func(*struct{}, *node.Outputs0) node.Result[int] { return node.Result[int]{Value: 7, Failed: true} },
		},
	}
	if err := normalizeExplicit0(target, 1, definition); err != nil {
		t.Fatal(err)
	}
	var input runtimeInput[int]
	edges.inputs[0].input(unsafe.Pointer(&input))
	target.runners[0].run() // accepted error batch
	target.runners[0].run() // full Drop queue
	var held batch.Batch[int]
	if !input.pop(&held) {
		t.Fatal("accepted error missing")
	}
	target.runners[0].run() // accepted into the freed queue slot
	target.runners[0].run() // both source slots are now owned; acquisition fails
	var event node.ErrorEvent
	var count int
	for l.rings[0].pop(&event) {
		count++
		if event.Code != 7 {
			t.Fatal("mapped payload changed")
		}
	}
	if mapped != 4 || count != 4 || store.loggers[0].accepted.Load() != 4 {
		t.Fatalf("mapping/event counts = %d/%d", mapped, count)
	}
	held.Release()
	edges.drains[0]()
}

func TestLoggerEventRingCounterWrap(t *testing.T) {
	r := &eventRing{values: make([]node.ErrorEvent, 3), counts: &loggerCounters{}}
	r.head.Store(^uint64(0) - 1)
	r.tail.Store(^uint64(0) - 1)
	for i := uint32(0); i < 3; i++ {
		if !r.push(node.ErrorEvent{Code: i + 1}) {
			t.Fatal("push across wrap failed")
		}
	}
	var event node.ErrorEvent
	for i := uint32(0); i < 3; i++ {
		if !r.pop(&event) || event.Code != i+1 {
			t.Fatal("event contents lost across wrap")
		}
	}
	if r.pop(&event) {
		t.Fatal("wrapped ring should be empty")
	}
}

func TestLoggerSequentialStartsAndStartupRollback(t *testing.T) {
	b := NewBuilder()
	var delivered atomic.Uint64
	_, err := AddNode0(b, node.Stateful[bool, node.Node0[bool, node.Outputs0, int]]{
		Factory: func() (bool, error) { return false, nil },
		Handlers: node.Node0[bool, node.Outputs0, int]{Handle: func(emitted *bool, _ *node.Outputs0) node.Result[int] {
			if *emitted {
				return node.Result[int]{}
			}
			*emitted = true
			return node.Result[int]{Failed: true}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := SetLogger(b, LoggerConfig{Capacity: 2, Handle: func(node.ErrorEvent) { delivered.Add(1) }}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	var previous *Executor
	for i := uint64(1); i <= 2; i++ {
		e, err := d.Start()
		if err != nil {
			t.Fatal(err)
		}
		waitForLogger(t, func() bool { return delivered.Load() == i })
		if err := e.Drain(); err != nil {
			t.Fatal(err)
		}
		if got := e.Metrics().Loggers[0]; got.AcceptedEvents != 1 || got.DeliveredEvents != 1 {
			t.Fatalf("fresh start counters = %+v", got)
		}
		if previous != nil && previous.Metrics().Loggers[0].AcceptedEvents != 1 {
			t.Fatal("restart mutated prior counters")
		}
		previous = e
	}
	failing := NewBuilder()
	trigger := errors.New("init failed")
	var closed int
	_, err = AddNode0(failing, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, int]]{
		Factory:  func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs0, int]{Handle: func(*struct{}, *node.Outputs0) node.Result[int] { return node.Result[int]{} }},
		Init:     func(*struct{}) error { return trigger },
		Close:    func(*struct{}) error { closed++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := SetLogger(failing, LoggerConfig{Capacity: 2, Handle: func(node.ErrorEvent) { t.Error("logger ran before successful startup") }}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, failing, DefaultGroupID, 0)
	bad, err := failing.Build()
	if err != nil {
		t.Fatal(err)
	}
	if e, err := bad.Start(); e != nil || !errors.Is(err, trigger) || closed != 1 {
		t.Fatalf("rollback = %v/%v close=%d", e, err, closed)
	}
}

func TestLoggerPollsRingsRoundRobin(t *testing.T) {
	var codes []uint32
	l := newRuntimeLogger([]groupPlan{{id: 1}, {id: 3}}, LoggerConfig{Capacity: 2, Handle: func(event node.ErrorEvent) { codes = append(codes, event.Code) }}, &metricStore{})
	l.rings[0].push(node.ErrorEvent{Code: 1})
	l.rings[0].push(node.ErrorEvent{Code: 2})
	l.rings[1].push(node.ErrorEvent{Code: 3})
	for l.pass() {
	}
	if len(codes) != 3 || codes[0] != 1 || codes[1] != 3 || codes[2] != 2 {
		t.Fatalf("polling order = %v", codes)
	}
}
