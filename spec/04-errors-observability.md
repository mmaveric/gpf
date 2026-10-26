# Errors and observability

This document consolidates the current normative requirements. Scenarios are retained where they add behavioral detail; shared concepts should be read together across sections.


## typed-error-routing

Define explicit statically typed error routing and failed-result ownership semantics.


### Explicit statically typed failure route

The graph package SHALL expose opaque `ErrorPort[E]` handles and `func ConnectError[E any](builder *Builder, output ErrorPort[E], input InPort[E], config EdgeConfig[E]) (EdgeID, error)`. `ErrorPort[E]` SHALL expose value-receiver `NodeID() NodeID`, `PortID() PortID`, and `Valid() bool` accessors, retain private provenance, and identify only the registered error output at `PortID(9)`. Ordinary `Connect` SHALL accept only `OutPort[T]`, not `ErrorPort[T]`. A `ConnectError` call with mismatched error and input element types SHALL be rejected by the Go compiler without a runtime type registry, conversion, reflection, or type assertion.

#### Connect a matching error route
- a registered node with error type `DomainError` passes its `ErrorPort[DomainError]` and another node's `InPort[DomainError]` to `ConnectError`
- the program compiles and the builder records one typed error edge

#### Reject a mismatched error route at compile time
- a program passes `ErrorPort[DomainError]` and `InPort[OtherError]` to `ConnectError`
- compilation fails before graph validation or runtime construction


### Failed result publication and ownership

After a handler returns `node.Result[E]{Failed: true}`, the executor SHALL discard every acquired or committed ordinary output, release every un-moved input, and never publish an ordinary output. If its `ErrorPort[E]` is unconnected, it SHALL discard `Result.Value` without allocating a batch or retaining an error resource. If connected, it SHALL acquire one `batch.Batch[E]` from that error edge's source pool, write `Result.Value` at index zero, set its logical length to one, and publish it to the typed edge.

An exhausted error source pool SHALL discard the failure value without retaining a pending result. A successfully acquired error batch SHALL transfer ownership to the queue on push, SHALL be released once on Drop, and SHALL remain the runner's single pending output on Wait until it publishes. Queue draining, startup rollback, and Close SHALL release a queued or pending error batch to its originating pool. The consuming handler SHALL receive the one-element batch through its ordinary typed input and release it unless it moves it to a compatible ordinary output.

#### Drop an unconnected failure
- a handler returns a failed result and its error port has no error edge
- ordinary outputs and un-moved inputs are released, no error pool or queue is touched, and the failure value is not retained

#### Deliver one connected typed error
- a handler returns `Result[DomainError]{Value: err, Failed: true}` and its matching error edge accepts work
- the error consumer receives one `Batch[DomainError]` of length one containing `err`, while no ordinary output from that invocation is delivered

#### Retain a full Wait error edge
- a failed result's connected error queue is full under `queue.Wait`
- the runner retains exactly one owned `Batch[E]`, does not invoke its handler again until publication succeeds, and allows other runnable workers or runners to progress


### Error routing remains bounded and policy-free beyond transport

Each node SHALL have at most one error-edge consumer, and each target input SHALL have at most one producer across ordinary and error edges. Error routing SHALL use only the configured `EdgeConfig[E]` capacity, batch capacity, Drop or Wait policy, and eligible queue factory. The framework SHALL add no error mapper, logger, event log, metric, retry, panic recovery, fan-out, distribution, multiple-consumer error port, ingress, auto grouping, replica, or sharding behavior.

#### Reject a second error consumer
- a caller connects one `ErrorPort[E]` successfully and attempts to connect it to another input
- the second call is rejected as unsupported fan-out and leaves the second input unoccupied


## typed-error-mapping

Specify typed conversion of node failures into fixed-size events suitable for bounded logging transport.


### Typed fixed-size mapping contract

ErrorMapper[E] SHALL convert concrete E to an ErrorEvent containing numeric code/severity and fixed numeric attributes without boxed E. Configuration SHALL remain immutable and Build SHALL not invoke the mapper. A standalone conversion helper SHALL preserve the configured numeric payload.
#### Convert a typed failure
- a configured mapper converts a concrete failure value
- the result carries the expected fixed numeric payload without a runtime type assertion


### Mapping is independent of error-edge delivery

