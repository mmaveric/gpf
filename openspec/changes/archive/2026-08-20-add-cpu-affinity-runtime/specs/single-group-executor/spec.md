## MODIFIED Requirements

### Requirement: Start one executor from a valid definition
The graph package SHALL expose exactly `func (d *Definition) Start() (*Executor, error)`. `Start` on a nil receiver or zero-value or otherwise invalid definition SHALL return `(nil, ErrInvalidDefinition)` without invoking a factory, allocating a runtime resource, locking a thread, or mutating definition topology. A successful call SHALL create exactly one `Executor` and exactly one worker goroutine per immutable planned group; a definition whose nodes all have effective DefaultGroupID SHALL therefore create exactly one locked worker and one local execution group. Start SHALL use each group's immutable CPU binding but SHALL NOT accept group, replica, CPU, affinity, NUMA, timeout, or transport configuration. The definition SHALL retain no executor pointer or lifecycle state, and concurrent starts from one definition SHALL remain unsupported.

#### Scenario: Start a built CPU-bound default-group definition
- **WHEN** a caller starts an issue-free definition returned by Build whose nodes all use DefaultGroupID and the group has its required CPU binding
- **THEN** it receives a running one-worker executor after the worker has locked its thread and recorded an affinity outcome, while the definition's identity, topology, and later inspection results remain unchanged

#### Scenario: Reject an invalid definition
- **WHEN** Start is invoked on a nil or zero-value definition
- **THEN** it returns ErrInvalidDefinition without calling node or queue factories, locking a thread, or calling an affinity adapter

### Requirement: Transactional startup materializes dormant factories
For every Start, the executor SHALL process edges in ascending `EdgeID`, construct each edge pool, and invoke each non-nil `queue.Factory` once only for its stored local edge with its stored `queue.Config`; a nil local factory SHALL create the built-in local queue and a remote edge SHALL create the built-in SPSC queue without invoking a factory. It SHALL then process nodes in the definition's canonical topological order and invoke exactly one applicable `Stateful.Factory` or `ClosureFactory` per node. A custom local queue factory result SHALL have non-nil `TryPush`, `TryPop`, and `Len` callbacks or Start SHALL fail with `ErrInvalidQueueOps`. Closure normalization SHALL validate every handler and selector required by its registered arity before a worker is visible.

After successful resource and factory construction, Start SHALL create locked workers and record each affinity outcome before dispatching optional Init callbacks in canonical topological order to their owner workers. A worker SHALL call Init only after it locks its OS thread and attempts affinity, and no worker SHALL run handlers before every Init succeeds. A factory error SHALL transfer no lifecycle ownership. A successful explicit or closure factory result SHALL transfer lifecycle ownership even if its Init later fails. On any edge-factory, node-factory, normalization, closure-replica, or Init failure, Start SHALL not return an executor, SHALL release framework-created resources and queued or pending batches, SHALL invoke each owned node Close exactly once on its locked owner worker in reverse successful-construction order, and SHALL return `errors.Join` of the triggering error and any close errors. A custom queue factory owns cleanup for resources beyond the batches exposed through `QueueOps`; this stage SHALL NOT add a queue-close callback. An affinity adapter error SHALL not fail Start and SHALL record a locked-thread fallback outcome instead.

#### Scenario: Roll back a failed initialization
- **WHEN** a node Init fails after earlier node factories and inits have succeeded
- **THEN** Start returns no executor, closes the failed node and every previously owned node once on their locked workers in reverse construction order, releases framework resources, and surfaces the Init and any Close errors

#### Scenario: Start a fresh sequential executor
- **WHEN** an executor has stopped and the same definition is started again
- **THEN** every node and eligible local custom queue factory is invoked again for fresh runtime resources and each group receives a new locked worker and affinity outcome without Build rerunning or definition metadata changing

### Requirement: Drain and close executor lifecycle
`Executor` SHALL expose exactly `func (e *Executor) Drain() error` and `func (e *Executor) Close() error`. Drain SHALL stop new zero-input source invocations, process accepted local and remote work and pending Wait outputs until the fixed-plan executor has no runnable, queued, or pending batches, release any remaining queue entries, invoke owned node Close callbacks once on their still-locked owner workers in reverse successful-construction order, then stop and join every worker so each unlocks its OS thread. Close SHALL stop sources, release queued and pending local and remote batches without normal processing, invoke those Close callbacks once on still-locked owner workers in reverse successful-construction order, then stop and join every worker so each unlocks its OS thread. Both methods SHALL be idempotent after a stopped executor; concurrent lifecycle calls are unsupported. Close errors SHALL be surfaced with `errors.Join`.

#### Scenario: Gracefully drain default-group local work
- **WHEN** Drain is requested after a source has committed a batch to a local edge in a CPU-bound default-group definition
- **THEN** sources stop producing, the downstream handler receives accepted work, and the locked worker runs its Close callback only after queues and pending output are empty before unlocking and stopping

#### Scenario: Force close multi-group work
- **WHEN** Close is requested while a local or remote edge queue or Wait output holds batches
- **THEN** the executor releases those batches without invoking further handlers, closes each owned lifecycle once on its locked worker, and waits for every worker to unlock and stop
