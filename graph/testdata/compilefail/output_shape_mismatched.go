//go:build compilefail

package compilefail

import (
	"github.com/asharipov/gpf/graph"
	"github.com/asharipov/gpf/node"
)

func expandMismatchedOutput(reference graph.OutputRef[node.Outputs2[int, string]]) {
	_ = graph.OutputPorts1(reference)
}
