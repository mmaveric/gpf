## Context
See proposal.md and the existing node-factory-contracts capability. Replica state independence is already normative; missing work is executable coverage once both expansion modes exist.
## Goals / Non-Goals
Goals: test explicit and closure isolation plus lifecycle. Non-goals: new public API, deep-copying user captures, or duplicate specification requirements.
## Decisions
- Use maps/slices/pointers created inside factories and independently mutate each physical instance. Assert factory count equals physical node instance count per Start.
- Test reverse cleanup and failures at each construction/Init index, owner-worker affinity identity, and sequential restarts. Use deterministic gates, not sleep-based race assumptions.
- Keep tests in owning packages and run race mode. Deliberately shared mutable user captures remain a documented user error; the framework does not detect or repair them.
## Risks / Trade-offs
- Tests hide shared captures with trivial scalar state → Use nested mutable state and multiple workers.
- Verification starts before expansion exists → Require group replicas and node worker pools complete first.
## Migration Plan
No behavior migration. Add tests and documentation with skip_specs=true; failures reveal prerequisites needing correction before marking tasks complete.
