## Context
See proposal.md. Pure constraint normalization and coalescing are available; public automatic preview still needs stable identities.
## Goals / Non-Goals
Goals: read-only pre-runtime plan preview. Non-goals: CPU discovery or starting an unbound plan.
## Decisions
- Expose opt-in Builder planning that expands physical endpoints, normalizes constraints, coalesces, and selects queues in that order.
- Return copied rows with logical/physical node identity, stable group IDs, edge locality/models, and required CPU slots. Preserve explicit group IDs; generate remaining IDs deterministically without collisions.
- Stamp previews with builder owner identity and topology revision. Registration, placement, or connection invalidates a previous revision.
- Preview does not seal Build or allocate runtime pools/state. Executable auto-mode finalization remains unavailable until CPU bindings are provided by the next change.
## Risks / Trade-offs
- Preview IDs vary with hash-map order → Canonical sorted enumeration and repeat-plan tests.
- Users mistake preview for executable Definition → Distinct plan type and clear CPU-incomplete status.
## Migration Plan
Requires minimization and queue selection. Add preview now, then bind/freeze through add-auto-group-cpu-bindings without changing manual defaults.
