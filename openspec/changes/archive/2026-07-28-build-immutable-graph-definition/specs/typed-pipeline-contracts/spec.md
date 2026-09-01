## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
The queue and runtime contracts SHALL document typed `QueueOps[T]` operations, bounded queue behavior, `Wait` busy-spin and `Drop` release policies, and future CPU placement and affinity responsibilities. Successful Build SHALL make an opaque structurally immutable graph definition available for sequential reuse, with the builder's persistent non-zero owner identity and dormant `Stateful.Factory`, `ClosureFactory`, `queue.Factory`, and available lifecycle callback function values. Build SHALL NOT invoke those functions or retain factory-produced state, a `ClosureReplica`, live `QueueOps`, a queue implementation, or lifecycle status. Runtime startup, queue execution, handler execution, scheduling, CPU placement, affinity, and concurrent definition reuse SHALL remain unavailable in this stage.

#### Scenario: Review staged execution behavior
- **WHEN** a developer builds a valid graph and reviews graph, queue, and runtime documentation
- **THEN** the reusable immutable definition and dormant factory contracts are documented while runtime startup, live queue execution, handler execution, scheduling, CPU-affinity behavior, and concurrent starts are not presented as executable

#### Scenario: Build without runtime resource creation
- **WHEN** a valid graph with observable state, closure, and queue factories plus explicit lifecycle callbacks is built
- **THEN** Build returns immutable topology with every callback invocation count still zero and stores no factory-produced state or live queue operations
