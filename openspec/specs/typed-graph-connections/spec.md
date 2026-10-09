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
The graph contract SHALL declare `type EdgeConfig[T any] struct` with exactly `Capacity uint32`, `BatchCapacity uint32`, `Policy queue.Policy`, and `Factory queue.Factory[T]` fields. One package-private pure function with the exact signature `func validateEdgeConfig(queue.Config) error` SHALL accept only positive capacity and batch capacity and the declared `queue.Drop` and `queue.Wait` policy values. `Connect[T]` and `ConnectError[E]` SHALL convert their configuration to `queue.Config` and use this validator before retaining configuration. Graph-wide validation SHALL reuse the same validator against stored ordinary and error edge metadata rather than duplicating these rules. The validator and both connection functions SHALL NOT invoke a custom factory.

#### Scenario: Accept valid default queue configuration
- **WHEN** matching ordinary ports are connected with positive queue and batch capacities, a declared policy, and no custom factory
- **THEN** the connection succeeds with configuration for a future built-in typed queue

#### Scenario: Reject invalid edge configuration
- **WHEN** an ordinary connection uses zero capacity, zero batch capacity, or a policy value other than `queue.Drop` or `queue.Wait`
- **THEN** the shared validator returns `*EdgeConfigError` matching `ErrInvalidEdgeConfig`, and the connection records no edge, occupies neither port, invokes no factory, and consumes no edge ID

#### Scenario: Keep custom factory dormant
- **WHEN** matching ordinary ports are connected with a valid configuration containing a custom factory
- **THEN** `Connect` records the typed future binding without invoking the factory or retaining live `QueueOps[T]`

#### Scenario: Reuse configuration validation later
- **WHEN** graph-wide validation checks stored ordinary or error edge capacity, batch capacity, and policy
- **THEN** it calls the same pure `validateEdgeConfig` function used by both connection APIs and does not invoke the stored factory

#### Scenario: Accept a valid error edge configuration
- **WHEN** matching error and input ports are connected with positive queue and batch capacities, a declared policy, and no custom factory
- **THEN** the connection succeeds with configuration for a future built-in typed error queue

#### Scenario: Reject invalid edge configuration from either API
- **WHEN** an ordinary or error connection uses zero capacity, zero batch capacity, or a policy value other than `queue.Drop` or `queue.Wait`
- **THEN** the shared validator returns `*EdgeConfigError` matching `ErrInvalidEdgeConfig`, and the connection records no edge, occupies neither port, invokes no factory, and consumes no edge ID

### Requirement: Builder-targeted dense edge creation
The graph contract SHALL declare the exact APIs `func Connect[T any](builder *Builder, output OutPort[T], input InPort[T], config EdgeConfig[T]) (EdgeID, error)` and `func ConnectError[E any](builder *Builder, output ErrorPort[E], input InPort[E], config EdgeConfig[E]) (EdgeID, error)`. Each SHALL mutate only an open builder. `EdgeID(0)` SHALL be invalid, the first successful connection of either kind SHALL return ID 1, and subsequent successful connections SHALL receive consecutive IDs in commit order.

#### Scenario: Record a matching typed edge
- **WHEN** valid matching registered ordinary ports owned by one open builder are connected with valid configuration
- **THEN** the builder records exactly one edge and `Connect` returns the next dense nonzero edge ID

#### Scenario: Preserve density across an error
- **WHEN** one successful ordinary connection is followed by a rejected connection of either kind and then another successful connection
- **THEN** the successful calls return consecutive edge IDs and the rejected call returns the invalid zero ID

#### Scenario: Reject a sealed builder
- **WHEN** `Connect` or `ConnectError` targets a builder in the sealed state established by a successful Build
- **THEN** it returns `ErrBuilderSealed` and the invalid zero edge ID without changing topology

#### Scenario: Record ordinary and error edges together
- **WHEN** valid matching registered ordinary ports and valid matching registered error and input ports are connected in commit order
- **THEN** the builder records each edge and returns one dense shared nonzero EdgeID sequence

#### Scenario: Preserve density across an error-route failure
- **WHEN** one successful connection is followed by a rejected `ConnectError` call and then another successful connection
- **THEN** the successful calls return consecutive edge IDs and the rejected call returns the invalid zero ID

### Requirement: Port provenance and cardinality validation
`Connect` SHALL reject zero-value ordinary input or output handles and handles not owned by the target builder. `ConnectError` SHALL reject zero-value error or input handles and handles not owned by the target builder. Each input SHALL have at most one producer across ordinary and error edges. Each ordinary output and each error output SHALL have at most one consumer. A second consumer of either kind of output SHALL be rejected explicitly as unsupported fan-out rather than assigned implicit broadcast or distribution semantics.

#### Scenario: Reject a zero-value handle
- **WHEN** either endpoint passed to `Connect` or `ConnectError` is a zero-value typed port handle
- **THEN** the connection returns `ErrInvalidPort` and leaves builder topology and edge IDs unchanged

#### Scenario: Reject a foreign handle
- **WHEN** either endpoint passed to `Connect` or `ConnectError` belongs to a builder other than the target builder
- **THEN** the connection returns `ErrForeignPort` and leaves both builders unchanged

#### Scenario: Reject a second producer for one input
- **WHEN** a caller connects an ordinary or error output to an input that already has a producer
- **THEN** the connection returns `ErrInputConnected` and preserves the existing edge

