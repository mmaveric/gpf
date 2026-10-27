package graph

import (
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

func TestResolveQueueModel(t *testing.T) {
	tests := []struct {
		name                 string
		producers, consumers uint32
		sameWorker           bool
		want                 QueueModel
		wantErr              error
	}{
		{name: "local", producers: 1, consumers: 1, sameWorker: true, want: QueueModelLocal},
		{name: "remote single producer consumer", producers: 1, consumers: 1, want: QueueModelSPSC},
		{name: "multiple producers", producers: 3, consumers: 1, want: QueueModelMPSC},
		{name: "multiple consumers", producers: 1, consumers: 4, want: QueueModelSPMC},
		{name: "multiple producers and consumers", producers: 2, consumers: 4, want: QueueModelMPMC},
		{name: "zero producer", producers: 0, consumers: 1, wantErr: ErrInvalidQueueCardinality},
		{name: "zero consumer", producers: 1, consumers: 0, wantErr: ErrInvalidQueueCardinality},
		{name: "invalid same worker cardinality", producers: 2, consumers: 1, sameWorker: true, wantErr: ErrInvalidQueueCardinality},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveQueueModel(tt.producers, tt.consumers, tt.sameWorker)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("ResolveQueueModel() = (%s, %v), want (%s, %v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestDefinitionEdgeQueuePlansAreOrderedCopies(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		builder := NewBuilder()
		source, _ := AddNode0(builder, validSource())
		sink, _ := AddNode1(builder, validSink())
		if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
			t.Fatal(err)
		}
		bindCPU(t, builder, DefaultGroupID, 0)
		definition, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		plans := definition.EdgeQueuePlans()
		if len(plans) != 1 || plans[0] != (EdgeQueuePlan{EdgeID: 1, LogicalEdgeID: 1, Model: QueueModelLocal, ProducerWorkers: 1, ConsumerWorkers: 1}) {
			t.Fatalf("plans = %+v", plans)
		}
		plans[0].Model = QueueModelMPMC
		if got := definition.EdgeQueuePlans()[0].Model; got != QueueModelLocal {
			t.Fatalf("mutated plan affected definition: %s", got)
		}
	})
	t.Run("remote", func(t *testing.T) {
		builder := NewBuilder()
		source, _ := AddNode0(builder, validSource())
		sink, _ := AddNode1(builder, validSink())
		if err := Place(builder, sink.NodeID(), 8); err != nil {
			t.Fatal(err)
		}
		if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
			t.Fatal(err)
		}
		bindCPU(t, builder, DefaultGroupID, 0)
		bindCPU(t, builder, 8, 1)
		definition, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		want := []EdgeQueuePlan{{EdgeID: 1, LogicalEdgeID: 1, Model: QueueModelSPSC, ProducerWorkers: 1, ConsumerWorkers: 1, Remote: true}}
		if !reflect.DeepEqual(definition.EdgeQueuePlans(), want) {
			t.Fatalf("plans = %+v, want %+v", definition.EdgeQueuePlans(), want)
		}
	})
	var nilDefinition *Definition
	if nilDefinition.EdgeQueuePlans() != nil {
		t.Fatal("nil definition returned queue plans")
	}
}

func TestQueueOperationsMaterializeSelectedModels(t *testing.T) {
	models := []QueueModel{QueueModelSPSC, QueueModelMPSC, QueueModelSPMC, QueueModelMPMC}
	for _, model := range models {
		t.Run(model.String(), func(t *testing.T) {
			ops, err := queueOperationsForModel[int](model, queue.Config{Capacity: 2, BatchCapacity: 1}, nil)
			if err != nil {
				t.Fatal(err)
			}
			pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
			if err != nil {
				t.Fatal(err)
			}
			input, ok := pool.Acquire()
			if !ok {
				t.Fatal("pool acquisition failed")
			}
			input.Mutable()[0] = 42
			input.SetLen(1)
			if !ops.TryPush(input) {
				t.Fatal("selected queue rejected available slot")
			}
			var output batch.Batch[int]
			if !ops.TryPop(&output) || output.Items()[0] != 42 {
				t.Fatal("selected queue did not transfer contents")
			}
			output.Release()
		})
	}
}

