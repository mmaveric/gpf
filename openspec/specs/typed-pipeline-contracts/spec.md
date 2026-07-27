## ADDED Requirements

### Requirement: Go module and public contract packages
The repository SHALL define a Go module requiring Go 1.22 or newer and SHALL provide compileable public contract packages for batch handles and pools, typed graph ports and connections, node result types, typed queue operations, and runtime configuration. The declarations SHALL not use reflection, runtime type assertions, or `interface{}` to represent future steady-state data-plane values.

#### Scenario: Compile public contracts
- **WHEN** the module is tested with Go 1.22 or newer
- **THEN** all public contract packages compile without requiring an executable pipeline runtime

### Requirement: Statically typed port connections
The graph contract SHALL expose `Connect[T]` between an `OutPort[T]` and an `InPort[T]`. A program connecting matching port element types SHALL compile without a runtime type registry, and an incompatible connection SHALL be rejected by the Go compiler.

#### Scenario: Connect matching heterogeneous graph ports
- **WHEN** a program connects an `OutPort[int]` to an `InPort[int]` and an `OutPort[string]` to an `InPort[string]`
- **THEN** the program compiles using the typed `Connect` contract

### Requirement: Batch ownership contract
The batch contract SHALL document that a handler receives ownership of its input batch, MAY move it only to a compatible output, and SHALL have an un-moved input returned to its source pool by the future runtime. The contract SHALL document that pool exhaustion drops the requested batch and SHALL NOT allocate a replacement batch during steady-state operation.

#### Scenario: Review ownership semantics
- **WHEN** a developer reads the batch and pool API documentation
- **THEN** the ownership transfer, compatible move, automatic release, and exhaustion behavior are explicitly defined

### Requirement: Node and error result contract
The node contract SHALL define fixed-arity extension points for source-like zero-input nodes, one through eight input nodes, and zero through eight ordinary outputs. It SHALL define `Result[E]` as the typed handler outcome and SHALL reserve a typed error port outside the ordinary eight-output limit.

#### Scenario: Declare typed handler outcome
- **WHEN** a developer declares a handler with an error type `E`
- **THEN** the handler can return the generic `Result[E]` contract without boxing the error value

### Requirement: Queue and execution semantics contract
The queue and runtime contracts SHALL document typed `QueueOps[T]` operations, bounded queue behavior, `Wait` busy-spin and `Drop` release policies, immutable definitions after build, and fresh runtime creation for every restart. They SHALL identify CPU placement and affinity as later executable responsibilities rather than claiming support in this change.

#### Scenario: Review deferred execution behavior
- **WHEN** a developer reviews the queue and runtime package documentation
- **THEN** the future queue and lifecycle semantics are defined and unimplemented CPU-affinity behavior is not presented as available
