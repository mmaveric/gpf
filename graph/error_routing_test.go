package graph

import (
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

func TestErrorRoutingAllocs(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "remote"}[remote], func(t *testing.T) {
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
			producer, err := AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs0, int]]{
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
			if remote {
				if err := Place(builder, sink.NodeID(), 2); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := ConnectError(builder, producer.Errors(), sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
				t.Fatal(err)
			}
			bindCPU(t, builder, DefaultGroupID, 0)
			if remote {
				bindCPU(t, builder, 2, 1)
			}
			definition, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			executor, err := definition.Start()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = executor.Close() })
			request := func() {
				target := shared.requested.Add(1)
				for shared.delivered.Load() < target {
					runtime.Gosched()
				}
			}
			request()
			if allocations := testing.AllocsPerRun(100, request); allocations != 0 {
				t.Fatalf("error delivery allocations = %f", allocations)
			}
		})
	}
}

func TestConnectErrorRecordsTypedEdgeAndSharesCardinality(t *testing.T) {
	builder := NewBuilder()
	producer, err := AddNode0(builder, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs1[string], string]]{})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, string, node.Outputs0, error]]{})
	if err != nil {
		t.Fatal(err)
	}
	config := EdgeConfig[string]{Capacity: 1, BatchCapacity: 2, Policy: queue.Wait}
	id, err := ConnectError(builder, producer.Errors(), sink.Inputs().In1, config)
	if err != nil || id != 1 {
		t.Fatalf("ConnectError() = (%d, %v), want (1, nil)", id, err)
	}
	if !builder.edges[0].isError || builder.edges[0].config.BatchCapacity != 2 {
		t.Fatalf("stored error edge = %#v", builder.edges[0])
	}
	if id, err := Connect(builder, OutputPorts1(producer.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[string]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); id != InvalidEdgeID || !errors.Is(err, ErrInputConnected) {
		t.Fatalf("ordinary connection after error input = (%d, %v)", id, err)
	}
	if id, err := ConnectError(builder, producer.Errors(), inputPort[string](t, builder), config); id != InvalidEdgeID || !errors.Is(err, ErrFanOutUnsupported) {
		t.Fatalf("error fan-out = (%d, %v)", id, err)
	}
}

func TestConnectErrorRejectsInvalidHandlesAndConfigurationAtomically(t *testing.T) {
	builder := NewBuilder()
	output := errorPort[int](t, builder)
	input := inputPort[int](t, builder)
	factoryCalls := 0
	config := EdgeConfig[int]{
		Capacity:      1,
		BatchCapacity: 1,
		Policy:        queue.Drop,
		Factory: func(queue.Config) (queue.QueueOps[int], error) {
			factoryCalls++
			return queue.QueueOps[int]{}, nil
		},
	}
	config.Capacity = 0
	if id, err := ConnectError(builder, output, input, config); id != InvalidEdgeID || !errors.Is(err, ErrInvalidEdgeConfig) {
		t.Fatalf("invalid configuration = (%d, %v)", id, err)
	}
	if len(builder.edges) != 0 || factoryCalls != 0 || builder.inputs[endpointID{nodeID: input.NodeID(), portID: input.PortID()}].hasProducer {
		t.Fatal("rejected error connection mutated topology or invoked factory")
	}
	if id, err := ConnectError(builder, ErrorPort[int]{}, input, config); id != InvalidEdgeID || !errors.Is(err, ErrInvalidPort) {
		t.Fatalf("invalid error port = (%d, %v)", id, err)
	}
	foreign := errorPort[int](t, NewBuilder())
	if id, err := ConnectError(builder, foreign, input, config); id != InvalidEdgeID || !errors.Is(err, ErrForeignPort) {
		t.Fatalf("foreign error port = (%d, %v)", id, err)
	}
}

