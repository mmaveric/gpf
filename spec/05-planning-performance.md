# Planning and performance

This document consolidates the current normative requirements. Scenarios are retained where they add behavioral detail; shared concepts should be read together across sections.


## auto-grouping-constraints

Define immutable placement and physical boundary constraints consumed by opt-in automatic grouping.


### Honor explicit placement as a hard constraint

Automatic constraints SHALL preserve explicit manual Place labels and mandatory physical worker boundaries. Effective default placement SHALL not falsely become an explicit hard label. Conflicting must-link/manual-label or must-link/must-separate requirements SHALL reject without topology mutation or callback invocation.
#### Preserve distinct manual groups
- adjacent nodes have different explicit manual placements
- constraint normalization prohibits merging them into one physical worker


### Copy dormant planning constraints

Constraint previews SHALL be independent immutable numeric copies formed after group-replica/node-pool physical expansion. `EnableAutomaticPlanning` SHALL mark unplaced nodes as free in the preview without changing effective manual Build/Start placement. Manual grouping SHALL remain the default and its existing placements SHALL be unchanged. Preview SHALL not merge nodes, construct queues, invoke factories, or enable automatic runtime scheduling.
#### Inspect constraints
- a caller requests and mutates a returned constraint preview
- the builder's topology and later previews remain unchanged and no factory was invoked

#### Leave unplaced nodes free in automatic preview
- automatic planning preview is enabled but several nodes have no explicit Place labels
- their preview constraint rows use an invalid/no-hard-label group while normal Build/Start keeps the manual default group


## automatic-group-planning

Specify independent deterministic automatic-group previews formed before any runtime materialization.


### Deterministic copied plan preview

Opt-in automatic planning SHALL report physical node/group identities, edge locality, queue models, and required CPU slots after expansion and constrained coalescing. Explicit group IDs SHALL be preserved and generated IDs SHALL be stable for unchanged topology. Returned rows SHALL not alias mutable builder metadata.
#### Repeat an unchanged preview
- a builder is planned twice without mutation
- both previews have identical identities, groups, and queue choices


### Preview without runtime side effects

Preview SHALL not seal the builder, invoke factories, allocate live pools/queues, or start workers. The copied AutomaticGroupPreview SHALL report CPUComplete and per-group CPUBound status; it SHALL not be executable or accepted as a Definition until a later CPU-binding finalization capability. A topology revision and builder ownership token SHALL identify freshness; changes to registration, connections, or placement SHALL invalidate previous previews.
#### Mutate after preview
- a caller changes topology after obtaining a plan
- the prior plan is stale and cannot be used as an executable finalization input

#### Preview an automatic plan without CPU bindings
- an opt-in builder has valid graph topology and constraints but no CPU assigned to its proposed free group
- PlanGroups returns a deterministic CPU-incomplete preview while factories remain dormant and ordinary Build/Start remains manual


## automatic-group-cpu-bindings

Define explicit CPU assignment and immutable executable finalization for automatic physical group plans.


### Validate plan-bound CPU assignments

Automatic finalization SHALL require one unique nonnegative explicit CPU for every physical group. Foreign/stale plan revisions and missing/duplicate/negative bindings SHALL reject before sealing or factory invocation. Topology mutation SHALL require a fresh preview and CPU assignment.
#### Reject a stale CPU plan
- topology changes after CPUs were associated with an earlier preview
- executable finalization rejects the stale assignments without starting workers


### Execute immutable automatic plans

A successfully finalized automatic Definition SHALL retain immutable physical groups and CPU assignments and create fresh resources per sequential Start. Every physical worker SHALL lock its thread and attempt affinity with the existing portable fallback outcome contract.
#### Start a finalized plan
- a complete automatic definition starts on a platform rejecting affinity
- every planned worker remains thread-locked and reports its assigned CPU and fallback outcome


## numa-allocation-metadata

Specify dormant NUMA annotations and typed allocation descriptors without activating locality behavior.


### Immutable optional NUMA metadata

Pool, physical group, edge, and ingress-pool configuration SHALL accept validated optional NUMA annotations with an unspecified zero default. Definitions and copied plan introspection SHALL preserve annotations without changing CPU assignment, queue selection, or existing portable allocation behavior.
#### Configure locality intent
- a caller builds and starts a valid graph carrying NUMA annotations
- copied metadata retains the intent while processing and allocation follow the existing portable behavior


### Dormant typed allocator descriptors

