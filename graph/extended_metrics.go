package graph

import (
	"errors"
	"sort"
	"sync/atomic"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/numa"
)

type CounterID = node.CounterID

var (
	ErrInvalidCounterID  = errors.New("invalid user counter ID")
	ErrCounterRegistered = errors.New("user counter already registered")
	ErrUnknownCounter    = errors.New("unknown user counter")
)

type counterDefinition struct {
	NodeID    NodeID
	CounterID CounterID
}
type userCounter struct {
	logical    NodeID
	replica    uint32
	definition counterDefinition
	group      GroupID
	value      atomic.Uint64
}

// UserCounterMetrics is a copied numeric application counter row.
type UserCounterMetrics struct {
	Replica   uint32
	NodeID    NodeID
	CounterID CounterID
	GroupID   GroupID
	Value     uint64
}

// PoolMetrics identifies one edge-origin pool's utilization observations.
type PoolMetrics struct {
	Replica                                uint32
	SourceReplica                          uint32
	EdgeID                                 EdgeID
	MergeID                                EdgeID
	BroadcastID                            EdgeID
	ShardID                                EdgeID
	SourceNodeID                           NodeID
	SourceGroupID                          GroupID
	Capacity, InUseSample, HighWaterSample uint32
	NUMA                                   numa.Placement
}

type poolMetricsBinding struct {
	identity PoolMetrics
	read     func() batch.PoolMetrics
}

// RegisterCounter reserves a numeric slot before Build without executing user
// callbacks. IDs are unique within a logical node; different nodes may reuse ID.
func RegisterCounter(b *Builder, id NodeID, counter CounterID) error {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return ErrBuilderSealed
	}
	if id == 0 || int(id) > len(b.nodes) {
		return ErrUnknownNode
	}
	if counter == 0 {
		return ErrInvalidCounterID
	}
	key := counterDefinition{id, counter}
	if _, ok := b.counters[key]; ok {
		return ErrCounterRegistered
	}
	if b.counters == nil {
		b.counters = make(map[counterDefinition]struct{})
	}
	b.counters[key] = struct{}{}
	b.revision++
	return nil
}

func compileCounters(b *Builder) []counterDefinition {
	defs := make([]counterDefinition, 0, len(b.counters))
	for def := range b.counters {
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].NodeID != defs[j].NodeID {
			return defs[i].NodeID < defs[j].NodeID
		}
		return defs[i].CounterID < defs[j].CounterID
	})
	return defs
}

func counterAdd(store *metricStore, counter *userCounter) func(uint64) {
	return func(value uint64) {
		if store.live.Load() {
			counter.value.Add(value)
		}
	}
}

// Counter resolves a registered update function in the control plane. Updates
// use only its bound atomic slot and become no-ops after completed shutdown.
func (e *Executor) Counter(id NodeID, counter CounterID) (func(uint64), error) {
	if e == nil || e.metrics == nil {
		return nil, ErrInvalidDefinition
	}
	var selected *userCounter
	for _, row := range e.metrics.users {
		if row.logical == id && row.definition.CounterID == counter {
			if selected != nil {
				return nil, ErrAmbiguousCounter
			}
			selected = row
		}
	}
	if selected == nil {
		return nil, ErrUnknownCounter
	}
	return counterAdd(e.metrics, selected), nil
}

// CounterForReplica resolves a single zero-based replica-local counter slot.
func (e *Executor) CounterForReplica(id NodeID, replica uint32, counter CounterID) (func(uint64), error) {
	if e == nil || e.metrics == nil {
		return nil, ErrInvalidDefinition
	}
	for _, row := range e.metrics.users {
		if row.logical == id && row.replica == replica && row.definition.CounterID == counter {
			return counterAdd(e.metrics, row), nil
		}
	}
	return nil, ErrUnknownCounter
}

func bindCounters[S any](target *normalizationTarget, id NodeID, state *S, bind node.CounterBinder[S]) {
	if bind == nil || target.metrics == nil {
		return
	}
	for _, row := range target.metrics.users {
		if row.definition.NodeID == id {
			bind(state, row.definition.CounterID, counterAdd(target.metrics, row))
		}
	}
}

func bindClosureCounters(target *normalizationTarget, id NodeID, bind node.ClosureCounterBinder) {
	if bind == nil || target.metrics == nil {
		return
	}
	for _, row := range target.metrics.users {
		if row.definition.NodeID == id {
			bind(row.definition.CounterID, counterAdd(target.metrics, row))
		}
	}
}

// WorkerTimingConfig samples complete scheduler-pass wall durations. It does
// not measure OS CPU usage. Every zero disables collection; nil Clock uses the
// monotonic default, independently of handler timing configuration.
type WorkerTimingConfig struct {
	Every uint64
	Clock func() uint64
}

func SetWorkerTiming(b *Builder, config WorkerTimingConfig) error {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return ErrBuilderSealed
	}
	if b.workerTiming != nil {
		return ErrTimingConfigured
	}
	copy := config
	b.workerTiming = &copy
	b.revision++
	return nil
}

func (m *groupCounters) beginTiming() (uint64, bool) {
	if m.clock == nil || m.timingEvery == 0 || m.passes.Load()%m.timingEvery != 0 {
		return 0, false
	}
	return m.clock(), true
}

func (m *groupCounters) recordPass(progress bool, started uint64, measured bool) {
	if measured {
		elapsed := m.clock() - started
		m.measured.Add(1)
		if progress {
			m.workNanos.Add(elapsed)
		} else {
			m.idleNanos.Add(elapsed)
		}
	}
	m.passes.Add(1)
	if !progress {
		m.idle.Add(1)
	}
}
