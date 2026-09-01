## Purpose
Define explicit CPU bindings, affinity application with a locked-thread fallback, and immutable affinity outcomes for fixed-group executors.

## Requirements

### Requirement: Fixed groups have an explicit CPU binding API
The graph package SHALL expose exactly `func BindCPU(builder *Builder, group GroupID, cpu int) error` and SHALL export `ErrInvalidCPU`, `ErrUnknownGroup`, `ErrGroupCPUAssigned`, and `ErrCPUAlreadyBound` as stable sentinels. BindCPU SHALL accept only an initialized open builder, a nonzero currently effective group, and a nonnegative CPU ID. A group is effective when it has at least one registered node after applying its explicit manual placement or DefaultGroupID. BindCPU SHALL record exactly one CPU ID for that group without modifying placement, topology, identities, factories, callbacks, or runtime resources.

BindCPU SHALL choose errors in this order: invalid builder, sealed builder, invalid group ID, negative CPU, unknown effective group, group already assigned, then CPU already assigned to another effective group. It SHALL add no Start configuration, CPU discovery, CPU-set inspection, NUMA setting, replica setting, reassignment, or removal API.

#### Scenario: Bind sparse effective groups
- **WHEN** a caller places nodes in groups 3 and 9 and binds distinct nonnegative CPUs to groups 3 and 9 before Build
- **THEN** Build can compile those bindings without changing either group ID or placement

#### Scenario: Reject an invalid binding without mutation
- **WHEN** BindCPU receives an invalid builder, sealed builder, zero group, negative CPU, ineffective group, duplicate group, or duplicate CPU
- **THEN** it returns the first applicable documented sentinel and leaves the builder unchanged

### Requirement: Build validates and retains complete CPU bindings
Validate and Build SHALL validate CPU bindings without invoking node, queue, lifecycle, or affinity functions. For every effective group, there SHALL be exactly one binding; every binding CPU SHALL be nonnegative and unique; and no binding SHALL target an ineffective group. Validation SHALL report `ValidationCodeMissingGroupCPU`, `ValidationCodeInvalidGroupCPU`, `ValidationCodeDuplicateGroupCPU`, or `ValidationCodeIneffectiveGroupCPU` as applicable, identifying the group and CPU through the structured validation issue. Build SHALL return the existing sorted `*ValidationError`, leave the builder open, and publish no definition when any CPU-binding issue exists.

For an issue-free graph, Build SHALL copy one CPU binding with every private ascending effective group-plan entry. The copied plan SHALL be immutable and reusable by sequential Start calls; it SHALL contain no OS thread, adapter, worker, mutable status, or runtime affinity result. Build SHALL enforce no upper CPU bound or host CPU-set policy.

#### Scenario: Reject incomplete CPU coverage
- **WHEN** a valid graph has effective groups 1 and 7 but binds a CPU only for group 1
- **THEN** Validate and Build report `ValidationCodeMissingGroupCPU` for group 7, Build publishes no definition, and the caller can bind group 7 and retry

#### Scenario: Reject corrupted duplicate or invalid binding data
- **WHEN** same-package builder metadata contains a negative CPU, one CPU bound to multiple effective groups, or a binding for no effective group
- **THEN** Validate reports the corresponding deterministic CPU-binding issues without invoking a factory or affinity adapter

### Requirement: Affinity uses a tagged adapter and a mandatory locked-thread fallback
The graph package SHALL export `ErrAffinityUnavailable` as a stable sentinel for a worker that continues on the mandatory locked-thread fallback. The runtime SHALL use an internal affinity adapter with a Linux implementation selected by a `linux` build tag and an unsupported-platform implementation selected by a complementary build tag. The Linux adapter SHALL attempt only the requested worker CPU affinity operation. The unsupported implementation SHALL report `ErrAffinityUnavailable`. Every adapter error, including Linux affinity failure, SHALL select the mandatory pure-Go fallback rather than fail Start: the worker SHALL remain on its locked OS thread and SHALL make no further affinity attempt.

The adapter constructor SHALL be injectable only through an internal test seam. The seam SHALL support deterministic success, failure, and ordering tests, SHALL not be exported as application configuration, and SHALL serialize installation and restoration so adapter replacement is not raced by parallel tests.

#### Scenario: Use the unsupported-platform fallback
- **WHEN** a supported graph starts on a build that selects the unsupported adapter
- **THEN** every worker remains locked, processing starts, and its observable affinity result records a fallback warning wrapping ErrAffinityUnavailable

#### Scenario: Fall back after a Linux adapter failure
- **WHEN** the Linux adapter or injected adapter returns an error for a requested CPU
- **THEN** Start succeeds with the worker locked, records that error as the fallback cause, and does not report CPU affinity as applied

### Requirement: Executor exposes immutable affinity outcomes
The graph package SHALL expose `type AffinityOutcome uint8`, constants `AffinityOutcomeApplied` and `AffinityOutcomeLockedThreadFallback`, `type AffinityStatus struct { GroupID GroupID; CPU int; Outcome AffinityOutcome; Warning error }`, and exactly `func (e *Executor) AffinityStatus() []AffinityStatus`. Start SHALL record exactly one status per planned group in ascending GroupID order before returning a successful executor. An applied status SHALL have nil Warning. A fallback status SHALL have non-nil Warning that matches `ErrAffinityUnavailable` and retains the adapter cause when one exists.

AffinityStatus SHALL return an independent slice and SHALL not expose a mutable worker, binding-plan, adapter, or lifecycle resource. It SHALL return nil for a nil or zero executor. Statuses SHALL be immutable after Start succeeds and SHALL remain available after Drain or Close. This is the sole warning/status surface for affinity; the runtime SHALL add no log, metric, event, callback, or error-routing behavior.

#### Scenario: Inspect mixed affinity results
- **WHEN** an injected adapter applies affinity for one ascending group and fails for a later group
- **THEN** AffinityStatus returns two ordered independent entries with applied and fallback outcomes respectively

#### Scenario: Protect the returned status slice
- **WHEN** a caller changes the slice returned by AffinityStatus
- **THEN** a later AffinityStatus call returns the original recorded outcomes

### Requirement: Platform affinity behavior is tested without platform assumptions
The repository SHALL test CPU-binding error precedence, total binding coverage, immutable copied bindings, duplicate and negative validation, locked-before-Init behavior, applied and fallback status values, Init and Close worker ownership, rollback ordering, Drain and Close behavior, and status immutability. It SHALL test the Linux adapter under its Linux build tag and the unsupported adapter under its complementary build tag. Adapter-injected tests SHALL prove behavior without requiring a particular host CPU topology or affinity permission.

The repository SHALL run the affinity startup, status, and lifecycle tests under `go test -race` without data races or goroutine leaks. Race coverage SHALL include reading immutable status after Start while workers process and lifecycle control commands cross groups; concurrent Start, Drain, and Close remain unsupported.

#### Scenario: Run adapter-independent race coverage
- **WHEN** the injected-adapter startup and lifecycle suite runs with the race detector
- **THEN** worker readiness, status publication, pause barriers, and shutdown complete without a data race or leaked worker
