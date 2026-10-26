# Reduce scheduler metric overhead without weakening snapshots

Type: task
Mode: AFK
Status: resolved
Blocked by: 02

## Question

Can the worker-owned scheduler metric update path be made measurably cheaper while preserving live `Executor.Metrics()` behavior and exact stopped totals?

### Work

- Capture a fresh same-environment baseline with the lossless capacities and unchanged dataset. Use repeated `BenchmarkRun` and full prebuilt-binary runs; retain raw results and environment details.
- Investigate `groupCounters.recordPass` and its single owning executor worker. Reduce per-pass update cost only if live snapshots remain race-free, current, and behaviorally compatible.
- Add or adapt public-seam tests for live metrics reads during execution and exact final metrics after stop. Preserve existing counter and sampling semantics.
- Compare before/after with repeated samples and `benchstat`; record peak RSS and allocations as secondary metrics.
- Keep the change only if the measured speedup is statistically significant. Otherwise revert the optimization and report the result without changing the metrics contract.

### Resolution evidence

Attach the benchmark comparison, profiles, test results, and the final implementation or no-change finding. Any requirement to weaken live snapshot semantics becomes a separate HITL ticket.

## Answer

- Tried worker-owned pass/idle counters with atomic snapshot publication while preserving live `Metrics()` reads. Correctness checks passed, but no statistically significant speedup was measured; the candidate was reverted and no code commit was retained.
- `BenchmarkRun` (`n=10` each): 718.9 ms/op before vs 742.5 ms/op candidate (`p=0.971`); bytes/op were 225.7 MiB both (`p=0.436`); allocs/op 1.028k vs 1.325k (`p=0.481`).
- Full app: all ten before and after runs produced exact totals and zero drops/errors. Median wall time was 1.08 s before and 1.09 s after; median peak RSS was 255,320 KB before and 242,172 KB after.
- Existing tests covered concurrent live reads and exact post-stop metrics; `go test ./...` and `go test -race ./graph` passed after revert.
- No optimization was retained. Raw benchmark and full-run evidence: `/tmp/opencode/showcase-scheduler-metrics/`.
