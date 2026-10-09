## MODIFIED Requirements

### Requirement: Fresh factory result per replica and start
Each successful explicit StateFactory invocation and ClosureFactory invocation SHALL produce state/environment owned by exactly one physical node replica for one Start. Mutable state returned by separate invocations SHALL be independent unless the application deliberately shares captured mutable references, which remains caller-owned and unsupported for concurrent mutation without application synchronization. Build SHALL invoke no factories. Start SHALL invoke each factory once per physical instance, and each successfully created lifecycle SHALL initialize and close at most once on that instance's locked owner worker; partial startup failure SHALL close every created lifecycle exactly once.

#### Scenario: Observe independent mutable state
- **WHEN** a logical node is expanded to multiple group replicas and each factory creates a mutable map
- **THEN** each physical handler observes and mutates only its own map unless application code deliberately shares a captured map

#### Scenario: Start multiple replicas
- **WHEN** one node is started with multiple logical replicas
- **THEN** the runtime calls its factory once per replica and each successful result has independent mutable state

#### Scenario: Restart a node
- **WHEN** a previously stopped node starts again with the same declaration
- **THEN** the runtime calls its factory again and does not reuse mutable state from the prior start
