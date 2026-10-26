# Execution and lifecycle

This document consolidates the current normative requirements. Scenarios are retained where they add behavioral detail; shared concepts should be read together across sections.


## single-group-executor

Define one-worker execution for a valid immutable graph definition.


### Start one executor from a valid definition

The graph package SHALL expose exactly `func (d *Definition) Start() (*Executor, error)`. `Start` on a nil receiver or zero-value or otherwise invalid definition SHALL return `(nil, ErrInvalidDefinition)` without invoking a factory, allocating a runtime resource, locking a thread, or mutating definition topology. A successful call SHALL create exactly one `Executor` and exactly one worker goroutine per immutable planned group; a definition whose nodes all have effective DefaultGroupID SHALL therefore create exactly one locked worker and one local execution group. Start SHALL use each group's immutable CPU binding but SHALL NOT accept group, replica, CPU, affinity, NUMA, timeout, or transport configuration. The definition SHALL retain no executor pointer or lifecycle state, and concurrent starts from one definition SHALL remain unsupported.

#### Start a built CPU-bound default-group definition
- a caller starts an issue-free definition returned by Build whose nodes all use DefaultGroupID and the group has its required CPU binding
- it receives a running one-worker executor after the worker has locked its thread and recorded an affinity outcome, while the definition's identity, topology, and later inspection results remain unchanged

#### Reject an invalid definition
- Start is invoked on a nil or zero-value definition
- it returns ErrInvalidDefinition without calling node or queue factories, locking a thread, or calling an affinity adapter


### Transactional startup materializes dormant factories

For every Start, the executor SHALL process edges in ascending `EdgeID`, construct each edge pool, and invoke each non-nil `queue.Factory` once only for its stored local edge with its stored `queue.Config`; a nil local factory SHALL create the built-in local queue and a remote edge SHALL create the built-in SPSC queue without invoking a factory. It SHALL then process nodes in the definition's canonical topological order and invoke exactly one applicable `Stateful.Factory` or `ClosureFactory` per node. A custom local queue factory result SHALL have non-nil `TryPush`, `TryPop`, and `Len` callbacks or Start SHALL fail with `ErrInvalidQueueOps`. Closure normalization SHALL validate every handler and selector required by its registered arity before a worker is visible.

After successful resource and factory construction, Start SHALL create locked workers and record each affinity outcome before dispatching optional Init callbacks in canonical topological order to their owner workers. A worker SHALL call Init only after it locks its OS thread and attempts affinity, and no worker SHALL run handlers before every Init succeeds. A factory error SHALL transfer no lifecycle ownership. A successful explicit or closure factory result SHALL transfer lifecycle ownership even if its Init later fails. On any edge-factory, node-factory, normalization, closure-replica, or Init failure, Start SHALL not return an executor, SHALL release framework-created resources and queued or pending batches, SHALL invoke each owned node Close exactly once on its locked owner worker in reverse successful-construction order, and SHALL return `errors.Join` of the triggering error and any close errors. A custom queue factory owns cleanup for resources beyond the batches exposed through `QueueOps`; this stage SHALL NOT add a queue-close callback. An affinity adapter error SHALL not fail Start and SHALL record a locked-thread fallback outcome instead.

#### Roll back a failed initialization
- a node Init fails after earlier node factories and inits have succeeded
- Start returns no executor, closes the failed node and every previously owned node once on their locked workers in reverse construction order, releases framework resources, and surfaces the Init and any Close errors

#### Start a fresh sequential executor
- an executor has stopped and the same definition is started again
- every node and eligible local custom queue factory is invoked again for fresh runtime resources and each group receives a new locked worker and affinity outcome without Build rerunning or definition metadata changing


### Execute generated typed handlers on fixed-plan workers

