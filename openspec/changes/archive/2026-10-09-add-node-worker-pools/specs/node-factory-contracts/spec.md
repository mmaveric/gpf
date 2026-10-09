## MODIFIED Requirements

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
