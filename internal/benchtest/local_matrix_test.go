package benchtest

import (
	"sync/atomic"
	"testing"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/graph"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func benchmarkNodeCall(b *testing.B) {
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 64})
	if err != nil {
		b.Fatal(err)
	}
	input, ok := pool.Acquire()
	if !ok {
		b.Fatal("input pool exhausted")
	}
	for index := range input.Mutable() {
		input.Mutable()[index] = index + 1
	}
	if !input.SetLen(64) {
		b.Fatal("failed to set input length")
	}
	var calls uint64
	var outputs node.Outputs0
	handler := node.Node1[uint64, int, node.Outputs0, error]{Handle1: func(state *uint64, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
		for range input.Items() {
			*state++
		}
		return node.Result[error]{}
	}}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		handler.Handle1(&calls, input, &outputs)
	}
	b.StopTimer()
	if calls != uint64(b.N)*64 {
		b.Fatalf("processed elements = %d, want %d", calls, uint64(b.N)*64)
	}
	pool.Release(&input)
}

func newLocalEdgeFixture(tb testing.TB) func() {
	tb.Helper()
	type gate struct {
		requested atomic.Uint64
		delivered atomic.Uint64
		valid     atomic.Bool
	}
	type sourceState struct {
		gate    *gate
		emitted uint64
	}
	shared := &gate{}
	shared.valid.Store(true)
	builder := graph.NewBuilder()
	source, err := graph.AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs1[int], error]]{
		Factory: func() (sourceState, error) { return sourceState{gate: shared}, nil },
		Handlers: node.Node0[sourceState, node.Outputs1[int], error]{Handle: func(state *sourceState, outputs *node.Outputs1[int]) node.Result[error] {
			requested := state.gate.requested.Load()
			if requested == state.emitted {
				return node.Result[error]{}
			}
			value, ok := outputs.Out1.Acquire()
			if !ok {
				return node.Result[error]{}
			}
			for index := range value.Mutable() {
				value.Mutable()[index] = int(requested) + index
			}
			value.SetLen(64)
			if !outputs.Out1.Commit(&value) {
				value.Release()
				return node.Result[error]{}
			}
			state.emitted = requested
			return node.Result[error]{}
		}},
	})
	if err != nil {
		tb.Fatal(err)
	}
	sink, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			items := input.Items()
			if len(items) != 64 || items[63]-items[0] != 63 {
				shared.valid.Store(false)
			}
			shared.delivered.Add(1)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := graph.Connect(builder, graph.OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, graph.EdgeConfig[int]{Capacity: 1, BatchCapacity: 64, Policy: queue.Wait}); err != nil {
		tb.Fatal(err)
	}
	if err := graph.BindCPU(builder, graph.DefaultGroupID, 0); err != nil {
		tb.Fatal(err)
	}
	definition, err := builder.Build()
	if err != nil {
		tb.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = executor.Close() })
	request := func() {
		target := shared.requested.Add(1)
		for shared.delivered.Load() < target {
		}
	}
	tb.Cleanup(func() {
		if !shared.valid.Load() {
			tb.Error("local edge did not preserve the logical 64-element batch")
		}
	})
	return request
}

func benchmarkLocalEdge(b *testing.B) {
	request := newLocalEdgeFixture(b)
	request()
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		request()
	}
	b.StopTimer()
}

func TestCompletedNodeCallAllocs(t *testing.T) {
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 64})
	if err != nil {
		t.Fatal(err)
	}
	input, ok := pool.Acquire()
	if !ok {
		t.Fatal("input pool exhausted")
	}
	for index := range input.Mutable() {
		input.Mutable()[index] = index
	}
	if !input.SetLen(64) {
		t.Fatal("failed to set input length")
	}
	var calls uint64
	var outputs node.Outputs0
	handler := node.Node1[uint64, int, node.Outputs0, error]{Handle1: func(state *uint64, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
		for range input.Items() {
			*state++
		}
		return node.Result[error]{}
	}}
	AssertZeroAllocs(t, 1_000, func() { handler.Handle1(&calls, input, &outputs) })
	if calls != 64_064 {
		t.Fatalf("processed elements = %d, want 64064", calls)
	}
	pool.Release(&input)
}

func TestCompletedLocalEdgeAllocs(t *testing.T) {
	request := newLocalEdgeFixture(t)
	request()
	AssertZeroAllocs(t, 100, request)
}
