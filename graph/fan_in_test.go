package graph

import (
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func TestFanInMergesOriginsWithoutOrderingAssumption(t *testing.T) {
	b := NewBuilder()
	type sourceState struct {
		emitted bool
		value   int
	}
	addSource := func(value int, group GroupID) NodePorts[InPorts0, node.Outputs1[int], error] {
		ports, err := AddNode0(b, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs1[int], error]]{
			Factory: func() (sourceState, error) { return sourceState{value: value}, nil },
			Handlers: node.Node0[sourceState, node.Outputs1[int], error]{Handle: func(s *sourceState, out *node.Outputs1[int]) node.Result[error] {
				if s.emitted {
					return node.Result[error]{}
				}
				v, ok := out.Out1.Acquire()
				if ok {
					v.Mutable()[0] = s.value
					v.SetLen(1)
					out.Out1.Commit(&v)
					s.emitted = true
				}
				return node.Result[error]{}
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := Place(b, ports.NodeID(), group); err != nil {
			t.Fatal(err)
		}
		return ports
	}
	a, c, d := addSource(17, 1), addSource(31, 2), addSource(47, 3)
	var received atomic.Uint64
	seen := make(map[int]bool)
	var seenMu atomic.Bool
	sink, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, value batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			for !seenMu.CompareAndSwap(false, true) {
				runtime.Gosched()
			}
			n := value.Items()[0]
			if seen[n] {
				t.Errorf("fan-in duplicate %d", n)
			}
			seen[n] = true
			seenMu.Store(false)
			received.Add(1)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sink.NodeID(), 4); err != nil {
		t.Fatal(err)
	}
	branches, err := ConnectFanIn(b, []OutPort[int]{OutputPorts1(a.Outputs()).Out1, OutputPorts1(c.Outputs()).Out1, OutputPorts1(d.Outputs()).Out1}, sink.Inputs().In1, EdgeConfig[int]{Capacity: 8, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 3 || branches[0] >= branches[1] || branches[1] >= branches[2] {
		t.Fatalf("branch IDs=%v", branches)
	}
	for group, cpu := range map[GroupID]int{1: 0, 2: 1, 3: 2, 4: 3} {
		if err := BindCPU(b, group, cpu); err != nil {
			t.Fatal(err)
		}
	}
	definition, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	plans := definition.EdgeQueuePlans()
	if len(plans) != 3 {
		t.Fatalf("plans=%+v", plans)
	}
	for _, plan := range plans {
		if plan.MergeID != branches[0] || plan.Model != QueueModelMPSC || plan.ProducerWorkers != 3 || plan.ConsumerWorkers != 1 {
			t.Fatalf("fan-in plan=%+v", plan)
		}
	}
	e, err := definition.Start()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for received.Load() != 3 {
		if time.Now().After(deadline) {
			_ = e.Close()
			t.Fatal("fan-in deliveries timed out")
		}
		runtime.Gosched()
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 || !seen[17] || !seen[31] || !seen[47] {
		t.Fatalf("received=%v", seen)
	}
	metrics := e.Metrics()
	for index, id := range branches {
		row := metrics.Edges[index]
		if row.EdgeID != id || row.MergeID != branches[0] || row.AcceptedBatches != 1 || row.DequeuedBatches != 1 || row.AcceptedElements != row.DequeuedElements {
			t.Fatalf("branch metrics=%+v", row)
		}
	}
	for _, pool := range metrics.Pools {
		if pool.EdgeID != branches[0] && pool.MergeID != branches[0] {
			continue
		}
		if pool.InUseSample != 0 {
			t.Fatalf("producer pool leaked=%+v", pool)
		}
	}
}

func TestFanInRegistrationIsAtomicAndRejectsCycle(t *testing.T) {
	t.Run("duplicate and foreign producers", func(t *testing.T) {
		b := NewBuilder()
		a, _ := AddNode0(b, validSource())
		z, _ := AddNode1(b, validSink())
		priorSource, _ := AddNode0(b, validSource())
		priorSink, _ := AddNode1(b, validSink())
		if _, err := Connect(b, OutputPorts1(priorSource.Outputs()).Out1, priorSink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
			t.Fatal(err)
		}
		out := OutputPorts1(a.Outputs()).Out1
		if _, err := ConnectFanIn(b, []OutPort[int]{out, out}, z.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); !errors.Is(err, ErrFanInUnsupported) {
			t.Fatalf("duplicate error=%v", err)
		}
		foreign := NewBuilder()
		other, _ := AddNode0(foreign, validSource())
		if _, err := ConnectFanIn(b, []OutPort[int]{out, OutputPorts1(other.Outputs()).Out1}, z.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); !errors.Is(err, ErrForeignPort) {
			t.Fatalf("foreign error=%v", err)
		}
		if len(b.edges) != 1 || !b.outputs[endpointID{priorSource.NodeID(), 1}].hasConsumer || b.outputs[endpointID{a.NodeID(), 1}].hasConsumer || b.inputs[endpointID{z.NodeID(), 1}].hasProducer {
			t.Fatal("rejected fan-in partially mutated topology")
		}
	})
	t.Run("cycle", func(t *testing.T) {
		b := NewBuilder()
		a, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} }}})
		if err != nil {
			t.Fatal(err)
		}
		middle, _ := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} }}})
		other, _ := AddNode0(b, validSource())
		if _, err := Connect(b, OutputPorts1(a.Outputs()).Out1, middle.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
			t.Fatal(err)
		}
		if _, err := ConnectFanIn(b, []OutPort[int]{OutputPorts1(middle.Outputs()).Out1, OutputPorts1(other.Outputs()).Out1}, a.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err == nil {
			t.Fatal("cyclic fan-in accepted")
		}
	})
}