The executor SHALL use generated monomorphized bindings for all registered explicit-state and closure node arities zero through eight, including each registered concrete `E` error finalizer. Each group worker SHALL schedule only its planned runners in canonical node order. A zero-input node SHALL be eligible once per scheduler pass while sources are enabled. A one-input node SHALL invoke its only handler when its input has a batch. A multi-input node SHALL form a `node.ReadyMask`, call its selector only when two or more input positions are ready, and invoke exactly the typed handler selected by an in-range ready `node.PortIndex`. An out-of-range or not-ready selector result SHALL panic; the executor SHALL NOT recover it.

The scheduler SHALL use only preconstructed monomorphized function pointers and numeric metadata in its steady-state data path. It SHALL NOT use reflection, runtime type assertions, `any` or `interface{}` data-plane storage, mutexes, blocking data-plane channels, worker replicas, or a queue concurrency model other than local single-worker or remote SPSC.

#### Dispatch a selected heterogeneous input
- two differently typed input edges are ready for one multi-input node and its selector chooses the second ready position
- only that position's generated typed handler receives its batch and the first edge remains ready for a later pass

#### Run a source and downstream sink across groups
- a zero-input source commits a batch to an ordinary edge whose sink is in another planned group
- the assigned workers invoke the source and sink handlers through generated bindings and transfer the batch without a runtime data type assertion

#### Dispatch a typed error consumer
- a connected error edge delivers `Batch[E]` to a one-input error consumer
- its generated typed input handler receives that batch without reflection, a runtime type assertion, or a boxed error value


### Publish results and clean up batch ownership

After each handler returns a successful `node.Result`, the executor SHALL offer every committed ordinary output to its single connected edge and SHALL release every input batch not moved through `node.Output.Commit`. When a handler returns `Result{Failed: true}`, the executor SHALL release every un-moved input and every acquired or committed ordinary output, publish none of those outputs, and process the typed result value only through the node's optional connected `ErrorPort[E]`. An unconnected error port SHALL discard the result value. A connected error port SHALL create, publish, retry, drop, and release a one-element typed error batch according to that edge's pool and queue policy. `Result.Value` SHALL remain unboxed throughout this boundary. A sink SHALL release every input not moved by its handler.

#### Discard a failed result
- a handler commits an ordinary output and then returns a failed Result whose error port is unconnected
- the executor releases the committed output and un-moved input, does not invoke the downstream ordinary handler, and discards the typed result value

#### Preserve a moved input
- a handler commits a compatible input batch to an ordinary output before returning a successful result
- Commit clears the handler's input handle and the executor does not release that moved batch at the input boundary

#### Discard an unconnected failed result
- a handler commits an ordinary output and then returns a failed Result whose error port is unconnected
- the executor releases the committed output and un-moved input, does not invoke a downstream ordinary handler, and does not allocate or publish an error batch

#### Publish a connected failed result
- a handler returns a failed Result with a connected matching error edge
- the executor publishes exactly one one-element `Batch[E]` or applies that edge's explicit pool-exhaustion, Drop, or Wait behavior


### Local bounded queues honor edge policy

For a local edge without a custom factory, the executor SHALL construct a bounded typed local ring using preallocated batch-handle storage and ordinary single-worker head, tail, and count metadata. The built-in local ring and every custom `QueueOps` invocation SHALL be used by that local edge's group worker only. On a successful `TryPush`, ownership SHALL transfer to the edge queue. On a full local edge with `queue.Drop`, the executor SHALL release the output batch immediately. On a full local edge with `queue.Wait`, the executor SHALL retain the pending output, not invoke that runner again until it publishes, continue scheduling other runners, and retry on later passes. It SHALL NOT busy-spin inside the producing handler or block downstream progress. Cross-group policy is defined by the manual-multigroup-runtime capability.

#### Drop on a full local edge
- a committed output encounters a full built-in local edge configured with Drop
- the executor releases that output once and continues scheduling without allocating or blocking

#### Wait while allowing downstream progress
- a committed output encounters a full built-in local edge configured with Wait
- its runner retains that output, the scheduler runs an eligible downstream consumer, and a later pass retries the retained output before that runner handles new input


### Drain and close executor lifecycle

