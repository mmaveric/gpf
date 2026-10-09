## Context
See proposal.md. Fixed ErrorEvent values and dormant typed mappers are provided by add-error-mapping.
## Goals / Non-Goals
Goals: bounded worker-to-logger transport. Non-goals: a logging backend, reliable delivery, or data-plane formatting.
## Decisions
- Preallocate one SPSC event ring per physical worker and start one unpinned logger goroutine with a configured func(ErrorEvent) callback. Workers map and TryPush once; full rings drop immediately.
- Logger polls rings fairly and may format/allocate outside the data plane. Expose attempted/accepted/full-drop/delivered event counters; counts do not depend on error-port delivery.
- A shared drain deadline flushes queued events after data workers stop producing. Expiration discards unconsumed events. An in-flight logger callback must return before its goroutine can be joined; never detach a stuck callback and pretend cleanup completed.
- Panic remains a user-code panic. Default logging is disabled; no global logger singleton or backend dependency.
- Configure logging once via SetLogger and a copied LoggerConfig with per-worker Capacity and Handle callback. A missing mapper yields a zero payload with runtime identity. The logger checks stop commands and cancellation between individual callbacks, selecting rings in round-robin order; it starts only after all node Init callbacks succeed.
## Risks / Trade-offs
- Slow callback → Bounded loss counters; document cooperative shutdown and callback return obligations.
- Replica workers share a ring incorrectly → Assign ring ownership after physical expansion, not by logical group ID.
## Migration Plan
Requires mapper, base metrics, and deadline drain. Update current logging deferrals against the then-current main contracts before archive.
