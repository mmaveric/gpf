package graph

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

func TestConnectShardDistributesElementsAndFlushesPartialBatches(t *testing.T) {
	b := NewBuilder()
	ingressDef, err := AddIngress[int](b, IngressConfig{Capacity: 2, BatchCapacity: 9, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	relay, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(_ *struct{}, input batch.Batch[int], outputs *node.Outputs1[int]) node.Result[error] {
			outputs.Out1.Commit(&input)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got [3][]int
	sinks := make([]NodePorts[InPorts1[int], node.Outputs0, error], 3)
	for index := range sinks {
		index := index
		sinks[index], err = AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
			Factory: func() (struct{}, error) { return struct{}{}, nil },
			Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
				got[index] = append(got[index], input.Items()...)
				return node.Result[error]{}
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Connect(b, ingressDef.Output(), relay.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 9, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	var selected atomic.Uint64
	branches, err := ConnectShard(b, OutputPorts1(relay.Outputs()).Out1, []InPort[int]{sinks[0].Inputs().In1, sinks[1].Inputs().In1, sinks[2].Inputs().In1}, func(value int) uint32 {
		selected.Add(1)
		return uint32(value % 3)
	}, EdgeConfig[int]{Capacity: 4, BatchCapacity: 2, Policy: queue.Wait})
	if err != nil {
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
	defer e.Close()
	ingress, err := BindIngress(e, ingressDef)
	if err != nil {
		t.Fatal(err)
	}
	if status := ingress.Submit([]int{0, 1, 2, 3, 4, 5, 6, 7, 8}); status != IngressAccepted {
		t.Fatalf("submit status=%v", status)
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	if selected.Load() != 9 {
		t.Fatalf("selector calls=%d, want 9", selected.Load())
	}
	for branch := range got {
		want := []int{branch, branch + 3, branch + 6}
		if len(got[branch]) != len(want) {
			t.Fatalf("branch %d values=%v want=%v", branch, got[branch], want)
		}
		for index := range want {
			if got[branch][index] != want[index] {
				t.Fatalf("branch %d values=%v want=%v", branch, got[branch], want)
			}
		}
	}
	for index, row := range e.Metrics().Edges[1:] {
		if row.EdgeID != branches[index] || row.ShardID != branches[0] || row.AcceptedElements != 3 || row.AcceptedBatches != 2 || row.DequeuedElements != 3 {
			t.Fatalf("shard branch %d metrics=%+v", index, row)
		}
	}
}

func TestConnectShardCachesBranchAcrossWaitAndAccountsPoolLosses(t *testing.T) {
	b := NewBuilder()
	ingressDef, err := AddIngress[int](b, IngressConfig{Capacity: 2, BatchCapacity: 4, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	blockerDef, err := AddIngress[int](b, IngressConfig{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	relay, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(_ *struct{}, input batch.Batch[int], outputs *node.Outputs1[int]) node.Result[error] {
			outputs.Out1.Commit(&input)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}, 1), make(chan struct{})
	var received atomic.Uint64
	slow, err := AddNode2(b, node.Stateful[struct{}, node.Node2[struct{}, int, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node2[struct{}, int, int, node.Outputs0, error]{
			Handle1: func(_ *struct{}, _ batch.Batch[int], _ *node.Outputs0) node.Result[error] {
				received.Add(1)
				return node.Result[error]{}
			},
			Handle2: func(_ *struct{}, _ batch.Batch[int], _ *node.Outputs0) node.Result[error] {
				started <- struct{}{}
				<-release
				return node.Result[error]{}
			},
			Select: func(_ *struct{}, _ node.ReadyMask) node.PortIndex { return node.Port2 },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	fast, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} }}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(b, ingressDef.NodeID(), 1); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, blockerDef.NodeID(), 1); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, relay.NodeID(), 1); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, slow.NodeID(), 2); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, fast.NodeID(), 3); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, ingressDef.Output(), relay.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 4, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, blockerDef.Output(), slow.Inputs().In2, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Uint64
	branches, err := ConnectShard(b, OutputPorts1(relay.Outputs()).Out1, []InPort[int]{slow.Inputs().In1, fast.Inputs().In1}, func(int) uint32 { calls.Add(1); return 0 }, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, 1, 0)
	bindCPU(t, b, 2, 1)
	bindCPU(t, b, 3, 2)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		_ = e.Close()
	}()
	ingress, err := BindIngress(e, ingressDef)
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := BindIngress(e, blockerDef)
	if err != nil {
		t.Fatal(err)
	}
	if status := blocker.Submit([]int{99}); status != IngressAccepted {
		t.Fatalf("blocker submit status=%v", status)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("blocker handler did not start")
	}
	if status := ingress.Submit([]int{1, 2, 3, 4}); status != IngressAccepted {
		t.Fatalf("submit status=%v", status)
	}
	waitForLogger(t, func() bool { return e.Metrics().Edges[2].WaitRetries != 0 })
	if calls.Load() != 3 {
		t.Fatalf("selector calls while branch waits=%d, want 3", calls.Load())
	}
	close(release)
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 4 {
		t.Fatalf("selector called %d times, want once per element", calls.Load())
	}
	rows := e.Metrics().Edges[2:]
	if rows[0].EdgeID != branches[0] || rows[0].AcceptedElements+rows[0].PoolDroppedElements != 4 || rows[0].WaitRetries == 0 || rows[0].QueueDroppedElements != 0 || received.Load() != rows[0].AcceptedBatches {
		t.Fatalf("wait branch metrics=%+v", rows[0])
	}
}

func TestConnectShardPoolExhaustionIsElementLocal(t *testing.T) {
	b := NewBuilder()
	ingressDef, err := AddIngress[int](b, IngressConfig{Capacity: 2, BatchCapacity: 6, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	relay, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(_ *struct{}, input batch.Batch[int], out *node.Outputs1[int]) node.Result[error] {
		out.Out1.Commit(&input)
		return node.Result[error]{}
	}}})
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}, 1), make(chan struct{})
	slow, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] {
		started <- struct{}{}
		<-release
		return node.Result[error]{}
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var fastCount atomic.Uint64
	fast, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] {
		fastCount.Add(1)
		return node.Result[error]{}
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(b, ingressDef.NodeID(), 1); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, relay.NodeID(), 1); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, slow.NodeID(), 2); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, fast.NodeID(), 3); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, ingressDef.Output(), relay.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 6, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	branches, err := ConnectShard(b, OutputPorts1(relay.Outputs()).Out1, []InPort[int]{slow.Inputs().In1, fast.Inputs().In1}, func(value int) uint32 {
		if value == 1 {
			return 1
		}
		return 0
	}, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, 1, 0)
	bindCPU(t, b, 2, 1)
	bindCPU(t, b, 3, 2)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		_ = e.Close()
	}()
	ingress, err := BindIngress(e, ingressDef)
	if err != nil {
		t.Fatal(err)
	}
	if status := ingress.Submit([]int{0, 0, 0, 0, 0, 1}); status != IngressAccepted {
		t.Fatalf("submit=%v", status)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("slow branch did not retain its first batch")
	}
	deadline := time.Now().Add(5 * time.Second)
	for e.Metrics().Edges[1].PoolDroppedElements == 0 || fastCount.Load() != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("pool exhaustion not observed: branches=%+v/%+v fast=%d", e.Metrics().Edges[1], e.Metrics().Edges[2], fastCount.Load())
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	rows := e.Metrics().Edges[1:]
	if rows[0].EdgeID != branches[0] || rows[0].PoolExhaustions == 0 || rows[0].PoolDroppedElements == 0 || rows[0].QueueDroppedElements != 0 {
		t.Fatalf("exhausted shard=%+v", rows[0])
	}
	if rows[1].EdgeID != branches[1] || rows[1].PoolDroppedElements != 0 || rows[1].AcceptedElements != 1 || rows[1].DequeuedElements != 1 {
		t.Fatalf("unaffected shard=%+v", rows[1])
	}
}

