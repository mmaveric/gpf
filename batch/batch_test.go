package batch_test

import (
	"testing"

	"github.com/asharipov/gpf/batch"
)

func TestBatchZeroValue(t *testing.T) {
	t.Parallel()

	var value batch.Batch[int]
	if value.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", value.Len())
	}
	if value.Cap() != 0 {
		t.Fatalf("Cap() = %d, want 0", value.Cap())
	}
}

func TestPoolConfigDeclaration(t *testing.T) {
	t.Parallel()

	config := batch.PoolConfig{
		Capacity:       4,
		BatchCapacity:  64,
		ClearOnRelease: true,
	}
	if config.Capacity != 4 || config.BatchCapacity != 64 || !config.ClearOnRelease {
		t.Fatalf("PoolConfig declaration was not preserved: %+v", config)
	}
}
