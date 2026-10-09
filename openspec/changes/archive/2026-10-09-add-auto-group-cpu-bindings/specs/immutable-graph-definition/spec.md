## MODIFIED Requirements

### Requirement: Runtime-neutral reusable definition
A CPU-complete automatic preview SHALL finalize into a Definition retaining a copied immutable physical plan and explicit CPU binding for each resulting group, while its public logical NodeCount/EdgeCount/TopologicalOrder continue to describe original registration topology. Build on an unfinalized builder SHALL retain the existing manual semantics. Finalization SHALL invoke no factories and retain no live queues, pools, state, executor, worker, thread, affinity adapter, or outcome. Sequential Start from a finalized definition SHALL create fresh resources per physical node/worker and materialize queues from the selected models; stop/join and restart SHALL not mutate the definition.

#### Scenario: Keep factories dormant through finalization
- **WHEN** a complete preview is assigned CPUs and finalized
- **THEN** no state, closure, or queue factory has run and the definition contains only immutable physical plans and dormant bindings

#### Scenario: Reuse a finalized automatic definition
- **WHEN** its executor stops and the same automatic definition starts again
- **THEN** fresh resources and affinity outcomes are created while logical topology and physical plan inspection remain unchanged

#### Scenario: Keep error factories and pools dormant during Build
- **WHEN** a graph contains a typed error factory before automatic finalization
- **THEN** Build invokes no error factory and stores no live pool or queue

#### Scenario: Keep factories and CPU plan dormant during Build
- **WHEN** an automatic preview is built with dormant node factories
- **THEN** no callback or runtime state is created before finalization and Start

#### Scenario: Reuse a CPU-bound multi-group definition sequentially
- **WHEN** a CPU-bound physical definition restarts
- **THEN** it creates fresh resources and affinity results without changing topology or bindings

#### Scenario: Reuse a definition with error routing sequentially
- **WHEN** an automatic definition with typed error edges restarts
- **THEN** error pools and queue operations are fresh

#### Scenario: Inspect during executor lifetime
- **WHEN** a caller inspects a running finalized automatic definition
- **THEN** only copied logical topology and plan data are exposed

#### Scenario: Keep error factories and pools dormant during Build
- **WHEN** a valid definition has a configured typed error factory
- **THEN** Build invokes no factory and retains no live pool or queue operation

#### Scenario: Keep factories and CPU plan dormant during Build
- **WHEN** explicit and closure factories and lifecycle callbacks are registered with complete CPU bindings
- **THEN** Build invokes none and stores no runtime state, workers, threads, adapters, or outcomes

#### Scenario: Reuse a CPU-bound multi-group definition sequentially
- **WHEN** a CPU-bound definition restarts after stop
- **THEN** resources and affinity results are fresh while the stored plan is unchanged

#### Scenario: Reuse a definition with error routing sequentially
- **WHEN** an error-routed definition restarts
- **THEN** local queue resources and error pools are fresh without mutating the definition

#### Scenario: Inspect during executor lifetime
- **WHEN** a caller inspects a running definition
- **THEN** accessors expose only copied logical topology and plan metadata

#### Scenario: Keep factories and resource construction dormant during Build
- **WHEN** a valid graph has observable factories, lifecycle callbacks, queues, and constraints
- **THEN** Build leaves callback counts at zero and stores only immutable plans, not runtime resources

#### Scenario: Reuse fixed and replicated definitions sequentially
- **WHEN** an executor stops and its definition starts again
- **THEN** state, queues, pools, workers, and affinity outcomes are fresh while topology and numeric previews remain unchanged

#### Scenario: Reuse a node-pool definition sequentially
- **WHEN** a stopped node-pool executor starts again from the same definition
- **THEN** each physical worker receives fresh state/resources and logical topology/worker plan storage remains unchanged

#### Scenario: Inspect constraints during executor lifetime
- **WHEN** a caller reads Constraints during execution
- **THEN** only an independent numeric snapshot is returned and no runtime resource is exposed

#### Scenario: Inspect logical topology with physical expansion
- **WHEN** a caller reads topology accessors on a replicated definition
- **THEN** logical counts/order remain unchanged, returned storage is independent, and physical expansion remains inaccessible except through copied read-only execution-plan/affinity/metrics identities
