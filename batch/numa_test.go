package batch_test

import (
	"sync/atomic"
	"testing"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/numa"
)

func TestNUMAPoolMetadataAndDormantTypedAllocator(t *testing.T) {
	var calls atomic.Uint64
	placement := numa.On(0)
	pool, err := batch.NewPoolWithOptions[int](batch.PoolOptions[int]{Config: batch.PoolConfig{Capacity: 2, BatchCapacity: 1, NUMA: placement}, Allocator: func(int) ([]int, error) { calls.Add(1); return make([]int, 2), nil }})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || pool.Metrics().NUMA != placement {
		t.Fatalf("allocator calls=%d NUMA=%+v", calls.Load(), pool.Metrics().NUMA)
	}
	if !placement.Present || placement.Node != 0 {
		t.Fatal("explicit NUMA node zero was confused with unspecified")
	}
	if _, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1, NUMA: numa.Placement{Node: 2}}); err == nil {
		t.Fatal("nonzero absent NUMA node accepted")
	}
}
