## MODIFIED Requirements

### Requirement: Build validates and compiles the fixed group plan
`Validate` and `Build` SHALL validate stored manual placement and CPU binding without invoking node, queue, lifecycle, or affinity functions. `ValidationCodeInvalidGroupPlacement` SHALL identify malformed stored group placement with its affected NodeID. `ValidationCodeRemoteQueueFactory` SHALL identify an ordinary or error cross-group edge with a non-nil custom queue factory with its EdgeID. CPU-binding diagnostics SHALL identify missing, invalid, duplicate, or ineffective group bindings with the affected GroupID and CPU as defined by cpu-affinity-runtime. Any such issue SHALL produce the existing sorted `ValidationReport` and `*ValidationError`; Build SHALL leave the builder open and publish no definition.

For an issue-free graph, Build SHALL resolve every omitted placement to DefaultGroupID; enumerate distinct effective groups in ascending GroupID order; retain each group’s nodes in canonical topological order; assign every group its validated explicit CPU ID; and classify every ordinary and error edge local when its source and target effective groups match or remote when they differ. The resulting private group plan, CPU bindings, and edge classifications SHALL be copied into the immutable definition and SHALL contain no live queue, pool, error batch, worker, thread, adapter, factory result, executor, affinity result, or lifecycle state. Build SHALL not renumber or merge groups, create a group for an unused ID, infer placement or CPU binding, validate a platform CPU upper bound, or choose a queue concurrency model.

#### Scenario: Compile a deterministic mixed-group CPU plan
- **WHEN** a valid graph uses default group 1 and explicitly places nodes in sparse groups 3 and 9, with a distinct nonnegative CPU bound to each effective group
- **THEN** Build records groups in order 1, 3, 9, pairs each with its supplied CPU, retains each group's canonical node order, and classifies every ordinary and error edge from endpoint placement

#### Scenario: Reject a missing CPU before Start
- **WHEN** a valid manually placed graph has an effective group with no CPU binding
- **THEN** Build returns a ValidationError without invoking a factory or affinity adapter and the builder remains correctable

#### Scenario: Reject a remote custom queue before Start
- **WHEN** a valid ordinary edge has a non-nil queue factory and its endpoint nodes have different effective groups
- **THEN** Validate reports ValidationCodeRemoteQueueFactory for that EdgeID, Build returns a ValidationError without invoking the factory, and the builder remains correctable

#### Scenario: Reject a remote error queue factory before Start
- **WHEN** a valid typed error edge has a non-nil queue factory and its endpoint nodes have different effective groups
- **THEN** Validate reports `ValidationCodeRemoteQueueFactory` for that EdgeID, Build returns a ValidationError without invoking the factory, and the builder remains correctable

#### Scenario: Keep same-group factory eligibility
- **WHEN** a valid ordinary or error edge with a non-nil queue factory has endpoints in the same effective group and every effective group has a valid CPU binding
- **THEN** Build succeeds without invoking the factory and preserves it for that local edge's future Start

### Requirement: One worker serves each planned group
`Definition.Start` SHALL retain signature `func (d *Definition) Start() (*Executor, error)` and SHALL create exactly one worker goroutine for each group in the immutable plan, in ascending GroupID startup order. Before any group Init callback or runner processing, each worker SHALL lock its own OS thread, attempt its immutable CPU binding through the internal affinity adapter, record an applied or locked-thread-fallback outcome, and report ready. Each worker SHALL run only that group’s canonical-order runners. Start SHALL create no worker replicas, node worker pool, transport, ingress endpoint, automatic group, CPU discovery, NUMA placement, or Start configuration.

Every ordinary or error edge with different endpoint groups SHALL use one framework-owned bounded typed SPSC ring. Its source group worker SHALL be its sole producer and its target group worker SHALL be its sole consumer. The ring SHALL use preallocated batch-handle storage and atomic cursor publication suitable for that handoff, and SHALL not use locks, channels, reflection, runtime type assertions, data-plane interfaces, or steady-state allocation. Same-group edges SHALL retain local queue behavior.

