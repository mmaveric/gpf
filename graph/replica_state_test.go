package graph

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func TestGroupReplicasOwnNestedExplicitAndClosureStateAcrossRestarts(t *testing.T) {
	b := NewBuilder()
	var sourceFactories, sinkFactories, sourceCloses, sinkCloses atomic.Uint64
	type explicitState struct {
		data    map[string][]*int
		emitted bool
	}
	source, err := AddNode0(b, node.Stateful[explicitState, node.Node0[explicitState, node.Outputs1[int], error]]{
		Factory: func() (explicitState, error) {
			id := int(sourceFactories.Add(1)-1) % 3
			return explicitState{data: map[string][]*int{"nested": {&id}}}, nil
		},
		Handlers: node.Node0[explicitState, node.Outputs1[int], error]{Handle: func(s *explicitState, out *node.Outputs1[int]) node.Result[error] {
			if s.emitted {
				return node.Result[error]{}
			}
			*s.data["nested"][0]++
			value, ok := out.Out1.Acquire()
			if !ok {
				return node.Result[error]{}
			}
			value.Mutable()[0] = *s.data["nested"][0]
			value.SetLen(1)
			out.Out1.Commit(&value)
			s.emitted = true
			return node.Result[error]{}
		}},
		Close: func(s *explicitState) error {
			if len(s.data) != 1 || len(s.data["nested"]) != 1 || *s.data["nested"][0] < 1 {
				t.Error("explicit nested state was shared or lost")
			}
			sourceCloses.Add(1)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddClosureNode1(b, func() (node.ClosureReplica[node.ClosureNode1[int, node.Outputs0, error]], error) {
		id := int(sinkFactories.Add(1)-1) % 3
		owner := id
		data := map[string][]*int{"nested": {&owner}}
		seen := map[int]bool{}
		return node.ClosureReplica[node.ClosureNode1[int, node.Outputs0, error]]{
			Handlers: node.ClosureNode1[int, node.Outputs0, error]{Handle1: func(input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
				if len(data["nested"]) != 1 || input.Len() != 1 || input.Items()[0] != *data["nested"][0]+1 || seen[input.Items()[0]] {
					t.Errorf("closure replica %d observed foreign/repeated state: %v", id, input.Items())
				}
				seen[input.Items()[0]] = true
				*data["nested"][0]++
				return node.Result[error]{}
			}},
			Close: func() error {
				if len(data) != 1 || len(data["nested"]) != 1 || *data["nested"][0] != id+1 || len(seen) != 1 {
					t.Errorf("closure environment did not remain replica-local: id=%d data=%v seen=%v", id, *data["nested"][0], seen)
				}
				sinkCloses.Add(1)
				return nil
			},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	if err := ReplicateGroup(b, DefaultGroupID, 3); err != nil {
		t.Fatal(err)
	}
	for replica, cpu := range []int{1, 2} {
		if err := BindReplicaCPU(b, DefaultGroupID, uint32(replica+1), cpu); err != nil {
			t.Fatal(err)
		}
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		e, err := d.Start()
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for e.Metrics().Nodes[1].HandlerCalls < 3 {
			if time.Now().After(deadline) {
				_ = e.Close()
				t.Fatal("replica sinks did not receive paired batches")
			}
		}
		if err := e.Drain(); err != nil {
			t.Fatal(err)
		}
	}
	if sourceFactories.Load() != 6 || sinkFactories.Load() != 6 || sourceCloses.Load() != 6 || sinkCloses.Load() != 6 {
		t.Fatalf("factory/close counts=%d/%d/%d/%d", sourceFactories.Load(), sinkFactories.Load(), sourceCloses.Load(), sinkCloses.Load())
	}
}

func TestNodeWorkersOwnNestedStateAndCloseOnce(t *testing.T) {
	b := NewBuilder()
	in, err := AddIngress[int](b, IngressConfig{Capacity: 32, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		values map[string][]*uint64
		add    func(uint64)
	}
	var factories, closes atomic.Uint64
	worker, err := AddNode1(b, node.Stateful[state, node.Node1[state, int, node.Outputs1[int], error]]{
		Factory: func() (state, error) {
			id := factories.Add(1)
			owner := id
			return state{values: map[string][]*uint64{"owner": {&owner}}}, nil
		},
		Handlers: node.Node1[state, int, node.Outputs1[int], error]{BindCounter: func(s *state, _ node.CounterID, add func(uint64)) { s.add = add }, Handle1: func(s *state, input batch.Batch[int], out *node.Outputs1[int]) node.Result[error] {
			if len(s.values["owner"]) != 1 || *s.values["owner"][0] == 0 {
				t.Error("worker state missing owner")
			}
			s.add(1)
			out.Out1.Commit(&input)
			return node.Result[error]{}
		}},
		Close: func(s *state) error {
			if len(s.values) != 1 || len(s.values["owner"]) != 1 || *s.values["owner"][0] == 0 {
				t.Error("worker close observed shared/replaced state")
			}
			closes.Add(1)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, _ batch.Batch[int], _ *node.Outputs0) node.Result[error] {
		return node.Result[error]{}
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, in.Output(), worker.Inputs().In1, EdgeConfig[int]{Capacity: 32, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(worker.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 32, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
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
	for range 2 {
		e, err := d.Start()
		if err != nil {
			t.Fatal(err)
		}
		producer, err := BindIngress(e, in)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 12; i++ {
			if status := producer.Submit([]int{i}); status != IngressAccepted {
				t.Fatalf("submit=%d", status)
			}
		}
		if err := e.Drain(); err != nil {
			t.Fatal(err)
		}
		rows := e.Metrics().Counters
		if len(rows) != 3 {
			t.Fatalf("worker counters=%+v", rows)
		}
		var total uint64
		for _, row := range rows {
			total += row.Value
		}
		if total != 12 {
			t.Fatalf("worker counter total=%d", total)
		}
	}
	if factories.Load() != 6 || closes.Load() != 6 {
		t.Fatalf("worker factory/close=%d/%d", factories.Load(), closes.Load())
	}
}
