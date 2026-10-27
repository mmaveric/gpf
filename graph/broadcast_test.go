package graph

import (
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

type broadcastValue struct {
	Value   int
	Pointer *int
}

func TestBroadcastMakesDistinctShallowTypedCopies(t *testing.T) {
	b := NewBuilder()
	shared := 17
	source, err := AddNode0(b, node.Stateful[bool, node.Node0[bool, node.Outputs1[broadcastValue], error]]{
		Factory: func() (bool, error) { return false, nil },
		Handlers: node.Node0[bool, node.Outputs1[broadcastValue], error]{Handle: func(done *bool, out *node.Outputs1[broadcastValue]) node.Result[error] {
			if *done {
				return node.Result[error]{}
			}
			v, ok := out.Out1.Acquire()
			if !ok {
				return node.Result[error]{}
			}
			v.Mutable()[0] = broadcastValue{Value: 4, Pointer: &shared}
			v.Mutable()[1] = broadcastValue{Value: 9, Pointer: &shared}
			v.SetLen(2)
			out.Out1.Commit(&v)
			*done = true
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	type observed struct {
		address uintptr
		pointer *int
		values  [2]int
	}
	received := make(chan observed, 2)
	addSink := func() NodePorts[InPorts1[broadcastValue], node.Outputs0, error] {
		p, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, broadcastValue, node.Outputs0, error]]{
			Factory: func() (struct{}, error) { return struct{}{}, nil },
			Handlers: node.Node1[struct{}, broadcastValue, node.Outputs0, error]{Handle1: func(_ *struct{}, value batch.Batch[broadcastValue], _ *node.Outputs0) node.Result[error] {
				items := value.Items()
				if len(items) != 2 {
					received <- observed{}
					return node.Result[error]{}
				}
				received <- observed{address: uintptr(unsafe.Pointer(&items[0])), pointer: items[0].Pointer, values: [2]int{items[0].Value, items[1].Value}}
				return node.Result[error]{}
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	first, second := addSink(), addSink()
	branches, err := ConnectBroadcast(b, OutputPorts1(source.Outputs()).Out1, []InPort[broadcastValue]{first.Inputs().In1, second.Inputs().In1}, EdgeConfig[broadcastValue]{Capacity: 2, BatchCapacity: 2, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 || d.EdgeCount() != 2 {
		t.Fatalf("broadcast branches=%v edges=%d", branches, d.EdgeCount())
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	var got [2]observed
	for i := range got {
		select {
		case got[i] = <-received:
		case <-time.After(5 * time.Second):
			t.Fatal("broadcast delivery timeout")
		}
	}
	if got[0].address == got[1].address || got[0].pointer != &shared || got[1].pointer != &shared || got[0].values != [2]int{4, 9} || got[1].values != [2]int{4, 9} {
		t.Fatalf("broadcast copies=%+v", got)
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	metrics := e.Metrics()
	for index, row := range metrics.Edges {
		if row.EdgeID != branches[index] || row.BroadcastID != branches[0] || row.AcceptedBatches != 1 || row.DequeuedBatches != 1 {
			t.Fatalf("branch metrics=%+v", row)
		}
	}
}

func broadcastPolicyFixture(t *testing.T, policy queue.Policy) (*Executor, []EdgeID, *Ingress[int], <-chan struct{}, chan struct{}, *[2]atomic.Uint64) {
	t.Helper()
	b := NewBuilder()
	ingressDef, err := AddIngress[int](b, IngressConfig{Capacity: 8, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	counts := new([2]atomic.Uint64)
	var sinks [2]NodePorts[InPorts1[int], node.Outputs0, error]
	for index := range sinks {
		idx := index
		sinks[index], err = AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
			Factory: func() (struct{}, error) { return struct{}{}, nil },
			Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, _ batch.Batch[int], _ *node.Outputs0) node.Result[error] {
				if idx == 0 && counts[0].Add(1) == 1 {
					started <- struct{}{}
					<-release
				} else {
					counts[idx].Add(1)
				}
				return node.Result[error]{}
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := Place(b, ingressDef.NodeID(), 1); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sinks[0].NodeID(), 2); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sinks[1].NodeID(), 3); err != nil {
		t.Fatal(err)
	}
	branches, err := ConnectBroadcast(b, ingressDef.Output(), []InPort[int]{sinks[0].Inputs().In1, sinks[1].Inputs().In1}, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	for group, cpu := range map[GroupID]int{1: 0, 2: 1, 3: 2} {
		if err := BindCPU(b, group, cpu); err != nil {
			t.Fatal(err)
		}
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		_ = e.Close()
	})
	producer, err := BindIngress(e, ingressDef)
	if err != nil {
		t.Fatal(err)
	}
	return e, branches, producer, started, release, counts
}

func TestBroadcastBranchWaitAndDropAreIndependent(t *testing.T) {
	for _, policy := range []queue.Policy{queue.Drop, queue.Wait} {
		t.Run(map[queue.Policy]string{queue.Drop: "drop", queue.Wait: "wait"}[policy], func(t *testing.T) {
			e, branches, producer, started, release, counts := broadcastPolicyFixture(t, policy)
			for value := 1; value <= 3; value++ {
				if status := producer.Submit([]int{value}); status != IngressAccepted {
					t.Fatalf("ingress submit=%d", status)
				}
				if value == 1 {
					select {
					case <-started:
					case <-time.After(5 * time.Second):
						t.Fatal("branch one did not enter sink")
					}
				}
				if value < 3 {
					waitForLogger(t, func() bool { return counts[1].Load() >= uint64(value) })
				}
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				rows := e.Metrics().Edges
				ready := rows[0].QueueDroppedBatches > 0
				if policy == queue.Wait {
					ready = rows[0].WaitRetries > 0
				}
				if ready {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("branch saturation timeout: policy=%d rows=%+v counts=%d", policy, rows, counts[1].Load())
				}
				runtime.Gosched()
			}
			select {
			case <-release:
			default:
				close(release)
			}
			if err := e.Drain(); err != nil {
				t.Fatal(err)
			}
			rows := e.Metrics().Edges
			if rows[0].EdgeID != branches[0] || rows[1].EdgeID != branches[1] {
				t.Fatalf("edge order=%+v", rows)
			}
			if policy == queue.Drop && rows[0].QueueDroppedBatches == 0 {
				t.Fatal("blocked branch did not drop")
			}
			if policy == queue.Wait && (rows[0].QueueDroppedBatches != 0 || rows[0].WaitRetries == 0) {
				t.Fatal("Wait branch did not retry independently")
			}
			if rows[1].QueueDroppedBatches != 0 || rows[1].AcceptedBatches != 3 || counts[1].Load() != 3 {
				t.Fatalf("unblocked branch was affected: %+v count=%d", rows[1], counts[1].Load())
			}
			for _, pool := range e.Metrics().Pools {
				if pool.InUseSample != 0 {
					t.Fatalf("broadcast pool leak=%+v", pool)
				}
			}
		})
	}
}

func TestBroadcastCloseDiscardsQueuedAndWaitCopiesOnce(t *testing.T) {
	e, branches, producer, started, release, _ := broadcastPolicyFixture(t, queue.Wait)
	if producer.Submit([]int{1}) != IngressAccepted {
		t.Fatal("first submit")
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("blocked consumer did not start")
	}
	if producer.Submit([]int{2}) != IngressAccepted {
		t.Fatal("second submit")
	}
	waitForLogger(t, func() bool { return e.Metrics().Edges[1].DequeuedBatches >= 2 })
	if producer.Submit([]int{3}) != IngressAccepted {
		t.Fatal("third submit")
	}
	waitForLogger(t, func() bool { return e.Metrics().Edges[0].WaitRetries > 0 })
	done := make(chan error, 1)
	go func() { done <- e.Close() }()
	waitForLogger(t, func() bool { return e.closing.Load() })
	waitForLogger(t, func() bool { return e.workers[0].paused.Load() })
	select {
	case <-release:
	default:
		close(release)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("broadcast close did not join")
	}
	rows := e.Metrics().Edges
	if rows[0].EdgeID != branches[0] || rows[0].ShutdownDiscardedBatches == 0 || rows[0].QueueDroppedBatches != 0 {
		t.Fatalf("branch close accounting=%+v", rows[0])
	}
	for _, pool := range e.Metrics().Pools {
		if pool.InUseSample != 0 {
			t.Fatalf("close leaked broadcast slot=%+v", pool)
		}
	}
}

func TestBroadcastPoolExhaustionIsBranchLocal(t *testing.T) {
	b := NewBuilder()
	source, err := AddNode0(b, node.Stateful[uint64, node.Node0[uint64, node.Outputs1[int], error]]{
		Factory: func() (uint64, error) { return 0, nil },
		Handlers: node.Node0[uint64, node.Outputs1[int], error]{Handle: func(next *uint64, out *node.Outputs1[int]) node.Result[error] {
			if *next == 4 {
				return node.Result[error]{}
			}
			value, ok := out.Out1.Acquire()
			if !ok {
				return node.Result[error]{}
			}
			*next++
			value.Mutable()[0] = int(*next)
			value.SetLen(1)
			out.Out1.Commit(&value)
			return node.Result[error]{}
		}},
	})
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
	var slowCount, fastCount atomic.Uint64
	slow, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, _ batch.Batch[int], _ *node.Outputs0) node.Result[error] {
		if slowCount.Add(1) == 1 {
			started <- struct{}{}
			<-release
		}
		return node.Result[error]{}
	}}})
	if err != nil {
		t.Fatal(err)
	}
	fast, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, _ batch.Batch[int], _ *node.Outputs0) node.Result[error] {
		fastCount.Add(1)
		return node.Result[error]{}
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for id, group := range map[NodeID]GroupID{source.NodeID(): 1, relay.NodeID(): 2, slow.NodeID(): 3, fast.NodeID(): 4} {
		if err := Place(b, id, group); err != nil {
			t.Fatal(err)
		}
	}
	branches, err := ConnectBroadcast(b, OutputPorts1(source.Outputs()).Out1, []InPort[int]{relay.Inputs().In1, fast.Inputs().In1}, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(relay.Outputs()).Out1, slow.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	for group, cpu := range map[GroupID]int{1: 0, 2: 1, 3: 2, 4: 3} {
		if err := BindCPU(b, group, cpu); err != nil {
			t.Fatal(err)
		}
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		_ = e.Close()
	})
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("slow branch did not hold origin batch")
	}
	waitForLogger(t, func() bool { return e.Metrics().Edges[0].PoolExhaustions > 0 })
	waitForLogger(t, func() bool { return fastCount.Load() == 4 })
	select {
	case <-release:
	default:
		close(release)
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	rows := e.Metrics().Edges
	if rows[0].EdgeID != branches[0] || rows[0].PoolExhaustions != 1 || rows[0].QueueDroppedBatches != 0 {
		t.Fatalf("exhausted branch=%+v", rows[0])
	}
	if rows[1].PoolExhaustions != 0 || rows[1].QueueDroppedBatches != 0 || rows[1].DequeuedBatches != 4 {
		t.Fatalf("independent branch=%+v", rows[1])
	}
}

func TestBroadcastRejectsOversizedMovedBatchWithDistinctCounters(t *testing.T) {
	b := NewBuilder()
	ingress, err := AddIngress[int](b, IngressConfig{Capacity: 2, BatchCapacity: 2, Policy: queue.Wait})
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
	var delivered atomic.Uint64
	addSink := func() NodePorts[InPorts1[int], node.Outputs0, error] {
		p, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] {
			delivered.Add(1)
			return node.Result[error]{}
		}}})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	first, second := addSink(), addSink()
	if _, err := Connect(b, ingress.Output(), relay.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 2, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	branches, err := ConnectBroadcast(b, OutputPorts1(relay.Outputs()).Out1, []InPort[int]{first.Inputs().In1, second.Inputs().In1}, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait})
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
	producer, err := BindIngress(e, ingress)
	if err != nil {
		t.Fatal(err)
	}
	if producer.Submit([]int{2, 4}) != IngressAccepted {
		t.Fatal("oversized-source setup submit")
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	if delivered.Load() != 0 {
		t.Fatal("oversized batch was partially delivered")
	}
	for _, row := range e.Metrics().Edges {
		if row.EdgeID == branches[0] || row.EdgeID == branches[1] {
			if row.CapacityRejectedBatches != 1 || row.CapacityRejectedElements != 2 || row.QueueDroppedBatches != 0 {
				t.Fatalf("capacity rejection metrics=%+v", row)
			}
		}
	}
}

func TestBroadcastRegistrationAtomicAndExpansionBoundary(t *testing.T) {
	t.Run("duplicate and foreign", func(t *testing.T) {
		b := NewBuilder()
		source, _ := AddNode0(b, validSource())
		first, _ := AddNode1(b, validSink())
		second, _ := AddNode1(b, validSink())
		out := OutputPorts1(source.Outputs()).Out1
		if _, err := ConnectBroadcast(b, out, []InPort[int]{first.Inputs().In1, first.Inputs().In1}, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); !errors.Is(err, ErrInvalidBroadcast) {
			t.Fatalf("duplicate=%v", err)
		}
		foreign := NewBuilder()
		foreignSink, _ := AddNode1(foreign, validSink())
		if _, err := ConnectBroadcast(b, out, []InPort[int]{first.Inputs().In1, foreignSink.Inputs().In1}, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); !errors.Is(err, ErrForeignPort) {
			t.Fatalf("foreign=%v", err)
		}
		if len(b.edges) != 0 || b.outputs[endpointID{source.NodeID(), 1}].hasConsumer || b.inputs[endpointID{first.NodeID(), 1}].hasProducer {
			t.Fatal("rejected broadcast mutated topology")
		}
		_ = second
	})
	t.Run("group replication rejected", func(t *testing.T) {
		b := NewBuilder()
		source, _ := AddNode0(b, validSource())
		first, _ := AddNode1(b, validSink())
		second, _ := AddNode1(b, validSink())
		if _, err := ConnectBroadcast(b, OutputPorts1(source.Outputs()).Out1, []InPort[int]{first.Inputs().In1, second.Inputs().In1}, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
			t.Fatal(err)
		}
		bindCPU(t, b, DefaultGroupID, 0)
		if err := ReplicateGroup(b, DefaultGroupID, 2); err != nil {
			t.Fatal(err)
		}
		if err := BindReplicaCPU(b, DefaultGroupID, 1, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := b.Build(); !errors.Is(err, ErrBroadcastParallelExpansion) {
			t.Fatalf("parallel broadcast error=%v", err)
		}
	})
	t.Run("cycle rejected atomically", func(t *testing.T) {
		b := NewBuilder()
		source, _ := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} }}})
		upstream, _ := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} }}})
		other, _ := AddNode1(b, validSink())
		if _, err := Connect(b, OutputPorts1(upstream.Outputs()).Out1, source.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
			t.Fatal(err)
		}
		out := OutputPorts1(source.Outputs()).Out1
		if _, err := ConnectBroadcast(b, out, []InPort[int]{upstream.Inputs().In1, other.Inputs().In1}, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); !errors.Is(err, ErrInvalidBroadcast) {
			t.Fatalf("cycle error=%v", err)
		}
		if len(b.edges) != 1 || b.outputs[endpointID{source.NodeID(), 1}].hasConsumer || b.inputs[endpointID{other.NodeID(), 1}].hasProducer {
			t.Fatal("cycle rejection partially mutated topology")
		}
	})
}

