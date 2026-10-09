## Context
See proposal.md. Exclusive Batch ownership forbids simply enqueueing the same slot handle to multiple consumers.
## Goals / Non-Goals
Goals: explicit ordinary broadcast copies between fixed-topology endpoints. Non-goals: reference counting, deep-copying element referents, error broadcasts, or broadcast composition with whole-group replicas and node-worker pools.
## Decisions
- Introduce a typed Broadcast connection with a fixed compatible destination list and uniform configured BatchCapacity. Validate all destinations/configuration before committing any branch edge.
- Acquire one pooled copy per branch and copy T elements; pointer-containing T copies are shallow and application mutation must respect that contract. Every branch owns a distinct storage slot.
- For valid batches within configured capacity, create branch copies once. Pool exhaustion drops that branch immediately; Wait queues retain one branch copy and retry independently without recopies. Release source after copies are established; producer remains pending until all Wait branches finish.
- Keep ordinary Connect one-consumer and ErrorPort one-route semantics. Reject mixed branch batch capacities at construction; oversized moved batches violate the documented broadcast-capacity precondition.
- Reject Build when a broadcast source or destination is expanded by whole-group replication or a node-worker pool. This keeps producer-to-branch and branch-to-consumer cardinality unambiguous until those composition semantics are specified.
## Risks / Trade-offs
- Shared pointer referents are mistaken for independent objects → Document shallow-copy semantics and test distinct slots rather than deep copies.
- Retry duplicates deliveries → Record completion per branch and never publish a completed branch again.
## Migration Plan
Requires ownership and metrics. Integrate bounded multi-branch pending state into Drain/Close without weakening ordinary-output limits.