func TestValidateIncludesErrorEdgesAndRejectsRemoteFactoryDormantly(t *testing.T) {
	builder := NewBuilder()
	producer, err := AddNode0(builder, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, int]]{
		Factory:  func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs0, int]{Handle: func(*struct{}, *node.Outputs0) node.Result[int] { return node.Result[int]{} }},
	})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory:  func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} }},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, sink.NodeID(), 2); err != nil {
		t.Fatal(err)
	}
	calls := 0
	id, err := ConnectError(builder, producer.Errors(), sink.Inputs().In1, EdgeConfig[int]{
		Capacity: 1, BatchCapacity: 1, Policy: queue.Drop,
		Factory: func(queue.Config) (queue.QueueOps[int], error) {
			calls++
			return queue.QueueOps[int]{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	bindCPU(t, builder, 2, 1)
	report, err := builder.Validate()
	var validationError *ValidationError
	if !errors.As(err, &validationError) || calls != 0 {
		t.Fatalf("Validate() = (%#v, %v), factory calls=%d", report, err, calls)
	}
	if issues := report.Issues(); len(issues) != 1 || issues[0].Code != ValidationCodeRemoteQueueFactory || issues[0].EdgeID != id {
		t.Fatalf("validation issues = %#v", issues)
	}
	if forward := report.ForwardAdjacency(); len(forward) != 2 || len(forward[0].Edges) != 1 || forward[0].Edges[0].EdgeID != id {
		t.Fatalf("error edge missing from forward adjacency: %#v", forward)
	}
}

func TestValidateRejectsErrorEdgeCycle(t *testing.T) {
	builder := NewBuilder()
	newNode := func() NodePorts[InPorts1[int], node.Outputs0, int] {
		ports, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, int]]{
			Factory:  func() (struct{}, error) { return struct{}{}, nil },
			Handlers: node.Node1[struct{}, int, node.Outputs0, int]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[int] { return node.Result[int]{} }},
		})
		if err != nil {
			t.Fatal(err)
		}
		return ports
	}
	first, second := newNode(), newNode()
	config := EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}
	if _, err := ConnectError(builder, first.Errors(), second.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	if _, err := ConnectError(builder, second.Errors(), first.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	report, err := builder.Validate()
	if err == nil || len(report.TopologicalOrder()) != 0 {
		t.Fatalf("Validate() = (%#v, %v)", report, err)
	}
	for _, issue := range report.Issues() {
		if issue.Code == ValidationCodeCycle {
			return
		}
	}
	t.Fatal("error-only cycle was not reported")
}

func TestExecutorRoutesFailedResultToTypedErrorInput(t *testing.T) {
	builder := NewBuilder()
	type sourceState struct{ emitted bool }
	producer, err := AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs0, string]]{
		Factory: func() (sourceState, error) { return sourceState{}, nil },
		Handlers: node.Node0[sourceState, node.Outputs0, string]{
			Handle: func(state *sourceState, _ *node.Outputs0) node.Result[string] {
				if state.emitted {
					return node.Result[string]{}
				}
				state.emitted = true
				return node.Result[string]{Value: "failed", Failed: true}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	received := make(chan string, 1)
	sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, string, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, string, node.Outputs0, error]{
			Handle1: func(_ *struct{}, input batch.Batch[string], _ *node.Outputs0) node.Result[error] {
				if input.Len() != 1 || input.Cap() != 3 {
					t.Errorf("error batch dimensions = %d/%d", input.Len(), input.Cap())
				}
				received <- input.Items()[0]
				return node.Result[error]{}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ConnectError(builder, producer.Errors(), sink.Inputs().In1, EdgeConfig[string]{Capacity: 1, BatchCapacity: 3, Policy: queue.Drop}); err != nil {
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
	select {
	case got := <-received:
		if got != "failed" {
			t.Fatalf("error value = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("failed result was not routed")
	}
}

func TestExecutorRoutesTypedErrorAcrossGroups(t *testing.T) {
	builder := NewBuilder()
	type sourceState struct{ emitted bool }
	producer, err := AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs0, int]]{
		Factory: func() (sourceState, error) { return sourceState{}, nil },
		Handlers: node.Node0[sourceState, node.Outputs0, int]{
			Handle: func(state *sourceState, _ *node.Outputs0) node.Result[int] {
				if state.emitted {
					return node.Result[int]{}
				}
				state.emitted = true
				return node.Result[int]{Value: 17, Failed: true}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	received := make(chan int, 1)
	sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{
			Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
				received <- input.Items()[0]
				return node.Result[error]{}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, producer.NodeID(), 3); err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, sink.NodeID(), 7); err != nil {
		t.Fatal(err)
	}
	if _, err := ConnectError(builder, producer.Errors(), sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, 3, 0)
	bindCPU(t, builder, 7, 1)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.edgeRemote) != 1 || !definition.edgeRemote[0] {
		t.Fatalf("error edge classification = %v", definition.edgeRemote)
	}
	executor, err := definition.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Close()
	select {
	case got := <-received:
		if got != 17 {
			t.Fatalf("remote error value = %d", got)
		}
	case <-time.After(time.Second):
		t.Fatal("remote failed result was not routed")
	}
}
