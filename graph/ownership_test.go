package graph

import (
	"testing"
	"time"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

func TestMovedInputRetainsContentsThroughGeneratedHandler(t *testing.T) {
	builder := NewBuilder()
	type sourceState struct{ emitted bool }
	source, err := AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs1[int], error]]{
		Factory: func() (sourceState, error) { return sourceState{}, nil },
		Handlers: node.Node0[sourceState, node.Outputs1[int], error]{Handle: func(state *sourceState, outputs *node.Outputs1[int]) node.Result[error] {
			if state.emitted {
				return node.Result[error]{}
			}
			value, ok := outputs.Out1.Acquire()
			if !ok {
				return node.Result[error]{}
			}
			items := value.Mutable()
			items[0], items[1], items[2] = 11, 22, 33
			value.SetLen(3)
			outputs.Out1.Commit(&value)
			state.emitted = true
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	middle, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(_ *struct{}, input batch.Batch[int], outputs *node.Outputs1[int]) node.Result[error] {
			if !outputs.Out1.Commit(&input) {
				return node.Result[error]{Failed: true}
			}
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	received := make(chan []int, 1)
	sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			items := append([]int(nil), input.Items()...)
			received <- items
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	config := EdgeConfig[int]{Capacity: 1, BatchCapacity: 3, Policy: queue.Wait}
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, middle.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(builder, OutputPorts1(middle.Outputs()).Out1, sink.Inputs().In1, config); err != nil {
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
		if len(got) != 3 || got[0] != 11 || got[1] != 22 || got[2] != 33 {
			t.Fatalf("forwarded items = %v, want [11 22 33]", got)
		}
	case <-time.After(time.Second):
		t.Fatal("forwarded input was not received")
	}
}

func TestMovedInputPreservesContentsAcrossLocalCustomAndRemoteEdges(t *testing.T) {
	for _, mode := range []string{"local", "custom", "remote"} {
		t.Run(mode, func(t *testing.T) {
			builder := NewBuilder()
			source, err := AddNode0(builder, node.Stateful[bool, node.Node0[bool, node.Outputs1[int], error]]{
				Factory: func() (bool, error) { return false, nil },
				Handlers: node.Node0[bool, node.Outputs1[int], error]{Handle: func(emitted *bool, outputs *node.Outputs1[int]) node.Result[error] {
					if *emitted {
						return node.Result[error]{}
					}
					value, ok := outputs.Out1.Acquire()
					if !ok {
						return node.Result[error]{}
					}
					value.Mutable()[0], value.Mutable()[1], value.Mutable()[2] = 5, 8, 13
					value.SetLen(3)
					outputs.Out1.Commit(&value)
					*emitted = true
					return node.Result[error]{}
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			middle, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{
				Factory: func() (struct{}, error) { return struct{}{}, nil },
				Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(_ *struct{}, input batch.Batch[int], outputs *node.Outputs1[int]) node.Result[error] {
					outputs.Out1.Commit(&input)
					return node.Result[error]{}
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			relay, err := AddClosureNode1(builder, func() (node.ClosureReplica[node.ClosureNode1[int, node.Outputs1[int], error]], error) {
				return node.ClosureReplica[node.ClosureNode1[int, node.Outputs1[int], error]]{Handlers: node.ClosureNode1[int, node.Outputs1[int], error]{Handle1: func(input batch.Batch[int], outputs *node.Outputs1[int]) node.Result[error] {
					outputs.Out1.Commit(&input)
					return node.Result[error]{}
				}}}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			received := make(chan []int, 1)
			sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
				Factory: func() (struct{}, error) { return struct{}{}, nil },
				Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
					received <- append([]int(nil), input.Items()...)
					return node.Result[error]{}
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			config := EdgeConfig[int]{Capacity: 1, BatchCapacity: 3, Policy: queue.Wait}
			if mode == "custom" {
				config.Factory = func(c queue.Config) (queue.QueueOps[int], error) {
					ring := newLocalRing[int](c.Capacity)
					return queue.QueueOps[int]{TryPush: ring.tryPush, TryPop: ring.tryPop, Len: ring.len}, nil
				}
			}
			if mode == "remote" {
				if err := Place(builder, relay.NodeID(), 2); err != nil {
					t.Fatal(err)
				}
				if err := Place(builder, sink.NodeID(), 2); err != nil {
					t.Fatal(err)
				}
			}
			for _, connection := range []struct {
				output OutPort[int]
				input  InPort[int]
			}{
				{OutputPorts1(source.Outputs()).Out1, middle.Inputs().In1},
				{OutputPorts1(middle.Outputs()).Out1, relay.Inputs().In1},
				{OutputPorts1(relay.Outputs()).Out1, sink.Inputs().In1},
			} {
				if _, err := Connect(builder, connection.output, connection.input, config); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "remote" {
				bindCPU(t, builder, DefaultGroupID, 0)
				bindCPU(t, builder, 2, 1)
			} else {
				bindCPU(t, builder, DefaultGroupID, 0)
			}
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
				if len(got) != 3 || got[0] != 5 || got[1] != 8 || got[2] != 13 {
					t.Fatalf("forwarded items = %v, want [5 8 13]", got)
				}
			case <-time.After(time.Second):
				t.Fatal("multi-hop forwarded batch was not received")
			}
		})
	}
}

func TestMovedTypedErrorInputPreservesValue(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "remote"}[remote], func(t *testing.T) {
			builder := NewBuilder()
			producer, err := AddNode0(builder, node.Stateful[bool, node.Node0[bool, node.Outputs0, int]]{
				Factory: func() (bool, error) { return false, nil },
				Handlers: node.Node0[bool, node.Outputs0, int]{Handle: func(emitted *bool, _ *node.Outputs0) node.Result[int] {
					if *emitted {
						return node.Result[int]{}
					}
					*emitted = true
					return node.Result[int]{Value: 73, Failed: true}
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			relay, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{
				Factory: func() (struct{}, error) { return struct{}{}, nil },
				Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(_ *struct{}, input batch.Batch[int], outputs *node.Outputs1[int]) node.Result[error] {
					outputs.Out1.Commit(&input)
					return node.Result[error]{}
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			received := make(chan []int, 1)
			sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
				Factory: func() (struct{}, error) { return struct{}{}, nil },
				Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
					received <- append([]int(nil), input.Items()...)
					return node.Result[error]{}
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ConnectError(builder, producer.Errors(), relay.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
				t.Fatal(err)
			}
			if _, err := Connect(builder, OutputPorts1(relay.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
				t.Fatal(err)
			}
			if remote {
				if err := Place(builder, producer.NodeID(), 3); err != nil {
					t.Fatal(err)
				}
				if err := Place(builder, relay.NodeID(), 7); err != nil {
					t.Fatal(err)
				}
				if err := Place(builder, sink.NodeID(), 7); err != nil {
					t.Fatal(err)
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
			executor, err := definition.Start()
			if err != nil {
				t.Fatal(err)
			}
			defer executor.Close()
			select {
			case got := <-received:
				if len(got) != 1 || got[0] != 73 {
					t.Fatalf("forwarded error items = %v, want [73]", got)
				}
			case <-time.After(time.Second):
				t.Fatal("forwarded typed error was not received")
			}
		})
	}
}
