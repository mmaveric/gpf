## Context
See proposal.md. Base runtime metrics owns numeric snapshots and semantic loss counters.
## Goals / Non-Goals
Goals: bounded utilization and user counters. Non-goals: labels, maps, exporter dependencies, or duplicating timing/log counter ownership.
## Decisions
- Add sampled pool in-use/capacity/high-water observations updated at acquisition/release boundaries with atomic publication; never walk batch storage from snapshots.
- Register fixed numeric user counter IDs before Build and bind replica-local increment functions during Start. Invalid/duplicate registrations fail without consuming IDs; updates cannot grow a registry.
- Extend worker activity with existing idle/work passes and optional every-N-pass idle/work nanosecond measurements using the timing infrastructure. Measure one complete pass at its boundaries and classify it by progress; these are measured polling wall durations, not OS CPU time or extrapolated unsampled totals. Default duration collection is off. Handler retries are not invented from Wait publication attempts.
- Reuse copied ordered snapshots; report per-pool/per-replica identity explicitly and avoid global totals that double-count moved work.
- Register IDs within each node's namespace. Generated BindCounter hooks install update functions into fresh explicit/closure state during normalization; optional Executor.Counter lookup belongs to the control plane. Registered records are ordered by node and ID, remain fixed through execution, and freeze after completed stop.
- Pool observers use framework-owned numeric accessors bound at startup, never user queue callbacks or batch-content reads. Output acquisitions use invocation scopes too: explicit release or move informs retained cleanup, and an unused scratch acquisition is reclaimed when another input is committed.
## Risks / Trade-offs
- Gauge contention on remote releases → Isolate writer records and profile update cost.
- Confusing idle passes with idle time → Name pass measures accurately; add time only through timing capabilities.
## Migration Plan
Requires base metrics and processing timing. Reconcile metric snapshot fields with logger/timing/replica implementations at apply time without resetting their counters.
