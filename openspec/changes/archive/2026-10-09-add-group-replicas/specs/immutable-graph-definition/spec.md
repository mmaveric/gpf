## MODIFIED Requirements

### Requirement: Runtime-neutral reusable definition
Build SHALL retain the logical immutable topology and a private copied physical execution expansion containing stable logical-to-physical node/group/edge identities, replica indices, internal pairing, boundary cardinalities, queue model plans, and explicit unique CPU bindings. Build SHALL reject missing/duplicate/negative replica CPU bindings, replicated ingress adapters, and node-pool overlap before invoking factories. It SHALL store no replica factory results, pools, queues, metrics, workers, or affinity outcomes. Each sequential Start SHALL create fresh state, closure environments, per-producer pools, shared boundary queues, one locked worker per physical group replica, and one affinity outcome per worker. Existing Definition NodeCount, EdgeCount, TopologicalOrder, and edge-plan inspection SHALL remain logical/copy-safe and shall not expose mutable execution resources.

#### Scenario: Inspect logical topology with physical expansion
- **WHEN** a caller reads topology accessors on a replicated definition
- **THEN** logical counts/order remain unchanged, returned storage is independent, and physical expansion remains inaccessible except through copied read-only execution-plan/affinity/metrics identities

#### Scenario: Keep error factories and pools dormant during Build
- **WHEN** a valid replicated graph with a configured local typed error queue factory is built
- **THEN** its factory invocation count remains zero and the definition stores no pool, queue operation, batch, or error value

#### Scenario: Keep factories and CPU plan dormant during Build
- **WHEN** registered nodes and ordinary/error edges contain observable factories and lifecycle callbacks with complete physical CPU bindings
- **THEN** every callback remains uncalled and the definition stores no state, queue operations, pools, worker, thread, adapter, or affinity outcome

#### Scenario: Reuse a CPU-bound multi-group definition sequentially
- **WHEN** a replicated executor stops and the same definition starts again
- **THEN** every physical instance receives fresh resources and affinity outcomes while logical identity and the physical plan remain unchanged

#### Scenario: Reuse a definition with error routing sequentially
- **WHEN** an executor with replicated error routes stops and starts again
- **THEN** it creates fresh error pools and edge queues without changing definition topology or CPU bindings

#### Scenario: Inspect during executor lifetime
- **WHEN** callers inspect topology during replicated execution
- **THEN** accessors expose copied logical structure and read-only plan metadata, not mutable runtime resources
