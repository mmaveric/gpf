## Purpose
Define typed graph edge configuration, connection validation, and dormant queue bindings.

## Requirements

### Requirement: Typed per-runtime queue factory
The queue contract SHALL declare the exact API `type Factory[T any] func(Config) (QueueOps[T], error)`. Each future runtime instance SHALL invoke a configured factory separately for each local edge it binds, and a factory invocation SHALL return queue operations for the same element type `T` or an error. A nil local-edge factory SHALL select the future built-in local queue. A cross-group edge classified by Build SHALL use the framework-owned remote SPSC queue and SHALL reject a non-nil configured factory before runtime creation.

#### Scenario: Preserve typed local custom queue construction
- **WHEN** an `EdgeConfig[int]` contains a custom `Factory[int]` and its endpoints have the same effective group
- **THEN** the edge retains a constructor that can produce only `QueueOps[int]` for that future local runtime edge

#### Scenario: Create fresh local operations for each future runtime
- **WHEN** two future runtime instances bind the same local edge with a custom factory
- **THEN** each runtime invokes the factory independently and does not reuse queue operations created for the other runtime

#### Scenario: Reject a custom remote queue
- **WHEN** a configured custom factory edge is classified cross-group by Build
- **THEN** Build rejects the graph without invoking the factory and the future runtime uses no custom remote operations

### Requirement: Valid typed edge configuration
The graph contract SHALL declare `type EdgeConfig[T any] struct` with exactly `Capacity uint32`, `Policy queue.Policy`, and `Factory queue.Factory[T]` fields. One package-private pure function with the exact signature `func validateEdgeConfig(queue.Config) error` SHALL accept only capacities greater than zero and the declared `queue.Drop` and `queue.Wait` policy values. `Connect[T]` SHALL convert capacity and policy to `queue.Config` and use this validator before retaining configuration. The later graph-wide validation capability SHALL reuse the same validator against stored edge metadata rather than duplicating these rules. The validator and `Connect` SHALL NOT invoke the custom factory.

#### Scenario: Accept valid default queue configuration
- **WHEN** matching ports are connected with positive capacity, a declared policy, and no custom factory
- **THEN** the connection succeeds with configuration for a future built-in typed queue

#### Scenario: Reject invalid edge configuration
- **WHEN** a connection uses zero capacity or a policy value other than `queue.Drop` or `queue.Wait`
- **THEN** the shared validator returns `*EdgeConfigError` matching `ErrInvalidEdgeConfig`, and the connection records no edge, occupies neither port, invokes no factory, and consumes no edge ID

#### Scenario: Keep custom factory dormant
- **WHEN** matching ports are connected with a valid configuration containing a custom factory
- **THEN** `Connect` records the typed future binding without invoking the factory or retaining live `QueueOps[T]`

#### Scenario: Reuse configuration validation later
- **WHEN** later graph-wide validation checks stored capacity and policy
- **THEN** it calls the same pure `validateEdgeConfig` function used by `Connect` and does not invoke the stored factory

### Requirement: Builder-targeted dense edge creation
The graph contract SHALL declare the exact API `func Connect[T any](builder *Builder, output OutPort[T], input InPort[T], config EdgeConfig[T]) (EdgeID, error)`. `Connect` SHALL mutate only an open builder. `EdgeID(0)` SHALL be invalid, the first successful connection SHALL return ID 1, and subsequent successful connections SHALL receive consecutive IDs in commit order.

#### Scenario: Record a matching typed edge
- **WHEN** valid matching registered ports owned by one open builder are connected with valid configuration
- **THEN** the builder records exactly one edge and `Connect` returns the next dense nonzero edge ID

#### Scenario: Preserve density across an error
- **WHEN** one successful connection is followed by a rejected connection and then another successful connection
- **THEN** the successful calls return consecutive edge IDs and the rejected call returns the invalid zero ID

#### Scenario: Reject a sealed builder
- **WHEN** `Connect` targets a builder in the sealed state established for a later successful Build
- **THEN** it returns `ErrBuilderSealed` and the invalid zero edge ID without changing topology

### Requirement: Port provenance and cardinality validation
`Connect` SHALL reject zero-value input or output handles and handles not owned by the target builder. Each input SHALL have at most one producer, and each output SHALL have at most one consumer. A second consumer of an output SHALL be rejected explicitly as unsupported fan-out rather than assigned implicit broadcast or distribution semantics.

