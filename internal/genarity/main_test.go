package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNodeGenerationStable(t *testing.T) {
	t.Parallel()

	targets, err := nodeTargets()
	if err != nil {
		t.Fatalf("nodeTargets() error = %v", err)
	}
	manifest := nodeManifest()
	if len(targets) != len(manifest) {
		t.Fatalf("target count = %d, manifest count = %d", len(targets), len(manifest))
	}
	for index := range targets {
		if targets[index].path != manifest[index] {
			t.Fatalf("target %d path = %q, want %q", index, targets[index].path, manifest[index])
		}
	}

	generatedDir := t.TempDir()
	if err := generateTargets(generatedDir, targets); err != nil {
		t.Fatalf("generateTargets() error = %v", err)
	}

	committedDir := filepath.Join("..", "..", "node")
	for _, path := range manifest {
		generated := readFile(t, filepath.Join(generatedDir, path))
		committed := readFile(t, filepath.Join(committedDir, path))
		if !bytes.Equal(generated, committed) {
			t.Errorf("generated %s differs from committed node file", path)
		}
	}
}

func TestGraphGenerationStable(t *testing.T) {
	t.Parallel()

	targets, err := graphTargets()
	if err != nil {
		t.Fatalf("graphTargets() error = %v", err)
	}
	manifest := graphManifest()
	if len(targets) != len(manifest) {
		t.Fatalf("target count = %d, manifest count = %d", len(targets), len(manifest))
	}
	for index := range targets {
		if targets[index].path != manifest[index] {
			t.Fatalf("target %d path = %q, want %q", index, targets[index].path, manifest[index])
		}
	}

	generatedDir := t.TempDir()
	if err := generateTargets(generatedDir, targets); err != nil {
		t.Fatalf("generateTargets() error = %v", err)
	}

	committedDir := filepath.Join("..", "..", "graph")
	for _, path := range manifest {
		generated := readFile(t, filepath.Join(generatedDir, path))
		committed := readFile(t, filepath.Join(committedDir, path))
		if !bytes.Equal(generated, committed) {
			t.Errorf("generated %s differs from committed graph file", path)
		}
	}
	entries, err := os.ReadDir(committedDir)
	if err != nil {
		t.Fatalf("reading graph directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_gen.go") && !slices.Contains(manifest, entry.Name()) {
			t.Errorf("unexpected graph-generated file %q", entry.Name())
		}
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %q: %v", path, err)
	}
	return content
}
