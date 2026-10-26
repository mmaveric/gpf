# Framework profile report — showcase Variant D

Date: 2026-10-26  
Workload: `BenchmarkFullShowcase`, five iterations, current Variant D  
Dataset SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`  
Environment: Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U

## Scope and method

Captured CPU and heap profiles from the full workflow benchmark, which includes reading `examples/data.json`, JSON decoding, graph setup/start, processing, Drain, and reporting. The profile ran 5 iterations in 4.63 s wall time and collected 10.70 s of CPU samples across workers. CPU percentages below are shares of sampled CPU, not shares of wall time.

To identify framework work, inspected `graph`, `queue`, and `batch` symbols separately from `examples/showcase` code. The Go runtime rows are shown as runtime overhead associated with the active worker/logger loops, not as framework source functions. Allocation totals are cumulative across the five benchmark iterations. `inuse_space` was captured after benchmark teardown and is not a peak-memory measurement.

Artifacts:

- `/tmp/opencode/showcase-D-cpu.prof`
- `/tmp/opencode/showcase-D-mem.prof`
- `/tmp/opencode/showcase-D-bench.txt` (benchmark output is also in the session transcript)

Profile command:

```sh
go test -run '^$' -bench '^BenchmarkFullShowcase$' -benchmem -benchtime=5x \
  -cpuprofile=/tmp/opencode/showcase-D-cpu.prof \
  -memprofile=/tmp/opencode/showcase-D-mem.prof ./examples/showcase
