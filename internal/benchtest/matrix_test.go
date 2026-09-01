package benchtest

import (
	"sync/atomic"
	"testing"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/graph"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func BenchmarkDataPlane(b *testing.B) {
	b.Run("node-call/batch=64", unavailableBenchmark)
	b.Run("pool-acquire-release/batch=64", func(b *testing.B) {
		benchmarkPoolLifecycle(b, false)
	})
	b.Run("local-edge/batch=64/group=1", unavailableBenchmark)
	b.Run("remote-queue/batch=64/groups=2", benchmarkRemoteDelivery)
	b.Run("local-error/batch=64/groups=1", func(b *testing.B) { benchmarkErrorDelivery(b, false) })
	b.Run("remote-error/batch=64/groups=2", func(b *testing.B) { benchmarkErrorDelivery(b, true) })
	b.Run("end-to-end-dag/batch=64/groups=2/depth=3", benchmarkTwoGroupDAG)
	b.Run("pool-release/clear=false/batch=64", func(b *testing.B) {
		benchmarkPoolLifecycle(b, false)
	})
	b.Run("pool-release/clear=true/batch=64", func(b *testing.B) {
		benchmarkPoolLifecycle(b, true)
	})
}

func benchmarkRemoteDelivery(b *testing.B) {
	type gate struct {
		requested atomic.Uint64
		delivered atomic.Uint64
	}
	type sourceState struct {
		gate    *gate
		emitted uint64
	}
	shared := &gate{}
	builder := graph.NewBuilder()
	source, err := graph.AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs1[int], error]]{
		Factory: func() (sourceState, error) { return sourceState{gate: shared}, nil },
		Handlers: node.Node0[sourceState, node.Outputs1[int], error]{Handle: func(state *sourceState, outputs *node.Outputs1[int]) node.Result[error] {
			value, ok := outputs.Out1.Acquire()
			if !ok {
				return node.Result[error]{}
			}
			requested := state.gate.requested.Add(1)
			value.Mutable()[0] = int(requested)
			value.SetLen(1)
			outputs.Out1.Commit(&value)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	sink, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			shared.delivered.Add(1)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	if err := graph.Place(builder, sink.NodeID(), 2); err != nil {
		b.Fatal(err)
	}
	if _, err := graph.Connect(builder, graph.OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, graph.EdgeConfig[int]{Capacity: 1, BatchCapacity: 64, Policy: queue.Wait}); err != nil {
		b.Fatal(err)
	}
	if err := graph.BindCPU(builder, graph.DefaultGroupID, 0); err != nil {
		b.Fatal(err)
	}
	if err := graph.BindCPU(builder, 2, 1); err != nil {
		b.Fatal(err)
	}
	definition, err := builder.Build()
	if err != nil {
		b.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = executor.Close() })

	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		target := shared.delivered.Load() + 1
		for shared.delivered.Load() < target {
		}
	}
}

func benchmarkTwoGroupDAG(b *testing.B) {
	type gate struct {
		requested atomic.Uint64
		delivered atomic.Uint64
	}
	type sourceState struct {
		gate    *gate
		emitted uint64
	}
	shared := &gate{}
	builder := graph.NewBuilder()
	source, err := graph.AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs1[int], error]]{
		Factory: func() (sourceState, error) { return sourceState{gate: shared}, nil },
		Handlers: node.Node0[sourceState, node.Outputs1[int], error]{Handle: func(state *sourceState, outputs *node.Outputs1[int]) node.Result[error] {
			value, ok := outputs.Out1.Acquire()
			if !ok {
				return node.Result[error]{}
			}
			requested := state.gate.requested.Add(1)
			value.Mutable()[0] = int(requested)
			value.SetLen(1)
			outputs.Out1.Commit(&value)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	transform, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(_ *struct{}, input batch.Batch[int], outputs *node.Outputs1[int]) node.Result[error] {
			value, ok := outputs.Out1.Acquire()
			if !ok {
				return node.Result[error]{Failed: true}
			}
			value.Mutable()[0] = input.Items()[0]
			value.SetLen(1)
			outputs.Out1.Commit(&value)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	sink, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			shared.delivered.Add(1)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	if err := graph.Place(builder, sink.NodeID(), 2); err != nil {
		b.Fatal(err)
	}
	config := graph.EdgeConfig[int]{Capacity: 1, BatchCapacity: 64, Policy: queue.Wait}
	if _, err := graph.Connect(builder, graph.OutputPorts1(source.Outputs()).Out1, transform.Inputs().In1, config); err != nil {
		b.Fatal(err)
	}
	if _, err := graph.Connect(builder, graph.OutputPorts1(transform.Outputs()).Out1, sink.Inputs().In1, config); err != nil {
		b.Fatal(err)
	}
	if err := graph.BindCPU(builder, graph.DefaultGroupID, 0); err != nil {
		b.Fatal(err)
	}
	if err := graph.BindCPU(builder, 2, 1); err != nil {
		b.Fatal(err)
	}
	definition, err := builder.Build()
	if err != nil {
		b.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = executor.Close() })

	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		target := shared.delivered.Load() + 1
		for shared.delivered.Load() < target {
		}
	}
}

func benchmarkErrorDelivery(b *testing.B, remote bool) {
	type gate struct {
		requested atomic.Uint64
		delivered atomic.Uint64
	}
	type sourceState struct {
		gate    *gate
		emitted uint64
	}
	shared := &gate{}
	builder := graph.NewBuilder()
	producer, err := graph.AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs0, int]]{
		Factory: func() (sourceState, error) { return sourceState{gate: shared}, nil },
		Handlers: node.Node0[sourceState, node.Outputs0, int]{Handle: func(state *sourceState, _ *node.Outputs0) node.Result[int] {
			requested := state.gate.requested.Load()
			if requested == state.emitted {
				return node.Result[int]{}
			}
			state.emitted = requested
			return node.Result[int]{Value: int(requested), Failed: true}
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	sink, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, _ batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			shared.delivered.Add(1)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	if remote {
		if err := graph.Place(builder, sink.NodeID(), 2); err != nil {
			b.Fatal(err)
		}
	}
	if _, err := graph.ConnectError(builder, producer.Errors(), sink.Inputs().In1, graph.EdgeConfig[int]{Capacity: 1, BatchCapacity: 64, Policy: queue.Wait}); err != nil {
		b.Fatal(err)
	}
	if err := graph.BindCPU(builder, graph.DefaultGroupID, 0); err != nil {
		b.Fatal(err)
	}
	if remote {
		if err := graph.BindCPU(builder, 2, 1); err != nil {
			b.Fatal(err)
		}
	}
	definition, err := builder.Build()
	if err != nil {
		b.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = executor.Close() })

	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		target := shared.requested.Add(1)
		for shared.delivered.Load() < target {
		}
	}
}

func unavailableBenchmark(b *testing.B) {
	b.ReportAllocs()
	b.Skip("data-plane path is not implemented")
}

func benchmarkPoolLifecycle(b *testing.B, clearOnRelease bool) {
	pool, err := batch.NewPool[int](batch.PoolConfig{
		Capacity:       1,
		BatchCapacity:  64,
		ClearOnRelease: clearOnRelease,
	})
	if err != nil {
		b.Fatalf("NewPool() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		value, ok := pool.Acquire()
		if !ok {
			b.Fatal("Acquire() = unavailable")
		}
		value.Mutable()[0] = iteration
		if !value.SetLen(1) {
			b.Fatal("SetLen(1) = false, want true")
		}
		pool.Release(&value)
	}
}
