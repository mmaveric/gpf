package batch_test

import (
	"testing"

	"github.com/mmaveric/gpf/batch"
)

func TestPoolUtilizationAndHighWater(t *testing.T) {
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 2, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	first, ok := pool.Acquire()
	if !ok {
		t.Fatal("first acquire failed")
	}
	second, ok := pool.Acquire()
	if !ok {
		t.Fatal("second acquire failed")
	}
	if _, ok := pool.Acquire(); ok {
		t.Fatal("exhaustion expected")
	}
	if got := pool.Metrics(); got != (batch.PoolMetrics{Capacity: 2, InUseSample: 2, HighWaterSample: 2}) {
		t.Fatalf("pool metrics = %+v", got)
	}
	first.Release()
	second.Release()
	if got := pool.Metrics(); got.InUseSample != 0 || got.HighWaterSample != 2 {
		t.Fatalf("released pool metrics = %+v", got)
	}
	var empty *batch.Pool[int]
	if empty.Metrics() != (batch.PoolMetrics{}) {
		t.Fatal("nil pool metrics are nonzero")
	}
}