`Executor` SHALL expose `func (e *Executor) Drain() error`, `func (e *Executor) DrainContext(context.Context) error`, and `func (e *Executor) Close() error`. Drain SHALL delegate to context-aware draining without a deadline. Draining SHALL close runtime-local admission, pause workers, stop new zero-input source invocations, and process accepted local and remote work and pending Wait outputs until the fixed-plan executor has no runnable, queued, or pending batches. Already committed source outputs SHALL continue retrying without reinvoking their source handlers. DrainContext cancellation SHALL request forced discard at an acknowledged all-worker boundary and SHALL preserve both cancellation and explicit context cause in its result alongside cleanup errors. It SHALL not forcibly interrupt user callbacks that do not return. Close SHALL stop sources and release queued and pending local and remote batches without normal processing. All stop paths SHALL invoke owned node Close callbacks once on their still-locked owner workers in reverse successful-construction order, then stop and join every worker so each unlocks its OS thread. Runtime-local admission/logger participants SHALL receive the original drain context, discard unconsumed work when forced, and join before completed shutdown. Repeated lifecycle calls SHALL retain the final stop error without duplicate cleanup; concurrent lifecycle calls remain unsupported. Cleanup errors SHALL be surfaced with `errors.Join`.

#### Gracefully drain default-group local work
- Drain is requested after a source has committed a batch to a local edge in a default-group definition
- sources stop producing, downstream receives accepted work, and the locked worker runs Close only after queues and pending output are empty before unlocking and stopping

#### Force close multi-group work
- Close is requested while a local or remote edge queue or Wait output holds batches
- the executor releases those batches without invoking further handlers, closes each owned lifecycle once on its locked worker, and waits for every worker to unlock and stop

#### Retry an existing source output while draining
- drain disables new source calls while an already committed source output waits for queue space
- scheduler retries preserve that batch's ownership and do not call the source handler again after the pending output publishes

#### Cancel at a safe barrier
- a supplied context cancels while queued or pending work remains and callbacks return
- the next acknowledged barrier discards remaining framework-owned work once and returns an error matching the cancellation and any lifecycle errors


### Zero-allocation built-in steady state

After a successful Start, the built-in fixed-plan data path SHALL allocate zero framework heap objects per operation for source-to-sink delivery, one-input dispatch, multi-input selector dispatch, typed ordinary or error output publication, input release, local queue push/pop, remote SPSC push/pop, Drop release, and Wait retry. The guarantee SHALL exclude startup, shutdown, factory calls, user handlers and selectors, and custom local queues. The repository SHALL enforce the built-in paths with `testing.AllocsPerRun` and retain benchmarks that report allocations after setup.

#### Measure steady-state local and remote delivery
- allocation tests and benchmarks run started built-in default-group and fixed multi-group graphs after setup
- the covered framework paths report zero allocations per operation

#### Measure steady-state error delivery
- allocation tests and benchmarks run a started built-in graph that routes a failed result to a local or fixed multi-group error consumer after setup
- the covered framework path reports zero allocations per operation


## manual-multigroup-runtime

Define manual fixed-group placement and multi-worker execution for immutable graph definitions.


### Fixed manual placement has a minimal graph API

The graph package SHALL expose `const InvalidGroupID GroupID = 0`, `const DefaultGroupID GroupID = 1`, and exactly `func Place(builder *Builder, node NodeID, group GroupID) error` for manual group placement. It SHALL export `ErrInvalidGroupID`, `ErrUnknownNode`, and `ErrNodeAlreadyPlaced` as stable sentinels. `Place` SHALL accept only an initialized open builder, a registered nonzero node ID owned by that builder, and a nonzero group ID; it SHALL record exactly one immutable intended placement for that node without invoking callbacks or factories, changing registration or connection metadata, consuming identities, or creating runtime resources.

`Place` SHALL select errors in this order: invalid builder, sealed builder, invalid group ID, unknown node ID, then an already explicitly placed node. A node with no explicit placement SHALL have effective `DefaultGroupID`. Positive group IDs SHALL be opaque, may be sparse, and SHALL not imply CPU, thread, replica, affinity, NUMA, or automatic-grouping behavior. This capability SHALL add no group configuration, plan accessor, reassignment, removal, replica, or Start configuration API.