#### Scenario: Reject unsupported fan-out
- **WHEN** a caller connects an ordinary or error output that already has a consumer to another input
- **THEN** the connection returns `ErrFanOutUnsupported` and preserves the existing edge without occupying the other input

#### Scenario: Reject a foreign error handle
- **WHEN** `ConnectError` receives an error output or input from a builder other than the target builder
- **THEN** it returns `ErrForeignPort` and leaves both builders unchanged

#### Scenario: Reject an ordinary producer for an error-fed input
- **WHEN** an input already has a successful error-edge producer and a caller attempts ordinary `Connect` to it
- **THEN** Connect returns `ErrInputConnected` and preserves the existing error edge

#### Scenario: Reject unsupported error fan-out
- **WHEN** a caller connects an error output that already has a consumer to another input
- **THEN** `ConnectError` returns `ErrFanOutUnsupported` and preserves the first edge without occupying the other input

### Requirement: Error-edge validation and dormant typed binding
`ConnectError` SHALL complete builder state, error-handle validity and provenance, input validity and provenance, input/output cardinality, and the shared pure edge-configuration validation before committing state. It SHALL then atomically append error-edge structural metadata, occupy its input and error output, and retain a monomorphized future-binding closure for `E`. A failure SHALL perform none of these actions, SHALL not consume an edge ID, and SHALL not retain or invoke the rejected factory. The binding SHALL preserve typed error pool and queue operations without reflection, runtime type assertions, or data-plane values stored in `any`.

#### Scenario: Keep an error queue factory dormant
- **WHEN** matching error and input ports are connected with a valid local custom `Factory[E]`
- **THEN** ConnectError records the future typed binding without invoking the factory or retaining live `QueueOps[E]`

#### Scenario: Reject invalid error configuration atomically
- **WHEN** ConnectError receives zero capacity, zero batch capacity, or an unsupported policy after all higher-precedence checks pass
- **THEN** it returns an `EdgeConfigError`, records no edge, occupies no port, invokes no factory, and consumes no edge ID

### Requirement: Stable connection errors and deterministic precedence
The graph package SHALL export `ErrBuilderSealed`, `ErrInvalidPort`, `ErrForeignPort`, `ErrInputConnected`, `ErrFanOutUnsupported`, and `ErrInvalidEdgeConfig` as stable sentinel errors. It SHALL export `type EdgeConfigError struct { Config queue.Config }` with `Error() string` and `Unwrap() error` methods, and `EdgeConfigError.Unwrap` SHALL return `ErrInvalidEdgeConfig`. Callers SHALL be able to match every category with `errors.Is` and inspect configuration failures with `errors.As`.

`Connect` SHALL select one error using this exact precedence: sealed builder; invalid ordinary output then invalid input; foreign ordinary output then foreign input; connected input; connected output as unsupported fan-out; invalid edge configuration. `ConnectError` SHALL use the equivalent precedence with invalid and foreign error output checks in the ordinary-output positions. Every failure SHALL return `EdgeID(0)`.

#### Scenario: Match an invalid configuration error
- **WHEN** `Connect` rejects capacity, batch capacity, or policy after all higher-precedence checks pass
- **THEN** `errors.Is` matches `ErrInvalidEdgeConfig` and `errors.As` yields `*EdgeConfigError` containing the rejected `queue.Config`

#### Scenario: Select the higher-precedence error
- **WHEN** one ordinary connection attempt violates more than one local invariant
- **THEN** `Connect` returns the sentinel or typed error for the first failing check in the specified precedence

#### Scenario: Observe sealed state without performing sealing
- **WHEN** a successful Build has transitioned a builder to sealed and a caller invokes `Connect` or `ConnectError`
- **THEN** the connection returns `ErrBuilderSealed`, while this capability itself defines no Build operation and performs no sealing transition

#### Scenario: Match an invalid error-edge configuration
- **WHEN** ConnectError rejects capacity, batch capacity, or policy after all higher-precedence checks pass
- **THEN** `errors.Is` matches `ErrInvalidEdgeConfig` and `errors.As` yields `*EdgeConfigError` containing the rejected `queue.Config`

#### Scenario: Select the higher-precedence error for an error route
- **WHEN** one ConnectError attempt violates more than one local invariant
- **THEN** it returns the sentinel or typed error for the first failing check in the specified precedence

### Requirement: Atomic connection errors and typed future binding
`Connect` SHALL complete every local validation, including the shared pure edge-configuration validator, before committing connection state. On success it SHALL atomically append structural edge metadata, occupy the input and output, and retain a monomorphized future-binding closure for `T`. On error it SHALL perform none of those actions and SHALL NOT retain or invoke the rejected factory or consume an edge ID. The future-binding closure SHALL remain dormant during connection and future graph Build validation and SHALL preserve typed queue operations without reflection, runtime type assertions, or storage of data-plane values in an `any` registry.

#### Scenario: Correct a rejected connection
- **WHEN** a connection attempt fails and the caller retries the same still-unoccupied ports with valid configuration
- **THEN** the retry succeeds with the next dense edge ID

#### Scenario: Preserve typed binding metadata
- **WHEN** `Connect[T]` records an edge with a custom `queue.Factory[T]`
- **THEN** the stored future binding remains specialized for `T` and no queue instance is created during connection
