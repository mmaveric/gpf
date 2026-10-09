## Context
See proposal.md. Automatic previews carry stable physical groups and a topology revision but deliberately lack executable CPU assignments.
## Goals / Non-Goals
Goals: freeze an explicitly CPU-complete automatic definition. Non-goals: CPU discovery, NUMA policy, or rebinding a sealed definition.
## Decisions
- Bind CPUs against the preview's physical IDs, owner, and revision. Reject missing/duplicate/negative assignments and foreign/stale plans before sealing.
- Final Build consumes the same verified preview, freezes assignments and typed bindings, and records dormant factories only. Every physical worker starts locked and attempts its assigned CPU with existing fallback reporting.
- Replanning after topology edits requires reassignment; do not silently carry CPUs onto changed physical IDs. CPU selection remains caller-owned.
- Preserve manual BindCPU semantics and expose copied per-physical-worker affinity rows, including replica identity.
## Risks / Trade-offs
- Stale preview keeps apparently valid CPU IDs → Validate revision at both binding and finalization.
- Platform rejects explicit CPU → Keep existing locked-thread fallback rather than treat Build as OS discovery.
## Migration Plan
Requires automatic preview. Enable automatic Build/Start only after CPU completeness; reconcile immutable-definition and affinity availability contracts at implementation.
