package batch_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/internal/benchtest"
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
	if value.Mutable() != nil || value.Items() != nil {
		t.Fatal("zero-value Batch exposes storage")
	}
}

func TestNewPool(t *testing.T) {
	tests := []struct {
		name    string
		config  batch.PoolConfig
		wantErr error
	}{
		{
			name: "valid configuration",
			config: batch.PoolConfig{
				Capacity:      2,
				BatchCapacity: 3,
			},
		},
		{
			name: "zero pool capacity",
			config: batch.PoolConfig{
				BatchCapacity: 1,
			},
			wantErr: batch.ErrInvalidPoolConfig,
		},
		{
			name: "zero batch capacity",
			config: batch.PoolConfig{
				Capacity: 1,
			},
			wantErr: batch.ErrInvalidPoolConfig,
		},
		{
			name: "storage size overflows int",
			config: batch.PoolConfig{
				Capacity:      ^uint32(0),
				BatchCapacity: ^uint32(0),
			},
			wantErr: batch.ErrPoolCapacityOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := batch.NewPool[int](tt.config)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewPool() error = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				if pool != nil {
					t.Fatal("NewPool() returned a pool with an error")
				}
				return
			}

			for range tt.config.Capacity {
				value, ok := pool.Acquire()
				if !ok {
					t.Fatal("Acquire() = unavailable, want pool slot")
				}
				if value.Cap() != int(tt.config.BatchCapacity) {
					t.Fatalf("Cap() = %d, want %d", value.Cap(), tt.config.BatchCapacity)
				}
			}
			if _, ok := pool.Acquire(); ok {
				t.Fatal("Acquire() succeeded after all slots were acquired")
			}
		})
	}
}

func TestBatchLifecycle(t *testing.T) {
	t.Parallel()

	pool, err := batch.NewPool[int](batch.PoolConfig{
		Capacity:      1,
		BatchCapacity: 4,
	})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	value, ok := pool.Acquire()
	if !ok {
		t.Fatal("Acquire() = unavailable")
	}
	writable := value.Mutable()
	if len(writable) != 4 || cap(writable) != 4 {
		t.Fatalf("Mutable() dimensions = %d/%d, want 4/4", len(writable), cap(writable))
	}
	writable[0] = 10
	writable[1] = 20
	if !value.SetLen(2) {
		t.Fatal("SetLen(2) = false, want true")
	}
	if !slices.Equal(value.Items(), []int{10, 20}) {
		t.Fatalf("Items() = %v, want [10 20]", value.Items())
	}
	if cap(value.Items()) != value.Len() {
		t.Fatalf("Items() capacity = %d, want %d", cap(value.Items()), value.Len())
	}
	if value.SetLen(5) {
		t.Fatal("SetLen(5) = true, want false")
	}
	if value.Len() != 2 {
		t.Fatalf("Len() after rejected SetLen = %d, want 2", value.Len())
	}

	moved := value.Move()
	if value.Cap() != 0 || value.Len() != 0 {
		t.Fatal("Move() did not clear source batch")
	}
	if !slices.Equal(moved.Items(), []int{10, 20}) {
		t.Fatalf("moved Items() = %v, want [10 20]", moved.Items())
	}
	pool.Release(&moved)
	if moved.Cap() != 0 || moved.Len() != 0 {
		t.Fatal("Release() did not clear batch handle")
	}

	reused, ok := pool.Acquire()
	if !ok {
		t.Fatal("Acquire() after Release() = unavailable")
	}
	if reused.Len() != 0 || reused.Cap() != 4 {
		t.Fatalf("reused batch dimensions = %d/%d, want 0/4", reused.Len(), reused.Cap())
	}
	pool.Release(&reused)
}

func TestBatchReleaseUsesOriginPoolAfterMove(t *testing.T) {
	t.Parallel()

	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	value, ok := pool.Acquire()
	if !ok {
		t.Fatal("Acquire() = unavailable")
	}
	moved := value.Move()
	moved.Release()
	if moved.Cap() != 0 {
		t.Fatal("Batch.Release() did not clear moved handle")
	}
	if _, ok := pool.Acquire(); !ok {
		t.Fatal("Batch.Release() did not return slot to origin pool")
	}
	var empty batch.Batch[int]
	empty.Release()
}

