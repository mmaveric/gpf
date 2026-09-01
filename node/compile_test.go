package node_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOutputSetIsSealed(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating test source")
	}

	command := exec.Command("go", "test", "./testdata/compilefail")
	command.Dir = filepath.Dir(currentFile)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("compile-fail fixture succeeded:\n%s", output)
	}
	if !bytes.Contains(output, []byte("outputSet")) {
		t.Fatalf("compile failure does not mention sealed OutputSet:\n%s", output)
	}
}
