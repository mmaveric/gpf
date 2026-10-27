package graph_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mmaveric/gpf/graph"
	"github.com/mmaveric/gpf/node"
)

func TestGeneratedPortFamiliesCompile(t *testing.T) {
	var reference0 graph.OutputRef[node.Outputs0]
	var reference1 graph.OutputRef[node.Outputs1[int]]
	var reference2 graph.OutputRef[node.Outputs2[int, string]]
	var reference3 graph.OutputRef[node.Outputs3[int, string, bool]]
	var reference4 graph.OutputRef[node.Outputs4[int, string, bool, byte]]
	var reference5 graph.OutputRef[node.Outputs5[int, string, bool, byte, rune]]
	var reference6 graph.OutputRef[node.Outputs6[int, string, bool, byte, rune, uint]]
	var reference7 graph.OutputRef[node.Outputs7[int, string, bool, byte, rune, uint, int64]]
	var reference8 graph.OutputRef[node.Outputs8[int, string, bool, byte, rune, uint, int64, uint64]]

	_ = graph.InPorts0{}
	_ = graph.InPorts1[int]{}
	_ = graph.InPorts2[int, string]{}
	_ = graph.InPorts3[int, string, bool]{}
	_ = graph.InPorts4[int, string, bool, byte]{}
	_ = graph.InPorts5[int, string, bool, byte, rune]{}
	_ = graph.InPorts6[int, string, bool, byte, rune, uint]{}
	_ = graph.InPorts7[int, string, bool, byte, rune, uint, int64]{}
	_ = graph.InPorts8[int, string, bool, byte, rune, uint, int64, uint64]{}
	_ = graph.OutPorts0{}
	_ = graph.OutPorts1[int]{}
	_ = graph.OutPorts2[int, string]{}
	_ = graph.OutPorts3[int, string, bool]{}
	_ = graph.OutPorts4[int, string, bool, byte]{}
	_ = graph.OutPorts5[int, string, bool, byte, rune]{}
	_ = graph.OutPorts6[int, string, bool, byte, rune, uint]{}
	_ = graph.OutPorts7[int, string, bool, byte, rune, uint, int64]{}
	_ = graph.OutPorts8[int, string, bool, byte, rune, uint, int64, uint64]{}
	_ = graph.OutputPorts0(reference0)
	_ = graph.OutputPorts1(reference1)
	_ = graph.OutputPorts2(reference2)
	_ = graph.OutputPorts3(reference3)
	_ = graph.OutputPorts4(reference4)
	_ = graph.OutputPorts5(reference5)
	_ = graph.OutputPorts6(reference6)
	_ = graph.OutputPorts7(reference7)
	_ = graph.OutputPorts8(reference8)
}

func TestCompileFailFixturesDoNotCompile(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating test source")
	}
	command := exec.Command("go", "test", "-tags=compilefail", "./testdata/compilefail")
	command.Dir = filepath.Dir(currentFile)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("compile-fail fixture succeeded:\n%s", output)
	}
	for _, symbol := range []string{"OutputPorts1", "Connect", "ConnectError"} {
		if !bytes.Contains(output, []byte(symbol)) {
			t.Fatalf("compile failure does not mention %s:\n%s", symbol, output)
		}
	}
}
