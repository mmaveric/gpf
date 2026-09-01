## MODIFIED Requirements

### Requirement: Statically typed port connections
The graph contract SHALL expose `func Connect[T any](builder *Builder, output OutPort[T], input InPort[T], config EdgeConfig[T]) (EdgeID, error)` between ordinary typed ports owned by an open `Builder`. `EdgeConfig[T]` SHALL expose `Capacity uint32`, `Policy queue.Policy`, and `Factory queue.Factory[T]`, where `queue.Factory[T]` has the exact signature `func(queue.Config) (queue.QueueOps[T], error)`. `Connect` SHALL mutate the builder only for a valid connection and return a dense stable `EdgeID`. A program connecting matching port element types SHALL compile without a runtime type registry, and an incompatible connection SHALL be rejected by the Go compiler. Valid port handles SHALL be created by typed node registration; zero-value and cross-builder handles SHALL be rejected at runtime without topology mutation or edge-ID consumption. Registered typed error outputs SHALL remain reserved but `Connect` SHALL reject them with `ErrErrorRoutingUnavailable` until error routing is implemented.

#### Scenario: Connect matching heterogeneous graph ports
- **WHEN** a program registers and connects matching `int` ports and matching `string` ports in one open builder
- **THEN** the program compiles and the builder records both typed edges with dense stable edge IDs

#### Scenario: Reject incompatible port types at compile time
- **WHEN** a program attempts to pass `OutPort[int]` and `InPort[string]` to one `Connect[T]` call
- **THEN** compilation fails before graph connection validation

#### Scenario: Reject a compile-compatible error output at runtime
- **WHEN** a registered typed error `OutPort[E]` and ordinary `InPort[E]` satisfy the generic `Connect[E]` signature
- **THEN** `Connect` returns `ErrErrorRoutingUnavailable` and records no edge
