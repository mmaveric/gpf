## Context
See proposal.md. Existing remote queues use typed handles and atomic SPSC indices; richer queue models must not reuse slots prematurely.
## Goals / Non-Goals
Goals: bounded portable MPSC/SPMC/MPMC rings. Non-goals: graph expansion, dynamic capacity, remote custom queues.
## Decisions
- Add typed constructors producing QueueOps[T] for each model. Validate capacities and index arithmetic at construction; preallocate sequence metadata and slots.
- Use per-slot Free/Writing/Ready/Reading atomic ownership states and producer/consumer cursors that wrap modulo the configured capacity. A successful reservation advances exactly one cursor; payload writes precede Ready publication, and consumers return slots to Free only after moving the handle. Try operations return unavailable on contention, full/empty state, or an unpublished head reservation rather than waiting for another goroutine's progress.
- Preserve exclusive ownership: failed push leaves its argument owned; successful pop owns a fully published handle. Len is a bounded concurrent observation, not a linearizable occupancy proof.
- Keep SPSC specialization; shared models trade atomic overhead for cardinality correctness. Test near counter wrap without requiring billions of operations.
## Risks / Trade-offs
- A producer reservation can delay visibility → Specify nonblocking availability and test a later ready slot behind a delayed head without spin-until-publication loops.
- False sharing or slot reuse races → Keep typed atomic slot states and verify repeated arbitrary-capacity cursor wrap under race/stress tests.
## Migration Plan
Requires the ownership fix. Queue primitives land before automatic selection; reconcile remote queue deferrals only when graph selection integrates them.
