//go:build compilefail

package compilefail

import (
	"github.com/mmaveric/gpf/graph"
	"github.com/mmaveric/gpf/node"
)

func expandMismatchedOutput(reference graph.OutputRef[node.Outputs2[int, string]]) {
	_ = graph.OutputPorts1(reference)
}
