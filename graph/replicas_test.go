package graph

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

func TestReplicatedGroupDistributesIngressAndPairsLocalEdges(t *testing.T) {
	b := NewBuilder()
	var factories, closed atomic.Uint64
	type state struct{ add func(uint64) }
	first, err := AddNode1(b, node.Stateful[state, node.Node1[state, int, node.Outputs1[int], int]]{
		Factory: func() (state, error) { factories.Add(1); return state{}, nil },
		Handlers: node.Node1[state, int, node.Outputs1[int], int]{BindCounter: func(s *state, _ node.CounterID, add func(uint64)) { s.add = add }, Handle1: func(s *state, in batch.Batch[int], out *node.Outputs1[int]) node.Result[int] {
			s.add(1)
			out.Out1.Commit(&in)
			return node.Result[int]{}
		}},
		Close: func(*state) error { closed.Add(1); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	relay, err := AddClosureNode1(b, func() (node.ClosureReplica[node.ClosureNode1[int, node.Outputs1[int], int]], error) {
		factories.Add(1)
		var add func(uint64)
		return node.ClosureReplica[node.ClosureNode1[int, node.Outputs1[int], int]]{Handlers: node.ClosureNode1[int, node.Outputs1[int], int]{
			BindCounter: func(_ node.CounterID, update func(uint64)) { add = update },
			Handle1: func(in batch.Batch[int], out *node.Outputs1[int]) node.Result[int] {
				add(1)
				out.Out1.Commit(&in)
				return node.Result[int]{}
			},
		}, Close: func() error { closed.Add(1); return nil }}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ingress, err := AddIngress[int](b, IngressConfig{Capacity: 8, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	var delivered atomic.Uint64
	var seen [100]atomic.Uint32
	sink, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, int]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, int]{Handle1: func(_ *struct{}, in batch.Batch[int], _ *node.Outputs0) node.Result[int] {
			if in.Len() != 1 {
				t.Error("replica reset moved input")
				return node.Result[int]{}
			}
			id := in.Items()[0]
			if id < 0 || id >= len(seen) || !seen[id].CompareAndSwap(0, 1) {
				t.Errorf("duplicate/invalid input %d", id)
			}
			delivered.Add(1)
			return node.Result[int]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(b, first.NodeID(), 3); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, relay.NodeID(), 3); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sink.NodeID(), 7); err != nil {
		t.Fatal(err)
	}
	if err := RegisterCounter(b, first.NodeID(), 1); err != nil {
		t.Fatal(err)
	}
	if err := RegisterCounter(b, relay.NodeID(), 2); err != nil {
		t.Fatal(err)
	}
	config := EdgeConfig[int]{Capacity: 8, BatchCapacity: 1, Policy: queue.Wait}
	if _, err := Connect(b, ingress.Output(), first.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(first.Outputs()).Out1, relay.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(relay.Outputs()).Out1, sink.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, 1, 0)
	bindCPU(t, b, 3, 1)
	bindCPU(t, b, 7, 2)
	if err := ReplicateGroup(b, 3, 3); err != nil {
		t.Fatal(err)
	}
	if err := BindReplicaCPU(b, 3, 1, 3); err != nil {
		t.Fatal(err)
	}
	if err := BindReplicaCPU(b, 3, 2, 4); err != nil {
		t.Fatal(err)
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if factories.Load() != 0 || d.NodeCount() != 4 || d.EdgeCount() != 3 {
		t.Fatal("Build materialized state or changed logical topology")
	}
	plans := d.EdgeQueuePlans()
	if len(plans) != 5 || plans[0].Model != QueueModelSPMC || plans[4].Model != QueueModelMPSC {
		t.Fatalf("expanded transport plans=%+v", plans)
	}
	if plans[0].LogicalEdgeID == 0 || plans[0].ProducerWorkers != 1 || plans[0].ConsumerWorkers != 3 || plans[1].LogicalEdgeID != plans[2].LogicalEdgeID || plans[4].ProducerWorkers != 3 || plans[4].ConsumerWorkers != 1 {
		t.Fatalf("physical edge metadata=%+v", plans)
	}
	for iteration := uint64(1); iteration <= 2; iteration++ {
		beforeDelivered := delivered.Load()
		for i := range seen {
			seen[i].Store(0)
		}
		e, err := d.Start()
		if err != nil {
			t.Fatal(err)
		}
		in, err := BindIngress(e, ingress)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Counter(first.NodeID(), 1); !errors.Is(err, ErrAmbiguousCounter) {
			t.Fatalf("ambiguous counter=%v", err)
		}
		for replica := uint32(0); replica < 3; replica++ {
			if _, err := e.CounterForReplica(first.NodeID(), replica, 1); err != nil {
				t.Fatal(err)
			}
		}
		for id := 0; id < 100; id++ {
			for in.Submit([]int{id}) != IngressAccepted {
				if !in.open.Load() {
					t.Fatal("admission closed")
				}
			}
			for delivered.Load() < beforeDelivered+uint64(id+1) {
				runtime.Gosched()
			}
		}
		if err := e.Drain(); err != nil {
			t.Fatal(err)
		}
		if delivered.Load() != iteration*100 || factories.Load() != iteration*6 || closed.Load() != iteration*6 {
			t.Fatalf("delivery/factory/close counts=%d/%d/%d", delivered.Load(), factories.Load(), closed.Load())
		}
		var count uint64
		counterRows := e.Metrics().Counters
		if len(counterRows) != 6 || counterRows[0].Replica != 0 || counterRows[1].Replica != 1 || counterRows[2].Replica != 2 || counterRows[3].Replica != 0 || counterRows[5].Replica != 2 {
			t.Fatalf("replica counter rows=%+v", counterRows)
		}
		for _, row := range counterRows {
			if (row.NodeID != first.NodeID() && row.NodeID != relay.NodeID()) || row.Replica > 2 {
				t.Fatalf("counter identity=%+v", row)
			}
			count += row.Value
		}
		if count != 200 {
			t.Fatalf("replica-local counter total=%d", count)
		}
		if len(e.AffinityStatus()) != 5 {
			t.Fatal("wrong physical worker count")
		}
		for _, pool := range e.Metrics().Pools {
			if pool.InUseSample != 0 {
				t.Fatalf("pool leaked=%+v", pool)
			}
		}
		for i := range seen {
			if seen[i].Load() != 1 {
				t.Fatalf("missing input %d", i)
			}
		}
	}
}

func TestReplicaValidationAndIngressRejection(t *testing.T) {
	if err := ReplicateGroup(NewBuilder(), 1, 0); !errors.Is(err, ErrInvalidReplicas) {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing-cpu", "duplicate-cpu", "ingress"} {
		t.Run(mode, func(t *testing.T) {
			b := NewBuilder()
			if mode == "ingress" {
				in, err := AddIngress[int](b, IngressConfig{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait})
				if err != nil {
					t.Fatal(err)
				}
				sink, err := AddNode1(b, validSink())
				if err != nil {
					t.Fatal(err)
				}
				if _, err := Connect(b, in.Output(), sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := AddNode0(b, validIsolated()); err != nil {
					t.Fatal(err)
				}
			}
			bindCPU(t, b, 1, 0)
			if err := ReplicateGroup(b, 1, 2); err != nil {
				t.Fatal(err)
			}
			if mode != "missing-cpu" {
				cpu := 1
				if mode == "duplicate-cpu" {
					cpu = 0
				}
				if err := BindReplicaCPU(b, 1, 1, cpu); err != nil {
					t.Fatal(err)
				}
			}
			_, err := b.Build()
			want := ErrInvalidReplicas
			if mode == "ingress" {
				want = ErrReplicatedIngress
			}
			if !errors.Is(err, want) || b.state != builderOpen {
				t.Fatalf("validation=%v state=%d", err, b.state)
			}
		})
	}
}

func TestReplicaValidationRejectsNodePoolOverlap(t *testing.T) {
	b := NewBuilder()
	_, err := AddNode0(b, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	b.nodePools = map[NodeID]nodePoolConfig{1: {count: 2}}
	bindCPU(t, b, DefaultGroupID, 0)
	if err := ReplicateGroup(b, DefaultGroupID, 2); err != nil {
		t.Fatal(err)
	}
	if err := BindReplicaCPU(b, DefaultGroupID, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Build(); !errors.Is(err, ErrInvalidReplicas) {
		t.Fatalf("overlapping node pool error=%v", err)
	}
}

func TestReplicaStartupRollbackClosesEveryCreatedState(t *testing.T) {
	for failAt := uint64(1); failAt <= 3; failAt++ {
		t.Run(fmt.Sprintf("replica-%d", failAt), func(t *testing.T) {
			b := NewBuilder()
			type state struct{ index uint64 }
			var made, initialized, closed atomic.Uint64
			trigger := errors.New("replica init failed")
			_, err := AddNode0(b, node.Stateful[state, node.Node0[state, node.Outputs0, error]]{
				Factory: func() (state, error) { return state{index: made.Add(1)}, nil },
				Init: func(s *state) error {
					initialized.Add(1)
					if s.index == failAt {
						return trigger
					}
					return nil
				},
				Close:    func(*state) error { closed.Add(1); return nil },
				Handlers: node.Node0[state, node.Outputs0, error]{Handle: func(*state, *node.Outputs0) node.Result[error] { return node.Result[error]{} }},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := ReplicateGroup(b, DefaultGroupID, 3); err != nil {
				t.Fatal(err)
			}
			bindCPU(t, b, DefaultGroupID, 0)
			for replica, cpu := range []int{1, 2} {
				if err := BindReplicaCPU(b, DefaultGroupID, uint32(replica+1), cpu); err != nil {
					t.Fatal(err)
				}
			}
			d, err := b.Build()
			if err != nil {
				t.Fatal(err)
			}
			if e, err := d.Start(); e != nil || !errors.Is(err, trigger) {
				if e != nil {
					_ = e.Close()
				}
				t.Fatalf("Start rollback=%v/%v", e, err)
			}
			if made.Load() != 3 || initialized.Load() != failAt || closed.Load() != 3 {
				t.Fatalf("made/init/close=%d/%d/%d", made.Load(), initialized.Load(), closed.Load())
			}
		})
	}
}

func TestMultipleReplicaGroupIDsAreDeterministic(t *testing.T) {
	b := NewBuilder()
	a, _ := AddNode0(b, validIsolated())
	c, _ := AddNode0(b, validIsolated())
	if err := Place(b, c.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	if err := ReplicateGroup(b, DefaultGroupID, 2); err != nil {
		t.Fatal(err)
	}
	if err := ReplicateGroup(b, 9, 2); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	bindCPU(t, b, 9, 1)
	if err := BindReplicaCPU(b, DefaultGroupID, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := BindReplicaCPU(b, 9, 1, 3); err != nil {
		t.Fatal(err)
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	first, err := expandReplicas(b, d)
	if err != nil {
		t.Fatal(err)
	}
	second, err := expandReplicas(b, d)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.groups, second.groups) || !reflect.DeepEqual(first.placements, second.placements) {
		t.Fatal("replica plan depends on map iteration")
	}
	want := []groupPlan{{id: DefaultGroupID, cpu: 0}, {id: 9, cpu: 1}, {id: 12, cpu: 2}, {id: 13, cpu: 3}}
	if got := first.groups; len(got) != 4 || got[0].id != want[0].id || got[1].id != want[1].id || got[2].id != want[2].id || got[3].id != want[3].id {
		t.Fatalf("group IDs=%+v", got)
	}
	_ = a
}
