package graph

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
)

var poolObservationSink batch.PoolMetrics

func TestRegisteredCountersBindFreshStateAndFreeze(t *testing.T) {
	type state struct{ addA, addB func(uint64) }
	b := NewBuilder()
	first, err := AddNode0(b, node.Stateful[state, node.Node0[state, node.Outputs0, int]]{
		Factory: func() (state, error) { return state{}, nil },
		Handlers: node.Node0[state, node.Outputs0, int]{
			BindCounter: func(s *state, id node.CounterID, add func(uint64)) {
				if id == 2 {
					s.addA = add
				} else {
					s.addB = add
				}
			},
			Handle: func(s *state, _ *node.Outputs0) node.Result[int] { s.addA(1); s.addB(2); return node.Result[int]{} },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := AddClosureNode0(b, func() (node.ClosureReplica[node.ClosureNode0[node.Outputs0, int]], error) {
		var add func(uint64)
		return node.ClosureReplica[node.ClosureNode0[node.Outputs0, int]]{Handlers: node.ClosureNode0[node.Outputs0, int]{
			BindCounter: func(_ node.CounterID, update func(uint64)) { add = update },
			Handle:      func(*node.Outputs0) node.Result[int] { add(1); return node.Result[int]{} },
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, def := range []counterDefinition{{first.NodeID(), 9}, {first.NodeID(), 2}, {second.NodeID(), 2}} {
		if err := RegisterCounter(b, def.NodeID, def.CounterID); err != nil {
			t.Fatal(err)
		}
	}
	if err := RegisterCounter(b, first.NodeID(), 2); !errors.Is(err, ErrCounterRegistered) {
		t.Fatal(err)
	}
	if err := RegisterCounter(b, first.NodeID(), 0); !errors.Is(err, ErrInvalidCounterID) {
		t.Fatal(err)
	}
	if err := RegisterCounter(b, 999, 1); !errors.Is(err, ErrUnknownNode) {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		e, err := d.Start()
		if err != nil {
			t.Fatal(err)
		}
		waitForLogger(t, func() bool { return e.Metrics().Nodes[0].HandlerCalls > 1 })
		add, err := e.Counter(first.NodeID(), 2)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Drain(); err != nil {
			t.Fatal(err)
		}
		got := e.Metrics()
		if len(got.Counters) != 3 || got.Counters[0].CounterID != 2 || got.Counters[1].CounterID != 9 || got.Counters[2].NodeID != second.NodeID() {
			t.Fatalf("counter ordering = %+v", got.Counters)
		}
		if got.Counters[0].Value != got.Nodes[0].HandlerCalls || got.Counters[1].Value != got.Nodes[0].HandlerCalls*2 || got.Counters[2].Value != got.Nodes[1].HandlerCalls {
			t.Fatalf("node/counter values = %+v/%+v", got.Nodes, got.Counters)
		}
		before := got.Counters[0].Value
		add(100)
		got.Counters[0].Value = 123
		if e.Metrics().Counters[0].Value != before {
			t.Fatal("stopped counter handle or snapshot changed final storage")
		}
	}
}

func TestCounterUpdateAllocs(t *testing.T) {
	store := &metricStore{}
	store.live.Store(true)
	row := &userCounter{}
	add := counterAdd(store, row)
	if allocations := testing.AllocsPerRun(100, func() { add(1) }); allocations != 0 {
		t.Fatalf("allocations = %v", allocations)
	}
	if row.value.Load() != 101 {
		t.Fatal("counter update lost increments")
	}
}

func TestWorkerTimingSamplesReportedProgress(t *testing.T) {
	var reads uint64
	counts := &groupCounters{identity: 1, timingEvery: 3, clock: func() uint64 { reads++; return reads * 10 }}
	var progress bool
	w := &executorWorker{metrics: counts, runners: []runner{{run: func() bool { return progress }}}}
	var sources atomic.Bool
	for i := 0; i < 7; i++ {
		progress = i%2 == 0
		w.pass(&sources)
	}
	got := (&Executor{metrics: &metricStore{groups: []*groupCounters{counts}}}).Metrics().Groups[0]
	if reads != 6 || got.Passes != 7 || got.IdlePasses != 3 || got.MeasuredPasses != 3 || got.WorkNanoseconds != 20 || got.IdleNanoseconds != 10 {
		t.Fatalf("worker timing reads=%d metrics=%+v", reads, got)
	}
	counts.timingEvery = 0
	w.pass(&sources)
	if reads != 6 {
		t.Fatal("disabled worker timing read the clock")
	}
}

func TestWorkerTimingUpdateAllocs(t *testing.T) {
	var ticks uint64
	w := &executorWorker{metrics: &groupCounters{timingEvery: 1, clock: func() uint64 { ticks++; return ticks }}}
	var sources atomic.Bool
	if allocations := testing.AllocsPerRun(100, func() { w.pass(&sources) }); allocations != 0 {
		t.Fatalf("allocations = %v", allocations)
	}
}

func TestGraphPoolSnapshotsUseOriginIdentity(t *testing.T) {
	bindings, _, output, finalize, input := metricEdge(t, 0, nil)
	commitMetricBatch(t, output, 1)
	finalize(false)
	got := (&Executor{metrics: bindings.metrics}).Metrics().Pools[0]
	if got.EdgeID != 1 || got.Capacity != 2 || got.InUseSample != 1 || got.HighWaterSample != 1 {
		t.Fatalf("graph pool metrics = %+v", got)
	}
	// Pop into a handle and release at the normal consumer ownership boundary.
	value, ok := output.Acquire()
	if !ok {
		t.Fatal("second slot unavailable")
	}
	value.Release()
	finalize(false)
	bindings.drains[0]()
	if input.ready() {
		t.Fatal("queue did not drain")
	}
	if (&Executor{metrics: bindings.metrics}).Metrics().Pools[0].InUseSample != 0 {
		t.Fatal("origin pool still in use after release")
	}
}

func BenchmarkExtendedMetrics(b *testing.B) {
	b.Run("counter-update", func(b *testing.B) {
		store := &metricStore{}
		store.live.Store(true)
		add := counterAdd(store, &userCounter{})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			add(1)
		}
	})
	b.Run("pool-observation", func(b *testing.B) {
		pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 2, BatchCapacity: 1})
		if err != nil {
			b.Fatal(err)
		}
		var got batch.PoolMetrics
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			got = pool.Metrics()
			poolObservationSink = got
		}
		b.StopTimer()
		if got.Capacity != 2 {
			b.Fatal("pool observation lost capacity")
		}
	})
	b.Run("worker-sampled-64", func(b *testing.B) {
		w := &executorWorker{metrics: &groupCounters{timingEvery: 64, clock: monotonicClock()}}
		var sources atomic.Bool
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.pass(&sources)
		}
	})
}