func TestConnectShardRejectsInvalidTopologyAtomically(t *testing.T) {
	t.Run("nil selector", func(t *testing.T) {
		b := NewBuilder()
		source, _ := AddIngress[int](b, IngressConfig{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait})
		first, _ := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} }}})
		second, _ := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} }}})
		before := len(b.edges)
		if _, err := ConnectShard(b, source.Output(), []InPort[int]{first.Inputs().In1, second.Inputs().In1}, nil, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); !errors.Is(err, ErrInvalidShard) {
			t.Fatalf("nil selector error=%v", err)
		}
		if len(b.edges) != before || b.inputs[endpointID{first.NodeID(), 1}].hasProducer || b.outputs[endpointID{source.NodeID(), 1}].hasConsumer {
			t.Fatal("rejected sharding registration mutated topology")
		}
	})
	t.Run("duplicate destination", func(t *testing.T) {
		b := NewBuilder()
		source, _ := AddIngress[int](b, IngressConfig{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait})
		first, _ := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} }}})
		before := len(b.edges)
		if _, err := ConnectShard(b, source.Output(), []InPort[int]{first.Inputs().In1, first.Inputs().In1}, func(int) uint32 { return 0 }, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); !errors.Is(err, ErrInvalidShard) {
			t.Fatalf("duplicate destination error=%v", err)
		}
		if len(b.edges) != before || b.inputs[endpointID{first.NodeID(), 1}].hasProducer {
			t.Fatal("rejected sharding registration mutated topology")
		}
	})
	t.Run("foreign destination", func(t *testing.T) {
		b, foreign := NewBuilder(), NewBuilder()
		source, _ := AddIngress[int](b, IngressConfig{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait})
		local, _ := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} }}})
		foreignNode, _ := AddNode1(foreign, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} }}})
		before := len(b.edges)
		if _, err := ConnectShard(b, source.Output(), []InPort[int]{local.Inputs().In1, foreignNode.Inputs().In1}, func(int) uint32 { return 0 }, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); !errors.Is(err, ErrForeignPort) {
			t.Fatalf("foreign destination error=%v", err)
		}
		if len(b.edges) != before || b.inputs[endpointID{local.NodeID(), 1}].hasProducer {
			t.Fatal("rejected sharding registration mutated local topology")
		}
	})
	t.Run("incompatible factory groups", func(t *testing.T) {
		b := NewBuilder()
		source, _ := AddIngress[int](b, IngressConfig{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait})
		first, _ := AddNode1(b, validSink())
		second, _ := AddNode1(b, validSink())
		if err := Place(b, source.NodeID(), 1); err != nil {
			t.Fatal(err)
		}
		if err := Place(b, first.NodeID(), 1); err != nil {
			t.Fatal(err)
		}
		if err := Place(b, second.NodeID(), 2); err != nil {
			t.Fatal(err)
		}
		before := len(b.edges)
		config := EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait, Factory: func(queue.Config) (queue.QueueOps[int], error) { return queue.QueueOps[int]{}, nil }}
		if _, err := ConnectShard(b, source.Output(), []InPort[int]{first.Inputs().In1, second.Inputs().In1}, func(int) uint32 { return 0 }, config); !errors.Is(err, ErrIncompatibleQueueFactory) {
			t.Fatalf("factory group error=%v", err)
		}
		if len(b.edges) != before || b.outputs[endpointID{source.NodeID(), 1}].hasConsumer || b.inputs[endpointID{first.NodeID(), 1}].hasProducer {
			t.Fatal("rejected factory configuration mutated topology")
		}
	})
}

