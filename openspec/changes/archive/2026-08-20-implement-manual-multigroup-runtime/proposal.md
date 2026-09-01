## Why

Stage 3 can execute a valid definition on one local worker, but it cannot express a fixed worker boundary or move a typed batch safely between workers. The next narrow step is manual multi-group execution so fixed graph-construction placement can be exercised without prematurely adding automatic planning or parallelism features.

## What Changes

- Add fixed manual node placement on an open graph builder using nonzero `GroupID` values. Nodes without an explicit placement remain in the default group so existing one-group graphs keep their behavior.
- Extend Build to validate placements and compile a deterministic immutable group plan, classifying every edge as local or remote before any runtime resource is created.
- Generalize `Definition.Start` to create exactly one worker goroutine for each planned group while retaining its configuration-free public signature and sequential-reuse contract.
- Keep same-group edges on the existing local queue path. Require every cross-group edge to use a framework-owned bounded typed SPSC ring; reject a configured custom queue factory for a remote edge because the existing factory contract does not establish concurrent memory-ordering guarantees.
- Define deterministic multi-group startup rollback, graceful drain, forced close, factory freshness, and reverse canonical lifecycle closing while preserving typed batch ownership and the one-group observable behavior.
- Add manual-placement, plan-validation, local/remote classification, SPSC ownership/FIFO, lifecycle, allocation, benchmark, and race coverage for the supported two-or-more-group path.
- Explicitly defer automatic grouping, CPU affinity and `LockOSThread`, replicas, MPSC/SPMC/MPMC queues, fan-out, sharding, metrics, logging, error routing, ingress, and concurrent starts.

## Capabilities

### New Capabilities
- `manual-multigroup-runtime`: Fixed graph-construction placement, immutable group planning, one worker per group, built-in remote SPSC queues, and deterministic multi-group lifecycle behavior.

### Modified Capabilities
- `immutable-graph-definition`: Preserve validated manual placements and the compiled group plan in an otherwise immutable reusable definition.
- `typed-graph-connections`: Restrict configured custom queue factories to local edges; Build rejects them when placement makes an edge remote.
- `single-group-executor`: Generalize the existing configuration-free executor startup and lifecycle requirements to a deterministic fixed group plan while preserving one-group semantics.
- `typed-pipeline-contracts`: Document the available manual multi-group and remote-SPSC execution boundary and the deferred concurrency and placement features.
- `pipeline-performance-harness`: Enable remote-SPSC and multi-group end-to-end allocation and benchmark coverage after setup.

## Impact

- Affects `graph` builder metadata, validation, immutable definitions, generated runtime bindings, executor scheduling, and package documentation; the `queue` public surface remains unchanged.
- Adds the minimal graph API for group placement and associated validation errors. `Definition.Start`, `Executor.Drain`, and `Executor.Close` retain their existing signatures.
- Requires synchronization only in the built-in remote SPSC data path; local paths retain their single-worker queue behavior and typed batches never cross an erased data-plane boundary.
