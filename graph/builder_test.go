package graph

import (
	"errors"
	"testing"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
)

type testState struct{}

func TestNewBuilderAssignsDistinctOwners(t *testing.T) {
	t.Parallel()

	first := NewBuilder()
	second := NewBuilder()
	if first.owner == second.owner {
		t.Fatal("NewBuilder() owners compare equal")
	}
}

func TestRegistrationRejectsInvalidAndSealedBuildersAtomically(t *testing.T) {
	t.Parallel()

	definition := sourceDefinition()
	var zero Builder
	for _, builder := range []*Builder{nil, &zero} {
		ports, err := AddNode0(builder, definition)
		if !errors.Is(err, ErrInvalidBuilder) {
			t.Fatalf("AddNode0(%v) error = %v, want ErrInvalidBuilder", builder, err)
		}
		if ports.NodeID() != InvalidNodeID || ports.Outputs().Valid() || ports.Errors().Valid() {
			t.Fatalf("invalid registration returned valid ports: %#v", ports)
		}
	}

	builder := NewBuilder()
	builder.state = builderSealed
	if _, err := AddNode0(builder, definition); !errors.Is(err, ErrBuilderSealed) {
		t.Fatalf("sealed AddNode0() error = %v, want ErrBuilderSealed", err)
	}
	if builder.nextNodeID != 1 || len(builder.nodes) != 0 {
		t.Fatalf("sealed registration mutated builder: next=%d nodes=%d", builder.nextNodeID, len(builder.nodes))
	}
	builder.state = builderOpen
	ports, err := AddNode0(builder, definition)
	if err != nil {
		t.Fatalf("AddNode0() error = %v", err)
	}
	if ports.NodeID() != 1 {
		t.Fatalf("node ID = %d, want 1 after rejected registration", ports.NodeID())
	}
}

func TestZeroValueHandlesAndNodePortsAreInvalid(t *testing.T) {
	t.Parallel()

	var input InPort[int]
	var output OutPort[string]
	var errorOutput ErrorPort[error]
	var reference OutputRef[node.Outputs0]
	var ports NodePorts[InPorts0, node.Outputs0, error]
	if input.NodeID() != InvalidNodeID || input.PortID() != InvalidPortID || input.Valid() {
		t.Fatalf("zero input = %#v", input)
	}
	if output.NodeID() != InvalidNodeID || output.PortID() != InvalidPortID || output.Valid() {
		t.Fatalf("zero output = %#v", output)
	}
	if errorOutput.NodeID() != InvalidNodeID || errorOutput.PortID() != InvalidPortID || errorOutput.Valid() {
		t.Fatalf("zero error output = %#v", errorOutput)
	}
	if reference.NodeID() != InvalidNodeID || reference.OutputCount() != 0 || reference.Valid() {
		t.Fatalf("zero output reference = %#v", reference)
	}
	if ports.NodeID() != InvalidNodeID || ports.Outputs().Valid() || ports.Errors().Valid() {
		t.Fatalf("zero node ports = %#v", ports)
	}
}

func TestRegistrationAssignsDenseIDsAndOneBasedPorts(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	first, err := AddNode0(builder, sourceDefinition())
	if err != nil {
		t.Fatalf("AddNode0() error = %v", err)
	}
	second, err := AddNode2(builder, twoInputDefinition())
	if err != nil {
		t.Fatalf("AddNode2() error = %v", err)
	}
	third, err := AddNode0(builder, sourceDefinition())
	if err != nil {
		t.Fatalf("second AddNode0() error = %v", err)
	}
	if first.NodeID() != 1 || second.NodeID() != 2 || third.NodeID() != 3 {
		t.Fatalf("node IDs = %d, %d, %d; want 1, 2, 3", first.NodeID(), second.NodeID(), third.NodeID())
	}
	inputs := second.Inputs()
	if inputs.In1.PortID() != 1 || inputs.In2.PortID() != 2 {
		t.Fatalf("input IDs = %d, %d; want 1, 2", inputs.In1.PortID(), inputs.In2.PortID())
	}
	outputs := OutputPorts2(second.Outputs())
	if outputs.Out1.PortID() != 1 || outputs.Out2.PortID() != 2 {
		t.Fatalf("ordinary output IDs = %d, %d; want 1, 2", outputs.Out1.PortID(), outputs.Out2.PortID())
	}
	if second.Errors().PortID() != 9 || !second.Errors().Valid() {
		t.Fatalf("error port = %#v, want valid PortID(9)", second.Errors())
	}
}

