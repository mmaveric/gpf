## Context
See proposal.md. Generated dispatch has exact handler entry/return boundaries and base metrics already supplies per-node rows.
## Goals / Non-Goals
Goals: opt-in full/sampled handler duration. Non-goals: selector/queue timing or extrapolated unsampled duration.
## Decisions
- Configure disabled, full, or every-N-call sampling before Build; reject N=0 for sampled mode. Sample returned handler calls 1, 1+N, and so forth independently per physical node.
- Read an executor-local monotonic nanosecond clock immediately around the selected handler only. Store MeasuredCalls, MeasuredNanoseconds, and MaxMeasuredNanoseconds; include successful and failed returns.
- Exclude selectors, Wait retries, output publication, and logger work. Disabled/unsampled dispatch performs no clock reads; snapshots never read the clock.
- Use modular uint64 subtraction with interval below 2^63; counters wrap. No allocation gate can certify user handler/clock code.
## Risks / Trade-offs
- Measurement perturbs hot paths → Default off and benchmark full versus sampled modes.
- Sampling bias → Report only measured totals, not invented whole-runtime estimates.
## Migration Plan
Requires base metrics. Retain public handler signatures; custom clock injection follows separately.
