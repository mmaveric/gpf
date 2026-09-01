## ADDED Requirements

### Requirement: Manual groups require explicit CPU bindings
Every effective group produced by manual placement, including DefaultGroupID, SHALL have one explicit CPU binding before Build succeeds. Group IDs remain opaque sparse positive identities and CPU IDs do not imply automatic grouping, CPU discovery, NUMA placement, replica count, or a queue choice. CPU binding does not alter the existing placement API or permit placement reassignment.

#### Scenario: Bind the implicit default group
- **WHEN** a valid graph uses no explicit node placement and the caller binds a CPU to DefaultGroupID
- **THEN** Build preserves one effective default group with that immutable CPU binding

## MODIFIED Requirements

### Requirement: Build validates and compiles the fixed group plan
`Validate` and `Build` SHALL validate stored manual placement and CPU binding without invoking node, queue, lifecycle, or affinity functions. `ValidationCodeInvalidGroupPlacement` SHALL identify malformed stored group placement with its affected NodeID. `ValidationCodeRemoteQueueFactory` SHALL identify a cross-group edge with a non-nil custom queue factory with its EdgeID. CPU-binding diagnostics SHALL identify missing, invalid, duplicate, or ineffective group bindings with the affected GroupID and CPU as defined by cpu-affinity-runtime. Any such issue SHALL produce the existing sorted `ValidationReport` and `*ValidationError`; Build SHALL leave the builder open and publish no definition.

For an issue-free graph, Build SHALL resolve every omitted placement to DefaultGroupID; enumerate distinct effective groups in ascending GroupID order; retain each group’s nodes in canonical topological order; assign every group its validated explicit CPU ID; and classify every edge local when its source and target effective groups match or remote when they differ. The resulting private group plan, CPU bindings, and edge classifications SHALL be copied into the immutable definition and SHALL contain no live queue, pool, worker, thread, adapter, factory result, executor, affinity result, or lifecycle state. Build SHALL not renumber or merge groups, create a group for an unused ID, infer placement or CPU binding, validate a platform CPU upper bound, or choose a queue concurrency model.

#### Scenario: Compile a deterministic mixed-group CPU plan
- **WHEN** a valid graph uses default group 1 and explicitly places nodes in sparse groups 3 and 9, with a distinct nonnegative CPU bound to each effective group
- **THEN** Build records groups in order 1, 3, 9, pairs each with its supplied CPU, retains each group’s canonical node order, and classifies every edge from endpoint placement

#### Scenario: Reject a missing CPU before Start
- **WHEN** a valid manually placed graph has an effective group with no CPU binding
- **THEN** Build returns a ValidationError without invoking a factory or affinity adapter and the builder remains correctable

#### Scenario: Reject a remote custom queue before Start
- **WHEN** a valid typed edge has a non-nil queue factory and its endpoint nodes have different effective groups
- **THEN** Validate reports ValidationCodeRemoteQueueFactory for that EdgeID, Build returns a ValidationError without invoking the factory, and the builder remains correctable

#### Scenario: Keep same-group factory eligibility
- **WHEN** a valid edge with a non-nil queue factory has endpoints in the same effective group and every effective group has a valid CPU binding
- **THEN** Build succeeds without invoking the factory and preserves it for that local edge’s future Start

### Requirement: One worker serves each planned group
`Definition.Start` SHALL retain signature `func (d *Definition) Start() (*Executor, error)` and SHALL create exactly one worker goroutine for each group in the immutable plan, in ascending GroupID startup order. Before any group Init callback or runner processing, each worker SHALL lock its own OS thread, attempt its immutable CPU binding through the internal affinity adapter, record an applied or locked-thread-fallback outcome, and report ready. Each worker SHALL run only that group’s canonical-order runners. Start SHALL create no worker replicas, node worker pool, transport, ingress endpoint, automatic group, CPU discovery, NUMA placement, or Start configuration.

Every edge with different endpoint groups SHALL use one framework-owned bounded typed SPSC ring. Its source group worker SHALL be its sole producer and its target group worker SHALL be its sole consumer. The ring SHALL use preallocated batch-handle storage and atomic cursor publication suitable for that handoff, and SHALL not use locks, channels, reflection, runtime type assertions, data-plane interfaces, or steady-state allocation. Same-group edges SHALL retain the local queue behavior supplied by the existing executor capability.

#### Scenario: Start two locked manually separated groups
- **WHEN** a valid definition contains nodes placed in two effective groups joined by one edge and both groups have CPU bindings
- **THEN** Start creates exactly two locked workers, records one affinity outcome for each, runs each node only on its assigned worker, and transfers the edge batch through the built-in SPSC ring

#### Scenario: Preserve the one-group execution shape
- **WHEN** a valid definition has only DefaultGroupID nodes and a binding for DefaultGroupID
- **THEN** Start creates exactly one locked worker, records its affinity outcome, and every edge uses the existing local queue path

### Requirement: Multi-group lifecycle is globally deterministic
Before publishing a successful executor, Start SHALL materialize edge resources in ascending EdgeID order and invoke one applicable factory for every logical node in canonical topological order. It SHALL then create and ready all locked workers in ascending GroupID order. The coordinator SHALL invoke optional Init callbacks one at a time in global canonical order by dispatching each callback to its already locked owner worker; no worker SHALL process a runner until all Init callbacks succeed. A local custom queue factory SHALL be called exactly once for its edge per Start; a remote edge SHALL not call one. Each successful node factory result transfers lifecycle ownership and each later sequential Start SHALL obtain fresh node state, closure replica, pool, eligible local queue operations, locked workers, and affinity outcomes.

On startup failure before or during Init, Start SHALL publish no executor, release framework-owned queued or pending batches, and dispatch every owned node Close callback to its still-locked owner worker exactly once in reverse canonical successful-construction order before stopping and unlocking every worker. It SHALL return `errors.Join` of the trigger and Close errors. Adapter unavailability or failure is not a startup failure and SHALL instead select the recorded fallback. After successful Start, Drain and Close SHALL coordinate every group at runner boundaries, disable sources, and preserve global reverse canonical Close order by executing Close callbacks on their locked owner workers before the workers stop and unlock. Drain SHALL repeatedly pause all workers and inspect pending outputs and all local and remote queues; it SHALL resume processing until a pause barrier observes every such location empty, then close lifecycles, stop workers, and unlock threads. Close SHALL pause all workers, discard queued and pending batches without further handlers, close lifecycles on locked workers, then stop workers and unlock threads. Both methods remain idempotent after stop and concurrent lifecycle calls remain unsupported.

#### Scenario: Roll back a later-group initialization failure
- **WHEN** a node Init in a later planned group fails after earlier canonical nodes succeeded
- **THEN** no executor is returned, every owned node closes once on its locked owner worker in reverse global canonical order, all framework-managed batches are released, every worker unlocks before exit, and the returned error joins the failure and Close errors

#### Scenario: Drain work crossing multiple locked groups
- **WHEN** Drain is requested while local or remote queues and Wait outputs hold accepted batches
- **THEN** sources stop, workers remain locked while they process to a global paused empty snapshot, downstream handlers receive accepted work, lifecycles close once in reverse canonical order on owner workers, and every worker then unlocks and stops

#### Scenario: Force close cross-group work
- **WHEN** Close is requested while a remote queue or producer pending output holds a batch
- **THEN** no further handler runs, each held batch is released exactly once, Close callbacks run on locked owner workers in reverse canonical order, and all workers unlock and stop