#### Place a registered node in a sparse group
- a caller places a registered node in nonzero GroupID 7 on an open builder
- the builder records GroupID 7 without invoking any factory or changing node and edge IDs

#### Preserve the default one-group graph
- a caller builds a valid graph without calling Place
- every node has effective DefaultGroupID and the resulting execution contains one group

#### Reject invalid placement deterministically
- Place receives a sealed or invalid builder, zero group ID, unknown node ID, or a second placement for one node
- it returns the first applicable documented sentinel and leaves builder state unchanged


### Build validates and compiles the fixed group plan

`Validate` and `Build` SHALL validate stored manual placement and CPU binding without invoking node, queue, lifecycle, or affinity functions. `ValidationCodeInvalidGroupPlacement` SHALL identify malformed stored group placement with its affected NodeID. `ValidationCodeRemoteQueueFactory` SHALL identify an ordinary or error cross-group edge with a non-nil custom queue factory with its EdgeID. CPU-binding diagnostics SHALL identify missing, invalid, duplicate, or ineffective group bindings with the affected GroupID and CPU as defined by cpu-affinity-runtime. Any such issue SHALL produce the existing sorted `ValidationReport` and `*ValidationError`; Build SHALL leave the builder open and publish no definition.

For an issue-free graph, Build SHALL resolve every omitted placement to DefaultGroupID; enumerate distinct effective groups in ascending GroupID order; retain each group’s nodes in canonical topological order; assign every group its validated explicit CPU ID; and classify every ordinary and error edge local when its source and target effective groups match or remote when they differ. The resulting private group plan, CPU bindings, and edge classifications SHALL be copied into the immutable definition and SHALL contain no live queue, pool, error batch, worker, thread, adapter, factory result, executor, affinity result, or lifecycle state. Build SHALL not renumber or merge groups, create a group for an unused ID, infer placement or CPU binding, validate a platform CPU upper bound, or choose a queue concurrency model.

#### Compile a deterministic mixed-group CPU plan
- a valid graph uses default group 1 and explicitly places nodes in sparse groups 3 and 9, with a distinct nonnegative CPU bound to each effective group
- Build records groups in order 1, 3, 9, pairs each with its supplied CPU, retains each group's canonical node order, and classifies every ordinary and error edge from endpoint placement

#### Reject a missing CPU before Start
- a valid manually placed graph has an effective group with no CPU binding
- Build returns a ValidationError without invoking a factory or affinity adapter and the builder remains correctable

#### Reject a remote custom queue before Start
- a valid ordinary edge has a non-nil queue factory and its endpoint nodes have different effective groups
- Validate reports ValidationCodeRemoteQueueFactory for that EdgeID, Build returns a ValidationError without invoking the factory, and the builder remains correctable

#### Reject a remote error queue factory before Start
- a valid typed error edge has a non-nil queue factory and its endpoint nodes have different effective groups
- Validate reports `ValidationCodeRemoteQueueFactory` for that EdgeID, Build returns a ValidationError without invoking the factory, and the builder remains correctable

#### Keep same-group factory eligibility
- a valid ordinary or error edge with a non-nil queue factory has endpoints in the same effective group and every effective group has a valid CPU binding
- Build succeeds without invoking the factory and preserves it for that local edge's future Start


### One worker serves each planned group

`Definition.Start` SHALL retain signature `func (d *Definition) Start() (*Executor, error)` and SHALL create exactly one worker goroutine for each group in the immutable plan, in ascending GroupID startup order. Before any group Init callback or runner processing, each worker SHALL lock its own OS thread, attempt its immutable CPU binding through the internal affinity adapter, record an applied or locked-thread-fallback outcome, and report ready. Each worker SHALL run only that group’s canonical-order runners. Start SHALL create no worker replicas, node worker pool, transport, ingress endpoint, automatic group, CPU discovery, NUMA placement, or Start configuration.

