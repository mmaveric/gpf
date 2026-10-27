package graph

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

func TestDefinitionWriteDOTShowsLogicalTopology(t *testing.T) {
	t.Parallel()

	b := NewBuilder()
	source, err := AddNode0(b, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	isolated, err := AddNode0(b, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs0, error]{
			Handle: func(*struct{}, *node.Outputs0) node.Result[error] { return node.Result[error]{} },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	if err := ReplicateGroup(b, DefaultGroupID, 2); err != nil {
		t.Fatal(err)
	}
	if err := BindReplicaCPU(b, DefaultGroupID, 1, 1); err != nil {
		t.Fatal(err)
	}
	definition, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	var output strings.Builder
	if err := definition.WriteDOT(&output); err != nil {
		t.Fatal(err)
	}
	expected := "digraph G {\n" +
		"  rankdir=LR;\n" +
		"  node [shape=box, style=rounded, fontname=Helvetica];\n" +
		"  n1 [label=\"stateful #1\"];\n" +
		"  n2 [label=\"stateful #2\"];\n" +
		"  n3 [label=\"stateful #3\"];\n" +
		"  n1 -> n2 [label=\"data: out1 → in1\"];\n" +
		"}\n"
	if got := output.String(); got != expected {
		t.Fatalf("WriteDOT() =\n%s\nwant:\n%s", got, expected)
	}
	var repeated strings.Builder
	if err := definition.WriteDOT(&repeated); err != nil {
		t.Fatal(err)
	}
	if repeated.String() != expected {
		t.Fatalf("repeated WriteDOT() =\n%s\nwant stable output:\n%s", repeated.String(), expected)
	}
	if isolated.NodeID() != 3 {
		t.Fatalf("isolated node ID = %d, want 3", isolated.NodeID())
	}
}

func TestDefinitionWriteDOTDistinguishesErrorEdges(t *testing.T) {
	t.Parallel()

	b := NewBuilder()
	source, err := AddNode0(b, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	errorSink, err := AddNode1(b, node.Stateful[validationState, node.Node1[validationState, error, node.Outputs0, error]]{
		Factory: func() (validationState, error) { return validationState{}, nil },
		Handlers: node.Node1[validationState, error, node.Outputs0, error]{
			Handle1: func(*validationState, batch.Batch[error], *node.Outputs0) node.Result[error] {
				return node.Result[error]{}
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	config := EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	if _, err := ConnectError(b, source.Errors(), errorSink.Inputs().In1, EdgeConfig[error]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	definition, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	var output strings.Builder
	if err := definition.WriteDOT(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `label="error: error → in1"`) {
		t.Fatalf("WriteDOT() does not distinguish the error edge:\n%s", output.String())
	}
}

func TestDefinitionWriteDOTPropagatesWriterErrors(t *testing.T) {
	t.Parallel()

	definition := builtValidDefinition(t)
	if err := definition.WriteDOT(failingDOTWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("WriteDOT() error = %v, want %v", err, io.ErrClosedPipe)
	}
	if err := definition.WriteDOT(shortDOTWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("WriteDOT() short-write error = %v, want %v", err, io.ErrShortWrite)
	}
}

func TestDefinitionWriteDOTRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	if err := (*Definition)(nil).WriteDOT(io.Discard); !errors.Is(err, ErrInvalidDefinition) {
		t.Fatalf("nil Definition.WriteDOT() error = %v, want %v", err, ErrInvalidDefinition)
	}
	definition := builtValidDefinition(t)
	if err := definition.WriteDOT(nil); !errors.Is(err, ErrInvalidDOTWriter) {
		t.Fatalf("WriteDOT(nil) error = %v, want %v", err, ErrInvalidDOTWriter)
	}
}

type failingDOTWriter struct{}

func (failingDOTWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type shortDOTWriter struct{}

func (shortDOTWriter) Write([]byte) (int, error) { return 0, nil }

func builtValidDefinition(t *testing.T) *Definition {
	t.Helper()
	b := NewBuilder()
	connectValidPair(t, b)
	definition, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	return definition
}
