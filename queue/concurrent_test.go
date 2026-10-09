package queue

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/asharipov/gpf/batch"
)

func TestConcurrentQueueConstructorsAndOwnership(t *testing.T) {
	tests := []struct {
		name string
		new  func(uint32) (QueueOps[int], error)
	}{
		{name: "MPSC", new: func(capacity uint32) (QueueOps[int], error) {
			q, err := NewMPSC[int](capacity)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}},
		{name: "SPMC", new: func(capacity uint32) (QueueOps[int], error) {
			q, err := NewSPMC[int](capacity)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}},
		{name: "MPMC", new: func(capacity uint32) (QueueOps[int], error) {
			q, err := NewMPMC[int](capacity)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.new(0); err != ErrInvalidCapacity {
				t.Fatalf("zero capacity error = %v, want %v", err, ErrInvalidCapacity)
			}
			ops, err := tt.new(2)
			if err != nil {
				t.Fatal(err)
			}
			pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 3, BatchCapacity: 1})
			if err != nil {
				t.Fatal(err)
			}
			values := make([]batch.Batch[int], 3)
			for index := range values {
				value, ok := pool.Acquire()
				if !ok {
					t.Fatal("pool exhausted before queue test")
				}
				value.Mutable()[0] = index + 10
				value.SetLen(1)
				values[index] = value
			}
			if !ops.TryPush(values[0]) || !ops.TryPush(values[1]) {
				t.Fatal("queue rejected available capacity")
			}
			if ops.TryPush(values[2]) {
				t.Fatal("queue accepted beyond capacity")
			}
			if values[2].Len() != 1 || values[2].Items()[0] != 12 {
				t.Fatal("failed push changed caller-owned batch")
			}
			if got := ops.Len(); got > 2 {
				t.Fatalf("Len() = %d, exceeds capacity 2", got)
			}
			var first, second batch.Batch[int]
			if !ops.TryPop(&first) || !ops.TryPop(&second) {
				t.Fatal("queued values unavailable")
			}
			if first.Items()[0] != 10 || second.Items()[0] != 11 {
				t.Fatalf("FIFO values = %v, %v", first.Items(), second.Items())
			}
			first.Release()
			second.Release()
			values[2].Release()
			if ops.Len() != 0 {
				t.Fatalf("Len() after drain = %d, want 0", ops.Len())
			}
		})
	}
}

func TestConcurrentRingDelayedReservationAndWrap(t *testing.T) {
	q, err := newConcurrentRing[int](2)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 2, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := pool.Acquire()
	first.Mutable()[0] = 1
	first.SetLen(1)
	second, _ := pool.Acquire()
	second.Mutable()[0] = 2
	second.SetLen(1)
	position, cell, ok := q.reservePush()
	if !ok {
		t.Fatal("first reservation failed")
	}
	if !q.tryPush(second) {
		t.Fatal("second producer reservation failed")
	}
	var got batch.Batch[int]
	if q.tryPop(&got) {
		t.Fatal("consumer passed an unpublished head reservation")
	}
	q.publish(position, cell, first)
	if !q.tryPop(&got) || got.Items()[0] != 1 {
		t.Fatal("first publication unavailable after reservation completed")
	}
	got.Release()
	if !q.tryPop(&got) || got.Items()[0] != 2 {
		t.Fatal("second publication unavailable")
	}
	got.Release()

	wrapped, err := newConcurrentRingAt[int](3, 2)
	if err != nil {
		t.Fatal(err)
	}
	pool, err = batch.NewPool[int](batch.PoolConfig{Capacity: 4, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	for value := 0; value < 3; value++ {
		item, available := pool.Acquire()
		if !available {
			t.Fatal("wrap pool exhausted")
		}
		item.Mutable()[0] = value
		item.SetLen(1)
		if !wrapped.tryPush(item) {
			t.Fatalf("push %d failed near wrap", value)
		}
	}
	item, available := pool.Acquire()
	if !available {
		t.Fatal("wrap pool exhausted before full test")
	}
	item.Mutable()[0] = 3
	item.SetLen(1)
	if wrapped.tryPush(item) {
		t.Fatal("queue exceeded capacity across counter wrap")
	}
	for value := 0; value < 3; value++ {
		if !wrapped.tryPop(&got) || got.Items()[0] != value {
			t.Fatalf("wrap pop %d = %v", value, got.Items())
		}
		got.Release()
	}
	if !wrapped.tryPush(item) {
		t.Fatal("queue failed to reuse slot across counter wrap")
	}
	if !wrapped.tryPop(&got) || got.Items()[0] != 3 {
		t.Fatal("wrapped slot value unavailable")
	}
	got.Release()
}

func TestConcurrentQueueStressOwnership(t *testing.T) {
	tests := []struct {
		name                 string
		new                  func(uint32) (QueueOps[int], error)
		producers, consumers int
	}{
		{name: "MPSC", new: func(capacity uint32) (QueueOps[int], error) {
			q, err := NewMPSC[int](capacity)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}, producers: 4, consumers: 1},
		{name: "SPMC", new: func(capacity uint32) (QueueOps[int], error) {
			q, err := NewSPMC[int](capacity)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}, producers: 1, consumers: 4},
		{name: "MPMC", new: func(capacity uint32) (QueueOps[int], error) {
			q, err := NewMPMC[int](capacity)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}, producers: 4, consumers: 4},
	}
	const valuesPerProducer = 1_000
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops, err := tt.new(64)
			if err != nil {
				t.Fatal(err)
			}
			total := tt.producers * valuesPerProducer
			seen := make([]atomic.Uint32, total)
			pools := make([]*batch.Pool[int], tt.producers)
			for i := range pools {
				pools[i], err = batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
				if err != nil {
					t.Fatal(err)
				}
			}
			var consumed atomic.Int64
			var workers sync.WaitGroup
			workers.Add(tt.consumers)
			for range tt.consumers {
				go func() {
					defer workers.Done()
					for consumed.Load() < int64(total) {
						var value batch.Batch[int]
						if !ops.TryPop(&value) {
							runtime.Gosched()
							continue
						}
						id := value.Items()[0]
						if id < 0 || id >= total || !seen[id].CompareAndSwap(0, 1) {
							t.Errorf("invalid or duplicate value %d", id)
						}
						value.Release()
						consumed.Add(1)
					}
				}()
			}
			workers.Add(tt.producers)
			for producer := range tt.producers {
				go func(producer int) {
					defer workers.Done()
					for index := 0; index < valuesPerProducer; index++ {
						value, available := pools[producer].Acquire()
						for !available {
							runtime.Gosched()
							value, available = pools[producer].Acquire()
						}
						id := producer*valuesPerProducer + index
						value.Mutable()[0] = id
						value.SetLen(1)
						for !ops.TryPush(value) {
							runtime.Gosched()
						}
					}
				}(producer)
			}
			workers.Wait()
			if consumed.Load() != int64(total) {
				t.Fatalf("consumed = %d, want %d", consumed.Load(), total)
			}
			for id := range seen {
				if seen[id].Load() != 1 {
					t.Fatalf("value %d was lost", id)
				}
			}
			if ops.Len() != 0 {
				t.Fatalf("queue Len() = %d after stress run", ops.Len())
			}
		})
	}
}

