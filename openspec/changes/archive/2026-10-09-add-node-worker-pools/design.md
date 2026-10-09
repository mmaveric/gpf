## Context
See proposal.md. Group replication scales whole subgraphs; this change expands a single logical node's workers.
## Goals / Non-Goals
Goals: fixed node pools with typed dispatch. Non-goals: dynamic resizing or nesting a node pool inside a replicated group.
## Decisions
- Configure a positive worker count and explicit CPU per worker before Build. A pooled node creates dedicated physical worker groups; surrounding manually placed nodes remain unchanged.
- Each logical input has a shared queue with one consumer operation per pooled worker; outputs share queues according to resolved producer/consumer counts. Preserve per-input types and worker-local ReadyMask selectors.
- Factories run once per worker, and callbacks execute only on that worker. Assign (NodeID, worker index) identities to metrics and lifecycle records.
- Distribution is work-conserving at batch granularity, not round-robin guaranteed. Source-node pools run independent sources; no input ordering or duplication guarantee beyond accepted-batch ownership.
## Risks / Trade-offs
- Hot shared queues contend → Benchmark MPMC boundaries and retain fixed capacity.
- Ambiguous overlapping parallelism → Reject group-replica/node-pool overlap rather than create an implicit product of workers.
## Migration Plan
Requires queue selection, ownership, and metrics. Verify count-one compatibility and update current parallelism deferrals at implementation.
