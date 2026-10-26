# Batch group scheduler metrics

Type: task
Mode: AFK
Status: resolved
Blocked by: 17

## Goal

Accumulate scheduler-pass counters in worker-local state and publish atomically every configurable N passes, while retaining exact final counts.

## Resolution

- Added `graph.SetGroupMetricsFlushEvery(builder, n)`, rejecting zero and duplicate configuration. Default is `n=1` for compatibility.
- Workers keep cumulative local group pass/idle/timing counters and publish changed values with atomic `Store` every N passes. `WorkerTimingConfig.Every` uses the local pass sequence and remains independent from flush cadence.
- Flush before Pause acknowledgement and Stop so Drain/Close final metrics are exact. Live group metrics may lag at most N-1 passes; other metric families remain unchanged.
- Showcase was initially configured at N=256. It is now set to N=2048 per the user's follow-up request; topology and processing behavior are unchanged.
- Tests verify threshold publication, independent timing sampling, config propagation, live staleness, and exact post-Drain totals.

## Evidence

- `BenchmarkGroupMetricsPass` (10 samples): N=1 median 9.711 ns/op; N=256 median 3.525 ns/op (-63.7%); 0 allocations in both.
- 24 paired full launches: N=1 median 0.6702 s; N=256 median 0.6867 s (+2.46% estimate, p=0.839; 13 wins/11 losses). No significant end-to-end speedup or regression. Outputs identical and lossless.
- `BenchmarkRun`/`BenchmarkFullShowcase` matrix showed no significant `sec/op` change across N=1,8,32,64,256.
- `go test ./...`; `go test -race ./graph ./examples/showcase/...` passed.
- Detailed profile and trade-offs: `../profile-report-variant-d.md`. Raw artifacts in `/tmp/opencode/group-metrics-pass-final.txt` and `/tmp/opencode/showcase-D-flush-*`.

### Follow-up at N=2048

- 24 paired prebuilt launches: N=256 median 0.6661 s; N=2048 median 0.6627 s (-0.51% estimate), 9 wins/15 losses, exact sign-test `p=0.307`; no statistically significant full-process change.
- RSS medians/maxima: N=256 214,470/243,460 KiB; N=2048 209,550/242,904 KiB.
- N=256 and N=2048 output was byte-identical with exact totals and zero drops/errors. Standalone control median was 0.6929 s and remains unaffected.
- Full details and raw samples: `../profile-report-variant-d.md`, `/tmp/opencode/showcase-metrics-*`.
