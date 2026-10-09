package batch

import "github.com/asharipov/gpf/numa"

// PoolMetrics observes bounded utilization without reading batch contents.
// Fields are independently sampled; HighWaterSample retains observed peak use.
type PoolMetrics struct {
	Capacity, InUseSample, HighWaterSample uint32
	NUMA                                   numa.Placement
}

// Metrics returns allocation-free atomic utilization observations.
func (p *Pool[T]) Metrics() PoolMetrics {
	if p == nil {
		return PoolMetrics{}
	}
	return PoolMetrics{Capacity: uint32(len(p.slots)), InUseSample: p.inUse.Load(), HighWaterSample: p.highWater.Load(), NUMA: p.numa}
}

func (p *Pool[T]) acquired() {
	used := p.inUse.Add(1)
	for old := p.highWater.Load(); used > old; old = p.highWater.Load() {
		if p.highWater.CompareAndSwap(old, used) {
			break
		}
	}
}