func TestFanInRejectsRemoteCustomFactoryAtomically(t *testing.T) {
	b := NewBuilder()
	a, _ := AddNode0(b, validSource())
	c, _ := AddNode0(b, validSource())
	sink, _ := AddNode1(b, validSink())
	if err := Place(b, c.NodeID(), 8); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sink.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	calls := 0
	factory := func(config queue.Config) (queue.QueueOps[int], error) { calls++; return queue.QueueOps[int]{}, nil }
	_, err := ConnectFanIn(b, []OutPort[int]{OutputPorts1(a.Outputs()).Out1, OutputPorts1(c.Outputs()).Out1}, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait, Factory: factory})
	if !errors.Is(err, ErrIncompatibleQueueFactory) || calls != 0 || len(b.edges) != 0 {
		t.Fatalf("error/calls/edges=%v/%d/%d", err, calls, len(b.edges))
	}
}

func fanInBlockedSink(t *testing.T, policy queue.Policy) (*Executor, []EdgeID, <-chan struct{}, chan struct{}) {
	t.Helper()
	b := NewBuilder()
	sources := make([]NodePorts[InPorts0, node.Outputs1[int], error], 3)
	for index := range sources {
		value := index + 1
		ports, err := AddNode0(b, node.Stateful[bool, node.Node0[bool, node.Outputs1[int], error]]{
			Factory: func() (bool, error) { return false, nil },
			Handlers: node.Node0[bool, node.Outputs1[int], error]{Handle: func(emitted *bool, outputs *node.Outputs1[int]) node.Result[error] {
				if *emitted {
					return node.Result[error]{}
				}
				batchValue, ok := outputs.Out1.Acquire()
				if ok {
					batchValue.Mutable()[0] = value
					batchValue.SetLen(1)
					outputs.Out1.Commit(&batchValue)
					*emitted = true
				}
				return node.Result[error]{}
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		sources[index] = ports
		if err := Place(b, ports.NodeID(), GroupID(index+1)); err != nil {
			t.Fatal(err)
		}
	}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	sink, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] {
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sink.NodeID(), 4); err != nil {
		t.Fatal(err)
	}
	outputs := make([]OutPort[int], len(sources))
	for index, source := range sources {
		outputs[index] = OutputPorts1(source.Outputs()).Out1
	}
	branches, err := ConnectFanIn(b, outputs, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: policy})
	if err != nil {
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
		t.Fatal("sink handler did not block")
	}
	return e, branches, started, release
}

func TestFanInDropAndWaitCleanupOnce(t *testing.T) {
	for _, policy := range []queue.Policy{queue.Drop, queue.Wait} {
		t.Run(map[queue.Policy]string{queue.Drop: "drop", queue.Wait: "wait"}[policy], func(t *testing.T) {
			e, branches, _, release := fanInBlockedSink(t, policy)
			waitForLogger(t, func() bool {
				metrics := e.Metrics().Edges
				var dropped, retries uint64
				for _, row := range metrics {
					dropped += row.QueueDroppedBatches
					retries += row.WaitRetries
				}
				if policy == queue.Drop {
					return dropped > 0
				}
				return retries > 0
			})
			done := make(chan error, 1)
			go func() { done <- e.Close() }()
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
				t.Fatal("fan-in Close failed to join")
			}
			got := e.Metrics()
			var accepted, dropped, dequeued, discarded uint64
			for index, row := range got.Edges {
				if row.EdgeID != branches[index] || row.MergeID != branches[0] {
					t.Fatalf("branch identity=%+v", row)
				}
				accepted += row.AcceptedBatches
				dropped += row.QueueDroppedBatches
				dequeued += row.DequeuedBatches
				discarded += row.ShutdownDiscardedBatches
			}
			if policy == queue.Drop && dropped == 0 {
				t.Fatalf("Drop lost rejection count: %+v", got.Edges)
			}
			if policy == queue.Wait && dropped != 0 {
				t.Fatalf("Wait incorrectly dropped: %+v", got.Edges)
			}
			if dequeued > accepted || dequeued+discarded < accepted || dequeued+discarded > uint64(len(branches)) {
				t.Fatalf("fan-in accepted/pending ownership not accounted: accepted=%d dequeued=%d discarded=%d", accepted, dequeued, discarded)
			}
			for _, pool := range got.Pools {
				if pool.InUseSample != 0 {
					t.Fatalf("origin pool leaked: %+v", pool)
				}
			}
		})
	}
}

func TestFanInSharedPublicationAllocs(t *testing.T) {
	merge := &fanInRuntime[int]{mergeID: 1, sources: []endpointID{{1, 1}, {2, 1}}, target: endpointID{3, 1}, config: queue.Config{Capacity: 4, BatchCapacity: 1, Policy: queue.Wait}, branchIDs: []EdgeID{1, 2}}
	metrics := &metricStore{edges: []*edgeCounters{{identity: EdgeMetrics{EdgeID: 1, MergeID: 1, QueueCapacity: 4}}, {identity: EdgeMetrics{EdgeID: 2, MergeID: 1, QueueCapacity: 4}}}}
	bindings := &edgeBindingTarget{metrics: metrics}
	plan := EdgeQueuePlan{EdgeID: 1, MergeID: 1, Model: QueueModelMPSC, ProducerWorkers: 2, ConsumerWorkers: 1, Remote: true, endpoints: &edgeEndpoints{sources: merge.sources, targets: []endpointID{merge.target}}}
	if err := merge.bind(1, merge.sources[0])(bindings, plan); err != nil {
		t.Fatal(err)
	}
	var outputs [2]node.Output[int]
	var finalize [2]func(bool) bool
	for _, binding := range bindings.outputs {
		if binding.endpoint.nodeID == 1 {
			finalize[0] = binding.configure(unsafe.Pointer(&outputs[0]))
		} else {
			finalize[1] = binding.configure(unsafe.Pointer(&outputs[1]))
		}
	}
	var input runtimeInput[int]
	bindings.inputs[0].input(unsafe.Pointer(&input))
	var received [2]batch.Batch[int]
	op := func() {
		for index := range outputs {
			value, ok := outputs[index].Acquire()
			if !ok {
				panic("pool unavailable")
			}
			value.Mutable()[0] = index + 10
			value.SetLen(1)
			if !outputs[index].Commit(&value) {
				panic("commit rejected")
			}
			if !finalize[index](false) {
				panic("Wait unexpectedly full")
			}
		}
		for index := range received {
			if !input.pop(&received[index]) {
				panic("fan-in pop failed")
			}
		}
		for index := range received {
			received[index].Release()
		}
	}
	if allocations := testing.AllocsPerRun(100, op); allocations != 0 {
		t.Fatalf("fan-in allocations=%v", allocations)
	}
}
