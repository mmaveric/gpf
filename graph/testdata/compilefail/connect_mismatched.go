//go:build compilefail

package compilefail

import "github.com/asharipov/gpf/graph"

func connectMismatchedPorts() {
	var output graph.OutPort[int]
	var input graph.InPort[string]
	var errorOutput graph.ErrorPort[int]

	_, _ = graph.Connect(nil, output, input, graph.EdgeConfig[int]{})
	_, _ = graph.Connect(nil, errorOutput, graph.InPort[int]{}, graph.EdgeConfig[int]{})
	_, _ = graph.ConnectError(nil, errorOutput, input, graph.EdgeConfig[int]{})
}
