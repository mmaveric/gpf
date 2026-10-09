## Purpose
Define typed node state and closure factory contracts, including replica lifecycle and factory dormancy requirements.

## Requirements

### Requirement: Generated handler contracts are a prerequisite
The node factory contracts SHALL depend on the explicit-state and closure handler-set families supplied by `generate-typed-node-arities`. `StateFactory[S]`, `InitFunc[S]`, `CloseFunc[S]`, `Stateful[S, H]`, `ClosureReplica[H]`, and `ClosureFactory[H]` SHALL compose with those generated types without adding graph builders, ports, registration, connections, or topology behavior.

#### Scenario: Compose factories with generated handlers
- **WHEN** a developer instantiates explicit-state and closure factory declarations with generated handler sets
- **THEN** the declarations compile with the exact generated handler types as `H` and require no graph API

### Requirement: Exact explicit state factory declarations
The `node` package SHALL expose `StateFactory[S]` with underlying function signature `func() (S, error)`, `InitFunc[S]` with underlying function signature `func(*S) error`, and `CloseFunc[S]` with underlying function signature `func(*S) error`. It SHALL expose `Stateful[S, H]` as a struct with `Factory StateFactory[S]`, `Handlers H`, `Init InitFunc[S]`, and `Close CloseFunc[S]` fields. `Factory` is required while `Init` and `Close` are optional. A future runtime SHALL store each successfully returned `S` as one replica's state and SHALL pass its address only to that replica's explicit handlers and lifecycle hooks.

#### Scenario: Declare explicit replica state
- **WHEN** a developer constructs `Stateful[S, H]` with a state factory and a generated explicit-state handler set
- **THEN** the factory result, handlers, and optional lifecycle hooks remain statically typed to `S` and `H`

#### Scenario: Assign exact explicit callback types
- **WHEN** a developer assigns functions matching `func() (S, error)` and `func(*S) error` to the named explicit callback types
- **THEN** those values populate the corresponding `Stateful[S, H]` fields without adapters or runtime type assertions

#### Scenario: Omit explicit lifecycle hooks
- **WHEN** `Init` and `Close` are nil on a valid `Stateful[S, H]` declaration
- **THEN** the declaration remains valid and startup and shutdown require no lifecycle callback invocation

### Requirement: Exact closure replica factory declarations
The `node` package SHALL expose `ClosureReplica[H]` as a struct with `Handlers H`, `Init func() error`, and `Close func() error` fields. It SHALL expose `ClosureFactory[H]` with underlying function signature `func() (ClosureReplica[H], error)` so one successful call produces a coherent handler set and lifecycle callbacks sharing only that replica's closure environment. `Init` and `Close` are optional.

#### Scenario: Create a closure-bound replica
- **WHEN** a `ClosureFactory[H]` successfully creates handlers and lifecycle callbacks that capture one mutable environment
- **THEN** all returned functions belong to the same typed `ClosureReplica[H]`

#### Scenario: Omit closure lifecycle callbacks
- **WHEN** a closure factory returns a replica with nil `Init` and `Close`
- **THEN** the replica remains valid and startup and shutdown require no lifecycle callback invocation

### Requirement: Fresh factory result per replica and start
Each successful explicit or closure factory invocation SHALL create state owned by exactly one physical node instance, including each fixed node-worker-pool worker and each whole-group replica. Build SHALL remain dormant. Start SHALL invoke a factory once per physical instance, and successful lifecycle ownership SHALL be initialized/closed exactly once on that instance's locked worker, including rollback. Framework-created mutable state and closure environments SHALL not be shared between workers; deliberately shared user captures remain caller-owned and require user synchronization.

#### Scenario: Mutate separate node worker states
- **WHEN** one logical node starts with three workers whose factories create mutable maps
- **THEN** each worker's handler mutates only the map produced for its own physical instance

#### Scenario: Observe independent mutable state
- **WHEN** a logical node is expanded to multiple group replicas and each factory creates a mutable map
- **THEN** each physical handler observes and mutates only its own map unless application code deliberately shares a captured map

#### Scenario: Start multiple replicas
- **WHEN** one node is started with multiple logical replicas
- **THEN** the runtime calls its factory once per replica and each successful result has independent mutable state

#### Scenario: Restart a node
- **WHEN** a previously stopped node starts again with the same declaration
- **THEN** the runtime calls its factory again and does not reuse mutable state from the prior start

### Requirement: Lifecycle cleanup follows successful construction
A factory result SHALL transfer lifecycle ownership to the future runtime only when the factory returns a nil error. For each successfully returned lifecycle, the runtime SHALL invoke optional `Init` at most once and SHALL invoke a provided `Close` exactly once on normal shutdown or startup rollback. A failed `Init` SHALL count as partial initialization and SHALL cause `Close` for that lifecycle and every other successfully created lifecycle included in the rollback. A failed factory return SHALL NOT cause `Init` or `Close` from that result to be invoked.

#### Scenario: Close after normal operation
- **WHEN** a factory succeeds, optional initialization succeeds, and the replica later stops normally
- **THEN** the runtime invokes its provided `Close` exactly once

#### Scenario: Roll back partial initialization
- **WHEN** a factory succeeds but that replica's `Init` returns an error after partially acquiring resources
- **THEN** startup fails and the runtime invokes the replica's provided `Close` exactly once as part of rollback

#### Scenario: Roll back previously created replicas
- **WHEN** startup fails after other replica factories have already returned successfully
- **THEN** the runtime invokes each provided `Close` exactly once for every successfully created lifecycle included in the rollback

#### Scenario: Factory fails before ownership transfer
- **WHEN** an explicit or closure factory returns a non-nil error
- **THEN** the runtime does not invoke `Init` or `Close` from that failed result

### Requirement: Captured configuration remains immutable
Configuration captured by state factories, closure factories, handlers, or lifecycle callbacks SHALL be treated as immutable after the declaration is handed to the framework and for as long as a graph build or runtime start can use it. Mutable per-replica data SHALL be created by each factory call and SHALL reside only in that call's explicit state result or closure environment. The framework SHALL NOT claim to deep-copy or freeze caller-owned captures.

#### Scenario: Reuse immutable configuration
- **WHEN** multiple replica factories read the same captured immutable configuration
- **THEN** they may share that configuration while placing all runtime mutation in their independent factory results

#### Scenario: Avoid shared mutable captures
- **WHEN** a developer needs mutable counters, buffers, maps, or slices for handler execution
- **THEN** the developer creates them independently inside each factory result rather than mutating captured configuration

### Requirement: Deferred graph integration preserves factory dormancy
The node factory contracts SHALL require the future graph bridge to record static validation facts without invoking factories. Future registration SHALL record whether each required factory is nil and SHALL record statically visible explicit handler facts; for closure declarations it SHALL NOT invoke the closure factory to inspect returned handlers or lifecycle callbacks. Future graph Build SHALL consume those recorded facts without invoking `Stateful.Factory` or `ClosureFactory`. Implementation and executable tests for this bridge belong to the next graph change, not this capability.

#### Scenario: Record explicit static facts in a future bridge
- **WHEN** future graph registration receives a node backed by `Stateful[S, H]`
- **THEN** it records factory presence and statically visible handler facts without calling the state factory

#### Scenario: Record closure factory presence in a future bridge
- **WHEN** future graph registration receives a node backed by `ClosureFactory[H]`
- **THEN** it records whether the factory is nil without calling it to inspect returned handlers or lifecycle callbacks

#### Scenario: Preserve dormancy through future Build
- **WHEN** future graph Build validates the static facts recorded by registration
- **THEN** it reports any missing required factory without invoking any node factory