Typed batch-pool, graph-edge pool/queue, and ingress-pool allocator hook descriptors SHALL remain uninvoked during registration, Build, and Start in this capability. They SHALL not box batch storage or imply active NUMA allocation. Activation SHALL require a separately approved behavioral change.
#### Keep a configured hook inactive
- a definition with an observable allocator descriptor starts sequentially twice
- the descriptor's call count remains zero and fresh ordinary pools are created each time


## pipeline-performance-harness

Define the benchmark matrix and allocation-measurement requirements for pipeline data-plane paths.


### Stable data-plane benchmark matrix

The repository SHALL provide a Go benchmark harness with named cases for node invocation, pool acquire/release, local-edge transfer, remote-queue transfer, and end-to-end DAG processing. The harness SHALL encode batch size and applicable topology dimensions in benchmark case names. The `pool-acquire-release` case SHALL execute acquire, fixed-capacity fill, logical-length update, and release against a constructed pool; cases without an executable owner SHALL remain explicitly unavailable.

#### Discover benchmark coverage
- a developer lists the framework benchmarks
- they can identify a benchmark entry point for each planned data-plane path and its batch or topology configuration, including an executable pool acquire/release case


### Allocation measurement contract

The harness SHALL report allocations and SHALL expose `testing.AllocsPerRun` assertions for approved steady-state paths. An implementation stage SHALL enable a zero-allocation assertion only after the corresponding path is executable and has completed setup before measurement begins. The batch subsystem SHALL assert zero allocations for repeated pool acquire/fill/release work and for the pool-exhaustion path.

Every `testing.AllocsPerRun` assertion SHALL execute while its test package has no active parallel tests. A test that invokes the allocation harness SHALL remain serial and SHALL preserve the existing warm-up, run count, measured operation, and zero-allocation threshold across supported Go versions.

#### Measure a steady-state path
- an implemented data-plane benchmark runs after its setup phase
- its allocation assertion measures only repeated steady-state work and fails if allocations exceed zero

#### Measure pool lifecycle allocations
- the pool benchmark and allocation tests run against a constructed typed pool
- acquire, fixed-capacity fill, logical-length update, release, and exhaustion reporting each perform zero allocations per operation

#### Run allocation assertions on Go 1.26
- the repository test suite executes with a Go version that rejects `testing.AllocsPerRun` during parallel-test activity
- every allocation assertion runs serially, completes without a parallel-test panic, and retains its zero-allocation gate


### Measure built-in remote SPSC execution

After the manual-multigroup-runtime capability is implemented, the benchmark harness SHALL execute rather than mark unavailable the `remote-queue` case using a built-in typed SPSC transfer and the `end-to-end-dag` case using a fixed two-group DAG. Those benchmark names SHALL retain batch size and group-count dimensions, call `b.ReportAllocs`, and exclude definition building and Start setup from measured iterations. The repository SHALL add `testing.AllocsPerRun` assertions for the same built-in remote transfer and end-to-end paths and SHALL require zero framework allocations per operation.

#### Run the remote benchmark matrix entries
- a developer runs the benchmark matrix after multi-group runtime implementation
- remote-queue and end-to-end-dag execute built-in fixed-group work after setup, report allocations, and retain their batch and group dimensions

#### Detect a remote allocation regression
- the built-in remote SPSC transfer or two-group end-to-end path allocates during a measured operation
- its allocation assertion fails while handler, selector, factory, startup, shutdown, and custom queue allocation remain outside that assertion


### Non-blocking performance reporting

The benchmark harness SHALL allow throughput and latency measurements to be recorded for manual comparison without making them merge-blocking thresholds. The project SHALL document that a zero-allocation gate alone does not protect against instruction-count, cache-miss, throughput, or tail-latency regressions.

#### Interpret a passing allocation gate
- a data-plane path passes its zero-allocation assertion
- the benchmark documentation states that further throughput and latency review can still be required


### Measure typed error-routing data paths

After typed-error-routing is implemented, the benchmark harness SHALL execute built-in local and remote typed error-routing cases rather than mark them unavailable. Case names SHALL encode the batch size and applicable group count, call `b.ReportAllocs`, and exclude definition building and Start setup from measured iterations. The repository SHALL add `testing.AllocsPerRun` assertions for local failed-result-to-error-sink delivery, remote error SPSC transfer, error Drop, and error Wait retry and SHALL require zero framework allocations per operation.

#### Run the error-routing benchmark matrix entries
- a developer runs the benchmark matrix after this capability is implemented
- local and remote typed error-routing cases execute after setup, report allocations, and retain batch and group dimensions in their names

#### Detect an error-routing allocation regression
- a built-in failed-result publication, error queue transfer, Drop release, or Wait retry allocates during a measured operation
- its allocation assertion fails while handlers, selectors, factories, startup, shutdown, and custom queue allocation remain outside that assertion


