package graph

import (
	"errors"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func TestNodeWorkerPoolDistributesEachBatchOnce(t *testing.T) {
	b := NewBuilder()
	ingress, err := AddIngress[int](b, IngressConfig{Capacity: 64, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		add    func(uint64)
		closed *atomic.Uint64
	}
	var closes atomic.Uint64
	worker, err := AddNode1(b, node.Stateful[state, node.Node1[state, int, node.Outputs1[int], error]]{
		Factory: func() (state, error) { return state{closed: &closes}, nil },
		Handlers: node.Node1[state, int, node.Outputs1[int], error]{BindCounter: func(s *state, _ node.CounterID, add func(uint64)) { s.add = add }, Handle1: func(s *state, input batch.Batch[int], out *node.Outputs1[int]) node.Result[error] {
			s.add(1)
			out.Out1.Commit(&input)
			return node.Result[error]{}
		}},
		Close: func(s *state) error { s.closed.Add(1); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	var delivered atomic.Uint64
	var seen [96]atomic.Uint32
	sink, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			value := input.Items()[0]
			if value < 0 || value >= len(seen) || !seen[value].CompareAndSwap(0, 1) {
				t.Errorf("batch duplicated or invalid: %d", value)
			}
			delivered.Add(1)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, ingress.Output(), worker.Inputs().In1, EdgeConfig[int]{Capacity: 64, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(worker.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 64, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterCounter(b, worker.NodeID(), 1); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	if err := SetNodeWorkerPool(b, worker.NodeID(), 3); err != nil {
		t.Fatal(err)
	}
	for i, cpu := range []int{1, 2, 3} {
		if err := BindNodePoolCPU(b, worker.NodeID(), uint32(i), cpu); err != nil {
			t.Fatal(err)
		}
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if d.NodeCount() != 3 {
		t.Fatalf("logical node count=%d", d.NodeCount())
	}
	if plans := d.EdgeQueuePlans(); len(plans) != 2 || plans[0].Model != QueueModelSPMC || plans[1].Model != QueueModelMPSC {
		t.Fatalf("physical queue plans=%+v", plans)
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	in, err := BindIngress(e, ingress)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(seen); i++ {
		if status := in.Submit([]int{i}); status != IngressAccepted {
			t.Fatalf("ingress status=%d", status)
		}
		for delivered.Load() < uint64(i+1) {
			runtime.Gosched()
		}
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	if closes.Load() != 3 || len(e.AffinityStatus()) != 4 {
		t.Fatalf("close/affinity=%d/%+v", closes.Load(), e.AffinityStatus())
	}
	var total uint64
	rows := e.Metrics().Counters
	if len(rows) != 3 {
		t.Fatalf("worker counter rows=%+v", rows)
	}
	for index, row := range rows {
		if row.NodeID != worker.NodeID() || row.Replica != uint32(index) {
			t.Fatalf("worker metric identity=%+v", row)
		}
		total += row.Value
	}
	if total != uint64(len(seen)) {
		t.Fatalf("processed count=%d", total)
	}
	for i := range seen {
		if seen[i].Load() != 1 {
			t.Fatalf("missing input %d", i)
		}
	}
}

func TestNodeWorkerPoolValidation(t *testing.T) {
	for _, name := range []string{"zero", "missing-cpu", "duplicate-cpu", "group-replica-overlap"} {
		t.Run(name, func(t *testing.T) {
			b := NewBuilder()
			_, err := AddNode0(b, validIsolated())
			if err != nil {
				t.Fatal(err)
			}
			if name == "group-replica-overlap" {
				if err := ReplicateGroup(b, DefaultGroupID, 2); err != nil {
					t.Fatal(err)
				}
				bindCPU(t, b, DefaultGroupID, 0)
				if err := BindReplicaCPU(b, DefaultGroupID, 1, 2); err != nil {
					t.Fatal(err)
				}
			} else {
				bindCPU(t, b, DefaultGroupID, 0)
			}
			count := uint32(2)
			if name == "zero" {
				count = 0
			}
			if err := SetNodeWorkerPool(b, 1, count); (name == "zero" && !errors.Is(err, ErrInvalidWorkerPool)) || (name != "zero" && err != nil) {
				t.Fatalf("pool config error=%v", err)
			}
			if name != "zero" {
				if name == "duplicate-cpu" {
					_ = BindNodePoolCPU(b, 1, 0, 1)
					_ = BindNodePoolCPU(b, 1, 1, 1)
				} else if name == "missing-cpu" {
					_ = BindNodePoolCPU(b, 1, 0, 1)
				} else {
					_ = BindNodePoolCPU(b, 1, 0, 3)
					_ = BindNodePoolCPU(b, 1, 1, 4)
				}
				if _, err := b.Build(); !errors.Is(err, ErrInvalidWorkerPool) {
					t.Fatalf("validation error=%v", err)
				}
			}
		})
	}
}

func TestNodeWorkerPoolStartupRollback(t *testing.T) {
	b := NewBuilder()
	var made, closed atomic.Uint64
	initErr := errors.New("worker init")
	_, err := AddNode0(b, node.Stateful[uint64, node.Node0[uint64, node.Outputs0, error]]{
		Factory: func() (uint64, error) { return made.Add(1), nil },
		Init: func(state *uint64) error {
			if *state == 2 {
				return initErr
			}
			return nil
		},
		Close:    func(*uint64) error { closed.Add(1); return nil },
		Handlers: node.Node0[uint64, node.Outputs0, error]{Handle: func(*uint64, *node.Outputs0) node.Result[error] { return node.Result[error]{} }},
	})
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	if err := SetNodeWorkerPool(b, 1, 3); err != nil {
		t.Fatal(err)
	}
	for i, cpu := range []int{1, 2, 3} {
		if err := BindNodePoolCPU(b, 1, uint32(i), cpu); err != nil {
			t.Fatal(err)
		}
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if executor, err := d.Start(); executor != nil || !errors.Is(err, initErr) {
		if executor != nil {
			_ = executor.Close()
		}
		t.Fatalf("start rollback=%v/%v", executor, err)
	}
	if made.Load() != 3 || closed.Load() != 3 {
		t.Fatalf("factory/close=%d/%d", made.Load(), closed.Load())
	}
}

func BenchmarkNodeWorkerPoolIngress(b *testing.B) {
	builder := NewBuilder()
	ingress, err := AddIngress[int](builder, IngressConfig{Capacity: 64, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		b.Fatal(err)
	}
	worker, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(_ *struct{}, input batch.Batch[int], out *node.Outputs1[int]) node.Result[error] {
			out.Out1.Commit(&input)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	var delivered atomic.Uint64
	sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, _ batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			delivered.Add(1)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	if _, err := Connect(builder, ingress.Output(), worker.Inputs().In1, EdgeConfig[int]{Capacity: 64, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		b.Fatal(err)
	}
	if _, err := Connect(builder, OutputPorts1(worker.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 64, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		b.Fatal(err)
	}
	if err := BindCPU(builder, DefaultGroupID, 0); err != nil {
		b.Fatal(err)
	}
	if err := SetNodeWorkerPool(builder, worker.NodeID(), 3); err != nil {
		b.Fatal(err)
	}
	for i, cpu := range []int{1, 2, 3} {
		if err := BindNodePoolCPU(builder, worker.NodeID(), uint32(i), cpu); err != nil {
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
	defer executor.Close()
	producer, err := BindIngress(executor, ingress)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := uint64(i + 1)
		for producer.Submit([]int{i}) != IngressAccepted {
			runtime.Gosched()
		}
		for delivered.Load() < target {
			runtime.Gosched()
		}
	}
}
