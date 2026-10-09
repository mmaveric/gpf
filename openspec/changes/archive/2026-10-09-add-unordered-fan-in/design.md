## Context
See proposal.md. Ordinary Connect's producer exclusivity remains valuable; merging must be explicit rather than silently weakening it.
## Goals / Non-Goals
Goals: homogeneous multi-producer merge. Non-goals: sequence reconstruction or merging heterogeneous inputs into boxed values.
## Decisions
- Add typed FanIn with a fixed list of ordinary outputs and one input. Validate all owners, types, cardinalities, capacities, and DAG edges before mutation.
- Materialize one shared consumer queue with separately bound producer outputs/pools. Select queue model from physical endpoint counts; same-worker merging can use a local ring.
- Keep producer pool identity inside each Batch so sink release returns to its origin. Apply per-producer Wait/Drop while shared queue capacity remains bounded.
- Report both logical merge and producer branch edge identity; drain includes every pending producer and the shared queue once.
## Risks / Trade-offs
- Shared queue cleanup registered once per producer → Deduplicate transport ownership at materialization.
- Scheduler bias → Test progress without promising global FIFO or strict per-producer fairness.
## Migration Plan
Requires queue selection, ownership, and metrics. Preserve ordinary Connect errors; reconcile only explicit merge capability availability.