func TestBroadcastCopyAndPublicationAllocs(t *testing.T) {
	metrics := &metricStore{edges: []*edgeCounters{{identity: EdgeMetrics{EdgeID: 1, QueueCapacity: 2}}, {identity: EdgeMetrics{EdgeID: 2, QueueCapacity: 2}}}}
	binding := &broadcastRuntime[int]{group: 1, source: endpointID{1, 1}, destinations: []endpointID{{2, 1}, {3, 1}}, config: queue.Config{Capacity: 2, BatchCapacity: 2, Policy: queue.Wait}, plans: []EdgeQueuePlan{{EdgeID: 1, BroadcastID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}, {EdgeID: 2, BroadcastID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}}, ready: []bool{true, true}, branches: []edgeMetadata{{id: 1}, {id: 2}}}
	bindings := &edgeBindingTarget{metrics: metrics}
	binding.current = nil
	if err := binding.bind(0, 1)(bindings, binding.plans[0]); err != nil {
		t.Fatal(err)
	}
	var output node.Output[int]
	var finalize func(bool) bool
	for _, configured := range bindings.outputs {
		finalize = configured.configure(unsafe.Pointer(&output))
	}
	var received [2]batch.Batch[int]
	var inputs [2]runtimeInput[int]
	for index, item := range bindings.inputs {
		item.input(unsafe.Pointer(&inputs[index]))
	}
	items := []int{3, 7}
	operation := func() {
		value, ok := output.Acquire()
		if !ok {
			panic("acquire")
		}
		copy(value.Mutable(), items)
		value.SetLen(2)
		output.Commit(&value)
		if !finalize(false) {
			panic("broadcast waited unexpectedly")
		}
		for index := range inputs {
			if !inputs[index].pop(&received[index]) {
				panic("branch missing")
			}
			received[index].Release()
		}
	}
	if allocations := testing.AllocsPerRun(100, operation); allocations != 0 {
		t.Fatalf("allocations=%v", allocations)
	}
}