Every ordinary or error edge with different endpoint groups SHALL use one framework-owned bounded typed SPSC ring. Its source group worker SHALL be its sole producer and its target group worker SHALL be its sole consumer. The ring SHALL use preallocated batch-handle storage and atomic cursor publication suitable for that handoff, and SHALL not use locks, channels, reflection, runtime type assertions, data-plane interfaces, or steady-state allocation. Same-group edges SHALL retain local queue behavior.

#### Start two locked manually separated groups
- a valid definition contains nodes placed in two effective groups joined by one ordinary or error edge and both groups have CPU bindings
- Start creates exactly two locked workers, records one affinity outcome for each, runs each node only on its assigned worker, and transfers the edge batch through the built-in SPSC ring

#### Preserve the one-group execution shape
- a valid definition has only DefaultGroupID nodes and a binding for DefaultGroupID
- Start creates exactly one locked worker, records its affinity outcome, and every ordinary or error edge uses the existing local queue path

#### Transfer a typed error across groups
- a failed handler and its matching error consumer are placed in different valid CPU-bound groups
- their workers transfer the one-element typed error batch through the built-in SPSC ring without a runtime data type assertion


### Remote edges use the built-in SPSC queue policy

The existing `queue.Config` Capacity, BatchCapacity, and Policy SHALL apply to an ordinary or error remote SPSC edge. A successful remote TryPush SHALL transfer typed batch ownership to the ring. If a remote ring is full under `queue.Drop`, the producer SHALL release the output exactly once. If it is full under `queue.Wait`, the producer runner SHALL retain the one pending output, SHALL not accept new input until that output publishes, and SHALL retry in later scheduler passes without blocking inside a handler; the consumer group SHALL remain runnable independently.

A non-nil `queue.Factory` SHALL be valid only for a local edge. A remote edge factory SHALL be rejected by Build and SHALL never be invoked by Start. This stage SHALL not expose or select MPSC, SPMC, MPMC, remote custom queues, fan-out, sharding, or replica-aware queue behavior.

#### Deliver remote SPSC work in FIFO order
- a producer group successfully publishes multiple ordinary or error batches to an available remote edge
- the consumer group receives the typed batches once each in publication order with ownership transferred from the queue to its handler

#### Retry a full remote Wait edge while its consumer progresses
- a remote ordinary SPSC edge configured with Wait is full and its producer has a pending output
- the producer runner does not run another handler, the consumer worker can remove a batch, and a later producer pass publishes the retained output

#### Retry a full remote error Wait edge
- a remote error SPSC edge configured with Wait is full and its producer has a pending failed result batch
- the producer runner does not run another handler, the consumer worker can remove the first error batch, and a later producer pass publishes the retained batch


### Multi-group lifecycle is globally deterministic

Before publishing a successful executor, Start SHALL materialize edge resources in ascending EdgeID order and invoke one applicable factory for every logical node in canonical topological order. It SHALL then create and ready all locked workers in ascending GroupID order. The coordinator SHALL invoke optional Init callbacks one at a time in global canonical order by dispatching each callback to its already locked owner worker; no worker SHALL process a runner until all Init callbacks succeed. A local custom queue factory SHALL be called exactly once for its edge per Start; a remote edge SHALL not call one. Each successful node factory result transfers lifecycle ownership and each later sequential Start SHALL obtain fresh node state, closure replica, pool, eligible local queue operations, locked workers, and affinity outcomes.

On startup failure before or during Init, Start SHALL publish no executor, release framework-owned queued or pending batches, and dispatch every owned node Close callback to its still-locked owner worker exactly once in reverse canonical successful-construction order before stopping and unlocking every worker. It SHALL return `errors.Join` of the trigger and Close errors. Adapter unavailability or failure is not a startup failure and SHALL instead select the recorded fallback. After successful Start, Drain and Close SHALL coordinate every group at runner boundaries, disable sources, and preserve global reverse canonical Close order by executing Close callbacks on their locked owner workers before the workers stop and unlock. Drain SHALL repeatedly pause all workers and inspect pending outputs and all local and remote queues; it SHALL resume processing until a pause barrier observes every such location empty, then close lifecycles, stop workers, and unlock threads. Close SHALL pause all workers, discard queued and pending batches without further handlers, close lifecycles on locked workers, then stop workers and unlock threads. Both methods remain idempotent after stop and concurrent lifecycle calls remain unsupported.

