# Showcase performance work — current status

Saved on 2026-10-26. This is a checkpoint for resuming the performance investigation and verifying its state later; it does not replace the detailed reports or raw benchmark artifacts.

## Latest rerun: handler TimingFull/TSC configuration

**Newest measurement, after indexed traversal:** [profile-report-indexed-rerun2.md](profile-report-indexed-rerun2.md). Twenty-four pairs: showcase median 0.657016 s vs standalone 0.688890 s (−4.63%, 24/24 wins, p=1.1921e-7). Median process CPU 1.455 s vs 0.910 s; peak RSS median 202,386 vs 157,172 KiB. Binary hashes match prior indexed/standalone builds: lower absolute session times do not represent new code changes. Two CPU/heap captures per app confirm pass/polling remain the main orchestration costs and allocations remain 363.78 vs 188.41 MiB/op. No production changes or commits were made in this measurement session.

The subsequent measurement session found existing changes enabling full TSC **handler** timing in showcase; worker-pass timing remains disabled. The earlier disabled-handler-timing measurements below are historical, not an identical configuration baseline. See [profile-report-tsc-full-rerun.md](profile-report-tsc-full-rerun.md).

24 alternating paired prebuilt launches: showcase median 0.696395 s, standalone 0.739571 s (showcase −5.84%, 22/24 wins, exact sign-test p=0.0000359). All results and showcase no-loss checks passed. Median peak RSS: 202,512 vs 157,236 KiB. Fresh CPU/heap captures show worker pass dispatch remains the framework hotspot, logger flat CPU 0.43%, and approximately 363.78 vs 188.41 MiB allocated/op. This compares current applications, not the causal effect of the optimizations or TSC. No production source/config was changed during the measurement task.

## Goal and measurement rules

- Optimize the complete prebuilt `examples/showcase` run using the same `examples/data.json`; compilation is excluded from timings.
- Preserve exact results, no-loss behavior, demonstrated topology, and Drain/Close behavior.
- Treat a full-run improvement as established only when paired measurements support it statistically. Microbenchmarks identify local effects but do not substitute for full-run evidence.
- Keep showcase sources and the dataset uncommitted. Do not revert unrelated existing user changes or create commits without instruction.

## Current implementation

- Branch noted earlier: `integration/showcase-performance`.
- Scheduler group pass metrics use worker-local accumulation and periodic atomic flushes. Default flush interval is 1; `graph.SetGroupMetricsFlushEvery` configures a larger interval. Lifecycle barriers flush pending metrics.
- Showcase currently uses `groupMetricsFlushEvery = 2048` and the Variant D capacities documented in the profile report.
- `graph/logger.go`: logger cursor advances by increment and conditional wrap instead of variable modulo.
- `graph/executor.go`: workers snapshot whether group pass timing is enabled at startup. `pass` skips `beginPassTiming` on workers with timing disabled. Enabled timing retains the same sampling phase and clock behavior.
- No commits were created.

## Latest experiment: worker `beginPassTiming` fast path

Baseline binary: `/tmp/opencode/showcase-D-N2048-logger-after` (logger optimization included; worker timing fast path not included).

Microbenchmarks (Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U, 10 samples, `-benchtime=500ms`):

| Benchmark | Before | After | Change |
|---|---:|---:|---:|
| `GroupMetricsPass/flush-every-1` | 9.683 ns/op | 9.277 ns/op | −4.19% |
| `GroupMetricsPass/flush-every-8` | 3.929 ns/op | 3.612 ns/op | −8.06% |
| `GroupMetricsPass/flush-every-32` | 3.644 ns/op | 2.741 ns/op | −24.79% |
| `GroupMetricsPass/flush-every-64` | 3.510 ns/op | 2.603 ns/op | −25.85% |
| `GroupMetricsPass/flush-every-256` | 3.456 ns/op | 2.489 ns/op | −27.99% |
| `TimingModes/off` | 11.22 ns/op | 11.12 ns/op | −0.80% |
| `TimingModes/full` | 46.28 ns/op | 46.13 ns/op | no significant change |
| `TimingModes/sampled-64` | 14.35 ns/op | 14.08 ns/op | −1.88% |

All benchmark rows reported zero allocations. `TimingModes` measures per-handler timing, not worker-pass timing; it is a non-regression check.

Full prebuilt showcase: 24 alternating paired runs on the same dataset. Median **0.672424 s before**, **0.682889 s after** (+1.56% estimate); 12 wins each, exact sign-test `p=1.0`. All outputs matched and exact totals/no-loss checks passed. No end-to-end speedup was established. RSS shifts are considered noisy, not an optimization result.

Raw artifacts:

- `/tmp/opencode/begin-pass-timing-before.txt`
- `/tmp/opencode/begin-pass-timing-after.txt`
- `/tmp/opencode/showcase-timing-{before,after}-paired-*`
- Post-change showcase binary: `/tmp/opencode/showcase-D-N2048-timing-after`

## Previous logger experiment

- Isolated seven-empty-ring benchmark: 24.810 → 7.262 ns/op (−70.73%, 10 samples, no allocations).
- 24 paired full launches had no significant difference: 0.6671 s before vs 0.6722 s after; exact sign test `p=0.307`.
- Outputs matched; full/race tests passed.
- Details and artifacts are documented in `profile-report-variant-d.md` and issue 19.

## Earlier N=2048 metrics interval comparison

- N=2048 vs N=256 on 24 paired runs: medians 0.662729 s vs 0.666137 s (estimated −0.51%), 9 wins vs 15, exact sign-test `p=0.307`.
- No significant full-run improvement was established. N=2048 was left configured because the user explicitly requested it.
- Dataset SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`.

## Validation

After the worker timing change:

- `go test ./...` — passed.
- `go test -race ./graph ./examples/showcase/...` — passed.
- `git diff --check` — passed.

Earlier implementation of batched scheduler metrics was also tested with the full Go suite and race tests; see the reports for exact details.

## Working tree caution

The tree contains substantial pre-existing/uncommitted work, including graph/runtime changes, showcase restructuring, `examples/data.json`, standalone showcase files, and documentation. `graph/executor.go`, `graph/extended_metrics_test.go`, and other files already had unrelated changes before this latest experiment. Review `git status --short` and the diff before making further edits; do not clean or revert these changes.

## Suggested next step

Pass analysis completed without production changes: [pass-analysis.md](pass-analysis.md). The profiled binary copies a 48-byte runner per loop visit. Test-only indexed traversal improves isolated pass medians by 10–17%; next recommended experiment is applying only that loop change, protecting lifecycle semantics and measuring full launches. Source-free specialization was also measured but is a separate, more complex candidate.

Follow-up: indexed loop is now applied in `graph/executor.go`; no source checks/order/accounting were changed. Full/race suites pass. Twenty-four paired prebuilt launches produced −0.40% median estimate, 13 vs 11 wins, p=0.839: no significant end-to-end benefit. Deterministic output is identical except nondeterministic TSC timing figures. The per-pass microbench result does not yet establish application acceleration.

Latest profiling checkpoint: [profile-report-post-timing.md](profile-report-post-timing.md). Two fresh ten-iteration CPU/heap captures confirm no attributed samples for `beginPassTiming`, logger flat CPU at 0.50–0.58%, and worker pass dispatch still dominant. The report also corrects heap per-execution normalization to include Go benchmark calibration runs.

Continue with a fresh, separately measured hypothesis. The latest profile report identifies worker pass dispatch as the largest framework CPU cost, but cumulative samples include application runner code. Isolate executor scanning/dispatch before changing the production scheduler. No current experiment demonstrates a statistically significant full-showcase acceleration.