func TestPoolReleaseClearing(t *testing.T) {
	tests := []struct {
		name           string
		clearOnRelease bool
		wantRetained   bool
	}{
		{
			name:           "clear enabled",
			clearOnRelease: true,
		},
		{
			name:         "clear disabled",
			wantRetained: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker := 1
			pool, err := batch.NewPool[struct{ ref *int }](batch.PoolConfig{
				Capacity:       1,
				BatchCapacity:  4,
				ClearOnRelease: tt.clearOnRelease,
			})
			if err != nil {
				t.Fatalf("NewPool() error = %v", err)
			}

			value, ok := pool.Acquire()
			if !ok {
				t.Fatal("Acquire() = unavailable")
			}
			for index := range value.Mutable() {
				value.Mutable()[index].ref = &marker
			}
			if !value.SetLen(1) {
				t.Fatal("SetLen(1) = false, want true")
			}
			pool.Release(&value)

			reused, ok := pool.Acquire()
			if !ok {
				t.Fatal("Acquire() after Release() = unavailable")
			}
			for index, element := range reused.Mutable() {
				retained := element.ref != nil
				if retained != tt.wantRetained {
					t.Fatalf(
						"retained reference at index %d = %t, want %t",
						index,
						retained,
						tt.wantRetained,
					)
				}
			}
			pool.Release(&reused)
		})
	}
}

func TestPoolLifecycleAllocs(t *testing.T) {
	t.Parallel()

	pool, err := batch.NewPool[int](batch.PoolConfig{
		Capacity:      1,
		BatchCapacity: 64,
	})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	benchtest.AssertZeroAllocs(t, 1_000, func() {
		value, ok := pool.Acquire()
		if !ok {
			t.Fatal("Acquire() = unavailable")
		}
		value.Mutable()[0] = 1
		if !value.SetLen(1) {
			t.Fatal("SetLen(1) = false, want true")
		}
		pool.Release(&value)
	})
}

func TestPoolExhaustionAllocs(t *testing.T) {
	t.Parallel()

	pool, err := batch.NewPool[int](batch.PoolConfig{
		Capacity:      1,
		BatchCapacity: 1,
	})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	held, ok := pool.Acquire()
	if !ok {
		t.Fatal("Acquire() = unavailable")
	}
	defer pool.Release(&held)

	benchtest.AssertZeroAllocs(t, 1_000, func() {
		value, available := pool.Acquire()
		if available || value.Cap() != 0 {
			t.Fatal("Acquire() during exhaustion returned a batch")
		}
	})
}

func FuzzNewPoolConfig(f *testing.F) {
	f.Add(uint32(1), uint32(1))
	f.Add(uint32(0), uint32(1))
	f.Add(uint32(1), uint32(0))

	f.Fuzz(func(t *testing.T, capacity, batchCapacity uint32) {
		if capacity > 32 || batchCapacity > 32 {
			return
		}

		pool, err := batch.NewPool[int](batch.PoolConfig{
			Capacity:      capacity,
			BatchCapacity: batchCapacity,
		})
		if capacity == 0 || batchCapacity == 0 {
			if !errors.Is(err, batch.ErrInvalidPoolConfig) || pool != nil {
				t.Fatalf("NewPool() = (%v, %v), want invalid configuration", pool, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}

		value, ok := pool.Acquire()
		if !ok || value.Cap() != int(batchCapacity) {
			t.Fatalf("Acquire() dimensions = %d/%t, want %d/true", value.Cap(), ok, batchCapacity)
		}
		pool.Release(&value)
	})
}

func FuzzBatchSetLen(f *testing.F) {
	f.Add(0)
	f.Add(8)
	f.Add(-1)
	f.Add(9)

	f.Fuzz(func(t *testing.T, length int) {
		pool, err := batch.NewPool[int](batch.PoolConfig{
			Capacity:      1,
			BatchCapacity: 8,
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		value, ok := pool.Acquire()
		if !ok {
			t.Fatal("Acquire() = unavailable")
		}
		defer pool.Release(&value)

		set := value.SetLen(length)
		if set && (value.Len() != length || value.Len() > value.Cap()) {
			t.Fatalf("SetLen(%d) produced invalid length %d/%d", length, value.Len(), value.Cap())
		}
		if !set && value.Len() != 0 {
			t.Fatalf("rejected SetLen(%d) changed length to %d", length, value.Len())
		}
	})
}