#### Roll back a later-group initialization failure
- a node Init in a later planned group fails after earlier canonical nodes succeeded
- no executor is returned, every owned node closes once on its locked owner worker in reverse global canonical order, all framework-managed batches are released, every worker unlocks before exit, and the returned error joins the failure and Close errors

#### Drain work crossing multiple groups
- Drain is requested while local or remote queues and Wait outputs hold accepted batches
- sources stop, workers remain locked while they process to a global paused empty snapshot, downstream handlers receive accepted work, lifecycles close once in reverse canonical order on owner workers, and every worker then unlocks and stops

#### Force close cross-group work
- Close is requested while a remote queue or producer pending output holds a batch
- no further handler runs, each held batch is released exactly once, Close callbacks run on locked owner workers in reverse canonical order, and all workers unlock and stop


### Manual groups require explicit CPU bindings

Every effective group produced by manual placement, including DefaultGroupID, SHALL have one explicit CPU binding before Build succeeds. Group IDs remain opaque sparse positive identities and CPU IDs do not imply automatic grouping, CPU discovery, NUMA placement, replica count, or a queue choice. CPU binding does not alter the existing placement API or permit placement reassignment.

#### Bind the implicit default group
- a valid graph uses no explicit node placement and the caller binds a CPU to DefaultGroupID
- Build preserves one effective default group with that immutable CPU binding


### Remote execution is tested and measured rigorously

The repository SHALL test manual placement errors and defaulting, deterministic plan order and ordinary/error edge classification, remote-factory rejection without invocation, exact worker count, factory freshness and rollback order, SPSC capacity and FIFO ownership, remote Drop and Wait behavior, multi-group Drain and Close, and one-group equivalence. It SHALL run these concurrent handoff and lifecycle tests under `go test -race` without data races or goroutine leaks.

After Start, the built-in remote SPSC enqueue/dequeue and supported two-group source-to-error-sink path SHALL allocate zero framework heap objects per operation, excluding handlers, selectors, factories, local custom queues, startup, and shutdown. Benchmarks SHALL report allocations after setup for remote queue transfer and a multi-group error-routing DAG; throughput and latency remain non-blocking comparison signals.

#### Measure remote steady state
- allocation tests and benchmarks run a started built-in two-group graph after setup
- the covered remote SPSC path reports zero framework allocations per operation and benchmark names encode its batch and group dimensions

#### Measure remote error steady state
- allocation tests and benchmarks run a started built-in two-group graph that routes failed results after setup
- the covered remote error path reports zero framework allocations per operation and benchmark names encode batch and group dimensions


## cpu-affinity-runtime

Define explicit CPU bindings, affinity application with a locked-thread fallback, and immutable affinity outcomes for fixed-group executors.


### Fixed groups have an explicit CPU binding API

The graph package SHALL expose exactly `func BindCPU(builder *Builder, group GroupID, cpu int) error` and SHALL export `ErrInvalidCPU`, `ErrUnknownGroup`, `ErrGroupCPUAssigned`, and `ErrCPUAlreadyBound` as stable sentinels. BindCPU SHALL accept only an initialized open builder, a nonzero currently effective group, and a nonnegative CPU ID. A group is effective when it has at least one registered node after applying its explicit manual placement or DefaultGroupID. BindCPU SHALL record exactly one CPU ID for that group without modifying placement, topology, identities, factories, callbacks, or runtime resources.

BindCPU SHALL choose errors in this order: invalid builder, sealed builder, invalid group ID, negative CPU, unknown effective group, group already assigned, then CPU already assigned to another effective group. It SHALL add no Start configuration, CPU discovery, CPU-set inspection, NUMA setting, replica setting, reassignment, or removal API.

