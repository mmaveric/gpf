package node_test

import (
	"testing"

	"github.com/asharipov/gpf/node"
)

func TestNodeContractDeclarations(t *testing.T) {
	t.Parallel()

	_ = node.Outputs8[int, string, bool, byte, rune, float32, float64, complex64]{}
	_ = node.Node2[struct{}, int, string, node.Outputs0, error]{}

	result := node.Result[string]{
		Value:  "invalid record",
		Failed: true,
	}
	if !result.Failed || result.Value != "invalid record" {
		t.Fatalf("Result declaration was not preserved: %+v", result)
	}
}
