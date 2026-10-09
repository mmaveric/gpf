package graph

import (
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func TestMetricsNilAndZero(t *testing.T) {
	for _, e := range []*Executor{nil, {}} {
		if got := e.Metrics(); !reflect.DeepEqual(got, MetricsSnapshot{}) {
			t.Fatalf("zero metrics: %+v", got)
		}
	}
}

func TestMetricsDeliveryAndRestart(t *testing.T) {
	for _, mode := range []string{"local", "custom", "remote"} {
		t.Run(mode, func(t *testing.T) {
			builder := NewBuilder()
			factoryCalls := 0
			delivered := make(chan struct{}, 4)
			source, err := AddNode0(builder, node.Stateful[int, node.Node0[int, node.Outputs1[int], int]]{
				Factory: func() (int, error) { factoryCalls++; return 0, nil },
				Handlers: node.Node0[int, node.Outputs1[int], int]{Handle: func(state *int, outputs *node.Outputs1[int]) node.Result[int] {
					if *state >= 2 {
						return node.Result[int]{}
					}
					value, ok := outputs.Out1.Acquire()
					if !ok {
						return node.Result[int]{}
					}
					*state++
					value.SetLen(3)
					outputs.Out1.Commit(&value)
					return node.Result[int]{Value: 9, Failed: *state == 2}
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			middle, err := AddClosureNode1(builder, func() (node.ClosureReplica[node.ClosureNode1[int, node.Outputs1[int], int]], error) {
				return node.ClosureReplica[node.ClosureNode1[int, node.Outputs1[int], int]]{Handlers: node.ClosureNode1[int, node.Outputs1[int], int]{Handle1: func(value batch.Batch[int], outputs *node.Outputs1[int]) node.Result[int] {
					outputs.Out1.Commit(&value)
					return node.Result[int]{}
				}}}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			newSink := func() NodePorts[InPorts1[int], node.Outputs0, int] {
				ports, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, int]]{
					Factory: func() (struct{}, error) { return struct{}{}, nil },
					Handlers: node.Node1[struct{}, int, node.Outputs0, int]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[int] {
						delivered <- struct{}{}
						return node.Result[int]{}
					}},
				})
				if err != nil {
					t.Fatal(err)
				}
				return ports
			}
			sink, errorSink := newSink(), newSink()
			config := EdgeConfig[int]{Capacity: 2, BatchCapacity: 3, Policy: queue.Wait}
			if mode == "custom" {
				config.Factory = func(c queue.Config) (queue.QueueOps[int], error) {
					ring := newLocalRing[int](c.Capacity)
					return queue.QueueOps[int]{TryPush: ring.tryPush, TryPop: ring.tryPop, Len: ring.len}, nil
				}
			}
			if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, middle.Inputs().In1, config); err != nil {
				t.Fatal(err)
			}
			if _, err := Connect(builder, OutputPorts1(middle.Outputs()).Out1, sink.Inputs().In1, config); err != nil {
				t.Fatal(err)
			}
			if _, err := ConnectError(builder, source.Errors(), errorSink.Inputs().In1, config); err != nil {
				t.Fatal(err)
			}
			if mode == "remote" {
				if err := Place(builder, source.NodeID(), 3); err != nil {
					t.Fatal(err)
				}
				for _, id := range []NodeID{middle.NodeID(), sink.NodeID(), errorSink.NodeID()} {
					if err := Place(builder, id, 7); err != nil {
						t.Fatal(err)
					}
				}
				bindCPU(t, builder, 3, 0)
				bindCPU(t, builder, 7, 1)
			} else {
				bindCPU(t, builder, DefaultGroupID, 0)
			}
			definition, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			if factoryCalls != 0 {
				t.Fatal("Build invoked factory")
			}
			var previous *Executor
			var final MetricsSnapshot
			for iteration := 0; iteration < 2; iteration++ {
				e, err := definition.Start()
				if err != nil {
					t.Fatal(err)
				}
				for count := 0; count < 2; count++ {
					select {
					case <-delivered:
					case <-time.After(5 * time.Second):
						_ = e.Close()
						t.Fatal("delivery timeout")
					}
				}
				if err := e.Drain(); err != nil {
					t.Fatal(err)
				}
				got := e.Metrics()
				if len(got.Nodes) != 4 || len(got.Edges) != 3 {
					t.Fatalf("rows: %+v", got)
				}
				if got.Nodes[0].FailedResults != 1 || got.Nodes[0].UnconnectedErrors != 0 || got.Nodes[0].InputBatches != 0 {
					t.Fatalf("source: %+v", got.Nodes[0])
				}
				if got.Nodes[1].HandlerCalls != 1 || got.Nodes[1].InputBatches != 1 || got.Nodes[1].InputElements != 3 {
					t.Fatalf("moved input: %+v", got.Nodes[1])
				}
				for index, edge := range got.Edges {
					length := uint64(3)
					if index == 2 {
						length = 1
					}
					if edge.AcceptedBatches != 1 || edge.DequeuedBatches != 1 || edge.AcceptedElements != length || edge.DequeuedElements != length || edge.QueueDepthSample != 0 {
						t.Fatalf("edge %d: %+v", index, edge)
					}
				}
				if got.Edges[0].FailedOutputBatches != 1 || got.Edges[0].FailedOutputElements != 3 || !got.Edges[2].ErrorEdge {
					t.Fatalf("failure boundaries: %+v", got.Edges)
				}
				if mode == "remote" && (got.Groups[0].GroupID != 3 || got.Groups[1].GroupID != 7 || !got.Edges[0].Remote) {
					t.Fatalf("sparse identities: %+v", got)
				}
				if previous != nil && !reflect.DeepEqual(previous.Metrics(), final) {
					t.Fatal("restart changed prior executor")
				}
				previous, final = e, e.Metrics()
				got.Nodes[0].FailedResults = 500
				got.Edges[0].EdgeID = 500
				got.Groups[0].GroupID = 500
				if !reflect.DeepEqual(e.Metrics(), final) {
					t.Fatal("snapshot aliases runtime")
				}
				if err := e.Close(); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(e.Metrics(), final) {
					t.Fatal("repeated stop changed totals")
				}
			}
		})
	}
}

func TestMetricsHeterogeneousSelectorInputCounts(t *testing.T) {
	builder := NewBuilder()
	intSource, err := AddNode0(builder, node.Stateful[bool, node.Node0[bool, node.Outputs1[int], int]]{
		Factory: func() (bool, error) { return false, nil },
		Handlers: node.Node0[bool, node.Outputs1[int], int]{Handle: func(emitted *bool, outputs *node.Outputs1[int]) node.Result[int] {
			if !*emitted {
				value, ok := outputs.Out1.Acquire()
				if ok {
					value.SetLen(1)
					outputs.Out1.Commit(&value)
					*emitted = true
				}
			}
			return node.Result[int]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	stringSource, err := AddNode0(builder, node.Stateful[bool, node.Node0[bool, node.Outputs1[string], error]]{
		Factory: func() (bool, error) { return false, nil },
		Handlers: node.Node0[bool, node.Outputs1[string], error]{Handle: func(emitted *bool, outputs *node.Outputs1[string]) node.Result[error] {
			if !*emitted {
				value, ok := outputs.Out1.Acquire()
				if ok {
					value.Mutable()[0], value.Mutable()[1] = "a", "b"
					value.SetLen(2)
					outputs.Out1.Commit(&value)
					*emitted = true
				}
			}
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var selectorCalls atomic.Uint64
	var intCalls, stringCalls atomic.Uint64
	receiver, err := AddNode2(builder, node.Stateful[struct{}, node.Node2[struct{}, int, string, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node2[struct{}, int, string, node.Outputs0, error]{
			Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] {
				intCalls.Add(1)
				return node.Result[error]{}
			},
			Handle2: func(*struct{}, batch.Batch[string], *node.Outputs0) node.Result[error] {
				stringCalls.Add(1)
				return node.Result[error]{}
			},
			Select: func(*struct{}, node.ReadyMask) node.PortIndex { selectorCalls.Add(1); return node.Port2 },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	config := EdgeConfig[int]{Capacity: 2, BatchCapacity: 2, Policy: queue.Wait}
	if _, err := Connect(builder, OutputPorts1(intSource.Outputs()).Out1, receiver.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(builder, OutputPorts1(stringSource.Outputs()).Out1, receiver.Inputs().In2, EdgeConfig[string]{Capacity: 2, BatchCapacity: 2, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Close()
	deadline := time.After(time.Second)
	for selectorCalls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("heterogeneous selector was not called")
		default:
			runtime.Gosched()
		}
	}
	if err := executor.Drain(); err != nil {
		t.Fatal(err)
	}
	metrics := executor.Metrics()
	if metrics.Nodes[0].HandlerCalls == 0 || metrics.Nodes[1].HandlerCalls == 0 || metrics.Nodes[0].InputBatches != 0 || metrics.Nodes[1].InputBatches != 0 {
		t.Fatalf("source metrics: %+v", metrics.Nodes[:2])
	}
	got := metrics.Nodes[2]
	if got.HandlerCalls != 2 || got.InputBatches != 2 || got.InputElements != 3 {
		t.Fatalf("selected heterogeneous input metrics: %+v", got)
	}
	if selectorCalls.Load() != 1 || intCalls.Load() != 1 || stringCalls.Load() != 1 {
		t.Fatalf("selector/int/string calls = %d/%d/%d, want 1/1/1", selectorCalls.Load(), intCalls.Load(), stringCalls.Load())
	}
}

// Exercise the installed typed output boundary synchronously so rejected pushes
// and retries have deterministic counts independent of scheduler speed.
func metricEdge(t *testing.T, policy queue.Policy, factory queue.Factory[int]) (*edgeBindingTarget, *edgeCounters, *node.Output[int], func(bool) bool, runtimeInput[int]) {
	return metricEdgeKind(t, policy, factory, false)
}

func metricEdgeKind(t *testing.T, policy queue.Policy, factory queue.Factory[int], isError bool) (*edgeBindingTarget, *edgeCounters, *node.Output[int], func(bool) bool, runtimeInput[int]) {
	t.Helper()
	metric := &edgeCounters{identity: EdgeMetrics{EdgeID: 1, QueueCapacity: 1, ErrorEdge: isError}}
	bindings := &edgeBindingTarget{metrics: &metricStore{edges: []*edgeCounters{metric}}}
	plan := EdgeQueuePlan{EdgeID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}
	if err := futureBinding[int](1, endpointID{1, 1}, endpointID{2, 1}, queue.Config{Capacity: 1, BatchCapacity: 4, Policy: policy}, factory, isError)(bindings, plan); err != nil {
		t.Fatal(err)
	}
	output := &node.Output[int]{}
	finalize := bindings.outputs[0].configure(unsafe.Pointer(output))
	var input runtimeInput[int]
	bindings.inputs[0].input(unsafe.Pointer(&input))
	return bindings, metric, output, finalize, input
}

func commitMetricBatch(t *testing.T, output *node.Output[int], length int) {
	t.Helper()
	value, ok := output.Acquire()
	if !ok {
		t.Fatal("acquisition failed")
	}
	if !value.SetLen(length) || !output.Commit(&value) {
		t.Fatal("commit failed")
	}
}

func TestMetricsWaitDropExhaustionAndCleanup(t *testing.T) {
	t.Run("wait", func(t *testing.T) {
		bindings, metric, output, finalize, input := metricEdge(t, queue.Wait, nil)
		commitMetricBatch(t, output, 4)
		if !finalize(false) {
			t.Fatal("first push")
		}
		commitMetricBatch(t, output, 4)
		if finalize(false) {
			t.Fatal("full push")
		}
		if finalize(false) || finalize(false) {
			t.Fatal("retry accepted full queue")
		}
		var value batch.Batch[int]
		if !input.pop(&value) {
			t.Fatal("pop")
		}
		value.Release()
		if !finalize(false) {
			t.Fatal("successful retry")
		}
		got := (&Executor{metrics: bindings.metrics}).Metrics().Edges[0]
		if got.WaitRetries != 3 || got.AcceptedBatches != 2 || got.AcceptedElements != 8 || got.DequeuedBatches != 1 || got.QueueDroppedBatches != 0 || got.QueueHighWaterSample != 1 {
			t.Fatalf("retry counts: %+v", got)
		}
		bindings.discarding = true
		bindings.drains[0]()
		if metric.discarded.Load() != 1 || metric.discardedElements.Load() != 4 {
			t.Fatal("queued discard")
		}
	})
	t.Run("drop and exhaustion", func(t *testing.T) {
		bindings, metric, output, finalize, input := metricEdge(t, queue.Drop, nil)
		commitMetricBatch(t, output, 3)
		finalize(false)
		commitMetricBatch(t, output, 2)
		finalize(false)
		if metric.producer.dropped.Load() != 1 || metric.producer.droppedElements.Load() != 2 {
			t.Fatal("drop count")
		}
		var held batch.Batch[int]
		input.pop(&held)
		commitMetricBatch(t, output, 1)
		finalize(false)
		if _, ok := output.Acquire(); ok {
			t.Fatal("expected exhaustion")
		}
		if metric.producer.exhausted.Load() != 1 {
			t.Fatal("exhaustion count")
		}
		held.Release()
		value, ok := output.Acquire()
		if !ok {
			t.Fatal("released slot unavailable")
		}
		if _, ok := output.Acquire(); ok {
			t.Fatal("duplicate Acquire")
		}
		if metric.producer.exhausted.Load() != 1 {
			t.Fatal("duplicate counted as exhaustion")
		}
		value.SetLen(4)
		output.Commit(&value)
		finalize(true)
		if metric.producer.failed.Load() != 1 || metric.producer.failedElements.Load() != 4 {
			t.Fatal("failed committed output")
		}
		bindings.discarding = true
		bindings.drains[0]()
	})
	t.Run("typed error pool exhaustion", func(t *testing.T) {
		bindings, _, output, finalize, input := metricEdgeKind(t, queue.Drop, nil, true)
		var helds [2]batch.Batch[int]
		for index := range helds {
			commitMetricBatch(t, output, 1)
			if !finalize(false) {
				t.Fatal("error publication unexpectedly pending")
			}
			var held batch.Batch[int]
			if !input.pop(&held) {
				t.Fatal("error pop")
			}
			if held.Len() != 1 {
				t.Fatalf("error batch length = %d, want 1", held.Len())
			}
			if held.Cap() != 4 {
				t.Fatalf("error batch capacity = %d, want 4", held.Cap())
			}
			// Retain the first popped error batch while the next error occupies the queue.
			helds[index] = held
		}
		if _, ok := output.Acquire(); ok {
			t.Fatal("error pool acquisition succeeded while both slots are owned")
		}
		got := (&Executor{metrics: bindings.metrics}).Metrics().Edges[0]
		if !got.ErrorEdge || got.PoolExhaustions != 1 || got.AcceptedBatches != 2 || got.QueueDroppedBatches != 0 {
			t.Fatalf("typed exhaustion metrics: %+v", got)
		}
		for index := range helds {
			helds[index].Release()
		}
		var queued batch.Batch[int]
		if input.pop(&queued) {
			queued.Release()
		}
	})
	t.Run("pending and queued shutdown", func(t *testing.T) {
		bindings, metric, output, finalize, _ := metricEdge(t, queue.Wait, nil)
		commitMetricBatch(t, output, 3)
		finalize(false)
		commitMetricBatch(t, output, 2)
		finalize(false)
		bindings.discarding = true
		finalize(true)
		bindings.drains[0]()
		if metric.discarded.Load() != 2 || metric.discardedElements.Load() != 5 || metric.consumer.dequeued.Load() != 0 || metric.producer.failed.Load() != 0 {
			t.Fatal("shutdown attribution")
		}
	})
}

func TestMetricsConcurrentReadersAndStop(t *testing.T) {
	for _, stop := range []string{"close", "drain"} {
		t.Run(stop, func(t *testing.T) {
			builder := NewBuilder()
			var emitted atomic.Uint64
			source, err := AddClosureNode0(builder, func() (node.ClosureReplica[node.ClosureNode0[node.Outputs0, int]], error) {
				return node.ClosureReplica[node.ClosureNode0[node.Outputs0, int]]{Handlers: node.ClosureNode0[node.Outputs0, int]{Handle: func(*node.Outputs0) node.Result[int] { emitted.Add(1); return node.Result[int]{Failed: true} }}}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, int]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, int]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[int] { return node.Result[int]{} }}})
			if err != nil {
				t.Fatal(err)
			}
			if err := Place(builder, sink.NodeID(), 7); err != nil {
				t.Fatal(err)
			}
			if _, err := ConnectError(builder, source.Errors(), sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
				t.Fatal(err)
			}
			bindCPU(t, builder, 1, 0)
			bindCPU(t, builder, 7, 1)
			d, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			e, err := d.Start()
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			var readers sync.WaitGroup
			for i := 0; i < 3; i++ {
				readers.Add(1)
				go func() {
					defer readers.Done()
					for {
						select {
						case <-done:
							return
						default:
							_ = e.Metrics()
						}
					}
				}()
			}
			if stop == "close" {
				err = e.Close()
			} else {
				err = e.Drain()
			}
			close(done)
			readers.Wait()
			if err != nil {
				t.Fatal(err)
			}
			got := e.Metrics()
			if got.Nodes[0].FailedResults != emitted.Load() || got.Nodes[0].UnconnectedErrors != 0 || got.Edges[0].QueueDepthSample != 0 {
				t.Fatalf("final: %+v", got)
			}
		})
	}
}

func TestMetricsUnconnectedAndCustomSnapshot(t *testing.T) {
	builder := NewBuilder()
	_, err := AddNode0(builder, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, int]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node0[struct{}, node.Outputs0, int]{Handle: func(*struct{}, *node.Outputs0) node.Result[int] { return node.Result[int]{Failed: true} }}})
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, 1, 0)
	d, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	got := e.Metrics().Nodes[0]
	if got.HandlerCalls != got.FailedResults || got.FailedResults != got.UnconnectedErrors || got.InputBatches != 0 {
		t.Fatalf("unconnected: %+v", got)
	}
	calls := 0
	bindings, metric, output, finalize, _ := metricEdge(t, queue.Drop, func(c queue.Config) (queue.QueueOps[int], error) {
		ring := newLocalRing[int](c.Capacity)
		return queue.QueueOps[int]{TryPush: ring.tryPush, TryPop: ring.tryPop, Len: func() uint32 { calls++; return ring.len() }}, nil
	})
	commitMetricBatch(t, output, 1)
	finalize(false)
	before := calls
	_ = (&Executor{metrics: bindings.metrics}).Metrics()
	if calls != before || metric.high.Load() != 1 {
		t.Fatal("snapshot called custom queue")
	}
	bindings.drains[0]()
}

func TestMetricsInstrumentedEdgeAllocs(t *testing.T) {
	for _, policy := range []queue.Policy{queue.Drop, queue.Wait} {
		t.Run(map[queue.Policy]string{queue.Drop: "drop", queue.Wait: "wait"}[policy], func(t *testing.T) {
			_, _, output, finalize, input := metricEdge(t, policy, nil)
			var first, second, third, held batch.Batch[int]
			operation := func() {
				var ok bool
				first, ok = output.Acquire()
				if !ok {
					panic("acquire")
				}
				first.SetLen(1)
				output.Commit(&first)
				finalize(false)
				second, ok = output.Acquire()
				if !ok {
					panic("acquire")
				}
				second.SetLen(1)
				output.Commit(&second)
				finalize(false)
				input.pop(&held)
				if policy == queue.Wait {
					finalize(false)
				} else {
					third, ok = output.Acquire()
					if !ok {
						panic("acquire")
					}
					output.Commit(&third)
					finalize(false)
				}
				if _, ok := output.Acquire(); ok {
					panic("exhaustion")
				}
				held.Release()
				input.pop(&held)
				held.Release()
			}
			if allocations := testing.AllocsPerRun(100, operation); allocations != 0 {
				t.Fatalf("allocations = %v", allocations)
			}
		})
	}
}

func TestMetricsRemoteOrdinaryAllocs(t *testing.T) {
	type gate struct {
		requested atomic.Uint64
		delivered atomic.Uint64
	}
	type sourceState struct {
		gate    *gate
		emitted uint64
	}
	shared := &gate{}
	builder := NewBuilder()
	source, err := AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs1[int], error]]{
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
			value.Mutable()[0] = int(requested)
			value.SetLen(1)
			outputs.Out1.Commit(&value)
			state.emitted = requested
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, _ batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			shared.delivered.Add(1)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, sink.NodeID(), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	bindCPU(t, builder, 2, 1)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Close()
	request := func() {
		target := shared.requested.Add(1)
		for shared.delivered.Load() < target {
			runtime.Gosched()
		}
	}
	request()
	if allocations := testing.AllocsPerRun(100, request); allocations != 0 {
		t.Fatalf("remote ordinary allocations = %f", allocations)
	}
}