func TestRegistrationRejectsExhaustedNodeIDsAtomically(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	builder.nextNodeID = 0
	if _, err := AddNode0(builder, sourceDefinition()); !errors.Is(err, ErrNodeIDExhausted) {
		t.Fatalf("AddNode0() error = %v, want ErrNodeIDExhausted", err)
	}
	if builder.nextNodeID != 0 || len(builder.nodes) != 0 {
		t.Fatalf("exhausted registration mutated builder: next=%d nodes=%d", builder.nextNodeID, len(builder.nodes))
	}
}

func TestOutputExpansionRejectsInvalidReferences(t *testing.T) {
	t.Parallel()

	if ports := OutputPorts1(OutputRef[node.Outputs1[int]]{}); ports.Out1.Valid() {
		t.Fatalf("invalid reference expanded to valid port: %#v", ports)
	}
	mismatch := OutputRef[node.Outputs1[int]]{owner: &graphOwner{}, nodeID: 1, outputCount: 2}
	if ports := OutputPorts1(mismatch); ports.Out1.Valid() {
		t.Fatalf("mismatched reference expanded to valid port: %#v", ports)
	}
}

func TestRegistrationMetadataAndCallbacksRemainDormant(t *testing.T) {
	t.Parallel()

	var callbacks int
	builder := NewBuilder()
	explicit, err := AddNode2(builder, node.Stateful[testState, node.Node2[testState, int, string, node.Outputs3[int, string, bool], error]]{
		Factory: func() (testState, error) { callbacks++; return testState{}, nil },
		Handlers: node.Node2[testState, int, string, node.Outputs3[int, string, bool], error]{
			Handle1: func(*testState, batch.Batch[int], *node.Outputs3[int, string, bool]) node.Result[error] {
				callbacks++
				return node.Result[error]{}
			},
			Select: func(*testState, node.ReadyMask) node.PortIndex { callbacks++; return node.Port1 },
		},
		Init:  func(*testState) error { callbacks++; return nil },
		Close: func(*testState) error { callbacks++; return nil },
	})
	if err != nil {
		t.Fatalf("AddNode2() error = %v", err)
	}
	closure, err := AddClosureNode2(builder, func() (node.ClosureReplica[node.ClosureNode2[int, string, node.Outputs1[int], error]], error) {
		callbacks++
		return node.ClosureReplica[node.ClosureNode2[int, string, node.Outputs1[int], error]]{
			Handlers: node.ClosureNode2[int, string, node.Outputs1[int], error]{
				Handle1: func(batch.Batch[int], *node.Outputs1[int]) node.Result[error] {
					callbacks++
					return node.Result[error]{}
				},
				Handle2: func(batch.Batch[string], *node.Outputs1[int]) node.Result[error] {
					callbacks++
					return node.Result[error]{}
				},
				Select: func(node.ReadyMask) node.PortIndex { callbacks++; return node.Port1 },
			},
			Init:  func() error { callbacks++; return nil },
			Close: func() error { callbacks++; return nil },
		}, nil
	})
	if err != nil {
		t.Fatalf("AddClosureNode2() error = %v", err)
	}
	if callbacks != 0 {
		t.Fatalf("registration invoked %d callbacks, want 0", callbacks)
	}
	if explicit.NodeID() != 1 || closure.NodeID() != 2 {
		t.Fatalf("registered IDs = %d, %d; want 1, 2", explicit.NodeID(), closure.NodeID())
	}
	if len(builder.nodes) != 2 {
		t.Fatalf("metadata count = %d, want 2", len(builder.nodes))
	}
	explicitMetadata := builder.nodes[0]
	if explicitMetadata.kind != explicitRegistration || explicitMetadata.inputCount != 2 || explicitMetadata.outputCount != 3 {
		t.Fatalf("explicit metadata = %#v", explicitMetadata)
	}
	if want := factoryMask | input1Mask | input2Mask | selectorMask; explicitMetadata.validation.required != want {
		t.Fatalf("explicit required = %b, want %b", explicitMetadata.validation.required, want)
	}
	if want := factoryMask | input1Mask | selectorMask; explicitMetadata.validation.present != want {
		t.Fatalf("explicit validation = %#v", explicitMetadata.validation)
	}
	closureMetadata := builder.nodes[1]
	if closureMetadata.kind != closureRegistration || closureMetadata.inputCount != 2 || closureMetadata.outputCount != 1 {
		t.Fatalf("closure metadata = %#v", closureMetadata)
	}
	if closureMetadata.validation.required != factoryMask || closureMetadata.validation.present != factoryMask {
		t.Fatalf("closure validation = %#v", closureMetadata.validation)
	}
	if explicit.Errors().PortID() != errorPortID || closure.Errors().PortID() != errorPortID {
		t.Fatal("registered error ports do not use PortID(9)")
	}
}