Mapping hooks SHALL permit one event per failed return when an asynchronous destination is installed, regardless of error-edge pool exhaustion, Drop, or absence. With no logging destination installed, runtime mapping SHALL remain dormant. Callback allocation/blocking obligations SHALL be documented rather than guaranteed by the framework.
#### Keep mapping dormant
- a definition with mapper configuration builds and starts without a logger destination
- construction and processing do not call the dormant mapper merely to inspect its type or payload


## async-error-logging

Define bounded asynchronous logging outside pinned pipeline workers and its lifecycle loss accounting.


### Nonblocking bounded event delivery

Configured logging SHALL map one event for each failed result and publish once to a bounded worker-owned ring. Full rings SHALL drop the event and increment a separate loss counter without blocking data workers. Logger callbacks SHALL execute outside the data-plane workers. Disabled logging SHALL create no logger worker or invoke mapper callbacks.
#### Saturate logging transport
- a returning but slow logger leaves its ring full
- subsequent failures increment logging-drop counters while ordinary processing and typed error routing continue


### Join logger lifecycle cooperatively

Drain SHALL finish accepted logger events within the shared drain budget; cancellation SHALL discard unconsumed events. Stop SHALL join the logger after any in-flight callback returns, preserving exactly-once accepted-event delivery or discard. Nonreturning callbacks SHALL be documented as preventing completed shutdown.
#### Cancel logger flushing
- the drain deadline expires with queued events and returning callbacks
- queued leftovers are discarded, counters retain the loss, and the logger goroutine exits before stop completes


## runtime-metrics

Define inspectable numeric runtime outcomes and queue-pressure observations for typed fixed-group pipeline execution without changing ownership or scheduling semantics.


### Independent ordered executor snapshots

The graph package SHALL expose `func (e *Executor) Metrics() MetricsSnapshot` and numeric snapshot structs `MetricsSnapshot`, `GroupMetrics`, `NodeMetrics`, and `EdgeMetrics`. `MetricsSnapshot` SHALL contain `Groups []GroupMetrics`, `Nodes []NodeMetrics`, and `Edges []EdgeMetrics`. Group rows SHALL contain GroupID, node rows SHALL contain NodeID and GroupID, and edge rows SHALL contain EdgeID, SourceNodeID, TargetNodeID, SourceGroupID, TargetGroupID, ErrorEdge bool, and Remote bool. All identity fields SHALL use the existing graph ID types. Rows SHALL include every planned group, registered node, and connected ordinary or error edge, including rows with zero counts, ordered by ascending corresponding ID. Each snapshot SHALL own independent slice storage. Nil and zero-value executors SHALL return a zero snapshot with nil slices. Metrics SHALL be always enabled without new Start configuration.

#### Inspect graph identities
- a graph with sparse groups and ordinary and error edges is running and Metrics is called
- every entity appears once in numeric order with immutable placement and edge classification metadata

#### Protect returned storage
- a caller mutates rows or slices in a returned snapshot
- subsequent snapshots and execution are unaffected

#### Inspect an invalid executor
- Metrics is called on a nil or zero-value executor
- it returns a zero snapshot without panic


### Node processing and failure counts

NodeMetrics SHALL expose uint64 HandlerCalls, InputBatches, InputElements, FailedResults, and UnconnectedErrors. HandlerCalls SHALL increment once for each normally returned source or input handler invocation, not selector calls, empty polls, or Wait retries. InputBatches and InputElements SHALL count the batch supplied to an invoked input handler and its length before user ownership changes, regardless of success; source calls SHALL contribute no synthetic input. FailedResults SHALL count failed handler returns once each. UnconnectedErrors SHALL count failed returns with no connected error edge, and SHALL NOT count connected pool exhaustion or queue Drop. Successful results SHALL not increment either failure count. Counters SHALL wrap modulo 2^64.

#### Process a moved input
- a handler receives a batch of length 3, moves it to an ordinary output, and returns successfully
- its node records one HandlerCalls, one InputBatches, and three InputElements without counting output publication as another input

#### Discard an unconnected failure
- a source returns a failed result with no error edge
- HandlerCalls, FailedResults, and UnconnectedErrors each increase by one and input counts remain unchanged


### Edge transport and loss counts

EdgeMetrics SHALL expose all existing accepted/dequeued, policy-drop, failed-output, forced-shutdown, pool-exhaustion, and Wait retry counts and SHALL additionally expose CapacityRejectedBatches and CapacityRejectedElements for explicit broadcast branches that reject an oversized moved batch before copying. Capacity rejection SHALL be distinct from queue Drop and pool exhaustion. Existing semantics SHALL remain unchanged: accepted counts require successful queue transfer, dequeue counts require normal handler delivery, pool exhaustion is an attempt count without fabricated batch loss, and WaitRetries counts subsequent attempts but not the initial push. Batch lengths SHALL be captured before ownership clearing and counters SHALL wrap modulo 2^64.

