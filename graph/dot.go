package graph

import (
	"errors"
	"io"
	"strconv"
	"strings"
)

// ErrInvalidDOTWriter reports a nil writer passed to Definition.WriteDOT.
var ErrInvalidDOTWriter = errors.New("invalid graph dot writer")

// WriteDOT writes the definition's logical topology as a Graphviz DOT graph.
//
// Node labels identify the registration kind and node ID. Edges include their
// kind and endpoint ports. The output order follows the definition's stable
// node and edge order.
func (d *Definition) WriteDOT(w io.Writer) error {
	if d == nil || d.owner == nil {
		return ErrInvalidDefinition
	}
	if w == nil {
		return ErrInvalidDOTWriter
	}

	var output strings.Builder
	output.WriteString("digraph G {\n")
	output.WriteString("  rankdir=LR;\n")
	output.WriteString("  node [shape=box, style=rounded, fontname=Helvetica];\n")
	for _, item := range d.nodes {
		label := strconv.Quote(nodeRegistrationName(item.kind) + " #" + strconv.FormatUint(uint64(item.nodeID), 10))
		output.WriteString("  n")
		output.WriteString(strconv.FormatUint(uint64(item.nodeID), 10))
		output.WriteString(" [label=")
		output.WriteString(label)
		output.WriteString("];\n")
	}
	for _, edge := range d.edges {
		kind, sourcePort := "data", "out"+strconv.FormatUint(uint64(edge.source.portID), 10)
		if edge.isError {
			kind, sourcePort = "error", "error"
		}
		label := kind + ": " + sourcePort + " → in" + strconv.FormatUint(uint64(edge.target.portID), 10)
		output.WriteString("  n")
		output.WriteString(strconv.FormatUint(uint64(edge.source.nodeID), 10))
		output.WriteString(" -> n")
		output.WriteString(strconv.FormatUint(uint64(edge.target.nodeID), 10))
		output.WriteString(" [label=")
		output.WriteString(strconv.Quote(label))
		output.WriteString("];\n")
	}
	output.WriteString("}\n")

	data := []byte(output.String())
	written, err := w.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func nodeRegistrationName(kind registrationKind) string {
	switch kind {
	case explicitRegistration:
		return "stateful"
	case closureRegistration:
		return "closure"
	case ingressRegistration:
		return "ingress"
	default:
		return "node"
	}
}
