package graph

import (
	"errors"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/internal/benchtest"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

func TestPlacePrecedenceAndImmutablePlan(t *testing.T) {
	t.Parallel()

	var zero Builder
	if err := Place(&zero, 1, InvalidGroupID); !errors.Is(err, ErrInvalidBuilder) {
		t.Fatalf("zero Place() error = %v, want ErrInvalidBuilder", err)
	}
	builder := NewBuilder()
	source, err := AddNode0(builder, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(builder, validSink())
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, source.NodeID(), InvalidGroupID); !errors.Is(err, ErrInvalidGroupID) {
		t.Fatalf("zero group Place() error = %v", err)
	}
	if err := Place(builder, 99, 3); !errors.Is(err, ErrUnknownNode) {
		t.Fatalf("unknown Place() error = %v", err)
	}
	if err := Place(builder, source.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, source.NodeID(), 3); !errors.Is(err, ErrNodeAlreadyPlaced) {
		t.Fatalf("duplicate Place() error = %v", err)
	}
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	bindCPU(t, builder, 9, 1)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := definition.placements, []GroupID{0, 9, DefaultGroupID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("placements = %v, want %v", got, want)
	}
	if got, want := definition.groups, []groupPlan{{id: DefaultGroupID, cpu: 0, nodes: []NodeID{sink.NodeID()}}, {id: 9, cpu: 1, nodes: []NodeID{source.NodeID()}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("group plan = %#v, want %#v", got, want)
	}
	if len(definition.edgeRemote) != 1 || !definition.edgeRemote[0] {
		t.Fatalf("edge classification = %v, want one remote edge", definition.edgeRemote)
	}
	builder.placements[source.NodeID()] = 3
	if definition.placements[source.NodeID()] != 9 || definition.groups[1].nodes[0] != source.NodeID() {
		t.Fatal("definition retained mutable placement storage")
	}
	if err := Place(builder, sink.NodeID(), 3); !errors.Is(err, ErrBuilderSealed) {
		t.Fatalf("sealed Place() error = %v", err)
	}
}

func TestBuildRejectsRemoteFactoryWithoutInvocation(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	source, _ := AddNode0(builder, validSource())
	sink, _ := AddNode1(builder, validSink())
	if err := Place(builder, sink.NodeID(), 7); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{
		Capacity: 1, BatchCapacity: 1, Policy: queue.Drop,
		Factory: func(queue.Config) (queue.QueueOps[int], error) {
			calls++
			return queue.QueueOps[int]{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	bindCPU(t, builder, 7, 1)
	report, err := builder.Validate()
	var validationError *ValidationError
	if !errors.As(err, &validationError) || calls != 0 || !reflect.DeepEqual(validationCodes(report.Issues()), []ValidationCode{ValidationCodeRemoteQueueFactory}) {
		t.Fatalf("Validate() = (%#v, %v), calls=%d", report, err, calls)
	}
	if definition, err := builder.Build(); definition != nil || !errors.As(err, &validationError) || builder.state != builderOpen || calls != 0 {
		t.Fatalf("Build() = (%p, %v), state=%d calls=%d", definition, err, builder.state, calls)
	}
}

func TestRemoteRingFIFOConcurrentAndAllocs(t *testing.T) {
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 8, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	ring := newRemoteRing[int](8)
	const count = 1_000
	var producers, consumers sync.WaitGroup
	producers.Add(1)
	go func() {
		defer producers.Done()
		for value := 0; value < count; value++ {
			batchValue, ok := pool.Acquire()
			for !ok {
				runtime.Gosched()
				batchValue, ok = pool.Acquire()
			}
			batchValue.Mutable()[0] = value
			batchValue.SetLen(1)
			for !ring.tryPush(batchValue) {
				runtime.Gosched()
			}
		}
	}()
	consumers.Add(1)
	go func() {
		defer consumers.Done()
		for want := 0; want < count; want++ {
			var got batch.Batch[int]
			for !ring.tryPop(&got) {
				runtime.Gosched()
			}
			if got.Items()[0] != want {
				t.Errorf("remote FIFO value = %d, want %d", got.Items()[0], want)
			}
			got.Release()
		}
	}()
	producers.Wait()
	consumers.Wait()
	if ring.len() != 0 {
		t.Fatalf("remote ring length = %d, want 0", ring.len())
	}

	benchtest.AssertZeroAllocs(t, 1_000, func() {
		value, ok := pool.Acquire()
		if !ok || !ring.tryPush(value) {
			t.Fatal("remote push unavailable")
		}
		var received batch.Batch[int]
		if !ring.tryPop(&received) {
			t.Fatal("remote pop unavailable")
		}
		received.Release()
	})
}

func TestExecutorRunsFixedGroupsAndDrainsRemoteWork(t *testing.T) {
	builder := NewBuilder()
	type sourceState struct{ emitted bool }
	received := make(chan int, 1)
	source, err := AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs1[int], error]]{
		Factory: func() (sourceState, error) { return sourceState{}, nil },
		Handlers: node.Node0[sourceState, node.Outputs1[int], error]{Handle: func(state *sourceState, outputs *node.Outputs1[int]) node.Result[error] {
			if state.emitted {
				return node.Result[error]{}
			}
			state.emitted = true
			value, ok := outputs.Out1.Acquire()
			if !ok {
				return node.Result[error]{Failed: true}
			}
			value.Mutable()[0] = 42
			value.SetLen(1)
			outputs.Out1.Commit(&value)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			received <- input.Items()[0]
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, source.NodeID(), 3); err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, sink.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, 3, 0)
	bindCPU(t, builder, 9, 1)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.workers) != 2 {
		t.Fatalf("worker count = %d, want 2", len(executor.workers))
	}
	select {
	case value := <-received:
		if value != 42 {
			t.Fatalf("received = %d, want 42", value)
		}
	case <-time.After(time.Second):
		t.Fatal("remote source work was not delivered")
	}
	if err := executor.Drain(); err != nil {
		t.Fatal(err)
	}
	if err := executor.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExecutorRollbackClosesInReverseCanonicalOrder(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	var closed []int
	source, _ := AddNode0(builder, node.Stateful[int, node.Node0[int, node.Outputs1[int], error]]{
		Factory:  func() (int, error) { return 1, nil },
		Handlers: node.Node0[int, node.Outputs1[int], error]{Handle: func(*int, *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} }},
		Close:    func(*int) error { closed = append(closed, 1); return nil },
	})
	sink, _ := AddNode1(builder, node.Stateful[int, node.Node1[int, int, node.Outputs0, error]]{
		Factory:  func() (int, error) { return 2, nil },
		Handlers: node.Node1[int, int, node.Outputs0, error]{Handle1: func(*int, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} }},
		Init:     func(*int) error { return errors.New("init failure") },
		Close:    func(*int) error { closed = append(closed, 2); return nil },
	})
	_ = Place(builder, source.NodeID(), 3)
	_ = Place(builder, sink.NodeID(), 9)
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, 3, 0)
	bindCPU(t, builder, 9, 1)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if executor, err := definition.Start(); executor != nil || err == nil || !reflect.DeepEqual(closed, []int{2, 1}) {
		t.Fatalf("Start() = (%v, %v), close order=%v", executor, err, closed)
	}
}

func TestTwoGroupDeliveryAllocs(t *testing.T) {
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
	source, _ := AddNode0(builder, node.Stateful[sourceState, node.Node0[sourceState, node.Outputs1[int], error]]{
		Factory: func() (sourceState, error) { return sourceState{gate: shared}, nil },
		Handlers: node.Node0[sourceState, node.Outputs1[int], error]{Handle: func(state *sourceState, outputs *node.Outputs1[int]) node.Result[error] {
			requested := state.gate.requested.Load()
			if requested == state.emitted {
				return node.Result[error]{}
			}
			value, ok := outputs.Out1.Acquire()
			if !ok {
				return node.Result[error]{Failed: true}
			}
			value.Mutable()[0] = int(requested)
			value.SetLen(1)
			outputs.Out1.Commit(&value)
			state.emitted = requested
			return node.Result[error]{}
		}},
	})
	sink, _ := AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			shared.delivered.Store(uint64(input.Items()[0]))
			return node.Result[error]{}
		}},
	})
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
	requested := shared.requested.Add(1)
	for shared.delivered.Load() != requested {
		runtime.Gosched()
	}
}
