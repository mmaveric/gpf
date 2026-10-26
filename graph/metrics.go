package graph

import (
	"github.com/asharipov/gpf/numa"
	"sync/atomic"
)

// MetricsSnapshot owns independently sampled, ordered runtime metric rows.
// Live fields are not a transaction. After stop, cumulative counts are exact.
type MetricsSnapshot struct {
	Ingresses []IngressMetrics
	Pools     []PoolMetrics
	Counters  []UserCounterMetrics
	Loggers   []LoggerMetrics
	Groups    []GroupMetrics
	Nodes     []NodeMetrics
	Edges     []EdgeMetrics
}

// GroupMetrics counts completed scheduler passes, including idle passes.
type GroupMetrics struct {
	Replica                                          uint32
	MeasuredPasses, IdleNanoseconds, WorkNanoseconds uint64
	GroupID                                          GroupID
	Passes, IdlePasses                               uint64
}

// NodeMetrics counts returned handlers and their input before ownership moves.
// Sources have no synthetic input batch. Counters wrap modulo 2^64.
type NodeMetrics struct {
	Replica                                                                     uint32
	MeasuredCalls, MeasuredNanoseconds, MaxMeasuredNanoseconds                  uint64
	NodeID                                                                      NodeID
	GroupID                                                                     GroupID
	HandlerCalls, InputBatches, InputElements, FailedResults, UnconnectedErrors uint64
}

// EdgeMetrics counts ownership transfers and distinct loss boundaries.
// PoolExhaustions counts attempts, not lost batches; WaitRetries includes a
// successful retry but excludes the initial push. Queue gauges are samples and
// can miss transient peaks. Counters wrap modulo 2^64.
type EdgeMetrics struct {
	NUMA                                                                 numa.Placement
	Replica                                                              uint32
	EdgeID                                                               EdgeID
	MergeID                                                              EdgeID
	BroadcastID                                                          EdgeID
	ShardID                                                              EdgeID
	SourceNodeID, TargetNodeID                                           NodeID
	SourceGroupID, TargetGroupID                                         GroupID
	ErrorEdge, Remote                                                    bool
	QueueCapacity, QueueDepthSample, QueueHighWaterSample                uint32
	AcceptedBatches, AcceptedElements, DequeuedBatches, DequeuedElements uint64
	QueueDroppedBatches, QueueDroppedElements                            uint64
	CapacityRejectedBatches, CapacityRejectedElements                    uint64
	FailedOutputBatches, FailedOutputElements                            uint64
	ShutdownDiscardedBatches, ShutdownDiscardedElements                  uint64
	PoolExhaustions, WaitRetries                                         uint64
	PoolDroppedElements, ShardShutdownElements                           uint64
}

type groupCounters struct {
	replica                        uint32
	clock                          func() uint64
	timingEvery                    uint64
	flushEvery                     uint64
	measured, idleNanos, workNanos atomic.Uint64
	identity                       GroupID
	passes, idle                   atomic.Uint64
	_                              [128]byte
}

type nodeCounters struct {
	clock                                         func() uint64
	timingEvery                                   uint64
	measured, nanoseconds, maximum                atomic.Uint64
	identity                                      NodeMetrics
	calls, batches, elements, failed, unconnected atomic.Uint64
	_                                             [128]byte
}

func (m *nodeCounters) input(length int) {
	m.batches.Add(1)
	m.elements.Add(uint64(length))
}

func (m *nodeCounters) result(failed bool) {
	m.calls.Add(1)
	if failed {
		m.failed.Add(1)
	}
}

type edgeProducerCounters struct {
	accepted, elements, dropped, droppedElements atomic.Uint64
	capacityRejected, capacityRejectedElements   atomic.Uint64
	poolDroppedElements, shardShutdownElements   atomic.Uint64
	failed, failedElements, exhausted, retries   atomic.Uint64
	_                                            [128]byte
}

type edgeConsumerCounters struct {
	dequeued, elements atomic.Uint64
	_                  [128]byte
}

type edgeCounters struct {
	identity                     EdgeMetrics
	producer                     edgeProducerCounters
	consumer                     edgeConsumerCounters
	discarded, discardedElements atomic.Uint64
	depth, high                  atomic.Uint32
	_                            [128]byte
}

func (m *edgeCounters) sample(depth uint32) {
	if depth > m.identity.QueueCapacity {
		depth = m.identity.QueueCapacity
	}
	m.depth.Store(depth)
	for old := m.high.Load(); depth > old; old = m.high.Load() {
		if m.high.CompareAndSwap(old, depth) {
			break
		}
	}
}

