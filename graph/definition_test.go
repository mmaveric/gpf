package graph

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

func TestBuildRejectsInvalidBuildersAndAllowsCorrection(t *testing.T) {
	t.Parallel()

	var zero Builder
	for _, builder := range []*Builder{nil, &zero} {
		definition, err := builder.Build()
		if definition != nil || !errors.Is(err, ErrInvalidBuilder) {
			t.Fatalf("Build(%p) = (%p, %v), want (nil, ErrInvalidBuilder)", builder, definition, err)
		}
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
	definition, err := builder.Build()
	var validationError *ValidationError
	if definition != nil || !errors.As(err, &validationError) {
		t.Fatalf("invalid Build() = (%p, %v), want (nil, *ValidationError)", definition, err)
	}
	if builder.state != builderOpen || builder.definition != nil {
		t.Fatalf("failed Build changed builder state: state=%d definition=%p", builder.state, builder.definition)
	}
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatalf("Connect after failed Build() error = %v", err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	definition, err = builder.Build()
	if err != nil || definition == nil {
		t.Fatalf("corrected Build() = (%p, %v)", definition, err)
	}
}

func TestBuildSealsMutationAndCachesDefinition(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	connectValidPair(t, builder)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if builder.state != builderSealed || builder.definition != definition {
		t.Fatalf("successful Build did not publish a sealed cache: state=%d cache=%p definition=%p", builder.state, builder.definition, definition)
	}
	if _, err := AddNode0(builder, validIsolated()); !errors.Is(err, ErrBuilderSealed) {
		t.Fatalf("AddNode0 after Build error = %v, want ErrBuilderSealed", err)
	}
	if _, err := Connect(builder, OutPort[int]{}, InPort[int]{}, EdgeConfig[int]{}); !errors.Is(err, ErrBuilderSealed) {
		t.Fatalf("Connect after Build error = %v, want ErrBuilderSealed", err)
	}
	cached, err := builder.Build()
	if err != nil || cached != definition {
		t.Fatalf("cached Build() = (%p, %v), want (%p, nil)", cached, err, definition)
	}
}

func TestBuildCopiesCanonicalReportAndDoesNotRevalidateCache(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	first, second := connectValidPair(t, builder)
	isolated, err := AddNode0(builder, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	report, err := builder.Validate()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := definition.TopologicalOrder(), []NodeID{first, second, isolated.NodeID()}; !reflect.DeepEqual(got, want) {
		t.Fatalf("definition order = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(definition.forward, report.forward) || !reflect.DeepEqual(definition.reverse, report.reverse) {
		t.Fatalf("definition topology differs from validation report: forward=%#v reverse=%#v", definition.forward, definition.reverse)
	}

	// Only test code can corrupt sealed builder internals. A cached Build must
	// still return its snapshot rather than running validation again.
	builder.nodes = nil
	cached, err := builder.Build()
	if err != nil || cached != definition {
		t.Fatalf("cached Build after builder mutation = (%p, %v), want (%p, nil)", cached, err, definition)
	}
}

func TestDefinitionOwnsCopiedTopologyAndInspectionResults(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	connectValidPair(t, builder)
	report, err := builder.Validate()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := definition.TopologicalOrder()
	wantForward := copyValidationAdjacency(definition.forward)
	wantReverse := copyValidationAdjacency(definition.reverse)
	wantNode := definition.nodes[0]
	wantEdge := definition.edges[0]

	builder.nodes[0].inputCount = 99
	builder.edges[0].config.Capacity = 99
	report.forward[0].Edges[0].EdgeID = 99
	report.reverse[1].Edges[0].EdgeID = 99
	report.order[0] = 99
	first := definition.TopologicalOrder()
	second := definition.TopologicalOrder()
	first[0] = 99
	if &first[0] == &second[0] || !reflect.DeepEqual(second, wantOrder) {
		t.Fatalf("TopologicalOrder leaked backing storage: first=%v second=%v want=%v", first, second, wantOrder)
	}
	if definition.nodes[0].nodeID != wantNode.nodeID ||
		definition.nodes[0].kind != wantNode.kind ||
		definition.nodes[0].inputCount != wantNode.inputCount ||
		definition.nodes[0].outputCount != wantNode.outputCount ||
		definition.nodes[0].validation != wantNode.validation ||
		definition.edges[0] != wantEdge ||
		!reflect.DeepEqual(definition.forward, wantForward) || !reflect.DeepEqual(definition.reverse, wantReverse) {
		t.Fatalf("definition retained mutable builder or report storage: %#v", definition)
	}

	typeOfDefinition := reflect.TypeOf(&Definition{})
	if typeOfDefinition.NumMethod() != 8 {
		t.Fatalf("Definition exports %d methods, want exactly 8", typeOfDefinition.NumMethod())
	}
	for _, name := range []string{"NodeCount", "EdgeCount", "TopologicalOrder", "EdgeQueuePlans", "Constraints", "GroupNUMA", "Start", "WriteDOT"} {
		if _, ok := typeOfDefinition.MethodByName(name); !ok {
			t.Fatalf("Definition is missing %s", name)
		}
	}
	if _, ok := typeOfDefinition.MethodByName("ForwardAdjacency"); ok {
		t.Fatal("Definition exposes forward adjacency")
	}
}

func TestDefinitionAccessors(t *testing.T) {
	t.Parallel()

	var nilDefinition *Definition
	var zero Definition
	for _, definition := range []*Definition{nilDefinition, &zero} {
		if definition.NodeCount() != 0 || definition.EdgeCount() != 0 || definition.TopologicalOrder() != nil {
			t.Fatalf("zero definition accessors = (%d, %d, %v)", definition.NodeCount(), definition.EdgeCount(), definition.TopologicalOrder())
		}
	}
	validEmpty := &Definition{owner: &graphOwner{}, nodes: []nodeMetadata{}}
	order := validEmpty.TopologicalOrder()
	if validEmpty.NodeCount() != 0 || validEmpty.EdgeCount() != 0 || order == nil || len(order) != 0 {
		t.Fatalf("valid empty definition accessors = (%d, %d, %#v)", validEmpty.NodeCount(), validEmpty.EdgeCount(), order)
	}

	builder := NewBuilder()
	connectValidPair(t, builder)
	definition, err := builder.Build()
	if err != nil || definition.NodeCount() != 2 || definition.EdgeCount() != 1 {
		t.Fatalf("valid definition accessors = (%d, %d, %v)", definition.NodeCount(), definition.EdgeCount(), err)
	}
}

func TestDefinitionRetainsBuilderOwnerIdentity(t *testing.T) {
	t.Parallel()

	first := NewBuilder()
	connectValidPair(t, first)
	firstDefinition, err := first.Build()
	if err != nil {
		t.Fatal(err)
	}
	second := NewBuilder()
	connectValidPair(t, second)
	secondDefinition, err := second.Build()
	if err != nil {
		t.Fatal(err)
	}
	if firstDefinition.owner == nil || firstDefinition.owner != first.owner {
		t.Fatal("definition did not retain its builder's non-zero owner")
	}
	if firstDefinition.owner == second.owner || firstDefinition.owner == secondDefinition.owner || (&Definition{}).owner == firstDefinition.owner {
		t.Fatal("definition owner matched a foreign or zero owner")
	}
}

func TestBuildKeepsFactoriesAndCallbacksDormant(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	var stateFactoryCalls, closureFactoryCalls, queueFactoryCalls, lifecycleCalls, handlerCalls int
	explicitSource, err := AddNode0(builder, node.Stateful[validationState, node.Node0[validationState, node.Outputs1[int], error]]{
		Factory: func() (validationState, error) { stateFactoryCalls++; return validationState{}, nil },
		Handlers: node.Node0[validationState, node.Outputs1[int], error]{
			Handle: func(*validationState, *node.Outputs1[int]) node.Result[error] {
				handlerCalls++
				return node.Result[error]{}
			},
		},
		Init:  func(*validationState) error { lifecycleCalls++; return nil },
		Close: func(*validationState) error { lifecycleCalls++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	explicitSink, err := AddNode1(builder, node.Stateful[validationState, node.Node1[validationState, int, node.Outputs0, error]]{
		Factory: func() (validationState, error) { stateFactoryCalls++; return validationState{}, nil },
		Handlers: node.Node1[validationState, int, node.Outputs0, error]{
			Handle1: func(*validationState, batch.Batch[int], *node.Outputs0) node.Result[error] {
				handlerCalls++
				return node.Result[error]{}
			},
		},
		Init:  func(*validationState) error { lifecycleCalls++; return nil },
		Close: func(*validationState) error { lifecycleCalls++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	closureSource, err := AddClosureNode0(builder, func() (node.ClosureReplica[node.ClosureNode0[node.Outputs1[string], error]], error) {
		closureFactoryCalls++
		return node.ClosureReplica[node.ClosureNode0[node.Outputs1[string], error]]{
			Handlers: node.ClosureNode0[node.Outputs1[string], error]{
				Handle: func(*node.Outputs1[string]) node.Result[error] { handlerCalls++; return node.Result[error]{} },
			},
			Init:  func() error { lifecycleCalls++; return nil },
			Close: func() error { lifecycleCalls++; return nil },
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	closureSink, err := AddClosureNode1(builder, func() (node.ClosureReplica[node.ClosureNode1[string, node.Outputs0, error]], error) {
		closureFactoryCalls++
		return node.ClosureReplica[node.ClosureNode1[string, node.Outputs0, error]]{
			Handlers: node.ClosureNode1[string, node.Outputs0, error]{
				Handle1: func(batch.Batch[string], *node.Outputs0) node.Result[error] {
					handlerCalls++
					return node.Result[error]{}
				},
			},
			Init:  func() error { lifecycleCalls++; return nil },
			Close: func() error { lifecycleCalls++; return nil },
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(builder, OutputPorts1(explicitSource.Outputs()).Out1, explicitSink.Inputs().In1, EdgeConfig[int]{
		Capacity:      1,
		BatchCapacity: 1,
		Policy:        queue.Drop,
		Factory: func(queue.Config) (queue.QueueOps[int], error) {
			queueFactoryCalls++
			return queue.QueueOps[int]{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(builder, OutputPorts1(closureSource.Outputs()).Out1, closureSink.Inputs().In1, EdgeConfig[string]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if stateFactoryCalls != 0 || closureFactoryCalls != 0 || queueFactoryCalls != 0 || lifecycleCalls != 0 || handlerCalls != 0 {
		t.Fatalf("Build invoked dormant functions: state=%d closure=%d queue=%d lifecycle=%d handler=%d", stateFactoryCalls, closureFactoryCalls, queueFactoryCalls, lifecycleCalls, handlerCalls)
	}
	if len(definition.nodes) != 4 || len(definition.edgeBindings) != 2 || definition.edgeBindings[0] == nil || definition.edgeBindings[1] == nil {
		t.Fatalf("definition did not retain dormant recipes: nodes=%d bindings=%d", len(definition.nodes), len(definition.edgeBindings))
	}
}

func TestDefinitionSupportsSequentialInspectionAndCachedReuse(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	source, err := AddNode0(builder, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(builder, validSink())
	if err != nil {
		t.Fatal(err)
	}
	var queueFactoryCalls int
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{
		Capacity:      1,
		BatchCapacity: 1,
		Policy:        queue.Drop,
		Factory: func(queue.Config) (queue.QueueOps[int], error) {
			queueFactoryCalls++
			return queue.QueueOps[int]{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	owner := definition.owner
	order := definition.TopologicalOrder()
	binding := reflect.ValueOf(definition.edgeBindings[0]).Pointer()
	for range 3 {
		cached, err := builder.Build()
		if err != nil || cached != definition || cached.owner != owner ||
			!reflect.DeepEqual(cached.TopologicalOrder(), order) ||
			reflect.ValueOf(cached.edgeBindings[0]).Pointer() != binding {
			t.Fatalf("sequential reuse changed definition: cached=%p err=%v", cached, err)
		}
	}
	if queueFactoryCalls != 0 {
		t.Fatalf("cached Build invoked queue factory %d times", queueFactoryCalls)
	}
}