```

## Hottest framework CPU paths

| Rank | Framework/runtime path | CPU samples | Interpretation |
|---:|---|---:|---|
| 1 | `graph.(*groupCounters).recordPass` | **2.61 s flat / 24.39%** | Dominant framework cost. Called on every scheduler pass; increments `passes`, and increments `idle` on empty passes. If timing sampling is enabled, also updates measured/work/idle nanosecond atomics. Source: `graph/extended_metrics.go:197-211`. |
| 2 | `graph.(*executorWorker).pass` | **0.59 s flat / 5.51%; 4.74 s cumulative / 44.30%** | Runner scan and per-pass bookkeeping. Cumulative time includes callees, especially `recordPass`; do not add cumulative figures together. Source: `graph/executor.go:689-711`. |
| 3 | `graph.(*executorWorker).loop` | **5.44 s cumulative / 50.84%** | Worker scheduling/control loop, including `pass`; not an additional cost to sum with rank 2. Source: `graph/executor.go:612-659`. |
| 4 | `graph.(*runtimeLogger).pass` | **0.17 s flat / 1.59%** | Logger scans its per-group event rings even when no events are present. `loop` yields with `runtime.Gosched` after an empty scan. Sources: `graph/logger.go:142-154`, `165-190`. |
| 5 | `graph.(*groupCounters).beginTiming` | **0.12 s flat / 1.12%** | Per-pass timing-sampling eligibility check; source: `graph/extended_metrics.go:190-195`. |
| 6 | Queue and generated runner operations | Mostly **<1.5% flat each** | `concurrentRing.reservePop` was about 0.09% flat. The measured run does not point to the ring pop itself as a leading CPU target. |

Go runtime samples around these paths include `runtime.selectnbrecv` (1.59% flat), `runtime.chanrecv` (1.96%), `runtime.procyieldAsm` (1.03%), and scheduler/yield routines. Treat these as supporting evidence of polling/scheduling overhead, not independent framework functions.

## Framework allocations

The profile recorded about **2.15 GiB total allocation** across five full-workflow iterations. About **260.25 MiB (11.84%)** was attributed to framework pool construction:

| Allocator | Cumulative allocated |
|---|---:|
| `batch.NewPool[order]` backing storage | 180.49 MiB |
| `batch.NewPool[receipt]` backing storage | 74.03 MiB |
| Ingress `batch.NewPool[Order]` storage | 5.72 MiB |
| **Framework pool total** | **260.25 MiB / 5 ≈ 52.05 MiB per run** |

`graph.(*Definition).Start` has 264.10 MiB cumulative allocation attribution across the five runs; this is chiefly the pool setup above, not a separate 264 MiB to add. `ConnectShard` and `ConnectFanIn` cumulative rows likewise overlap with those pool allocations. `batch.NewPool` intentionally creates a single typed backing slice and per-batch slot metadata (`batch/batch.go:150-179`).

The full profile also shows large `showcase.Run`/`reflect.growslice` and `os.ReadFile` allocations; these are application/input costs and are excluded from the framework findings above. Final `inuse_space` was only about 4 MiB after teardown and must not be interpreted as the process peak RSS.

## Improvement candidates

1. **Revisit per-pass metrics atomics, with a narrower hypothesis.** This is the clear CPU hotspot. Measure `recordPass` variants in a synthetic executor benchmark and retain live `Executor.Metrics()` freshness and exact pass/idle counts. A prior worker-owned-counter candidate was statistically indistinguishable or slower in the full launch (ticket 04), so simply repeating that design is not recommended. Any design that weakens snapshot freshness/count semantics needs a separate decision.
2. **Test the dedicated logger's empty-ring polling.** The current no-error workload spends about 1.6% flat CPU in `runtimeLogger.pass`. A narrowly scoped notification/sleep mechanism could reduce idle work, but must preserve event ordering, logger metrics, Drain/Close, and error delivery. Worker wakeups previously regressed full-process time by 21.7% (ticket 14), so require an isolated prototype and full-launch evidence; expected ceiling is modest.
3. **Investigate pool startup cost as an allocation/startup target.** Pool backing arrays dominate framework allocations. Consider whether slot metadata initialization or pool construction can be reduced without lazy hot-path allocation, changing exhaustion behavior, or weakening fixed-capacity ownership. This is more promising for startup/RSS than CPU: no claim of speedup until measured.
4. **Do not prioritize queue atomics from this profile.** Ring reservation/pop functions account for little flat CPU here. Avoid a queue redesign unless a focused queue profile or contention benchmark changes that evidence.

## Follow-up: configurable pass-metric batching

Implemented the leading CPU-path change in the framework:

- Added `graph.SetGroupMetricsFlushEvery(builder, n)`. Default is `1`, preserving the old per-pass publication behavior unless configured.
- Each executor worker now accumulates absolute pass, idle, timing-sample, and timing-nanosecond totals in worker-owned fields. It publishes changed counters with atomic `Store` every `n` passes, without resetting the local totals.
- The local pass sequence now drives `WorkerTimingConfig.Every`; its sampling phase is independent of publication flushes.
- Workers flush before acknowledging Pause and before Stop. Thus Drain/Close snapshots after the barrier contain exact final pass counts. Concurrent live `Metrics()` snapshots may lag by up to `n-1` passes; other metric rows are unchanged.
- Showcase Variant D uses `n=256` explicitly. This reduces updates on its scheduler pass path while keeping exact post-Drain metrics.

### Post-change measurements

| Measurement | `n=1` | `n=256` | Result |
|---|---:|---:|---|
| `BenchmarkGroupMetricsPass`, median of 10 | 9.711 ns/op | 3.525 ns/op | **63.7% lower** at 256; both 0 allocs/op |
| Full prebuilt showcase launches, median of 24 paired runs | 0.6702 s | 0.6867 s | 2.46% slower estimate at 256, **not significant** (`n=24`, 13 wins/11 losses, `p=0.839`) |

All paired outputs were identical and retained exact totals and zero loss/errors. RSS median was 198,990 KiB (`n=1`) vs 199,994 KiB (`n=256`); maximum samples were 225,484 vs 234,280 KiB.

The `BenchmarkRun`/`BenchmarkFullShowcase` 10-sample matrix also showed no significant `sec/op` change for any tested interval (1, 8, 32, 64, 256). The local pass benchmark and profile confirm the intended internal reduction, but **there is no demonstrated full-process speedup** for this change.

In the same 5-iteration benchmark profile setup, `flushPassMetrics` at `n=1` accounted for 24.41% of CPU samples; with `n=256`, `recordPassMetrics` was 3.21% flat and flush work no longer appeared among the leading graph functions. Profiles: `/tmp/opencode/showcase-D-flush-N1-cpu.prof` and `/tmp/opencode/showcase-D-flush-N256-cpu.prof`.

Validation: `go test ./...` and `go test -race ./graph ./examples/showcase/...` passed. Raw samples: `/tmp/opencode/group-metrics-pass-final.txt` and `/tmp/opencode/showcase-D-flush-*-paired-*`.

### Remaining direction

Keep batching configurable and retain the explicit showcase value because the focused worker-pass cost drops significantly with no significant full-process regression. Do not claim an application speedup. If a stricter full-process speed criterion is required, leave the showcase at `n=1` while retaining the opt-in framework setting; logger idle polling and pool initialization remain separate lower-priority candidates.

## Follow-up: showcase interval 2048

At the user's request, showcase was rerun at `N=2048` against the prior `N=256` binary, plus the unchanged standalone app as a control. All binaries used the same dataset SHA-256 listed above; compilation was excluded.

| App/config | 24-launch median | Median RSS | Maximum RSS |
|---|---:|---:|---:|
| Showcase, N=256 | 0.6661 s | 214,470 KiB | 243,460 KiB |
| Showcase, N=2048 | 0.6627 s | 209,550 KiB | 242,904 KiB |
| Standalone showcase | 0.6929 s | 157,236 KiB | 157,876 KiB |

N=2048's median was 0.51% lower than N=256, but N=2048 won only 9/24 paired launches (15 losses; exact sign-test `p=0.307`). Therefore the end-to-end difference is not statistically significant. Both framework showcase binaries produced byte-identical output and exact totals with zero queue/pool drops, logged errors, or pools in use. Standalone processed the same exact order totals; its output differs by design and it is unaffected by the framework config. `go test ./...` passed.

Current local showcase config is `N=2048`, as requested; the result does not establish a full-process speedup over N=256. Per-run outputs/timings are `/tmp/opencode/showcase-metrics-{N256,N2048,standalone}-rerun-*`.

## Fresh framework profile at N=2048

Captured on 2026-10-26 with the current source/config and same dataset. Command:

```sh
go test -run '^$' -bench '^BenchmarkFullShowcase$' -benchmem -benchtime=5x \
  -cpuprofile=/tmp/opencode/showcase-D-N2048-cpu.prof \
  -memprofile=/tmp/opencode/showcase-D-N2048-mem.prof ./examples/showcase
