## Purpose
Define explicit statically typed error routing and failed-result ownership semantics.

## Requirements

### Requirement: Explicit statically typed failure route
The graph package SHALL expose opaque `ErrorPort[E]` handles and `func ConnectError[E any](builder *Builder, output ErrorPort[E], input InPort[E], config EdgeConfig[E]) (EdgeID, error)`. `ErrorPort[E]` SHALL expose value-receiver `NodeID() NodeID`, `PortID() PortID`, and `Valid() bool` accessors, retain private provenance, and identify only the registered error output at `PortID(9)`. Ordinary `Connect` SHALL accept only `OutPort[T]`, not `ErrorPort[T]`. A `ConnectError` call with mismatched error and input element types SHALL be rejected by the Go compiler without a runtime type registry, conversion, reflection, or type assertion.

#### Scenario: Connect a matching error route
- **WHEN** a registered node with error type `DomainError` passes its `ErrorPort[DomainError]` and another node's `InPort[DomainError]` to `ConnectError`
- **THEN** the program compiles and the builder records one typed error edge

#### Scenario: Reject a mismatched error route at compile time
- **WHEN** a program passes `ErrorPort[DomainError]` and `InPort[OtherError]` to `ConnectError`
- **THEN** compilation fails before graph validation or runtime construction

### Requirement: Failed result publication and ownership
After a handler returns `node.Result[E]{Failed: true}`, the executor SHALL discard every acquired or committed ordinary output, release every un-moved input, and never publish an ordinary output. If its `ErrorPort[E]` is unconnected, it SHALL discard `Result.Value` without allocating a batch or retaining an error resource. If connected, it SHALL acquire one `batch.Batch[E]` from that error edge's source pool, write `Result.Value` at index zero, set its logical length to one, and publish it to the typed edge.

An exhausted error source pool SHALL discard the failure value without retaining a pending result. A successfully acquired error batch SHALL transfer ownership to the queue on push, SHALL be released once on Drop, and SHALL remain the runner's single pending output on Wait until it publishes. Queue draining, startup rollback, and Close SHALL release a queued or pending error batch to its originating pool. The consuming handler SHALL receive the one-element batch through its ordinary typed input and release it unless it moves it to a compatible ordinary output.

#### Scenario: Drop an unconnected failure
- **WHEN** a handler returns a failed result and its error port has no error edge
- **THEN** ordinary outputs and un-moved inputs are released, no error pool or queue is touched, and the failure value is not retained

#### Scenario: Deliver one connected typed error
- **WHEN** a handler returns `Result[DomainError]{Value: err, Failed: true}` and its matching error edge accepts work
- **THEN** the error consumer receives one `Batch[DomainError]` of length one containing `err`, while no ordinary output from that invocation is delivered

#### Scenario: Retain a full Wait error edge
- **WHEN** a failed result's connected error queue is full under `queue.Wait`
- **THEN** the runner retains exactly one owned `Batch[E]`, does not invoke its handler again until publication succeeds, and allows other runnable workers or runners to progress

### Requirement: Error routing remains bounded and policy-free beyond transport
Each node SHALL have at most one error-edge consumer, and each target input SHALL have at most one producer across ordinary and error edges. Error routing SHALL use only the configured `EdgeConfig[E]` capacity, batch capacity, Drop or Wait policy, and eligible queue factory. The framework SHALL add no error mapper, logger, event log, metric, retry, panic recovery, fan-out, distribution, multiple-consumer error port, ingress, auto grouping, replica, or sharding behavior.

#### Scenario: Reject a second error consumer
- **WHEN** a caller connects one `ErrorPort[E]` successfully and attempts to connect it to another input
- **THEN** the second call is rejected as unsupported fan-out and leaves the second input unoccupied