func TestConcurrentQueueSteadyStateAllocs(t *testing.T) {
	tests := []struct {
		name string
		new  func() (QueueOps[int], error)
	}{
		{name: "MPSC", new: func() (QueueOps[int], error) {
			q, err := NewMPSC[int](1)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}},
		{name: "SPMC", new: func() (QueueOps[int], error) {
			q, err := NewSPMC[int](1)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}},
		{name: "MPMC", new: func() (QueueOps[int], error) {
			q, err := NewMPMC[int](1)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops, err := tt.new()
			if err != nil {
				t.Fatal(err)
			}
			pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
			if err != nil {
				t.Fatal(err)
			}
			value, ok := pool.Acquire()
			if !ok {
				t.Fatal("pool acquisition failed")
			}
			operation := func() {
				value.Mutable()[0] = 9
				value.SetLen(1)
				if !ops.TryPush(value) {
					panic("queue push failed")
				}
				if !ops.TryPop(&value) || value.Items()[0] != 9 {
					panic("queue pop failed")
				}
			}
			if allocations := testing.AllocsPerRun(1_000, operation); allocations != 0 {
				t.Fatalf("allocations = %f, want 0", allocations)
			}
			pool.Release(&value)
		})
	}
}

func BenchmarkConcurrentQueues(b *testing.B) {
	benchmarks := []struct {
		name string
		new  func() (QueueOps[int], error)
	}{
		{name: "MPSC", new: func() (QueueOps[int], error) {
			q, err := NewMPSC[int](64)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}},
		{name: "SPMC", new: func() (QueueOps[int], error) {
			q, err := NewSPMC[int](64)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}},
		{name: "MPMC", new: func() (QueueOps[int], error) {
			q, err := NewMPMC[int](64)
			if err != nil {
				return QueueOps[int]{}, err
			}
			return q.Ops(), nil
		}},
	}
	for _, benchmark := range benchmarks {
		b.Run(benchmark.name+"/capacity=64", func(b *testing.B) {
			ops, err := benchmark.new()
			if err != nil {
				b.Fatal(err)
			}
			pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
			if err != nil {
				b.Fatal(err)
			}
			var received batch.Batch[int]
			b.ReportAllocs()
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				value, ok := pool.Acquire()
				if !ok {
					b.Fatal("pool acquisition failed")
				}
				value.Mutable()[0] = iteration
				value.SetLen(1)
				if !ops.TryPush(value) {
					b.Fatal("queue push failed")
				}
				if !ops.TryPop(&received) {
					b.Fatal("queue pop failed")
				}
				received.Release()
			}
		})
	}
}