#### Retry a retained output
- an initial Wait push fails, two subsequent retries fail, and a third retry succeeds for a batch of length four
- WaitRetries increases by three, accepted counts by one/four, and queue-drop/capacity-rejection counts remain unchanged

#### Distinguish connected failure losses
- a connected error publication exhausts its pool and another acquired error batch encounters a full Drop queue
- PoolExhaustions records one acquisition attempt and QueueDropped counts only the dropped committed batch

#### Close pending and queued work
- Close discards one queued batch and one Wait-pending batch
- shutdown discard counts each committed batch once and normal dequeue counts do not increase

#### Distinguish sharding pool loss from queue Drop
- a sharding branch cannot acquire a staging slot for one element and a different branch successfully accepts its elements
- the exhausted branch increments `PoolExhaustions` and `PoolDroppedElements` for the element, while the other branch's metrics remain unaffected

#### Account for unassigned elements at forced shutdown
- Close discards a pending sharding operation before its cursor reaches the source batch end
- the first branch's `ShardShutdownElements` increases by the remaining unassigned source-element count


### Scheduler activity and sampled queue pressure

GroupMetrics SHALL expose uint64 Passes and IdlePasses. Each completed scheduler pass SHALL count once; a pass reporting no runner progress SHALL additionally count as idle. EdgeMetrics SHALL expose uint32 QueueCapacity, QueueDepthSample, and QueueHighWaterSample. Queue samples SHALL remain bounded by capacity, SHALL be updated from queue-owner observations after transport operations, and SHALL never call a custom queue callback from a snapshot reader. High-water values SHALL represent the maximum observed depth sample, not guarantee detection of every transient occupancy peak. Running snapshots SHALL not promise instantaneous queue depth or cross-field consistency. After completed Drain or Close, depth samples SHALL be zero and high-water samples SHALL be retained.

#### Inspect custom queue pressure
- Metrics reads an executor using a custom local queue
- it reads published numeric samples without invoking TryPush, TryPop, or Len on the reader goroutine

#### Observe stopped queue pressure
- the executor finishes Drain or Close
- its final snapshots report zero sampled depth and retain the previously observed high-water values


### Race-safe runtime-local lifecycle

Metrics SHALL be safe concurrently with worker processing, other snapshot calls, and a single supported Drain or Close operation without pausing workers or changing source activation. Running snapshots SHALL use individually race-safe values but SHALL not promise a globally atomic snapshot. Completed stop SHALL retain exact cumulative counts and immutable metadata for future snapshots, including after repeated lifecycle calls. Each sequential Start SHALL create fresh counters; Definition and Build SHALL retain no mutable metric resources or execute callbacks to collect metrics. Existing unsupported concurrent lifecycle calls and concurrent Starts SHALL remain unsupported.

#### Observe during processing and shutdown
- snapshot readers overlap local and remote processing followed by one Close operation under the race detector
- reads complete without data races, queue callback ownership violations, or changes to processing and cleanup semantics

#### Restart with fresh metrics
- an executor stops and its definition is started again
- the new executor has no counts inherited from the prior execution and the prior executor retains its final snapshot values


### Bounded typed instrumentation

Metric updates SHALL preserve zero framework heap allocations on supported built-in steady-state data paths. Instrumentation SHALL not box batch or error values, introduce reflection or runtime data type assertions, block processing on logging or exporting, or change pool, queue, failure, affinity, and ownership policies. Snapshot allocation SHALL be permitted in the control plane. Timing, timestamp callbacks, custom user metrics, exporters, logging, and pool-usage gauges SHALL remain deferred.

#### Exercise instrumented built-in paths
- local or remote ordinary and error delivery, Drop, Wait retry, or pool exhaustion executes after setup
- numeric outcomes update without framework heap allocation or altered ownership transfer


## extended-runtime-metrics

Extend numeric runtime snapshots with bounded pool utilization and fixed application counters.


### Pool utilization observations