```

The five iterations took 4.46 s wall and collected 10.35 s of CPU samples. Benchmark output: `763.68 ms/op`, `381,454,734 B/op` (363.8 MiB/op), `1,000 allocs/op`. Percentages below refer to sampled CPU, not wall time.

### Framework CPU hotspots

| Framework path | CPU samples | Notes |
|---|---:|---|
| `graph.(*executorWorker).pass` | **1.08 s flat (10.43%); 4.43 s cumulative (42.80%)** | The pass scan/dispatch is the largest identifiable framework function. Cumulative time includes runners and their callees; do not treat it as pure framework CPU. |
| `graph.(*executorWorker).recordPassMetrics` | **0.25 s flat (2.42%)** | Now mostly worker-local increments and the flush threshold check. `flushPassMetrics` did not appear among the top profile rows at N=2048. The repeated atomic flush hotspot seen at N=1 is no longer dominant. |
| `graph.(*executorWorker).beginPassTiming` | **0.20 s flat (1.93%)** | Still called every pass even when worker timing is disabled. `executor.go:731` checks the nil clock/zero interval on the hot path. A precomputed disabled-timing fast path is a candidate. |
| `graph.(*runtimeLogger).pass` | **0.16 s flat (1.55%); 0.17 s cumulative** | `pprof -list=runtimeLogger.*pass` attributes 0.10 s (about 63% of this function's flat samples) to `l.cursor = (l.cursor + 1) % len(l.rings)` at `graph/logger.go:145`. Replacing variable modulo with increment-and-wrap is a small, semantics-preserving candidate. |
| `graph.ConnectShard` output binding | **0.38 s flat (3.67%)** | This closure also invokes the application-supplied selector (`f.selectBranch`); do not attribute all of it to the framework without an isolated benchmark. |

Go runtime support in the profile includes `runtime.chanrecv` (3.67% flat), `runtime.selectnbrecv` (1.74%), and `runtime.procyieldAsm` (1.74%). These are runtime symbols on the worker/logger paths, not framework functions. Generated `normalizeExplicit1` runner closures also have several high rows, but they include showcase handlers; those rows are intentionally not presented as pure framework costs.

### Framework allocations

The heap profile recorded 2,192.15 MiB of cumulative allocations over five iterations. Framework `batch.NewPool` construction accounts for **258.61 MiB (11.80%)**, or about **51.72 MiB per iteration**:

| Framework pool backing | Cumulative allocation |
|---|---:|
| `NewPool[order]` | 176.56 MiB |
| `NewPool[receipt]` | 77.76 MiB |
| `NewPool[Order]` ingress pool | 4.29 MiB |

`graph.(*Definition).Start` has 265.41 MiB cumulative attribution, largely overlapping these pool allocations; do not add those values together. `ConnectShard` and `ConnectFanIn` cumulative allocation rows also overlap with pool construction. The profile's post-teardown `inuse_space` was 4.6 MiB and is not a peak-RSS measure.

For context only, the full profile also attributes 789.76 MiB to `showcase.Run`, 788.18 MiB to `reflect.growslice`, and 342.09 MiB to `os.readFile`. These are application/input costs and excluded from the framework findings.

### Recommended next experiments

1. **Worker pass scan:** remains the largest framework orchestration cost, but cumulative time includes user runners. Use an isolated executor/runner microbenchmark before a production design change; previous polling and wake-based scheduler variants did not improve full launch.
2. **Pool setup:** investigate only if reducing startup allocation/RSS is a goal. Pool arrays are intentional fixed-capacity storage; lazy allocation risks moving cost/GC into the hot path.
3. **Shard path:** profile it with a framework-controlled selector before optimizing. The showcase's selector is user callback work mixed into the current top row.

End-to-end context: the 24 paired prebuilt launches at N=2048 vs N=256 showed a 0.51% lower N=2048 median, but only 9/24 wins (`p=0.307`); no statistically significant application speedup is established.

## Logger cursor-wrap optimization

Applied the profile-guided change in `graph/logger.go:145`: replace `(cursor + 1) % len(rings)` with increment and conditional wrap. The existing `TestLoggerPollsRingsRoundRobin` protects the observable polling order.

| Measurement | Before | After | Difference |
|---|---:|---:|---:|
| `BenchmarkRuntimeLoggerPass` (7 empty rings, 10 samples) | 24.810 ns/op | 7.262 ns/op | **−70.73%**, `p=0.000`; 0 allocs/op |
| Full prebuilt showcase launch (24 paired runs) | 0.6671 s median | 0.6722 s median | +0.75% slower estimate; not significant (`15` wins / `9` losses for after, exact sign-test `p=0.307`) |

All 24 before/after outputs were byte-identical with exact totals and zero queue/pool drops, logged errors, and pools in use. Median/max RSS changed from 222,504/242,520 KiB to 202,614/226,256 KiB; treat this one-sample-set RSS shift as diagnostic only. `go test ./...`, `go test -race ./graph ./examples/showcase/...`, and `git diff --check` passed.

Artifacts: `/tmp/opencode/logger-pass-{before,after}.txt` and `/tmp/opencode/showcase-logger-{before,after}-paired-*`. The change substantially speeds the isolated logger scan but does **not** demonstrate an end-to-end showcase speedup.

## Disabled worker pass-timing fast path

`executorWorker` now snapshots whether group pass timing is enabled when workers are created. `pass` only calls `beginPassTiming` when that immutable flag is true; enabled timing continues to use the same `passes % timingEvery` phase and clock reads. `beginPassTiming` retains its defensive checks. Worker unit fixtures explicitly set the flag when testing enabled timing.

Benchmarked on Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U, 10 samples (`-benchtime=500ms`):

| Benchmark | Before | After | Difference |
|---|---:|---:|---:|
| `GroupMetricsPass/flush-every-1` | 9.683 ns/op | 9.277 ns/op | −4.19% (`p=0.003`) |
| `GroupMetricsPass/flush-every-8` | 3.929 ns/op | 3.612 ns/op | −8.06% (`p=0.000`) |
| `GroupMetricsPass/flush-every-32` | 3.644 ns/op | 2.741 ns/op | −24.79% (`p=0.000`) |
| `GroupMetricsPass/flush-every-64` | 3.510 ns/op | 2.603 ns/op | −25.85% (`p=0.000`) |
| `GroupMetricsPass/flush-every-256` | 3.456 ns/op | 2.489 ns/op | −27.99% (`p=0.000`) |
| `TimingModes/off` | 11.22 ns/op | 11.12 ns/op | −0.80% (`p=0.005`) |
| `TimingModes/full` | 46.28 ns/op | 46.13 ns/op | no significant difference (`p=0.493`) |
| `TimingModes/sampled-64` | 14.35 ns/op | 14.08 ns/op | −1.88% (`p=0.003`) |

The `TimingModes` benchmark measures handler timing rather than worker-pass timing; its near-flat result serves as a non-regression check. All rows reported zero allocations.

For end-to-end validation, compared 24 alternating paired prebuilt launches against `/tmp/opencode/showcase-D-N2048-logger-after` (same logger optimization, prior worker-timing path). Median was 0.672424 s before and 0.682889 s after (+1.56% estimate), with 12 wins each and exact sign-test `p=1.0`. Output bytes matched across all pairs and exact results/no-loss checks passed. Thus the worker microbenchmark improves, but the full showcase shows no measurable speedup. Median RSS in this sample was 198,866 KiB before and 223,432 KiB after; this noisy shift is not treated as an effect. `go test ./...` and `go test -race ./graph ./examples/showcase/...` passed.

Artifacts: `/tmp/opencode/begin-pass-timing-{before,after}.txt` and `/tmp/opencode/showcase-timing-{before,after}-paired-*`.
