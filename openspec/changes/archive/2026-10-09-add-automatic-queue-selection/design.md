## Context
See proposal.md. Fixed edges currently have 1:1 physical endpoint cardinality; later expansion will change that while logical port types stay concrete.
## Goals / Non-Goals
Goals: one deterministic cardinality resolver and copied queue-plan metadata. Non-goals: inventing replicas or allowing remote user factories.
## Decisions
- Add a pure resolver for physical producer/consumer counts: 1:1 SPSC, N:1 MPSC, 1:N SPMC, N:M MPMC. Same-owner single-worker paths remain local rings.
- Add dormant typed constructors for selected models to edge bindings. Build consumes numeric metadata only; Start creates runtime storage.
- Expose copied per-edge model/count metadata. Require positive cardinalities and reject unsupported custom factories on multi-worker transport before invocation.
- Test expanded numeric plans through internal fixtures now; group/node expansion changes later provide the actual endpoint instances.
## Risks / Trade-offs
- Selecting before expansion → Resolve only finalized physical cardinalities and assert no hidden producer exists.
- Misclassified same-group replicas → Distinguish logical groups from physical worker ownership.
## Migration Plan
Requires bounded concurrent queues. Keep existing manual 1:1 definitions behavior-compatible; update current queue availability contracts at implementation.
