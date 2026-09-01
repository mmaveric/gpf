package graph

import (
	"errors"
	"testing"
	"time"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func TestExecutorExplicitPipelineAndSequentialReuse(t *testing.T) {
	builder := NewBuilder()
	type sourceState struct{ emitted bool }
	var received []int
	delivered := make(chan struct{}, 2)
	source, err := AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs1[int], error]]{
		Factory: func() (sourceState, error) { return sourceState{}, nil },
		Handlers: node.Node0[sourceState, node.Outputs1[int], error]{
			Handle: func(state *sourceState, outputs *node.Outputs1[int]) node.Result[error] {
				if state.emitted {
					return node.Result[error]{}
				}
				state.emitted = true
				value, ok := outputs.Out1.Acquire()
				if !ok {
					t.Fatal("source output exhausted")
				}
				value.Mutable()[0] = 7
				value.SetLen(1)
				if !outputs.Out1.Commit(&value) {
					t.Fatal("source output commit failed")
				}
				return node.Result[error]{}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{
			Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
				received = append(received, input.Items()[0])
				delivered <- struct{}{}
				return node.Result[error]{}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		executor, err := definition.Start()
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-delivered:
		case <-time.After(time.Second):
			t.Fatal("source batch was not delivered")
		}
		if err := executor.Drain(); err != nil {
			t.Fatal(err)
		}
		if err := executor.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if len(received) != 2 || received[0] != 7 || received[1] != 7 {
		t.Fatalf("received = %v, want [7 7]", received)
	}
}

func TestExecutorClosureAndQueueRollback(t *testing.T) {
	builder := NewBuilder()
	var closes int
	_, err := AddClosureNode0(builder, func() (node.ClosureReplica[node.ClosureNode0[node.Outputs0, error]], error) {
		return node.ClosureReplica[node.ClosureNode0[node.Outputs0, error]]{
			Close: func() error { closes++; return nil },
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if executor, err := definition.Start(); executor != nil || !errors.Is(err, errInvalidClosureReplica) || closes != 1 {
		t.Fatalf("Start() = (%v, %v), closes=%d", executor, err, closes)
	}

	invalid := NewBuilder()
	source, _ := AddNode0(invalid, validSource())
	sink, _ := AddNode1(invalid, validSink())
	var queueFactoryCalls int
	if _, err := Connect(invalid, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{
		Capacity:      1,
		BatchCapacity: 1,
		Policy:        queue.Drop,
		Factory: func(config queue.Config) (queue.QueueOps[int], error) {
			queueFactoryCalls++
			if config.BatchCapacity != 1 {
				t.Fatalf("queue config = %#v", config)
			}
			return queue.QueueOps[int]{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, invalid, DefaultGroupID, 0)
	definition, err = invalid.Build()
	if err != nil {
		t.Fatal(err)
	}
	if executor, err := definition.Start(); executor != nil || !errors.Is(err, ErrInvalidQueueOps) || queueFactoryCalls != 1 {
		t.Fatalf("Start() = (%v, %v), factory calls=%d, want ErrInvalidQueueOps", executor, err, queueFactoryCalls)
	}
}

func TestExecutorInitRollbackAndLocalRing(t *testing.T) {
	builder := NewBuilder()
	var closed int
	_, err := AddNode0(builder, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs0, error]{
			Handle: func(*struct{}, *node.Outputs0) node.Result[error] { return node.Result[error]{} },
		},
		Init:  func(*struct{}) error { return errors.New("init failed") },
		Close: func(*struct{}) error { closed++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if executor, err := definition.Start(); executor != nil || err == nil || closed != 1 {
		t.Fatalf("Start() = (%v, %v), closes=%d", executor, err, closed)
	}
	var zero Definition
	if executor, err := zero.Start(); executor != nil || !errors.Is(err, ErrInvalidDefinition) {
		t.Fatalf("zero Start() = (%v, %v)", executor, err)
	}

	ring := newLocalRing[int](2)
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 2, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := pool.Acquire()
	second, _ := pool.Acquire()
	if !ring.tryPush(first) || !ring.tryPush(second) || ring.tryPush(batch.Batch[int]{}) || ring.len() != 2 {
		t.Fatal("local ring capacity handling failed")
	}
	var got batch.Batch[int]
	if !ring.tryPop(&got) || got.Cap() != 1 {
		t.Fatal("local ring pop failed")
	}
	got.Release()
	if !ring.tryPop(&got) {
		t.Fatal("local ring FIFO delivery failed")
	}
	got.Release()
	if allocations := testing.AllocsPerRun(1_000, func() {
		value, ok := pool.Acquire()
		if !ok || !ring.tryPush(value) || !ring.tryPop(&got) {
			t.Fatal("local ring allocation path failed")
		}
		got.Release()
	}); allocations != 0 {
		t.Fatalf("local ring allocations = %f, want 0", allocations)
	}
}