func (m *edgeCounters) discard(length int) {
	m.discarded.Add(1)
	m.discardedElements.Add(uint64(length))
}

type metricStore struct {
	ingresses []*ingressCounters
	live      atomic.Bool
	users     []*userCounter
	pools     []poolMetricsBinding
	loggers   []*loggerCounters
	groups    []*groupCounters
	nodes     []*nodeCounters
	edges     []*edgeCounters
}

func newMetricStore(d *Definition) *metricStore {
	m := &metricStore{}
	m.live.Store(true)
	var clock func() uint64
	var every uint64
	if d.timing != nil && d.timing.Mode != TimingDisabled {
		clock = d.timing.Clock
		if clock == nil {
			clock = timingClock(d.timing.ClockSource)
		}
		every = d.timing.Every
		if d.timing.Mode == TimingFull {
			every = 1
		}
	}
	for index, group := range d.groups {
		row := &groupCounters{identity: group.id}
		row.flushEvery = d.groupMetricsFlushEvery
		if row.flushEvery == 0 {
			row.flushEvery = 1
		}
		if len(d.groupInstances) != 0 {
			row.identity = d.groupInstances[index].logical
			row.replica = d.groupInstances[index].replica
		}
		if d.workerTiming != nil && d.workerTiming.Every != 0 {
			row.timingEvery = d.workerTiming.Every
			row.clock = d.workerTiming.Clock
			if row.clock == nil {
				row.clock = timingClock(d.workerTiming.ClockSource)
			}
		}
		m.groups = append(m.groups, row)
	}
	for _, counter := range d.counters {
		row := &userCounter{definition: counter, logical: counter.NodeID, group: d.placements[counter.NodeID]}
		if len(d.nodeInstances) != 0 {
			row.logical = d.nodeInstances[counter.NodeID-1].logical
			row.replica = d.nodeInstances[counter.NodeID-1].replica
			row.group = logicalGroupFor(d, row.group)
		}
		m.users = append(m.users, row)
	}
	for _, item := range d.nodes {
		m.nodes = append(m.nodes, &nodeCounters{clock: clock, timingEvery: every, identity: NodeMetrics{NodeID: item.nodeID, GroupID: d.placements[item.nodeID]}})
		if len(d.nodeInstances) != 0 {
			row := m.nodes[len(m.nodes)-1]
			row.identity.NodeID = d.nodeInstances[item.nodeID-1].logical
			row.identity.Replica = d.nodeInstances[item.nodeID-1].replica
			row.identity.GroupID = logicalGroupFor(d, row.identity.GroupID)
		}
	}
	for index, edge := range d.edges {
		m.edges = append(m.edges, &edgeCounters{identity: EdgeMetrics{
			EdgeID: edge.id, MergeID: d.edgeQueuePlans[index].MergeID, BroadcastID: d.edgeQueuePlans[index].BroadcastID, ShardID: d.edgeQueuePlans[index].ShardID, SourceNodeID: edge.source.nodeID, TargetNodeID: edge.target.nodeID,
			SourceGroupID: d.placements[edge.source.nodeID], TargetGroupID: d.placements[edge.target.nodeID],
			ErrorEdge: edge.isError, Remote: d.edgeRemote[index], QueueCapacity: edge.config.Capacity,
		}})
		if len(d.edgeInstances) != 0 {
			row := m.edges[len(m.edges)-1]
			row.identity.EdgeID = d.edgeInstances[index].logical
			row.identity.Replica = d.edgeInstances[index].replica
			row.identity.SourceNodeID = d.nodeInstances[edge.source.nodeID-1].logical
			row.identity.TargetNodeID = d.nodeInstances[edge.target.nodeID-1].logical
			row.identity.SourceGroupID = logicalGroupFor(d, row.identity.SourceGroupID)
			row.identity.TargetGroupID = logicalGroupFor(d, row.identity.TargetGroupID)
			row.identity.Replica = d.edgeInstances[index].replica
		}
	}
	return m
}

func (t *normalizationTarget) nodeMetrics(id NodeID) *nodeCounters {
	if t.metrics == nil {
		return &nodeCounters{}
	}
	return t.metrics.nodes[id-1]
}

