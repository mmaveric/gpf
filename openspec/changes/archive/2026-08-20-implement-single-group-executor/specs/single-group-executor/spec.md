## ADDED Requirements

### Requirement: Start one executor from a valid definition
The graph package SHALL expose exactly `func (d *Definition) Start() (*Executor, error)`. `Start` on a nil receiver or zero-value or otherwise invalid definition SHALL return `(nil, ErrInvalidDefinition)` without invoking a factory, allocating a runtime resource, or mutating definition topology. A successful call SHALL create exactly one `Executor` with exactly one worker goroutine and one local execution group; it SHALL NOT accept group, replica, CPU, affinity, NUMA, timeout, or transport configuration. The definition SHALL retain no executor pointer or lifecycle state, and concurrent starts from one definition SHALL remain unsupported.

#### Scenario: Start a built definition
- **WHEN** a caller starts an issue-free definition returned by Build
- **THEN** it receives a running one-worker executor while the definition's identity, topology, and later inspection results remain unchanged

#### Scenario: Reject an invalid definition
- **WHEN** Start is invoked on a nil or zero-value definition
- **THEN** it returns `ErrInvalidDefinition` without calling node or queue factories

### Requirement: Transactional startup materializes dormant factories
For every Start, the executor SHALL process edges in ascending `EdgeID`, construct each edge pool, and invoke each non-nil `queue.Factory` once with its stored `queue.Config`; a nil factory SHALL create the built-in local queue. It SHALL then process nodes in the definition's canonical topological order and invoke exactly one applicable `Stateful.Factory` or `ClosureFactory` per node, followed by optional `Init` in that same order. A custom queue factory result SHALL have non-nil `TryPush`, `TryPop`, and `Len` callbacks or Start SHALL fail with `ErrInvalidQueueOps`. Closure normalization SHALL validate every handler and selector required by its registered arity before the worker is visible.

A factory error SHALL transfer no lifecycle ownership. A successful explicit or closure factory result SHALL transfer lifecycle ownership even if its Init later fails. On any edge-factory, node-factory, normalization, closure-replica, or init failure, Start SHALL not return an executor, SHALL release framework-created resources and queued or pending batches, SHALL invoke each owned node Close exactly once in reverse successful-construction order, and SHALL return `errors.Join` of the triggering error and any close errors. A custom queue factory owns cleanup for resources beyond the batches exposed through `QueueOps`; this stage SHALL NOT add a queue-close callback.

#### Scenario: Roll back a failed initialization
- **WHEN** a node Init fails after earlier node factories and inits have succeeded
- **THEN** Start returns no executor, closes the failed node and every previously owned node once in reverse construction order, and surfaces the init and any close failures

#### Scenario: Start a fresh sequential executor
- **WHEN** an executor has stopped and the same definition is started again
- **THEN** every node and custom queue factory is invoked again for fresh runtime resources without Build rerunning or definition metadata changing

### Requirement: Execute generated typed handlers on one worker
The executor SHALL use generated monomorphized bindings for all registered explicit-state and closure node arities zero through eight. A zero-input node SHALL be eligible once per scheduler pass while sources are enabled. A one-input node SHALL invoke its only handler when its input has a batch. A multi-input node SHALL form a `node.ReadyMask`, call its selector only when two or more input positions are ready, and invoke exactly the typed handler selected by an in-range ready `node.PortIndex`. An out-of-range or not-ready selector result SHALL panic; the executor SHALL NOT recover it.

The scheduler SHALL use only preconstructed function pointers and numeric metadata in its steady-state data path. It SHALL NOT use reflection, runtime type assertions, `any` or `interface{}` data-plane storage, channels, mutexes, atomics, multiple workers, or a cross-group queue.

#### Scenario: Dispatch a selected heterogeneous input
- **WHEN** two differently typed input edges are ready for one multi-input node and its selector chooses the second ready position
- **THEN** only that position's generated typed handler receives its batch and the first edge remains ready for a later pass