func TestConnectShardRejectsExpandedEndpoints(t *testing.T) {
	b := NewBuilder()
	source, err := AddNode0(b, validSource())
	if err != nil {
		t.Fatal(err)
	}
	first, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	second, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(b, source.NodeID(), 1); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, first.NodeID(), 2); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, second.NodeID(), 3); err != nil {
		t.Fatal(err)
	}
	if _, err := ConnectShard(b, OutputPorts1(source.Outputs()).Out1, []InPort[int]{first.Inputs().In1, second.Inputs().In1}, func(value int) uint32 { return uint32(value & 1) }, EdgeConfig[int]{Capacity: 2, BatchCapacity: 2, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, 1, 0)
	bindCPU(t, b, 2, 2)
	bindCPU(t, b, 3, 3)
	if err := ReplicateGroup(b, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := BindReplicaCPU(b, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Build(); !errors.Is(err, ErrShardParallelExpansion) {
		t.Fatalf("replicated shard error=%v", err)
	}
}

func TestShardSelectorInvalidIndexPanicsWithContractError(t *testing.T) {
	runtime := &shardRuntime[int]{group: 1, source: endpointID{1, 1}, destinations: []endpointID{{2, 1}, {3, 1}}, selectBranch: func(int) uint32 { return 2 }, config: queue.Config{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}, plans: []EdgeQueuePlan{{EdgeID: 1, ShardID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}, {EdgeID: 2, ShardID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}}, ready: []bool{true, true}, branches: []edgeMetadata{{id: 1}, {id: 2}}}
	bindings := &edgeBindingTarget{}
	for index := range runtime.plans {
		if err := runtime.bind(index, EdgeID(index+1))(bindings, runtime.plans[index]); err != nil {
			t.Fatal(err)
		}
	}
	var output node.Output[int]
	finalize := bindings.outputs[0].configure(unsafe.Pointer(&output))
	value, ok := output.Acquire()
	if !ok {
		t.Fatal("source pool exhausted")
	}
	value.Mutable()[0] = 7
	value.SetLen(1)
	output.Commit(&value)
	defer func() {
		if got := recover(); got != ErrInvalidShardIndex {
			t.Fatalf("selector panic=%v", got)
		}
		finalize(true)
	}()
	finalize(false)
}

func TestShardPublicationHasNoPerElementAllocations(t *testing.T) {
	const capacity = 128
	definition := &shardRuntime[int]{group: 1, source: endpointID{1, 1}, destinations: []endpointID{{2, 1}, {3, 1}}, selectBranch: func(value int) uint32 { return uint32(value & 1) }, config: queue.Config{Capacity: capacity, BatchCapacity: 1, Policy: queue.Wait}, plans: []EdgeQueuePlan{{EdgeID: 1, ShardID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}, {EdgeID: 2, ShardID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}}, ready: []bool{true, true}, branches: []edgeMetadata{{id: 1}, {id: 2}}}
	bindings := &edgeBindingTarget{}
	for index := range definition.plans {
		if err := definition.bind(index, EdgeID(index+1))(bindings, definition.plans[index]); err != nil {
			t.Fatal(err)
		}
	}
	var output node.Output[int]
	finalize := bindings.outputs[0].configure(unsafe.Pointer(&output))
	allocations := testing.AllocsPerRun(64, func() {
		value, ok := output.Acquire()
		if !ok {
			panic("shard source pool exhausted")
		}
		value.Mutable()[0] = 7
		value.SetLen(1)
		output.Commit(&value)
		if !finalize(false) {
			panic("unexpected Wait retry")
		}
	})
	if allocations != 0 {
		t.Fatalf("shard publication allocs/op=%f, want zero", allocations)
	}
}

func TestShardCloseReleasesQueuedPendingAndUnassignedElements(t *testing.T) {
	const capacity = 1
	metrics := &metricStore{edges: []*edgeCounters{{identity: EdgeMetrics{EdgeID: 1}}, {identity: EdgeMetrics{EdgeID: 2}}}}
	runtime := &shardRuntime[int]{group: 1, source: endpointID{1, 1}, destinations: []endpointID{{2, 1}, {3, 1}}, selectBranch: func(int) uint32 { return 0 }, config: queue.Config{Capacity: capacity, BatchCapacity: 1, Policy: queue.Wait}, plans: []EdgeQueuePlan{{EdgeID: 1, ShardID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}, {EdgeID: 2, ShardID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}}, ready: []bool{true, true}, branches: []edgeMetadata{{id: 1}, {id: 2}}}
	bindings := &edgeBindingTarget{metrics: metrics}
	for index := range runtime.plans {
		if err := runtime.bind(index, EdgeID(index+1))(bindings, runtime.plans[index]); err != nil {
			t.Fatal(err)
		}
	}
	var output node.Output[int]
	finalize := bindings.outputs[0].configure(unsafe.Pointer(&output))
	sourcePool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 3})
	if err != nil {
		t.Fatal(err)
	}
	source, ok := sourcePool.Acquire()
	if !ok {
		t.Fatal("source pool exhausted")
	}
	copy(source.Mutable(), []int{10, 11, 12})
	source.SetLen(3)
	if !output.Commit(&source) {
		t.Fatal("source commit failed")
	}
	if finalize(false) {
		t.Fatal("expected the full branch to wait")
	}
	bindings.discarding = true
	if !finalize(true) {
		t.Fatal("forced output discard did not complete")
	}
	for _, drain := range bindings.drains {
		drain()
	}
	if usage := sourcePool.Metrics(); usage.InUseSample != 0 {
		t.Fatalf("source slot leaked: %+v", usage)
	}
	for _, pool := range metrics.pools {
		if usage := pool.read(); usage.InUseSample != 0 {
			t.Fatalf("shard staging slot leaked: identity=%+v usage=%+v", pool.identity, usage)
		}
	}
	row := metrics.edges[0]
	if row.discarded.Load() != 2 || row.discardedElements.Load() != 2 || row.producer.shardShutdownElements.Load() != 1 {
		t.Fatalf("forced shard cleanup queue=%d/%d unassigned=%d", row.discarded.Load(), row.discardedElements.Load(), row.producer.shardShutdownElements.Load())
	}
}

