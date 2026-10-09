# runtime-metrics Specification

## Purpose
Define inspectable numeric runtime outcomes and queue-pressure observations for typed fixed-group pipeline execution without changing ownership or scheduling semantics.

## Requirements

### Requirement: Independent ordered executor snapshots
The graph package SHALL expose `func (e *Executor) Metrics() MetricsSnapshot` and numeric snapshot structs `MetricsSnapshot`, `GroupMetrics`, `NodeMetrics`, and `EdgeMetrics`. `MetricsSnapshot` SHALL contain `Groups []GroupMetrics`, `Nodes []NodeMetrics`, and `Edges []EdgeMetrics`. Group rows SHALL contain GroupID, node rows SHALL contain NodeID and GroupID, and edge rows SHALL contain EdgeID, SourceNodeID, TargetNodeID, SourceGroupID, TargetGroupID, ErrorEdge bool, and Remote bool. All identity fields SHALL use the existing graph ID types. Rows SHALL include every planned group, registered node, and connected ordinary or error edge, including rows with zero counts, ordered by ascending corresponding ID. Each snapshot SHALL own independent slice storage. Nil and zero-value executors SHALL return a zero snapshot with nil slices. Metrics SHALL be always enabled without new Start configuration.

#### Scenario: Inspect graph identities
- **WHEN** a graph with sparse groups and ordinary and error edges is running and Metrics is called
- **THEN** every entity appears once in numeric order with immutable placement and edge classification metadata

#### Scenario: Protect returned storage
- **WHEN** a caller mutates rows or slices in a returned snapshot
- **THEN** subsequent snapshots and execution are unaffected

#### Scenario: Inspect an invalid executor
- **WHEN** Metrics is called on a nil or zero-value executor
- **THEN** it returns a zero snapshot without panic

### Requirement: Node processing and failure counts
NodeMetrics SHALL expose uint64 HandlerCalls, InputBatches, InputElements, FailedResults, and UnconnectedErrors. HandlerCalls SHALL increment once for each normally returned source or input handler invocation, not selector calls, empty polls, or Wait retries. InputBatches and InputElements SHALL count the batch supplied to an invoked input handler and its length before user ownership changes, regardless of success; source calls SHALL contribute no synthetic input. FailedResults SHALL count failed handler returns once each. UnconnectedErrors SHALL count failed returns with no connected error edge, and SHALL NOT count connected pool exhaustion or queue Drop. Successful results SHALL not increment either failure count. Counters SHALL wrap modulo 2^64.

#### Scenario: Process a moved input
- **WHEN** a handler receives a batch of length 3, moves it to an ordinary output, and returns successfully
- **THEN** its node records one HandlerCalls, one InputBatches, and three InputElements without counting output publication as another input

#### Scenario: Discard an unconnected failure
- **WHEN** a source returns a failed result with no error edge
- **THEN** HandlerCalls, FailedResults, and UnconnectedErrors each increase by one and input counts remain unchanged

### Requirement: Edge transport and loss counts
EdgeMetrics SHALL expose all existing accepted/dequeued, policy-drop, failed-output, forced-shutdown, pool-exhaustion, and Wait retry counts and SHALL additionally expose CapacityRejectedBatches and CapacityRejectedElements for explicit broadcast branches that reject an oversized moved batch before copying. Capacity rejection SHALL be distinct from queue Drop and pool exhaustion. Existing semantics SHALL remain unchanged: accepted counts require successful queue transfer, dequeue counts require normal handler delivery, pool exhaustion is an attempt count without fabricated batch loss, and WaitRetries counts subsequent attempts but not the initial push. Batch lengths SHALL be captured before ownership clearing and counters SHALL wrap modulo 2^64.

#### Scenario: Retry a retained output
- **WHEN** an initial Wait push fails, two subsequent retries fail, and a third retry succeeds for a batch of length four
- **THEN** WaitRetries increases by three, accepted counts by one/four, and queue-drop/capacity-rejection counts remain unchanged