func TestRegistrationAtBoundaryArities(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	if _, err := AddNode0(builder, sourceDefinition()); err != nil {
		t.Fatalf("AddNode0() error = %v", err)
	}
	if _, err := AddClosureNode0(builder, func() (node.ClosureReplica[node.ClosureNode0[node.Outputs0, error]], error) {
		return node.ClosureReplica[node.ClosureNode0[node.Outputs0, error]]{}, nil
	}); err != nil {
		t.Fatalf("AddClosureNode0() error = %v", err)
	}
	if _, err := AddNode1(builder, node.Stateful[testState, node.Node1[testState, float64, node.Outputs8[int, string, bool, byte, rune, uint, int64, uint64], error]]{}); err != nil {
		t.Fatalf("AddNode1() error = %v", err)
	}
	if _, err := AddClosureNode1(builder, func() (node.ClosureReplica[node.ClosureNode1[float64, node.Outputs8[int, string, bool, byte, rune, uint, int64, uint64], error]], error) {
		return node.ClosureReplica[node.ClosureNode1[float64, node.Outputs8[int, string, bool, byte, rune, uint, int64, uint64], error]]{}, nil
	}); err != nil {
		t.Fatalf("AddClosureNode1() error = %v", err)
	}
	if _, err := AddNode8(builder, node.Stateful[testState, node.Node8[testState, int, string, bool, byte, rune, uint, int64, uint64, node.Outputs0, error]]{}); err != nil {
		t.Fatalf("AddNode8() error = %v", err)
	}
	if _, err := AddClosureNode8(builder, func() (node.ClosureReplica[node.ClosureNode8[int, string, bool, byte, rune, uint, int64, uint64, node.Outputs0, error]], error) {
		return node.ClosureReplica[node.ClosureNode8[int, string, bool, byte, rune, uint, int64, uint64, node.Outputs0, error]]{}, nil
	}); err != nil {
		t.Fatalf("AddClosureNode8() error = %v", err)
	}
}

func sourceDefinition() node.Stateful[testState, node.Node0[testState, node.Outputs0, error]] {
	return node.Stateful[testState, node.Node0[testState, node.Outputs0, error]]{}
}

func twoInputDefinition() node.Stateful[testState, node.Node2[testState, int, string, node.Outputs2[int, string], error]] {
	return node.Stateful[testState, node.Node2[testState, int, string, node.Outputs2[int, string], error]]{}
}
