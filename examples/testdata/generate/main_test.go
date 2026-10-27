package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCommandRejectsUnsafeOutputs(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.json")
	if err := os.WriteFile(existing, []byte("preserve me"), 0o600); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(dir, "new.json")
	tests := []struct {
		name string
		args []string
	}{
		{name: "no implicit outputs"},
		{name: "existing valid", args: []string{"-valid-out", existing}},
		{name: "existing mixed preflight", args: []string{"-valid-out", newPath, "-mixed-out", existing}},
		{name: "same output", args: []string{"-valid-out", newPath, "-mixed-out", newPath}},
		{name: "directory", args: []string{"-valid-out", dir}},
		{name: "positional argument", args: []string{"unexpected"}},
		{name: "unknown flag", args: []string{"-unknown"}},
		{name: "missing parent", args: []string{"-valid-out", filepath.Join(dir, "missing", "data.json")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := runCommand(tt.args, io.Discard); err == nil {
				t.Fatal("error = nil, want refusal")
			}
			data, err := os.ReadFile(existing)
			if err != nil || string(data) != "preserve me" {
				t.Fatalf("existing bytes changed: %q, %v", data, err)
			}
			if _, err := os.Lstat(newPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected new output after refusal: %v", err)
			}
		})
	}
}

func TestRunCommandHelp(t *testing.T) {
	if err := runCommand([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v", err)
	}
}

func TestRunCommandCreatesOutputAndRefusesSecondRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "valid.json")
	args := []string{"-valid-out", path}
	if err := runCommand(args, io.Discard); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		t.Fatal(err)
	}
	want := "ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460"
	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != want {
		t.Fatalf("generated file SHA-256 = %s, want %s", got, want)
	}
	if err := runCommand(args, io.Discard); err == nil {
		t.Fatal("second run overwrote existing output")
	}
}

func TestRunCommandRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link.json")
	target := filepath.Join(dir, "missing.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := runCommand([]string{"-valid-out", link}, io.Discard); err == nil {
		t.Fatal("accepted an existing symlink")
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink target created: %v", err)
	}
}

func TestGenerateKnownHashes(t *testing.T) {
	tests := []struct {
		name  string
		mixed bool
		want  string
	}{
		{
			name: "valid",
			want: "ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460",
		},
		{
			name:  "mixed",
			mixed: true,
			want:  "e77a1fcf1b062915b644308422ca53601e83550d547c63de4cd453ad783a15f0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := sha256.New()
			if err := generate(hash, orderCount, tt.mixed); err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", hash.Sum(nil)); got != tt.want {
				t.Fatalf("SHA-256 = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGenerateMixedRecords(t *testing.T) {
	var output bytes.Buffer
	if err := generate(&output, 64, true); err != nil {
		t.Fatal(err)
	}
	var orders []struct {
		ID         uint64 `json:"id"`
		CustomerID uint64 `json:"customerId"`
		Units      uint32 `json:"units"`
		UnitCents  uint32 `json:"unitCents"`
	}
	if err := json.Unmarshal(output.Bytes(), &orders); err != nil {
		t.Fatal(err)
	}
	if len(orders) != 64 || orders[0].ID != 0 || orders[21].Units != 0 || orders[42].UnitCents != 0 {
		t.Fatalf("wrong rotating invalid records: %+v", orders)
	}
	if orders[1].ID != 2 || orders[1].CustomerID != 2 || orders[1].Units != 2 || orders[1].UnitCents != 87 {
		t.Fatalf("valid record changed: %+v", orders[1])
	}
}

var errWrite = errors.New("test write failure")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestGeneratePropagatesWriteError(t *testing.T) {
	for _, count := range []int{1, 2_000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			if err := generate(failingWriter{}, count, false); !errors.Is(err, errWrite) {
				t.Fatalf("write error = %v", err)
			}
		})
	}
}