### Measure instrumented runtime paths

Allocation gates SHALL run with runtime metrics enabled on built-in local and remote ordinary and error delivery, queue Drop, Wait retry, and source-pool exhaustion after setup, and SHALL require zero framework allocations per operation. Snapshot construction SHALL remain outside steady-state allocation measurement. The benchmark harness SHALL retain ReportAllocs and batch/group dimensions and SHALL record comparable before-and-after throughput and allocation results for the instrumentation change without a throughput pass/fail threshold. Allocation assertions SHALL remain serial under the existing allocation measurement contract.

#### Detect instrumentation allocations
- a metric update allocates on an instrumented built-in measured operation
- its zero-allocation assertion fails even if the transport itself is allocation-free

#### Compare instrumentation overhead
- benchmark results for matching pre-change and instrumented cases are compared
- batch and group configuration, allocations, and throughput are reported with setup and snapshots excluded from timed iterations


### Execute node-call and local-edge matrix entries

The reserved node-call/batch=64 and local-edge/batch=64/group=1 entries SHALL execute concrete typed handlers and built-in local transport instead of skipping. Each measured operation SHALL process a logical 64-element batch, exclude setup and teardown, retain ReportAllocs, and expose a serial zero-framework-allocation assertion.
#### Run the completed matrix
- the benchmark matrix runs after setup
- both reserved cases execute, report allocations, and verify the advertised batch size without timing snapshots or lifecycle work
#### Detect a local allocation regression
- an executable node-call or local-edge fixture allocates during its measured framework operation
- its serial allocation assertion fails against a zero threshold


## platform-fast-path-policy

Define evidence, compatibility, and portable fallback conditions for optional platform-specific optimization.


### Evidence gates optional optimized paths

An optimized variant SHALL be introduced only with recorded same-toolchain/hardware benchmark evidence for its affected path. A measured no-go result SHALL be an acceptable outcome without optimized code. Reports SHALL include variability and allocation results and SHALL not claim a gain from unsupported evidence.
#### Reject an unproven variant
- measured samples do not support a reproducible improvement
- the report records no-go and the portable implementation remains the only enabled path


### Preserve pure-Go fallback conformance

Any optimized variant SHALL be isolated by platform/build tags and preserve the same ownership, capacity, and zero-allocation contracts as pure Go. Unsupported platforms, explicit portable builds, and no-cgo builds SHALL retain a working fallback.
#### Build without optimized support
- the target lacks the optimized implementation or cgo is disabled
- the package compiles using the portable path and passes applicable semantic conformance checks

## NUMA placement: preparation only

No NUMA discovery, memory binding, CPU-to-node inference, allocator activation,
or NUMA-aware dispatch is part of the current contract. NUMA annotations and
typed allocator descriptors are behavior-neutral metadata; ordinary Build and
Start keep using portable Go allocation and explicit CPU-affinity/fallback
behavior. A requested node is not proof that the host has that node or that
memory was placed there. CPU-affinity success does not prove memory locality.

Any future activation is a separate design change and must first settle:

- strict binding versus preferred placement and fallback semantics;
- topology source, sparse node IDs, allowed cpusets, memory-bearing nodes, and
  staleness after hotplug/container policy changes;
- allocator lease ownership/release, alignment, clearing, pointer-containing
  values, and origin-pool release after remote movement;
- which locked worker allocates and first-touches storage, including rollback
  and sequential restarts;
- deterministic behavior on unsupported, restricted, or memory-starved hosts.

Platform APIs do not make the contract portable by themselves: Linux memory
policy is separate from per-thread CPU affinity and page migration; Windows
node IDs may be sparse and allocation to a preferred node may fall back. The
Go public runtime exposes no stable NUMA placement API, and ordinary Go heap
allocations cannot be assumed to follow an OS placement request.

An activation proposal must test unique ownership, origin-pool release, rollback,
drain/close cleanup, sparse/restricted topology, and unsupported-platform
fallback. Placement must be verified with OS evidence, not inferred from an
affinity result. Benchmarks must compare portable, preferred, strict, and
fallback paths on representative local/cross-node workloads, report repeated
distributions and startup/throughput/tail-latency/footprint costs, and keep a
reversible portable selection. A single microbenchmark is insufficient. The
original NUMA preparation research and platform references are consolidated
here.

Research basis (reviewed 2026-10-08): Linux `numa(7)`, `mbind(2)`,
`set_mempolicy(2)`, and `sched_setaffinity(2)`; Go runtime allocator source;
Microsoft NUMA support, `VirtualAllocExNuma`, and `GetNumaHighestNodeNumber`.