#### Scenario: Distinguish connected failure losses
- **WHEN** a connected error publication exhausts its pool and another acquired error batch encounters a full Drop queue
- **THEN** PoolExhaustions records one acquisition attempt and QueueDropped counts only the dropped committed batch

#### Scenario: Close pending and queued work
- **WHEN** Close discards one queued batch and one Wait-pending batch
- **THEN** shutdown discard counts each committed batch once and normal dequeue counts do not increase

#### Scenario: Distinguish sharding pool loss from queue Drop
- **WHEN** a sharding branch cannot acquire a staging slot for one element and a different branch successfully accepts its elements
- **THEN** the exhausted branch increments `PoolExhaustions` and `PoolDroppedElements` for the element, while the other branch's metrics remain unaffected

#### Scenario: Account for unassigned elements at forced shutdown
- **WHEN** Close discards a pending sharding operation before its cursor reaches the source batch end
- **THEN** the first branch's `ShardShutdownElements` increases by the remaining unassigned source-element count

### Requirement: Scheduler activity and sampled queue pressure
GroupMetrics SHALL expose uint64 Passes and IdlePasses. Each completed scheduler pass SHALL count once; a pass reporting no runner progress SHALL additionally count as idle. EdgeMetrics SHALL expose uint32 QueueCapacity, QueueDepthSample, and QueueHighWaterSample. Queue samples SHALL remain bounded by capacity, SHALL be updated from queue-owner observations after transport operations, and SHALL never call a custom queue callback from a snapshot reader. High-water values SHALL represent the maximum observed depth sample, not guarantee detection of every transient occupancy peak. Running snapshots SHALL not promise instantaneous queue depth or cross-field consistency. After completed Drain or Close, depth samples SHALL be zero and high-water samples SHALL be retained.

#### Scenario: Inspect custom queue pressure
- **WHEN** Metrics reads an executor using a custom local queue
- **THEN** it reads published numeric samples without invoking TryPush, TryPop, or Len on the reader goroutine

#### Scenario: Observe stopped queue pressure
- **WHEN** the executor finishes Drain or Close
- **THEN** its final snapshots report zero sampled depth and retain the previously observed high-water values

### Requirement: Race-safe runtime-local lifecycle
Metrics SHALL be safe concurrently with worker processing, other snapshot calls, and a single supported Drain or Close operation without pausing workers or changing source activation. Running snapshots SHALL use individually race-safe values but SHALL not promise a globally atomic snapshot. Completed stop SHALL retain exact cumulative counts and immutable metadata for future snapshots, including after repeated lifecycle calls. Each sequential Start SHALL create fresh counters; Definition and Build SHALL retain no mutable metric resources or execute callbacks to collect metrics. Existing unsupported concurrent lifecycle calls and concurrent Starts SHALL remain unsupported.

#### Scenario: Observe during processing and shutdown
- **WHEN** snapshot readers overlap local and remote processing followed by one Close operation under the race detector
- **THEN** reads complete without data races, queue callback ownership violations, or changes to processing and cleanup semantics

#### Scenario: Restart with fresh metrics
- **WHEN** an executor stops and its definition is started again
- **THEN** the new executor has no counts inherited from the prior execution and the prior executor retains its final snapshot values

### Requirement: Bounded typed instrumentation
Metric updates SHALL preserve zero framework heap allocations on supported built-in steady-state data paths. Instrumentation SHALL not box batch or error values, introduce reflection or runtime data type assertions, block processing on logging or exporting, or change pool, queue, failure, affinity, and ownership policies. Snapshot allocation SHALL be permitted in the control plane. Timing, timestamp callbacks, custom user metrics, exporters, logging, and pool-usage gauges SHALL remain deferred.

#### Scenario: Exercise instrumented built-in paths
- **WHEN** local or remote ordinary and error delivery, Drop, Wait retry, or pool exhaustion executes after setup
- **THEN** numeric outcomes update without framework heap allocation or altered ownership transfer
