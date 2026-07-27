//go:build ignore

package compilefail

import "github.com/asharipov/gpf/graph"

func connectMismatchedPorts() {
	var definition graph.Definition
	var output graph.OutPort[int]
	var input graph.InPort[string]

	_ = graph.Connect(&definition, output, input, graph.EdgeConfig{})
}