// Metrics returns independent numeric rows without pausing workers or invoking
// user callbacks. It is safe alongside processing, readers, and one Drain or
// Close operation. Group pass metrics may lag by up to the configured flush
// interval minus one passes. Snapshot allocation belongs to the control plane.
func (e *Executor) Metrics() MetricsSnapshot {
	if e == nil || e.metrics == nil {
		return MetricsSnapshot{}
	}
	m := e.metrics
	s := MetricsSnapshot{Groups: make([]GroupMetrics, len(m.groups)), Nodes: make([]NodeMetrics, len(m.nodes)), Edges: make([]EdgeMetrics, len(m.edges))}
	if len(m.ingresses) != 0 {
		s.Ingresses = make([]IngressMetrics, len(m.ingresses))
		for i, row := range m.ingresses {
			s.Ingresses[i] = IngressMetrics{NodeID: row.id, GroupID: row.group, AcceptedBatches: row.accepted.Load(), AcceptedElements: row.elements.Load(), QueueDroppedBatches: row.dropped.Load(), QueueDroppedElements: row.droppedElements.Load(), PoolExhaustions: row.exhausted.Load()}
		}
	}
	if len(m.users) != 0 {
		s.Counters = make([]UserCounterMetrics, len(m.users))
		for i, row := range m.users {
			s.Counters[i] = UserCounterMetrics{NodeID: row.logical, Replica: row.replica, CounterID: row.definition.CounterID, GroupID: row.group, Value: row.value.Load()}
		}
	}
	if len(m.pools) != 0 {
		s.Pools = make([]PoolMetrics, len(m.pools))
		for i, pool := range m.pools {
			row := pool.identity
			usage := pool.read()
			row.Capacity, row.InUseSample, row.HighWaterSample = usage.Capacity, usage.InUseSample, usage.HighWaterSample
			s.Pools[i] = row
		}
	}
	if len(m.loggers) != 0 {
		s.Loggers = make([]LoggerMetrics, len(m.loggers))
		for i, row := range m.loggers {
			s.Loggers[i] = LoggerMetrics{GroupID: row.group, Replica: row.replica, AttemptedEvents: row.attempted.Load(), AcceptedEvents: row.accepted.Load(), QueueDroppedEvents: row.dropped.Load(), DeliveredEvents: row.delivered.Load(), ShutdownDiscardedEvents: row.discarded.Load()}
		}
	}
	for i, row := range m.groups {
		s.Groups[i] = GroupMetrics{GroupID: row.identity, Replica: row.replica, Passes: row.passes.Load(), IdlePasses: row.idle.Load(), MeasuredPasses: row.measured.Load(), IdleNanoseconds: row.idleNanos.Load(), WorkNanoseconds: row.workNanos.Load()}
	}
	for i, row := range m.nodes {
		v := row.identity
		v.HandlerCalls, v.InputBatches, v.InputElements = row.calls.Load(), row.batches.Load(), row.elements.Load()
		v.FailedResults, v.UnconnectedErrors = row.failed.Load(), row.unconnected.Load()
		v.MeasuredCalls, v.MeasuredNanoseconds, v.MaxMeasuredNanoseconds = row.measured.Load(), row.nanoseconds.Load(), row.maximum.Load()
		s.Nodes[i] = v
	}
	for i, row := range m.edges {
		v := row.identity
		v.AcceptedBatches, v.AcceptedElements = row.producer.accepted.Load(), row.producer.elements.Load()
		v.DequeuedBatches, v.DequeuedElements = row.consumer.dequeued.Load(), row.consumer.elements.Load()
		v.QueueDroppedBatches, v.QueueDroppedElements = row.producer.dropped.Load(), row.producer.droppedElements.Load()
		v.CapacityRejectedBatches, v.CapacityRejectedElements = row.producer.capacityRejected.Load(), row.producer.capacityRejectedElements.Load()
		v.FailedOutputBatches, v.FailedOutputElements = row.producer.failed.Load(), row.producer.failedElements.Load()
		v.PoolExhaustions, v.WaitRetries = row.producer.exhausted.Load(), row.producer.retries.Load()
		v.PoolDroppedElements, v.ShardShutdownElements = row.producer.poolDroppedElements.Load(), row.producer.shardShutdownElements.Load()
		v.ShutdownDiscardedBatches, v.ShutdownDiscardedElements = row.discarded.Load(), row.discardedElements.Load()
		v.QueueDepthSample, v.QueueHighWaterSample = row.depth.Load(), row.high.Load()
		s.Edges[i] = v
	}
	return s
}
