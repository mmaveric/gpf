## Context
See proposal.md. Ordinary ports currently move whole batches to one consumer; sharding needs a typed transport adapter with branch-local pooled storage.
## Goals / Non-Goals
Goals: fixed branch selection by element. Non-goals: dynamic resharding, error fan-out, or restoring global order.
## Decisions
- Add an explicit typed distribution connection with a fixed destination list and func(T) uint32 selector. Validate distinct compatible destinations and nonzero capacities atomically; ordinary Connect remains 1:1.
- Preallocate one staging batch per branch and retain at most one source batch plus a cursor. Flush full branch batches incrementally and flush partials at input completion. Cache the selected branch when waiting so the selector runs once per element.
- Queue Wait retains a full branch batch and pauses this adapter without blocking downstream workers. Queue Drop releases that branch batch; pool exhaustion drops the affected element with a distinct element-loss reason. Invalid branch indices are user contract violations, not silent modulo remapping.
- Preserve typed source ownership until distribution completes or Close discards it; update branch element/drop metrics without pretending a missing acquisition is a dropped batch.
## Risks / Trade-offs
- Repeated selector calls during Wait alter routing → Retain cursor/branch choice until transfer completes.
- Staging grows with input size → Bound storage by branch count and fixed capacities, not number of input elements.
## Migration Plan
Requires ownership and metrics. Add additive distribution topology and reconcile ordinary fan-out deferrals when applying.