#### Scenario: Reject a zero-value handle
- **WHEN** either connection endpoint is a zero-value typed port handle
- **THEN** `Connect` returns `ErrInvalidPort` and leaves builder topology and edge IDs unchanged

#### Scenario: Reject a foreign handle
- **WHEN** either connection endpoint belongs to a builder other than the target builder
- **THEN** `Connect` returns `ErrForeignPort` and leaves both builders unchanged

#### Scenario: Reject a second producer for one input
- **WHEN** a caller connects another output to an input that already has a producer
- **THEN** `Connect` returns `ErrInputConnected` and preserves the existing edge

#### Scenario: Reject unsupported fan-out
- **WHEN** a caller connects an output that already has a consumer to another input
- **THEN** `Connect` returns `ErrFanOutUnsupported` and preserves the existing edge without occupying the other input

### Requirement: Reserved typed error outputs
Typed error outputs returned by node registration SHALL remain distinguishable in private port metadata from ordinary `OutPort[T]` handles. `Connect` SHALL reject a valid builder-owned typed error output with `ErrErrorRoutingUnavailable`. It SHALL NOT add an error edge to topology, occupy either endpoint, retain or invoke a queue factory, or consume an edge ID. Error routing SHALL remain unavailable until its dedicated capability defines routing and ownership semantics.

#### Scenario: Reject a typed error output
- **WHEN** a caller attempts to connect a registered node error `OutPort[E]` to a matching ordinary `InPort[E]`
- **THEN** `Connect` returns `ErrErrorRoutingUnavailable` and leaves ordinary topology and edge identities unchanged

#### Scenario: Keep reserved error output unconnected
- **WHEN** one or more typed error outputs are present during this stage
- **THEN** they remain reserved handles and no error edges enter builder topology

### Requirement: Stable connection errors and deterministic precedence
The graph package SHALL export `ErrBuilderSealed`, `ErrInvalidPort`, `ErrForeignPort`, `ErrInputConnected`, `ErrFanOutUnsupported`, `ErrInvalidEdgeConfig`, and `ErrErrorRoutingUnavailable` as stable sentinel errors. It SHALL export `type EdgeConfigError struct { Config queue.Config }` with `Error() string` and `Unwrap() error` methods, and `EdgeConfigError.Unwrap` SHALL return `ErrInvalidEdgeConfig`. Callers SHALL be able to match every category with `errors.Is` and inspect configuration failures with `errors.As`.

`Connect` SHALL select one error using this exact precedence: sealed builder; invalid output then invalid input; foreign output then foreign input; reserved error output; connected input; connected output as unsupported fan-out; invalid edge configuration. Every failure SHALL return `EdgeID(0)`.

#### Scenario: Match an invalid configuration error
- **WHEN** `Connect` rejects capacity or policy after all higher-precedence checks pass
- **THEN** `errors.Is` matches `ErrInvalidEdgeConfig` and `errors.As` yields `*EdgeConfigError` containing the rejected `queue.Config`

#### Scenario: Select the higher-precedence error
- **WHEN** one connection attempt violates more than one local invariant
- **THEN** `Connect` returns the sentinel or typed error for the first failing check in the specified precedence

#### Scenario: Observe sealed state without performing sealing
- **WHEN** a later successful Build has transitioned a builder to sealed and a caller invokes `Connect`
- **THEN** `Connect` returns `ErrBuilderSealed`, while this capability itself defines no Build operation and performs no sealing transition

### Requirement: Atomic connection errors and typed future binding
`Connect` SHALL complete every local validation, including the shared pure edge-configuration validator, before committing connection state. On success it SHALL atomically append structural edge metadata, occupy the input and output, and retain a monomorphized future-binding closure for `T`. On error it SHALL perform none of those actions and SHALL NOT retain or invoke the rejected factory or consume an edge ID. The future-binding closure SHALL remain dormant during connection and future graph Build validation and SHALL preserve typed queue operations without reflection, runtime type assertions, or storage of data-plane values in an `any` registry.

#### Scenario: Correct a rejected connection
- **WHEN** a connection attempt fails and the caller retries the same still-unoccupied ports with valid configuration
- **THEN** the retry succeeds with the next dense edge ID

#### Scenario: Preserve typed binding metadata
- **WHEN** `Connect[T]` records an edge with a custom `queue.Factory[T]`
- **THEN** the stored future binding remains specialized for `T` and no queue instance is created during connection
