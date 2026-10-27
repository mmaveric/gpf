package graph

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

type validationState struct{}

func validSource() node.Stateful[validationState, node.Node0[validationState, node.Outputs1[int], error]] {
	return node.Stateful[validationState, node.Node0[validationState, node.Outputs1[int], error]]{
		Factory: func() (validationState, error) { return validationState{}, nil },
		Handlers: node.Node0[validationState, node.Outputs1[int], error]{
			Handle: func(*validationState, *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} },
		},
	}
}

func validSink() node.Stateful[validationState, node.Node1[validationState, int, node.Outputs0, error]] {
	return node.Stateful[validationState, node.Node1[validationState, int, node.Outputs0, error]]{
		Factory: func() (validationState, error) { return validationState{}, nil },
		Handlers: node.Node1[validationState, int, node.Outputs0, error]{
			Handle1: func(*validationState, batch.Batch[int], *node.Outputs0) node.Result[error] {
				return node.Result[error]{}
			},
		},
	}
}

func validIsolated() node.Stateful[validationState, node.Node0[validationState, node.Outputs0, error]] {
	return node.Stateful[validationState, node.Node0[validationState, node.Outputs0, error]]{
		Factory: func() (validationState, error) { return validationState{}, nil },
		Handlers: node.Node0[validationState, node.Outputs0, error]{
			Handle: func(*validationState, *node.Outputs0) node.Result[error] { return node.Result[error]{} },
		},
	}
}

func connectValidPair(t *testing.T, builder *Builder) (NodeID, NodeID) {
	t.Helper()
	source, err := AddNode0(builder, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(builder, validSink())
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	if _, err := Connect(builder, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatal(err)
	}
	return source.NodeID(), sink.NodeID()
}

func bindCPU(t *testing.T, builder *Builder, group GroupID, cpu int) {
	t.Helper()
	if err := BindCPU(builder, group, cpu); err != nil {
		t.Fatal(err)
	}
}

func validationCodes(issues []ValidationIssue) []ValidationCode {
	codes := make([]ValidationCode, len(issues))
	for index, issue := range issues {
		codes[index] = issue.Code
	}
	return codes
}

func TestValidateStateMatrix(t *testing.T) {
	t.Parallel()

	valid := NewBuilder()
	if _, err := AddNode0(valid, validIsolated()); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, valid, DefaultGroupID, 0)
	sealed := NewBuilder()
	if _, err := AddNode0(sealed, validIsolated()); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, sealed, DefaultGroupID, 0)
	sealed.state = builderSealed
	var zero Builder
	for _, test := range []struct {
		name       string
		builder    *Builder
		wantReport bool
		wantCode   ValidationCode
	}{
		{name: "nil", wantCode: ValidationCodeInvalidBuilder},
		{name: "zero", builder: &zero, wantCode: ValidationCodeInvalidBuilder},
		{name: "empty", builder: NewBuilder(), wantReport: true, wantCode: ValidationCodeEmptyGraph},
		{name: "open", builder: valid, wantReport: true},
		{name: "sealed", builder: sealed, wantReport: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			report, err := test.builder.Validate()
			if (report != nil) != test.wantReport {
				t.Fatalf("report = %v, want present=%t", report, test.wantReport)
			}
			if test.wantCode == ValidationCodeInvalid {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			var validationError *ValidationError
			if !errors.As(err, &validationError) || len(validationError.Issues()) != 1 || validationError.Issues()[0].Code != test.wantCode {
				t.Fatalf("Validate() error = %#v, want one %s issue", err, test.wantCode)
			}
			if report != nil && !reflect.DeepEqual(report.Issues(), validationError.Issues()) {
				t.Fatalf("report/error issues differ: %#v / %#v", report.Issues(), validationError.Issues())
			}
		})
	}
	if sealed.state != builderSealed {
		t.Fatal("Validate changed sealed state")
	}
}