func TestSharedTransportRejectsFactoryWithoutInvocation(t *testing.T) {
	calls := 0
	factory := func(queue.Config) (queue.QueueOps[int], error) { calls++; return queue.QueueOps[int]{}, nil }
	for _, model := range []QueueModel{QueueModelSPSC, QueueModelMPSC, QueueModelSPMC, QueueModelMPMC} {
		if _, err := queueOperationsForModel[int](model, queue.Config{Capacity: 1, BatchCapacity: 1}, factory); !errors.Is(err, ErrIncompatibleQueueFactory) {
			t.Fatalf("model %s error = %v", model, err)
		}
	}
	if calls != 0 {
		t.Fatalf("incompatible factory calls = %d, want 0", calls)
	}
}

func TestDormantBindingUsesPhysicalCardinalities(t *testing.T) {
	for _, counts := range []struct {
		name                 string
		producers, consumers uint32
	}{
		{"MPSC", 3, 1}, {"SPMC", 1, 4}, {"MPMC", 3, 4},
	} {
		t.Run(counts.name, func(t *testing.T) {
			model, err := ResolveQueueModel(counts.producers, counts.consumers, false)
			if err != nil {
				t.Fatal(err)
			}
			plan := EdgeQueuePlan{EdgeID: 1, Model: model, ProducerWorkers: counts.producers, ConsumerWorkers: counts.consumers, Remote: true}
			bind := futureBinding[int](1, endpointID{1, 1}, endpointID{2, 1}, queue.Config{Capacity: 2, BatchCapacity: 3, Policy: queue.Wait}, nil, false)
			for range 2 {
				target := &edgeBindingTarget{}
				if err := bind(target, plan); err != nil {
					t.Fatal(err)
				}
				var output node.Output[int]
				finalize := target.outputs[0].configure(unsafe.Pointer(&output))
				var input runtimeInput[int]
				target.inputs[0].input(unsafe.Pointer(&input))
				if input.ready() {
					t.Fatal("fresh transport was not empty")
				}
				value, ok := output.Acquire()
				if !ok {
					t.Fatal("fresh pool was unavailable")
				}
				copy(value.Mutable(), []int{7, 11, 19})
				value.SetLen(3)
				if !output.Commit(&value) || !finalize(false) {
					t.Fatal("publication failed")
				}
				var received batch.Batch[int]
				if !input.pop(&received) || received.Len() != 3 || received.Items()[2] != 19 {
					t.Fatal("typed binding lost payload")
				}
				received.Release()
				if input.ready() {
					t.Fatal("drained transport still ready")
				}
			}
		})
	}
}

func TestSelectedModelOpsAllocs(t *testing.T) {
	for _, model := range []QueueModel{QueueModelLocal, QueueModelSPSC, QueueModelMPSC, QueueModelSPMC, QueueModelMPMC} {
		t.Run(model.String(), func(t *testing.T) {
			ops, err := queueOperationsForModel[int](model, queue.Config{Capacity: 2}, nil)
			if err != nil {
				t.Fatal(err)
			}
			pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
			if err != nil {
				t.Fatal(err)
			}
			value, ok := pool.Acquire()
			if !ok {
				t.Fatal("acquire failed")
			}
			value.SetLen(1)
			if allocations := testing.AllocsPerRun(100, func() {
				if !ops.TryPush(value) || !ops.TryPop(&value) {
					panic("round trip failed")
				}
			}); allocations != 0 {
				t.Fatalf("allocations = %v", allocations)
			}
			value.Release()
		})
	}
}

func TestEmptyAndZeroQueuePlans(t *testing.T) {
	var zero Definition
	if zero.EdgeQueuePlans() != nil {
		t.Fatal("zero definition returned plans")
	}
	valid := &Definition{owner: &graphOwner{}}
	if plans := valid.EdgeQueuePlans(); plans == nil || len(plans) != 0 {
		t.Fatal("valid empty definition should return an empty independent slice")
	}
}

func TestSPSCCursorWrapWithNonPowerOfTwoCapacity(t *testing.T) {
	q := newRemoteRing[int](3)
	q.head.Store(^uint64(0) - 1)
	q.tail.Store(^uint64(0) - 1)
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 3, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	for code := 1; code <= 3; code++ {
		value, ok := pool.Acquire()
		if !ok {
			t.Fatal("acquire failed")
		}
		value.Mutable()[0] = code
		value.SetLen(1)
		if !q.tryPush(value) {
			t.Fatal("push failed near wrap")
		}
	}
	var value batch.Batch[int]
	for code := 1; code <= 3; code++ {
		if !q.tryPop(&value) || value.Len() != 1 || value.Items()[0] != code {
			t.Fatalf("wrapped FIFO pop %d = %v", code, value.Items())
		}
		value.Release()
	}
}
