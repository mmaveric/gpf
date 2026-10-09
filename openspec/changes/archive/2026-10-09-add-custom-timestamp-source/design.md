## Context
See proposal.md. Processing timing already has a monotonic default timestamp function and known sampling boundaries.
## Goals / Non-Goals
Goals: deterministic injectable nanosecond clock. Non-goals: clock discovery, calibration, or certifying user callbacks.
## Decisions
- Add immutable optional func() uint64 timing source; nil retains the built-in clock. Build never calls it and snapshots never call it.
- Invoke exactly twice per sampled returned handler on its worker. A shared callback must be concurrency-safe across workers, nonblocking, and nonallocating.
- Define timestamps as monotonically progressing nanoseconds modulo uint64; valid measured intervals are below 2^63. Invalid-clock behavior is a violated user contract, not panic recovery or a hidden clock substitution.
- Use fake counter clocks for deterministic sampling/duration tests; avoid wall-clock sleeps.
## Risks / Trade-offs
- Wrong units or backward readings → Explicit units/interval contract and diagnostic test examples.
- Shared mutable fake clock → Use concurrency-safe fixture clocks for multi-worker tests.
## Migration Plan
Requires processing timing. The default remains behavior-compatible and untouched when no callback is configured.