func TestValidationIssuesAreSortedAndOwned(t *testing.T) {
	t.Parallel()

	issues := []ValidationIssue{
		{Code: ValidationCodeCycle, Cycle: []NodeID{2, 3, 2}},
		{Code: ValidationCodeCycle, Cycle: []NodeID{1, 1}},
		{Code: ValidationCodeMissingHandler, NodeID: 2, PortID: 2},
		{Code: ValidationCodeMissingHandler, NodeID: 2, PortID: 1},
		{Code: ValidationCodeMissingHandler, NodeID: 1, PortID: 8, EdgeID: 4},
		{Code: ValidationCodeMissingHandler, NodeID: 1, PortID: 8, EdgeID: 3},
	}
	sortValidationIssues(issues)
	if got := []ValidationIssue{issues[0], issues[1], issues[2], issues[3]}; got[0].NodeID != 1 || got[1].EdgeID != 4 || got[2].PortID != 1 || got[3].PortID != 2 || issues[4].Cycle[0] != 1 {
		t.Fatalf("unexpected issue order: %#v", issues)
	}
	report := &ValidationReport{issues: copyValidationIssues(issues), order: []NodeID{1, 2}, forward: []ValidationAdjacency{{NodeID: 1, Edges: []ValidationAdjacentEdge{{EdgeID: 1, AdjacentNodeID: 2}}}}}
	err := newValidationError(issues)
	returnedIssues := report.Issues()
	returnedIssues[4].Cycle[0] = 99
	returnedOrder := report.TopologicalOrder()
	returnedOrder[0] = 99
	returnedForward := report.ForwardAdjacency()
	returnedForward[0].Edges[0].EdgeID = 99
	returnedErrorIssues := err.Issues()
	returnedErrorIssues[4].Cycle[0] = 88
	if report.Issues()[4].Cycle[0] != 1 || report.TopologicalOrder()[0] != 1 || report.ForwardAdjacency()[0].Edges[0].EdgeID != 1 || err.Issues()[4].Cycle[0] != 1 {
		t.Fatal("validation snapshots leaked mutable storage")
	}
}

