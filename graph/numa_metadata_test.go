package graph

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/numa"
	"github.com/asharipov/gpf/queue"
)

func TestNUMAAnnotationsAndAllocatorDescriptorsStayDormant(t *testing.T) {
	b := NewBuilder()
	source, err := AddNode0(b, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	if err := SetGroupNUMA(b, DefaultGroupID, numa.On(0)); err != nil {
		t.Fatal(err)
	}
	var poolCalls, queueCalls atomic.Uint64
	var factoryCalls atomic.Uint64
	poolAllocator := func(int) ([]int, error) { poolCalls.Add(1); return make([]int, 3), nil }
	queueAllocator := func(uint32) ([]batch.Batch[int], error) { queueCalls.Add(1); return make([]batch.Batch[int], 3), nil }
	placement := numa.On(2)
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait, NUMA: placement, Factory: func(config queue.Config) (queue.QueueOps[int], error) {
		factoryCalls.Add(1)
		if config.NUMA != placement {
			t.Error("queue factory lost NUMA metadata")
		}
		ring := newLocalRing[int](config.Capacity)
		return queue.QueueOps[int]{TryPush: ring.tryPush, TryPop: ring.tryPop, Len: ring.len}, nil
	}, Allocators: AllocatorDescriptors[int]{Pool: poolAllocator, Queue: queueAllocator}}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if poolCalls.Load() != 0 || queueCalls.Load() != 0 || factoryCalls.Load() != 0 {
		t.Fatal("Build invoked dormant allocator/factory")
	}
	groups := d.GroupNUMA()
	plans := d.EdgeQueuePlans()
	if len(groups) != 1 || groups[0].Placement != numa.On(0) || len(plans) != 1 || plans[0].NUMA != placement {
		t.Fatalf("copied NUMA metadata=%+v/%+v", groups, plans)
	}
	groups[0].Placement = numa.Placement{}
	plans[0].NUMA = numa.Placement{}
	if d.GroupNUMA()[0].Placement != numa.On(0) || d.EdgeQueuePlans()[0].NUMA != placement {
		t.Fatal("metadata rows alias definition")
	}
	for range 2 {
		e, err := d.Start()
		if err != nil {
			t.Fatal(err)
		}
		if got := e.Metrics().Pools[0].NUMA; got != placement {
			t.Fatalf("runtime pool annotation=%+v", got)
		}
		if err := e.Drain(); err != nil {
			t.Fatal(err)
		}
	}
	if poolCalls.Load() != 0 || queueCalls.Load() != 0 || factoryCalls.Load() != 2 {
		t.Fatal("Start invoked inactive allocator descriptors")
	}
}

func TestIngressNUMAAllocatorDescriptorInactive(t *testing.T) {
	b := NewBuilder()
	var allocatorCalls atomic.Uint64
	placement := numa.On(0)
	ingress, err := AddIngress[int](b, IngressConfig{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait, NUMA: placement}, IngressAllocator[int]{Pool: func(size int) ([]int, error) { allocatorCalls.Add(1); return make([]int, size), nil }})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, ingress.Output(), sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
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
	endpoint, err := BindIngress(e, ingress)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Submit([]int{5}) != IngressAccepted {
		t.Fatal("ingress rejected")
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	if allocatorCalls.Load() != 0 {
		t.Fatal("ingress allocator ran")
	}
	found := false
	for _, pool := range e.Metrics().Pools {
		if pool.EdgeID == 0 && pool.SourceNodeID == ingress.NodeID() {
			found = true
			if pool.NUMA != placement {
				t.Fatalf("ingress pool NUMA=%+v", pool.NUMA)
			}
		}
	}
	if !found {
		t.Fatal("ingress source pool metadata missing")
	}
}

func TestNUMAConfigurationValidationAndReplicaCopy(t *testing.T) {
	b := NewBuilder()
	first, err := AddNode0(b, validSource())
	if err != nil {
		t.Fatal(err)
	}
	if err := SetGroupNUMA(b, 99, numa.On(0)); err == nil {
		t.Fatal("unknown group annotation accepted")
	}
	if err := SetGroupNUMA(b, DefaultGroupID, numa.Placement{Node: 1}); err != numa.ErrInvalidPlacement {
		t.Fatalf("invalid group placement=%v", err)
	}
	if err := SetGroupNUMA(b, DefaultGroupID, numa.On(3)); err != nil {
		t.Fatal(err)
	}
	if _, err := AddNode1(b, validSink()); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(first.Outputs()).Out1, InPort[int]{owner: b.owner, nodeID: 2, portID: 1}, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait, NUMA: numa.Placement{Node: 1}}); !errors.Is(err, ErrInvalidEdgeConfig) {
		t.Fatalf("invalid edge placement=%v", err)
	}
}

func TestGroupNUMAAnnotationCopiesToReplicas(t *testing.T) {
	b := NewBuilder()
	source, _ := AddNode0(b, validSource())
	sink, _ := AddNode1(b, validSink())
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, source.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sink.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	if err := SetGroupNUMA(b, 9, numa.On(4)); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, 9, 0)
	if err := ReplicateGroup(b, 9, 2); err != nil {
		t.Fatal(err)
	}
	if err := BindReplicaCPU(b, 9, 1, 1); err != nil {
		t.Fatal(err)
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	rows := d.GroupNUMA()
	if len(rows) != 2 || rows[0].Replica != 0 || rows[1].Replica != 1 || rows[0].Placement != numa.On(4) || rows[1].Placement != numa.On(4) {
		t.Fatalf("replicated NUMA rows=%+v", rows)
	}
	rows[0].Placement = numa.Placement{}
	if d.GroupNUMA()[0].Placement != numa.On(4) {
		t.Fatal("replica NUMA view aliases definition")
	}
}

func TestTypedAllocatorDescriptorDoesNotInvokeOnSequentialStart(t *testing.T) {
	b := NewBuilder()
	source, err := AddNode0(b, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Uint64
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop, Allocators: AllocatorDescriptors[int]{Pool: func(int) ([]int, error) { calls.Add(1); return []int{0}, nil }, Queue: func(uint32) ([]batch.Batch[int], error) { calls.Add(1); return make([]batch.Batch[int], 1), nil }}}); err != nil {
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
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("typed allocator calls=%d", calls.Load())
	}
}
