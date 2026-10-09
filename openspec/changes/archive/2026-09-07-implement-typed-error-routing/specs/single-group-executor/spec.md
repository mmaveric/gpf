## MODIFIED Requirements

### Requirement: Execute generated typed handlers on fixed-plan workers
The executor SHALL use generated monomorphized bindings for all registered explicit-state and closure node arities zero through eight, including each registered concrete `E` error finalizer. Each group worker SHALL schedule only its planned runners in canonical node order. A zero-input node SHALL be eligible once per scheduler pass while sources are enabled. A one-input node SHALL invoke its only handler when its input has a batch. A multi-input node SHALL form a `node.ReadyMask`, call its selector only when two or more input positions are ready, and invoke exactly the typed handler selected by an in-range ready `node.PortIndex`. An out-of-range or not-ready selector result SHALL panic; the executor SHALL NOT recover it.

The scheduler SHALL use only preconstructed monomorphized function pointers and numeric metadata in its steady-state data path. It SHALL NOT use reflection, runtime type assertions, `any` or `interface{}` data-plane storage, mutexes, blocking data-plane channels, worker replicas, or a queue concurrency model other than local single-worker or remote SPSC.

#### Scenario: Dispatch a selected heterogeneous input
- **WHEN** two differently typed input edges are ready for one multi-input node and its selector chooses the second ready position
- **THEN** only that position's generated typed handler receives its batch and the first edge remains ready for a later pass

#### Scenario: Run a source and downstream sink across groups
- **WHEN** a zero-input source commits a batch to an ordinary edge whose sink is in another planned group
- **THEN** the assigned workers invoke the source and sink handlers through generated bindings and transfer the batch without a runtime data type assertion

#### Scenario: Dispatch a typed error consumer
- **WHEN** a connected error edge delivers `Batch[E]` to a one-input error consumer
- **THEN** its generated typed input handler receives that batch without reflection, a runtime type assertion, or a boxed error value

### Requirement: Publish results and clean up batch ownership
After each handler returns a successful `node.Result`, the executor SHALL offer every committed ordinary output to its single connected edge and SHALL release every input batch not moved through `node.Output.Commit`. When a handler returns `Result{Failed: true}`, the executor SHALL release every un-moved input and every acquired or committed ordinary output, publish none of those outputs, and process the typed result value only through the node's optional connected `ErrorPort[E]`. An unconnected error port SHALL discard the result value. A connected error port SHALL create, publish, retry, drop, and release a one-element typed error batch according to that edge's pool and queue policy. `Result.Value` SHALL remain unboxed throughout this boundary. A sink SHALL release every input not moved by its handler.

#### Scenario: Discard a failed result
- **WHEN** a handler commits an ordinary output and then returns a failed Result whose error port is unconnected
- **THEN** the executor releases the committed output and un-moved input, does not invoke the downstream ordinary handler, and discards the typed result value

#### Scenario: Preserve a moved input
- **WHEN** a handler commits a compatible input batch to an ordinary output before returning a successful result
- **THEN** Commit clears the handler's input handle and the executor does not release that moved batch at the input boundary

#### Scenario: Discard an unconnected failed result
- **WHEN** a handler commits an ordinary output and then returns a failed Result whose error port is unconnected
- **THEN** the executor releases the committed output and un-moved input, does not invoke a downstream ordinary handler, and does not allocate or publish an error batch

#### Scenario: Publish a connected failed result
- **WHEN** a handler returns a failed Result with a connected matching error edge
- **THEN** the executor publishes exactly one one-element `Batch[E]` or applies that edge's explicit pool-exhaustion, Drop, or Wait behavior

### Requirement: Zero-allocation built-in steady state
After a successful Start, the built-in fixed-plan data path SHALL allocate zero framework heap objects per operation for source-to-sink delivery, one-input dispatch, multi-input selector dispatch, typed ordinary or error output publication, input release, local queue push/pop, remote SPSC push/pop, Drop release, and Wait retry. The guarantee SHALL exclude startup, shutdown, factory calls, user handlers and selectors, and custom local queues. The repository SHALL enforce the built-in paths with `testing.AllocsPerRun` and retain benchmarks that report allocations after setup.

#### Scenario: Measure steady-state local and remote delivery
- **WHEN** allocation tests and benchmarks run started built-in default-group and fixed multi-group graphs after setup
- **THEN** the covered framework paths report zero allocations per operation

#### Scenario: Measure steady-state error delivery
- **WHEN** allocation tests and benchmarks run a started built-in graph that routes a failed result to a local or fixed multi-group error consumer after setup
- **THEN** the covered framework path reports zero allocations per operation