#### Bind sparse effective groups
- a caller places nodes in groups 3 and 9 and binds distinct nonnegative CPUs to groups 3 and 9 before Build
- Build can compile those bindings without changing either group ID or placement

#### Reject an invalid binding without mutation
- BindCPU receives an invalid builder, sealed builder, zero group, negative CPU, ineffective group, duplicate group, or duplicate CPU
- it returns the first applicable documented sentinel and leaves the builder unchanged


### Build validates and retains complete CPU bindings

Validate and Build SHALL validate CPU bindings without invoking node, queue, lifecycle, or affinity functions. For every effective group, there SHALL be exactly one binding; every binding CPU SHALL be nonnegative and unique; and no binding SHALL target an ineffective group. Validation SHALL report `ValidationCodeMissingGroupCPU`, `ValidationCodeInvalidGroupCPU`, `ValidationCodeDuplicateGroupCPU`, or `ValidationCodeIneffectiveGroupCPU` as applicable, identifying the group and CPU through the structured validation issue. Build SHALL return the existing sorted `*ValidationError`, leave the builder open, and publish no definition when any CPU-binding issue exists.

For an issue-free graph, Build SHALL copy one CPU binding with every private ascending effective group-plan entry. The copied plan SHALL be immutable and reusable by sequential Start calls; it SHALL contain no OS thread, adapter, worker, mutable status, or runtime affinity result. Build SHALL enforce no upper CPU bound or host CPU-set policy.

#### Reject incomplete CPU coverage
- a valid graph has effective groups 1 and 7 but binds a CPU only for group 1
- Validate and Build report `ValidationCodeMissingGroupCPU` for group 7, Build publishes no definition, and the caller can bind group 7 and retry

#### Reject corrupted duplicate or invalid binding data
- same-package builder metadata contains a negative CPU, one CPU bound to multiple effective groups, or a binding for no effective group
- Validate reports the corresponding deterministic CPU-binding issues without invoking a factory or affinity adapter


### Affinity uses a tagged adapter and a mandatory locked-thread fallback

The graph package SHALL export `ErrAffinityUnavailable` as a stable sentinel for a worker that continues on the mandatory locked-thread fallback. The runtime SHALL use an internal affinity adapter with a Linux implementation selected by a `linux` build tag and an unsupported-platform implementation selected by a complementary build tag. The Linux adapter SHALL attempt only the requested worker CPU affinity operation. The unsupported implementation SHALL report `ErrAffinityUnavailable`. Every adapter error, including Linux affinity failure, SHALL select the mandatory pure-Go fallback rather than fail Start: the worker SHALL remain on its locked OS thread and SHALL make no further affinity attempt.

The adapter constructor SHALL be injectable only through an internal test seam. The seam SHALL support deterministic success, failure, and ordering tests, SHALL not be exported as application configuration, and SHALL serialize installation and restoration so adapter replacement is not raced by parallel tests.

#### Use the unsupported-platform fallback
- a supported graph starts on a build that selects the unsupported adapter
- every worker remains locked, processing starts, and its observable affinity result records a fallback warning wrapping ErrAffinityUnavailable

#### Fall back after a Linux adapter failure
- the Linux adapter or injected adapter returns an error for a requested CPU
- Start succeeds with the worker locked, records that error as the fallback cause, and does not report CPU affinity as applied


### Executor exposes immutable affinity outcomes

Every successfully finalized automatic physical group SHALL create one locked worker and one immutable ordered AffinityStatus row with logical/physical group identity, replica identity, explicit CPU, outcome, and warning. An applied affinity result SHALL carry no warning; fallback SHALL preserve ErrAffinityUnavailable and any adapter cause. The returned slice SHALL be independently mutable without changing executor state and remain available after stop. Unsupported-platform locked-thread fallback remains portable and does not fail Start.

#### Inspect automatic affinity fallback
- a finalized automatic plan starts with an adapter that rejects each explicit CPU
- every physical worker remains locked and has an ordered fallback status for its assigned CPU

