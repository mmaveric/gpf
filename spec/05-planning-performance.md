# Planning and performance

Manual fixed execution is the default. Automatic planning is an explicit,
side-effect-free preview followed by plan-bound CPU assignment and finalization.
NUMA remains preparation-only metadata; performance claims require measured
evidence rather than inference from allocation counts or affinity outcomes.

## Constraints and automatic preview

`EnableAutomaticPlanning(*Builder) error` opts unplaced nodes into free preview
constraints; it does **not** apply merges to ordinary Build/Start. Definition
`Constraints()` returns an independent numeric `PlanningConstraints` with physical
node and edge rows after [parallel expansion](03-topologies-transport.md#physical-parallelism):

| Row | Fields / interpretation |
| --- | --- |
| `NodeConstraint` | NodeID, LogicalNodeID, Replica, PhysicalGroup, ExplicitGroup, FixedGroup. ExplicitGroup zero means no Place call; an effective manual default is not a hard explicit label. |
| `EdgeConstraint` | EdgeID, LogicalEdgeID, Replica, SourceNodeID, TargetNodeID, MustLink, MustSeparate. Preserve explicit placement labels, paired replica membership, node-worker boundaries, and ingress cuts. |

Different explicit labels must not merge. Conflicting must-link/manual-label or
must-link/must-separate constraints must reject with ErrPlanningConstraintConflict
without topology mutation or callbacks. Free automatic-preview nodes have no hard
group, while ordinary manual Build still resolves them to DefaultGroupID. Returned
constraints must not alias builder/definition slices or expose live resources.

`(*Builder).PlanGroups() (AutomaticGroupPreview, error)` requires opt-in
(`ErrAutomaticPlanningDisabled` otherwise). It validates topology and constraints
but accepts missing group CPUs as an incomplete preview, not an executable plan.
It must report deterministic physical nodes/groups, logical/replica identities,
edge locality and selected queue models, required CPU slots, CPUComplete, and
per-group CPUBound. Existing hard group IDs are preserved; generated IDs and rows
must be stable for unchanged canonical inputs.

Preview must not seal, invoke factories/callbacks, create live pools/queues, or
start workers. It owns copied rows with private builder ownership/revision tokens.
`IsCurrent(*Builder) bool` and `ValidateCurrent(*Builder) error` check freshness;
foreign/stale plans use ErrStaleAutomaticPlan. Registration, connections, placement,
and other recorded configuration changes invalidate previous previews. Mutating
returned rows must not change builder metadata. A preview remains non-executable
until explicit finalization, even if CPUComplete is true.

## Constraint-preserving coalescing

`MinimizeRemoteEdges(PlanningConstraints) (GroupingResult, error)` must perform
deterministic component coalescing without Build, factories, queues, affinity, or
runtime work. It must preserve hard labels/mandatory cuts and never increase
remote-edge count. Every accepted adjacent improving merge must preserve the
**acyclic group quotient**, not just the original node DAG. For example, contracting
a shortcut's endpoints is invalid if a path through an intermediate component
would become a quotient cycle.

The result must terminate with no eligible adjacent improving merge and include
copied assignments, RemoteEdgesBefore/After, and deterministic rejection reasons.
It is a constrained **local minimum**, not globally optimal partitioning. A
connected unconstrained DAG can coalesce to one group with zero remote edges;
disconnected components need not become one group. Malformed/duplicate identities
or unknown endpoints use ErrInvalidConstraintPlan; an initially cyclic quotient
uses ErrCyclicConstraintPlan; contradictory constraints use
ErrPlanningConstraintConflict. Repeated equivalent canonical inputs must produce
equivalent plans.

## CPU assignment and finalization

```go
func BindAutomaticPlanCPU(*Builder, *AutomaticGroupPreview, GroupID, int) error
func FinalizeAutomaticPlan(*Builder, AutomaticGroupPreview) (*Definition, error)
```

Every proposed physical group must receive exactly one explicit unique
nonnegative CPU assignment, including already hard-bound rows whose CPU must be
preserved. BindAutomaticPlanCPU must validate builder state, current owner/revision,
group identity, negative CPU, duplicate group/CPU assignment, and hard-bound
compatibility without modifying the manual plan. Nil/malformed/incomplete
assignment sets use ErrInvalidAutomaticCPUPlan; negative CPU uses ErrInvalidCPU;
duplicate CPU uses ErrCPUAlreadyBound; foreign/stale previews use
ErrStaleAutomaticPlan. Configuration errors reject before factories or sealing.

Successful finalization must freeze physical nodes/groups/edges, queue models,
replica/worker identities, explicit CPU bindings, constraints, and dormant
configuration in an immutable Definition, cache it, and seal the builder.
Subsequent Build returns that finalized definition. Ordinary Build without this
step remains the manual fixed topology even after EnableAutomaticPlanning or
PlanGroups. Finalization must not invoke any state/closure/queue factory, create
error resources, or start workers. Topology mutation requires a fresh preview and
fresh CPU assignment.

Each sequential Start of a finalized definition must create fresh resources and
one locked worker per planned physical group, attempt its CPU affinity, and
preserve the [portable fallback/lifecycle contract](02-execution-lifecycle.md).
Start must not accept plan/CPU overrides, discover CPUs, or apply dynamic merging.
Definition inspection must remain immutable before, during, and after execution.

## NUMA metadata and inactive allocators

`numa.NodeID` is `uint32`. `numa.Placement{Node NodeID, Present bool}` distinguishes
an unspecified zero value from explicit node **zero** (`numa.On(0)`). An absent
placement must have Node zero; an explicit placement accepts any NodeID without
checking host existence. Invalid representation uses numa.ErrInvalidPlacement.

Pool, group, edge/queue, and ingress-pool configuration may retain validated
optional placement. `SetGroupNUMA(*Builder, GroupID, numa.Placement) error` records
intent once for an effective open-builder group; duplicate configuration uses
ErrGroupNUMAConfigured. Definitions copy annotations into GroupNUMA/EdgeQueuePlans
and pool metric metadata; caller changes must not alter those copies. GroupNUMA
is ordered by physical group identity and retains replica and presence data.

Typed descriptors are `batch.Allocator[T] func(int) ([]T,error)`,
`queue.Allocator[T] func(uint32) ([]batch.Batch[T],error)`, graph edge
`AllocatorDescriptors[T]{Pool,Queue}`, and ingress `IngressAllocator[T]{Pool}`.
`batch.PoolOptions[T]{Config,Allocator}` / `NewPoolWithOptions[T]` retain the pool
descriptor. Registration, Build, and **every Start** must leave these hooks
uninvoked; no boxed storage, NUMA discovery, memory binding, CPU-to-node inference,
NUMA-aware dispatch, or change to portable Go allocation is enabled. Requested
locality is not evidence of placement, and CPU-affinity success is not memory
locality. Activation requires a separate approved behavioral design.

### Preparation research and activation gates

Research basis reviewed **2026-10-08**: Linux [numa(7)](https://man7.org/linux/man-pages/man7/numa.7.html),
[mbind(2)](https://man7.org/linux/man-pages/man2/mbind.2.html),
[set_mempolicy(2)](https://man7.org/linux/man-pages/man2/set_mempolicy.2.html),
[sched_setaffinity(2)](https://man7.org/linux/man-pages/man2/sched_setaffinity.2.html),
[Go runtime allocator](https://go.dev/src/runtime/malloc.go), and Microsoft
[NUMA support](https://learn.microsoft.com/en-us/windows/win32/procthread/numa-support),
[VirtualAllocExNuma](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualallocexnuma),
and [GetNumaHighestNodeNumber](https://learn.microsoft.com/en-us/windows/win32/api/systemtopologyapi/nf-systemtopologyapi-getnumahighestnodenumber).

Linux memory policy differs from per-thread affinity and page migration; Windows
node IDs can be sparse and preferred-node allocation can fall back. Go exposes no
stable public NUMA placement API; ordinary Go heap allocation cannot be assumed
to follow an OS placement request. Future activation must settle and test:

| Design concern | Required evidence before activation |
| --- | --- |
| Policy / topology | Strict versus preferred binding/fallback; sparse IDs, allowed cpusets, memory-bearing nodes, hotplug/container-policy staleness. |
| Storage ownership | Allocator leases/release, alignment, clearing, pointer-bearing T, origin-pool release after remote movement. |
| Worker lifecycle | Which locked worker allocates/first-touches; startup rollback, drain/Close, sequential restarts. |
| Unsupported/restricted hosts | Deterministic portable fallback, permissions, restricted topology, memory pressure. |
| Placement proof | OS evidence of actual memory location, not an inferred affinity result. |
| Performance | Repeated representative local/cross-node distributions for portable/preferred/strict/fallback paths; startup, throughput, tail latency, and footprint; reversible portable selection. One microbenchmark is insufficient. |

#### Post-release NUMA milestone

For `v0.1.0 experimental`, NUMA placement is inactive and annotations are
descriptive metadata only. The owner defers both (1) designing and activating
actual memory placement and (2) propagating configured annotations through every
pool metric/snapshot path, including ordinary-edge and broadcast/shard scratch
pools, until after the first release. This milestone does not promise a date or
commit to activating NUMA if portability, ownership, placement evidence, or
representative performance requirements cannot be met.

## Benchmark and allocation conformance

The Go benchmark matrix must provide named executable cases for the approved
data-plane paths, encoding batch size and relevant group/depth dimensions:

| Path | Measurement contract |
| --- | --- |
| `node-call/batch=64` | Concrete typed handler processes logical 64-element input. |
| `pool-acquire-release/batch=64` | Construct pool before measurement; repeatedly acquire, indexed fill, SetLen, release. Cover exhaustion and clear-enabled/disabled reuse separately. |
| `local-edge/batch=64/group=1` | Built-in typed local delivery of logical 64-element batches, not a skipped reservation. |
| `remote-queue/batch=64/groups=2` | Executable built-in SPSC transfer, not an unavailable placeholder. |
| `end-to-end-dag/batch=64/groups=2/depth=3` | Executable fixed two-group DAG. |
| Local/remote typed errors | Failed result to typed sink, error SPSC, Drop release, and Wait retry. A failed handler produces one typed error value per error event, so names use `error-event=1` and group dimensions rather than implying 64 error elements. |

Each benchmark must call `b.ReportAllocs`, exclude Build/Start/setup and teardown
from measured iterations, and keep snapshots outside the steady-state operation.
Unimplemented paths may only be marked explicitly unavailable, not given a
misleading measurement; the implemented matrix entries above must execute.

The remote queue and end-to-end DAG benchmark operations must each deliver a
logical 64-element batch, not merely reserve capacity for 64. Their configured
capacity and actual logical length should be checked independently. Typed-error
benchmarks measure one error event per failed handler result and must not claim a
64-element error batch.

`testing.AllocsPerRun` gates must assert **zero framework allocations** after setup
for pool lifecycle/exhaustion, node calls, one-/multi-input dispatch, local/remote
ordinary and typed-error delivery, compatible forwarding/release, Drop, Wait retry,
and source-pool exhaustion. Always-enabled metric updates must stay inside those
gates; snapshot construction stays outside. Exclusions are startup/shutdown,
factories, user handlers/selectors/callbacks, and custom queues. Built-in bounded
concurrent queue and ingress steady-state operations must also remain framework-
allocation-free under their respective contracts.

Allocation assertions must run serially with **no active parallel tests in their
test package**; preserve warm-up, run count, measured operation, and zero threshold
across supported Go versions (including Go 1.26's parallel-activity checks).
Instrumented-path comparisons must record matching pre/post throughput and
allocations with the same batch/group setup, not introduce a throughput pass/fail
threshold. Handoff/lifecycle semantics must also have race and leak coverage;
zero allocation alone does not establish correctness or performance.

Throughput and latency results are non-blocking manual comparison signals. A
passing allocation gate does not protect instruction count, cache misses,
throughput, or tail latency. Keep the
[dated clock measurements and limitations](04-errors-observability.md#clock-source-decision-and-measured-evidence)
distinct from new benchmark claims.

## Optional fast-path policy

An optimized variant requires recorded same-toolchain/hardware benchmark evidence
for the affected path, with repeated variability and allocation results. No
reproducible gain is a valid **no-go** outcome: retain the portable path and do not
claim unsupported improvement. Platform/build-tag isolation must preserve typed
ownership, capacity, and zero-allocation semantics. Unsupported targets, explicit
portable selection, and no-cgo builds must retain a compiling working pure-Go
fallback with applicable semantic conformance tests. This policy does not itself
enable an optimization or activate NUMA allocation.