func TestRegistrationValidationDescriptorMasks(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	if _, err := AddNode0(builder, validSource()); err != nil {
		t.Fatal(err)
	}
	if _, err := AddNode1(builder, validSink()); err != nil {
		t.Fatal(err)
	}
	if _, err := AddNode2(builder, node.Stateful[validationState, node.Node2[validationState, int, string, node.Outputs0, error]]{}); err != nil {
		t.Fatal(err)
	}
	if _, err := AddClosureNode0(builder, func() (node.ClosureReplica[node.ClosureNode0[node.Outputs0, error]], error) {
		return node.ClosureReplica[node.ClosureNode0[node.Outputs0, error]]{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := AddClosureNode1(builder, func() (node.ClosureReplica[node.ClosureNode1[int, node.Outputs0, error]], error) {
		return node.ClosureReplica[node.ClosureNode1[int, node.Outputs0, error]]{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := AddClosureNode2(builder, func() (node.ClosureReplica[node.ClosureNode2[int, string, node.Outputs0, error]], error) {
		return node.ClosureReplica[node.ClosureNode2[int, string, node.Outputs0, error]]{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		index    int
		required uint16
		present  uint16
	}{
		{name: "explicit source", index: 0, required: factoryMask | sourceMask, present: factoryMask | sourceMask},
		{name: "explicit single input", index: 1, required: factoryMask | input1Mask, present: factoryMask | input1Mask},
		{name: "explicit multi input", index: 2, required: factoryMask | input1Mask | input2Mask | selectorMask},
		{name: "closure source", index: 3, required: factoryMask, present: factoryMask},
		{name: "closure single input", index: 4, required: factoryMask, present: factoryMask},
		{name: "closure multi input", index: 5, required: factoryMask, present: factoryMask},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := builder.nodes[test.index].validation
			if got.required != test.required || got.present != test.present {
				t.Fatalf("descriptor = %#v, want required=%b present=%b", got, test.required, test.present)
			}
		})
	}
}

func TestValidateStaticMasksAndDormantCallbacks(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	var calls int
	_, err := AddNode2(builder, node.Stateful[validationState, node.Node2[validationState, int, string, node.Outputs0, error]]{
		Factory: func() (validationState, error) { calls++; return validationState{}, nil },
		Handlers: node.Node2[validationState, int, string, node.Outputs0, error]{
			Handle1: func(*validationState, batch.Batch[int], *node.Outputs0) node.Result[error] {
				calls++
				return node.Result[error]{}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = AddClosureNode1(builder, func() (node.ClosureReplica[node.ClosureNode1[int, node.Outputs0, error]], error) {
		calls++
		return node.ClosureReplica[node.ClosureNode1[int, node.Outputs0, error]]{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := builder.Validate()
	if calls != 0 {
		t.Fatalf("Validate invoked %d callbacks", calls)
	}
	if err == nil {
		t.Fatal("Validate() error = nil, want static issues")
	}
	issues := report.Issues()
	if got, want := report.TopologicalOrder(), []NodeID{1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("topological order = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(validationCodes(issues[:2]), []ValidationCode{ValidationCodeMissingHandler, ValidationCodeMissingSelector}) {
		t.Fatalf("static issues = %#v", issues)
	}
	if builder.nodes[0].validation.required != factoryMask|input1Mask|input2Mask|selectorMask || builder.nodes[1].validation.required != factoryMask {
		t.Fatalf("unexpected static masks: %#v, %#v", builder.nodes[0].validation, builder.nodes[1].validation)
	}
}

func TestValidateAdjacencyOrderTopologyAndSnapshot(t *testing.T) {
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
	if got, want := report.TopologicalOrder(), []NodeID{first, second, isolated.NodeID()}; !reflect.DeepEqual(got, want) {
		t.Fatalf("topological order = %v, want %v", got, want)
	}
	if got := report.ForwardAdjacency(); len(got) != 3 || got[0].NodeID != first || len(got[0].Edges) != 1 || got[0].Edges[0].AdjacentNodeID != second || len(got[2].Edges) != 0 {
		t.Fatalf("forward adjacency = %#v", got)
	}
	before := report.ForwardAdjacency()
	if _, err := AddNode0(builder, validIsolated()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, report.ForwardAdjacency()) {
		t.Fatal("report changed after builder mutation")
	}
}

func TestValidateCycleReachabilityAndSharedEdgeChecks(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	loop, err := AddNode1(builder, node.Stateful[validationState, node.Node1[validationState, int, node.Outputs1[int], error]]{
		Factory: func() (validationState, error) { return validationState{}, nil },
		Handlers: node.Node1[validationState, int, node.Outputs1[int], error]{
			Handle1: func(*validationState, batch.Batch[int], *node.Outputs1[int]) node.Result[error] {
				return node.Result[error]{}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(builder, OutputPorts1(loop.Outputs()).Out1, loop.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatal(err)
	}
	configBuilder := NewBuilder()
	if _, err := Connect(configBuilder, sourcePort[int](t, configBuilder), inputPort[int](t, configBuilder), EdgeConfig[int]{Capacity: 0, BatchCapacity: 1, Policy: queue.Policy(99)}); !errors.Is(err, ErrInvalidEdgeConfig) {
		t.Fatalf("Connect() error = %v, want shared edge configuration rejection", err)
	}
	if err := validateEdgeConfig(queue.Config{Capacity: 0, BatchCapacity: 1, Policy: queue.Policy(99)}); !errors.Is(err, ErrInvalidEdgeConfig) {
		t.Fatalf("shared edge validation error = %v", err)
	}
	builder.edges[0].config = queue.Config{Capacity: 0, BatchCapacity: 1, Policy: queue.Policy(99)}
	bindCPU(t, builder, DefaultGroupID, 0)
	report, err := builder.Validate()
	if err == nil || len(report.TopologicalOrder()) != 0 {
		t.Fatalf("Validate() = (%#v, %v), want cyclic error and no order", report, err)
	}
	issues := report.Issues()
	var cycle []NodeID
	for _, issue := range issues {
		if issue.Code == ValidationCodeCycle {
			cycle = issue.Cycle
		}
	}
	if !reflect.DeepEqual(cycle, []NodeID{loop.NodeID(), loop.NodeID()}) {
		t.Fatalf("cycle witness = %v", cycle)
	}
	if !reflect.DeepEqual(validationCodes(issues), []ValidationCode{ValidationCodeInvalidEdgeCapacity, ValidationCodeInvalidEdgePolicy, ValidationCodeCycle, ValidationCodeUnreachableFromSource, ValidationCodeCannotReachSink}) {
		t.Fatalf("issue codes = %v", validationCodes(issues))
	}
}

func TestValidateCycleWitnessUsesFirstResidualPath(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	builder.nodes = []nodeMetadata{
		{nodeID: 1, inputCount: 1, outputCount: 2},
		{nodeID: 2, inputCount: 1, outputCount: 1},
		{nodeID: 3, inputCount: 1, outputCount: 1},
	}
	builder.inputs = map[endpointID]*inputMetadata{
		{nodeID: 1, portID: 1}: {},
		{nodeID: 2, portID: 1}: {},
		{nodeID: 3, portID: 1}: {},
	}
	builder.outputs = map[endpointID]*outputMetadata{
		{nodeID: 1, portID: 1}: {},
		{nodeID: 1, portID: 2}: {},
		{nodeID: 2, portID: 1}: {},
		{nodeID: 3, portID: 1}: {},
	}
	builder.edges = []edgeMetadata{
		{id: 1, source: endpointID{nodeID: 1, portID: 1}, target: endpointID{nodeID: 2, portID: 1}, config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}},
		{id: 2, source: endpointID{nodeID: 2, portID: 1}, target: endpointID{nodeID: 1, portID: 1}, config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}},
		{id: 3, source: endpointID{nodeID: 1, portID: 2}, target: endpointID{nodeID: 3, portID: 1}, config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}},
		{id: 4, source: endpointID{nodeID: 3, portID: 1}, target: endpointID{nodeID: 1, portID: 1}, config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}},
	}
	builder.cpuBindings = map[GroupID]int{DefaultGroupID: 0}
	report, err := builder.Validate()
	if err == nil {
		t.Fatal("cyclic topology unexpectedly validated")
	}
	for _, issue := range report.Issues() {
		if issue.Code == ValidationCodeCycle {
			if got, want := issue.Cycle, []NodeID{1, 2, 1}; !reflect.DeepEqual(got, want) {
				t.Fatalf("cycle witness = %v, want %v", got, want)
			}
			return
		}
	}
	t.Fatal("missing cycle issue")
}

func TestValidateDoesNotConsumeIdentityAndIssueFreeProperties(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	first, second := connectValidPair(t, builder)
	report, err := builder.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := report.TopologicalOrder(), []NodeID{first, second}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	third, err := AddNode0(builder, validIsolated())
	if err != nil || third.NodeID() != 3 {
		t.Fatalf("registration after validation = (%d, %v), want (3, nil)", third.NodeID(), err)
	}
	position := make(map[NodeID]int)
	for index, nodeID := range report.TopologicalOrder() {
		position[nodeID] = index
	}
	for _, row := range report.ForwardAdjacency() {
		for _, edge := range row.Edges {
			if position[row.NodeID] >= position[edge.AdjacentNodeID] {
				t.Fatalf("edge %d -> %d violates order", row.NodeID, edge.AdjacentNodeID)
			}
		}
	}
	if got := report.ReverseAdjacency(); len(got) != 2 || got[1].Edges[0].AdjacentNodeID != first {
		t.Fatalf("reverse adjacency = %#v", got)
	}
}

func TestValidateKeepsQueueFactoriesDormantAndExcludesErrorOutputs(t *testing.T) {
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
	var calls int
	if _, err := Connect(
		builder,
		OutputPorts1(source.Outputs()).Out1,
		sink.Inputs().In1,
		EdgeConfig[int]{
			Capacity:      1,
			BatchCapacity: 1,
			Policy:        queue.Drop,
			Factory: func(queue.Config) (queue.QueueOps[int], error) {
				calls++
				return queue.QueueOps[int]{}, nil
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	report, err := builder.Validate()
	if err != nil || calls != 0 {
		t.Fatalf("Validate() = (%#v, %v), factory calls = %d", report, err, calls)
	}
	if len(report.ForwardAdjacency()) != 2 || len(report.ForwardAdjacency()[0].Edges) != 1 || len(report.ReverseAdjacency()[1].Edges) != 1 {
		t.Fatalf("error outputs leaked into adjacency: forward=%#v reverse=%#v", report.ForwardAdjacency(), report.ReverseAdjacency())
	}
}

func TestValidateAdjacencyOrderingAndCorrectableTopology(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	builder.nodes = []nodeMetadata{
		{nodeID: 1, outputCount: 2},
		{nodeID: 2, inputCount: 1},
		{nodeID: 3, inputCount: 1},
	}
	builder.inputs = map[endpointID]*inputMetadata{
		{nodeID: 2, portID: 1}: {},
		{nodeID: 3, portID: 1}: {},
	}
	builder.outputs = map[endpointID]*outputMetadata{
		{nodeID: 1, portID: 1}: {},
		{nodeID: 1, portID: 2}: {},
	}
	builder.edges = []edgeMetadata{
		{id: 2, source: endpointID{nodeID: 1, portID: 1}, target: endpointID{nodeID: 3, portID: 1}, config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}},
		{id: 3, source: endpointID{nodeID: 1, portID: 2}, target: endpointID{nodeID: 2, portID: 1}, config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}},
	}
	builder.cpuBindings = map[GroupID]int{DefaultGroupID: 0}
	report, err := builder.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := report.TopologicalOrder(), []NodeID{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("topological order = %v, want %v", got, want)
	}
	forward := report.ForwardAdjacency()
	if got, want := forward[0].Edges, []ValidationAdjacentEdge{{EdgeID: 3, AdjacentNodeID: 2}, {EdgeID: 2, AdjacentNodeID: 3}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("forward edges = %#v, want %#v", got, want)
	}

	open := NewBuilder()
	source, err := AddNode0(open, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(open, validSink())
	if err != nil {
		t.Fatal(err)
	}
	bindCPU(t, open, DefaultGroupID, 0)
	if _, err := open.Validate(); err == nil {
		t.Fatal("unconnected topology unexpectedly validated")
	}
	if _, err := Connect(open, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatal(err)
	}
	if _, err := open.Validate(); err != nil {
		t.Fatalf("corrected topology Validate() error = %v", err)
	}
}

func FuzzValidate(f *testing.F) {
	for _, seed := range []uint8{0, 1, 2, 3, 7, 255} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, operations uint8) {
		var zero Builder
		for _, builder := range []*Builder{nil, &zero, NewBuilder()} {
			if builder != nil && builder.owner != nil && operations&4 != 0 {
				connectValidPair(t, builder)
			}
			if builder != nil && builder.owner != nil && operations&1 != 0 {
				_, _ = AddNode0(builder, validIsolated())
			}
			if builder != nil && builder.owner != nil && operations&2 != 0 {
				builder.state = builderSealed
			}
			firstReport, firstError := builder.Validate()
			secondReport, secondError := builder.Validate()
			if !reflect.DeepEqual(firstReport, secondReport) || errorText(firstError) != errorText(secondError) {
				t.Fatalf("non-deterministic validation for operations %08b", operations)
			}
		}
	})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
