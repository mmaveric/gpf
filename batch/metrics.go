package batch

import "github.com/asharipov/gpf/numa"

// PoolMetrics reports sampled pool utilization without reading batch contents.
// InUseSample and HighWaterSample are independent observations; the latter is
// the greatest use observed by the pool so far.
type PoolMetrics struct {
	Capacity, InUseSample, HighWaterSample uint32
	NUMA                                   numa.Placement
}

// Metrics returns allocation-free utilization observations. A nil pool returns
// zero-valued metrics; samples may reflect different instants.
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