Snapshots SHALL expose pool identity, capacity, sampled in-use count, and sampled high-water utilization bounded by capacity. Updates SHALL allocate no framework memory after setup and remote release SHALL remain race-safe. Gauges SHALL not be presented as a globally transactional sample.
#### Observe exhaustion and recovery
- a pool exhausts and owned slots are subsequently returned
- snapshots retain its observed high-water and show bounded current utilization without changing existing exhaustion counters


### Fixed registered numeric user counters

Applications SHALL register numeric counter slots before Build and update replica-local counters through bound runtime functions. Registration SHALL reject invalid or duplicate IDs atomically. Snapshot reads SHALL return independent ordered numeric rows; updates SHALL not create labels or grow storage.
#### Increment registered counters
- a handler updates its registered counter after setup
- the appropriate replica snapshot reflects the increment with zero framework allocation


### Sampled worker polling durations

Optional worker timing SHALL record measured idle and progressing scheduler-pass nanoseconds and measured pass counts using the established monotonic timing infrastructure. Collection SHALL be disabled by default and unsampled passes SHALL not read the clock. These measurements SHALL be documented as polling wall durations, not OS CPU usage, and SHALL not extrapolate unsampled time.
#### Measure an idle polling pass
- a sampled scheduler pass reports no progress
- its measured elapsed duration contributes only to idle polling time and its measured pass count increases once


## processing-timing

Define opt-in full and deterministic sampled handler duration measurements for runtime inspection.


### Explicit timing modes

Timing SHALL support disabled, full, and every-N-call modes configured before Build. Sampled N=0 SHALL be rejected. Sampling SHALL begin at call one independently per physical node and include successful and failed returned handlers. Disabled and unsampled calls SHALL perform no timestamp reads.
#### Sample every third call
- one node returns seven handlers with sampling interval three
- calls one, four, and seven are measured and all other calls avoid timestamp reads


### Report measured handler duration only

Snapshots SHALL report measured call count, total measured nanoseconds, and maximum measured nanoseconds, excluding selector, output publication, Wait retry, and logger work. Unsampled work SHALL not be extrapolated. Framework timing updates SHALL allocate no steady-state memory.
#### Retry an output
- a completed handler's output retries a full Wait queue
- retries do not increase measured handler count or handler duration totals


## custom-timestamp-source

Specify immutable user timestamp callbacks for deterministic and specialized processing timing.


### Injectable nanosecond timestamp function

Timing configuration SHALL accept an optional func() uint64 source; nil SHALL preserve the default monotonic clock. Build and snapshot reads SHALL not invoke it. Every sampled returned handler SHALL invoke it once at entry and once at return; disabled/unsampled calls SHALL not invoke it.
#### Use a deterministic clock
- a custom clock returns 100 and 140 around a measured handler
- its measured duration increases by forty nanoseconds and exactly two callback calls occur


### Explicit custom clock obligations

Documentation SHALL require concurrent safety when shared across workers, nonblocking/nonallocating execution, nanosecond units, and modular monotonic intervals below 2^63. Framework SHALL preserve concrete callback dispatch without reflection or boxed timestamps.
#### Leave timing disabled
- an executor has a configured custom clock but timing is disabled
- processing and snapshots make no custom-clock calls

## Clock-source decision and measured evidence

The default timing source SHALL remain Go's monotonic clock. An alternative
clock is an opt-in implementation detail only; it MUST satisfy the custom
timestamp contract (monotonic modular nanoseconds, nonblocking, allocation-free,
and safe for the way workers share it). Do not replace the `time` package or
change `time.Duration` APIs to optimize handler timing.

Local measurements on Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U found:

| Source | Isolated read | Allocation | Interpretation |
|---|---:|---:|---|
| `time.Since(epoch).Nanoseconds()` | 13.08 ns/op | 0 | Standard monotonic source; default. |
| `hrtime.Since(epoch).Nanoseconds()` | 23.76 ns/op | 0 | Slower on this host and loses monotonic semantics on non-Windows. |
| calibrated TSC approximate duration | 6.478 ns/op | 0 | Faster isolated read, but approximate and hardware-specific. |

The graph timing benchmark measured about 46.0 ns/op for full timing with the
standard source and 32.8 ns/op with TSC on this machine; sampled-64 was about
14.5 versus 14.3 ns/op. These are local microbenchmark results, not evidence of
application-level improvement or portable suitability. TSC adoption therefore
requires platform support checks, monotonicity/cross-core validation, and
representative workload evidence; sampled timing already amortizes much of the
clock cost. The original clock-source research and primary references are
consolidated here; no third-party clock dependency is implied.
