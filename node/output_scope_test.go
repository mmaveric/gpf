package node_test

import (
	"testing"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
)

func TestOutputExplicitReleaseIsNotRepeated(t *testing.T) {
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	var output node.Output[int]
	node.InstallOutput(&output, pool.Acquire)
	value, ok := output.Acquire()
	if !ok {
		t.Fatal("acquire failed")
	}
	value.Release()
	// Reuse before finalization proves a duplicate release would corrupt a new
	// owner, rather than merely looking harmless in an already empty pool.
	owned, ok := pool.Acquire()
	if !ok {
		t.Fatal("released slot unavailable")
	}
	owned.SetLen(1)
	if !node.FinalizeOutput(&output, false, func(batch.Batch[int]) bool { return true }) {
		t.Fatal("finalization failed")
	}
	if owned.Len() != 1 || pool.Metrics().InUseSample != 1 {
		t.Fatal("cleanup released a subsequent owner")
	}
	owned.Release()
}

func TestOutputReclaimsUnusedAcquisitionWhenCommittingInput(t *testing.T) {
	scratch, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	origin, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	var output node.Output[int]
	node.InstallOutput(&output, scratch.Acquire)
	if _, ok := output.Acquire(); !ok {
		t.Fatal("scratch acquisition failed")
	}
	input, ok := origin.Acquire()
	if !ok {
		t.Fatal("input acquisition failed")
	}
	input.SetLen(1)
	if !output.Commit(&input) {
		t.Fatal("input commit failed")
	}
	if !node.FinalizeOutput(&output, false, func(value batch.Batch[int]) bool {
		if value.Len() != 1 {
			t.Error("committed input was reset")
		}
		value.Release()
		return true
	}) {
		t.Fatal("publication failed")
	}
	if scratch.Metrics().InUseSample != 0 || origin.Metrics().InUseSample != 0 {
		t.Fatal("unused acquisition or committed input leaked")
	}
}