func TestShardDropCountsOnlyTheSelectedBranchesElements(t *testing.T) {
	metrics := &metricStore{edges: []*edgeCounters{{identity: EdgeMetrics{EdgeID: 1}}, {identity: EdgeMetrics{EdgeID: 2}}}}
	runtime := &shardRuntime[int]{group: 1, source: endpointID{1, 1}, destinations: []endpointID{{2, 1}, {3, 1}}, selectBranch: func(int) uint32 { return 0 }, config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}, plans: []EdgeQueuePlan{{EdgeID: 1, ShardID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}, {EdgeID: 2, ShardID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}}, ready: []bool{true, true}, branches: []edgeMetadata{{id: 1}, {id: 2}}}
	bindings := &edgeBindingTarget{metrics: metrics}
	for index := range runtime.plans {
		if err := runtime.bind(index, EdgeID(index+1))(bindings, runtime.plans[index]); err != nil {
			t.Fatal(err)
		}
	}
	var output node.Output[int]
	finalize := bindings.outputs[0].configure(unsafe.Pointer(&output))
	sourcePool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 3})
	if err != nil {
		t.Fatal(err)
	}
	source, ok := sourcePool.Acquire()
	if !ok {
		t.Fatal("source pool exhausted")
	}
	copy(source.Mutable(), []int{1, 2, 3})
	source.SetLen(3)
	output.Commit(&source)
	if !finalize(false) {
		t.Fatal("Drop retained a branch batch")
	}
	for _, drain := range bindings.drains {
		drain()
	}
	if got := metrics.edges[0].producer.droppedElements.Load(); got != 2 {
		t.Fatalf("queue-dropped elements=%d, want 2", got)
	}
	if got := metrics.edges[1].producer.droppedElements.Load(); got != 0 {
		t.Fatalf("unselected branch drops=%d, want 0", got)
	}
	for _, pool := range metrics.pools {
		if usage := pool.read(); usage.InUseSample != 0 {
			t.Fatalf("shard slot leaked: %+v", usage)
		}
	}
}
