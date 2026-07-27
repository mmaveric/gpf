// Package benchtest provides test support for data-plane allocation checks.
package benchtest

import "testing"

// AssertZeroAllocs verifies that operation performs no allocations after its
// caller has completed setup. It is intended for executable steady-state paths.
func AssertZeroAllocs(t *testing.T, runs int, operation func()) {
	t.Helper()
	if runs <= 0 {
		t.Fatal("runs must be greater than zero")
	}

	allocations := testing.AllocsPerRun(runs, operation)
	if allocations != 0 {
		t.Fatalf("allocations per run = %f, want 0", allocations)
	}
}