#### Scenario: Start two locked manually separated groups
- **WHEN** a valid definition contains nodes placed in two effective groups joined by one ordinary or error edge and both groups have CPU bindings
- **THEN** Start creates exactly two locked workers, records one affinity outcome for each, runs each node only on its assigned worker, and transfers the edge batch through the built-in SPSC ring

#### Scenario: Preserve the one-group execution shape
- **WHEN** a valid definition has only DefaultGroupID nodes and a binding for DefaultGroupID
- **THEN** Start creates exactly one locked worker, records its affinity outcome, and every ordinary or error edge uses the existing local queue path

#### Scenario: Transfer a typed error across groups
- **WHEN** a failed handler and its matching error consumer are placed in different valid CPU-bound groups
- **THEN** their workers transfer the one-element typed error batch through the built-in SPSC ring without a runtime data type assertion

### Requirement: Remote edges use the built-in SPSC queue policy
The existing `queue.Config` Capacity, BatchCapacity, and Policy SHALL apply to an ordinary or error remote SPSC edge. A successful remote TryPush SHALL transfer typed batch ownership to the ring. If a remote ring is full under `queue.Drop`, the producer SHALL release the output exactly once. If it is full under `queue.Wait`, the producer runner SHALL retain the one pending output, SHALL not accept new input until that output publishes, and SHALL retry in later scheduler passes without blocking inside a handler; the consumer group SHALL remain runnable independently.

A non-nil `queue.Factory` SHALL be valid only for a local edge. A remote edge factory SHALL be rejected by Build and SHALL never be invoked by Start. This stage SHALL not expose or select MPSC, SPMC, MPMC, remote custom queues, fan-out, sharding, or replica-aware queue behavior.

#### Scenario: Deliver remote SPSC work in FIFO order
- **WHEN** a producer group successfully publishes multiple ordinary or error batches to an available remote edge
- **THEN** the consumer group receives the typed batches once each in publication order with ownership transferred from the queue to its handler

#### Scenario: Retry a full remote Wait edge while its consumer progresses
- **WHEN** a remote ordinary SPSC edge configured with Wait is full and its producer has a pending output
- **THEN** the producer runner does not run another handler, the consumer worker can remove a batch, and a later producer pass publishes the retained output

#### Scenario: Retry a full remote error Wait edge
- **WHEN** a remote error SPSC edge configured with Wait is full and its producer has a pending failed result batch
- **THEN** the producer runner does not run another handler, the consumer worker can remove the first error batch, and a later producer pass publishes the retained batch

### Requirement: Remote execution is tested and measured rigorously
The repository SHALL test manual placement errors and defaulting, deterministic plan order and ordinary/error edge classification, remote-factory rejection without invocation, exact worker count, factory freshness and rollback order, SPSC capacity and FIFO ownership, remote Drop and Wait behavior, multi-group Drain and Close, and one-group equivalence. It SHALL run these concurrent handoff and lifecycle tests under `go test -race` without data races or goroutine leaks.

After Start, the built-in remote SPSC enqueue/dequeue and supported two-group source-to-error-sink path SHALL allocate zero framework heap objects per operation, excluding handlers, selectors, factories, local custom queues, startup, and shutdown. Benchmarks SHALL report allocations after setup for remote queue transfer and a multi-group error-routing DAG; throughput and latency remain non-blocking comparison signals.

#### Scenario: Measure remote steady state
- **WHEN** allocation tests and benchmarks run a started built-in two-group graph after setup
- **THEN** the covered remote SPSC path reports zero framework allocations per operation and benchmark names encode its batch and group dimensions

#### Scenario: Measure remote error steady state
- **WHEN** allocation tests and benchmarks run a started built-in two-group graph that routes failed results after setup
- **THEN** the covered remote error path reports zero framework allocations per operation and benchmark names encode batch and group dimensions