#### Inspect node-pool affinity outcomes
- a logical node has three CPU-bound worker instances
- affinity outcomes identify the same logical node with worker indices zero through two and their assigned CPUs

#### Inspect replicated worker affinity
- two replicas of one logical group start with explicit CPUs
- affinity outcomes contain the same logical group twice with distinct replica indices and assigned CPUs

#### Inspect mixed affinity results
- physical workers receive mixed applied and fallback outcomes
- the ordered snapshot preserves each outcome and fallback warning

#### Protect the returned status slice
- a caller mutates a returned automatic affinity status
- subsequent snapshots retain the original result

#### Inspect mixed affinity results
- applied and fallback outcomes occur across physical workers
- the snapshot preserves each outcome and fallback cause

#### Protect the returned status slice
- a caller mutates a returned affinity result slice
- subsequent reads preserve the immutable recorded results


### Platform affinity behavior is tested without platform assumptions

The repository SHALL test CPU-binding error precedence, total binding coverage, immutable copied bindings, duplicate and negative validation, locked-before-Init behavior, applied and fallback status values, Init and Close worker ownership, rollback ordering, Drain and Close behavior, and status immutability. It SHALL test the Linux adapter under its Linux build tag and the unsupported adapter under its complementary build tag. Adapter-injected tests SHALL prove behavior without requiring a particular host CPU topology or affinity permission.

The repository SHALL run the affinity startup, status, and lifecycle tests under `go test -race` without data races or goroutine leaks. Race coverage SHALL include reading immutable status after Start while workers process and lifecycle control commands cross groups; concurrent Start, Drain, and Close remain unsupported.

#### Run adapter-independent race coverage
- the injected-adapter startup and lifecycle suite runs with the race detector
- worker readiness, status publication, pause barriers, and shutdown complete without a data race or leaked worker


## deadline-drain

Define context-triggered transition from graceful draining to ownership-safe forced shutdown.


### Cooperative deadline drain

Executor SHALL expose DrainContext(context.Context) error while existing Drain remains unbounded. Cancellation SHALL close admission and request forced discard at the next safe all-worker boundary. Accepted batches SHALL be processed or reclaimed exactly once, and returned errors SHALL preserve the cancellation cause and lifecycle errors. Deadlines SHALL not promise interruption of nonreturning user callbacks.
#### Cancel pending drain
- the context cancels while returning callbacks leave queued or Wait-pending work
- the next acknowledged stop boundary discards remaining owned work and returns an error matching the context cause


### Share one lifecycle budget

Ingress closure, worker draining, and any configured logger draining SHALL use the same cancellation budget. Repeated stop calls SHALL preserve the established final outcome without duplicate cleanup.
#### Repeat a canceled stop
- DrainContext has completed forced cleanup and Close is called again
- no batch or lifecycle is released twice and the recorded stop error remains available


## typed-ingress

Define single-producer typed admission and caller/runtime ownership for external pipeline batches.


### Typed bounded producer operations

A registered ingress SHALL bind only to its owning executor Start and SHALL expose typed Acquire, Commit, and copying Submit. Accepted Commit SHALL clear the caller handle; a Wait-full or closed rejection SHALL leave caller ownership intact. Configured Drop SHALL consume and release a rejected full-queue batch. Oversized Submit SHALL reject without partial delivery, and empty Submit SHALL be a no-op. Built-in operations SHALL allocate no framework memory after setup.
#### Retry a full ingress
- a producer commits to a full Wait ingress
- it retains the unchanged batch and can retry or release it without partial publication


### Close admission before draining

Drain and Close SHALL reject new commits, finish admitted nonblocking commits, and account for all accepted batches. Uncommitted producer-owned batches SHALL remain releasable after stop. Restart SHALL produce fresh endpoint resources; stale endpoints SHALL reject new publication.
#### Commit overlaps stop
- Commit races a single supported stop operation
- it either publishes one accepted batch included in stop processing or rejects without transferring caller ownership