#### Scenario: Run a source and downstream sink
- **WHEN** a zero-input source commits a batch to its output and the connected sink is scheduled
- **THEN** the one worker invokes the source and sink handlers through generated bindings without a runtime data type assertion

### Requirement: Publish results and clean up batch ownership
After each handler returns a successful `node.Result`, the executor SHALL offer every committed ordinary output to its single connected edge and SHALL release every input batch not moved through `node.Output.Commit`. When a handler returns `Result{Failed: true}`, the executor SHALL release every un-moved input and every acquired or committed output, publish none of those outputs, and perform no error-port delivery, error routing, logging, retry, metric, or event action. `Result.Value` SHALL remain unboxed and SHALL not leave the typed handler boundary in this stage. A sink SHALL release every input not moved by its handler.

#### Scenario: Discard a failed result
- **WHEN** a handler commits an output and then returns a failed Result
- **THEN** the executor releases the committed output and un-moved input and does not invoke the downstream handler or use the reserved error output

#### Scenario: Preserve a moved input
- **WHEN** a handler commits a compatible input batch to an output
- **THEN** Commit clears the handler's input handle and the executor does not release that moved batch at the input boundary

### Requirement: Local bounded queues honor edge policy
For an edge without a custom factory, the executor SHALL construct a bounded typed local ring using preallocated batch-handle storage and ordinary single-worker head, tail, and count metadata. The built-in ring and every custom `QueueOps` invocation SHALL be used by the executor worker only. On a successful `TryPush`, ownership SHALL transfer to the edge queue. On a full edge with `queue.Drop`, the executor SHALL release the output batch immediately. On a full edge with `queue.Wait`, the executor SHALL retain the pending output, not invoke that runner again until it publishes, continue scheduling other runners, and retry on later passes. It SHALL NOT busy-spin inside the producing handler or block downstream progress.

#### Scenario: Drop on a full local edge
- **WHEN** a committed output encounters a full built-in local edge configured with Drop
- **THEN** the executor releases that output once and continues scheduling without allocating or blocking

#### Scenario: Wait while allowing downstream progress
- **WHEN** a committed output encounters a full built-in local edge configured with Wait
- **THEN** its runner retains that output, the scheduler runs an eligible downstream consumer, and a later pass retries the retained output before that runner handles new input

### Requirement: Drain and close executor lifecycle
`Executor` SHALL expose exactly `func (e *Executor) Drain() error` and `func (e *Executor) Close() error`. Drain SHALL stop new zero-input source invocations, process accepted local work and pending Wait outputs until the executor has no runnable, queued, or pending batches, release any remaining queue entries, then invoke owned node Close callbacks once in reverse construction order and wait for the worker to stop. Close SHALL stop sources, release queued and pending batches without normal processing, invoke those Close callbacks once in reverse construction order, and wait for worker stop. Both methods SHALL be idempotent after a stopped executor; concurrent lifecycle calls are unsupported. Close errors SHALL be surfaced with `errors.Join`.

#### Scenario: Gracefully drain local work
- **WHEN** Drain is requested after a source has committed a batch to a local edge
- **THEN** sources stop producing, the downstream handler receives accepted work, and Close callbacks run only after queues and pending output are empty

#### Scenario: Force close local work
- **WHEN** Close is requested while an edge queue or Wait output holds batches
- **THEN** the executor releases those batches without invoking further handlers and closes each owned lifecycle once

### Requirement: Zero-allocation built-in steady state
After a successful Start, the built-in single-worker data path SHALL allocate zero framework heap objects per operation for source-to-sink delivery, one-input dispatch, multi-input selector dispatch, typed output acquire/commit, input release, local queue push/pop, Drop release, and Wait retry. The guarantee SHALL exclude startup, shutdown, factory calls, user handlers and selectors, and custom queues. The repository SHALL enforce the built-in paths with `testing.AllocsPerRun` and retain benchmarks that report allocations after setup.

#### Scenario: Measure steady-state local delivery
- **WHEN** allocation tests and benchmarks run a started built-in local graph after setup
- **THEN** the covered framework path reports zero allocations per operation
