## MODIFIED Requirements

### Requirement: Runtime-neutral reusable definition
The immutable definition SHALL retain copied node-worker counts, stable physical worker identities, per-worker CPUs, and queue plans alongside its existing logical topology. Build SHALL reject incomplete/duplicate CPU bindings and group-replica overlap without invoking factories. It SHALL contain no per-worker factory results, queues, pools, workers, or affinity outcomes. Each successful sequential Start SHALL create one fresh explicit state or closure environment and lifecycle per physical worker, fresh typed queues/pools, and one locked worker/affinity outcome per assigned CPU. Existing logical counts and topological-order accessors remain logical and independent of execution resources.

#### Scenario: Reuse a node-pool definition sequentially
- **WHEN** a stopped node-pool executor starts again from the same definition
- **THEN** each physical worker receives fresh state/resources and logical topology/worker plan storage remains unchanged

#### Scenario: Keep error factories and pools dormant during Build
- **WHEN** a valid node-pool graph contains a configured local typed error queue factory
- **THEN** its factory remains uncalled and the definition stores no pool, queue operation, batch, or error value

#### Scenario: Keep factories and CPU plan dormant during Build
- **WHEN** registered nodes and ordinary/error edges contain observable factories and lifecycle callbacks with complete physical CPU bindings
- **THEN** callbacks remain uncalled and the definition stores no factory result, live queue operation, pool, worker, thread, adapter, or affinity outcome

#### Scenario: Reuse a CPU-bound multi-group definition sequentially
- **WHEN** a fixed-plan or node-pool executor stops and the definition starts again
- **THEN** every worker receives fresh resources and affinity outcomes without mutation of logical topology or CPU bindings

#### Scenario: Reuse a definition with error routing sequentially
- **WHEN** an executor with local or remote typed error routes restarts
- **THEN** it creates fresh error pools and queue operations while definition topology remains unchanged

#### Scenario: Inspect during executor lifetime
- **WHEN** callers inspect definition accessors during pooled execution
- **THEN** they observe only copied logical structure, not workers, queues, state, affinity adapters, or mutable lifecycle data

#### Scenario: Inspect logical topology with physical expansion
- **WHEN** a caller reads topology accessors on a replicated definition
- **THEN** logical counts/order remain unchanged, returned storage is independent, and physical expansion remains inaccessible except through copied read-only execution-plan/affinity/metrics identities
