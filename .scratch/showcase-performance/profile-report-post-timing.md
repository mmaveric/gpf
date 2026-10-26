# Showcase profile after logger and worker timing optimizations

Date: 2026-10-26. No production code changed during this profiling task.

## Setup

- Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U; benchmark GOMAXPROCS suffix: 14.
- Current Variant D capacities and group metrics flush interval 2048.
- Logger conditional cursor wrap and startup `timingEnabled` worker flag are both included.
- Dataset SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`.
- Two sequential profiling runs, each with ten measured full-showcase benchmark iterations. Reading and decoding input are included.

```sh
go test -o /tmp/opencode/showcase-post-timing.test -run '^$' \
  -bench '^BenchmarkFullShowcase$' -benchmem -benchtime=10x \
  -cpuprofile=/tmp/opencode/showcase-post-timing-cpu.prof \
  -memprofile=/tmp/opencode/showcase-post-timing-mem.prof ./examples/showcase
```

Repeat uses the same command with `showcase-post-timing-repeat` as the artifact prefix. Explicit binary paths preserve symbolization without creating a test binary in the repository.

| Measurement | Run 1 | Run 2 |
|---|---:|---:|
| Profile duration | 8.98 s | 8.95 s |
| Sampled CPU time | 21.92 s | 24.34 s |
| Benchmark time | 835.834 ms/op | 832.328 ms/op |
| Bytes allocated | 381,456,026 B/op | 381,455,089 B/op |
| Allocations | 1,008/op | 1,006/op |

These are profiled benchmark measurements, not ordinary prebuilt process latency. Two runs are insufficient to claim statistically significant end-to-end change. CPU time can exceed wall time because workers execute concurrently.

## CPU findings

Percentages use total sampled CPU as denominator. Flat is self time; cumulative includes callees. Rows overlap and must not be added as independent costs.

| Function | Run 1 flat / cumulative | Run 2 flat / cumulative |
|---|---:|---:|
| `executorWorker.pass` | 12.73% / 44.80% | 14.34% / 47.82% |
| `executorWorker.recordPassMetrics` | 3.24% / 3.28% | 3.33% / 3.37% |
| `executorWorker.loop` | 2.51% / 55.29% | 2.05% / 58.83% |
| `runtime.chanrecv` | 4.38% / 6.25% | 4.56% / 7.03% |
| `runtime.selectnbrecv` | 2.05% / 8.30% | 2.30% / 9.33% |
| `runtimeLogger.pass` | 0.50% / 0.68% | 0.58% / 0.66% |

### Effects of the last two optimizations

- `beginPassTiming` has no attributed samples in either full CPU function listing. Source confirms the helper is bypassed when timing is disabled. This is consistent with the intended fast path, not proof of a wall-time improvement.
- `runtimeLogger.pass` now accounts for 0.50–0.58% flat CPU, versus 1.55% in the earlier N=2048 profile. Run 1 attributes only 10 ms each to cursor increment and wrap check. Different profile totals/workloads prevent treating the percentage ratio as an isolated benchmark result; the earlier isolated benchmark already showed a 70.73% improvement.
- The timing-enabled branch itself still receives samples within `pass`: 380 ms in run 1. Removing a helper does not remove all timing-related control flow. Instruction/line attribution is approximate and not a direct measurement of branch cost.

### Remaining orchestration costs

`graph/executor.go:714` runner iteration receives 880 ms flat in run 1. Source/admission checks at lines 715 and 718 receive 440 ms and 180 ms flat respectively. `runner.run()` receives 6.41 s cumulative, including application handlers and downstream framework work. Therefore the 44.80–47.82% cumulative `pass` total is not pure scheduler overhead.

`recordPassMetrics` remains 3.24–3.33% flat. Samples fall on local updates, measured/idle checks, flush initialization/threshold checks, and function entry/exit. Batched atomic publication is no longer the obvious dominant cost; increasing the flush interval again is not justified by this profile.

Channel receive/polling runtime functions remain visible. They are shared runtime support, not independent framework functions. Their percentages overlap with callers; do not add `chanrecv` to `selectnbrecv` cumulative time.

Run 1 also shows a generated receipt-producing `normalizeExplicit1` closure at 6.02% flat / 12.23% cumulative and shard output binding at 2.74% flat / 4.84% cumulative. These closures mix framework plumbing with application callbacks; isolated measurements are needed before assigning costs to either side.

JSON decoding remains substantial: run 2 attributes 25.27% cumulative to `encoding/json.Unmarshal`. It belongs to input preparation, not framework execution.

## Memory findings

Run 1 `alloc_space` totals 4,020.35 MiB:

| Allocation site | MiB | Share |
|---|---:|---:|
| `reflect.growslice` (JSON input slice growth) | 1,449.82 | 36.06% |
| `showcase.Run` (chiefly valid/invalid slice growth) | 1,448.03 flat | 36.02% |
| `os.readFileContents` | 627.17 | 15.60% |
| All `batch.NewPool` instances | 478.21 | 11.89% |

Pool breakdown: internal order 327.19 MiB, receipt 141.01 MiB, ingress Order 10.02 MiB. `Definition.Start` has 485.51 MiB cumulative attribution, overlapping the pools; do not add these totals.

**Important normalization correction:** Go benchmark profiling also includes the initial one-iteration `run1` calibration. The profile explicitly attributes 44.87 MiB of pool allocations to `testing.(*B).run1.func1` and 433.34 MiB to `testing.(*B).launch`. There are eleven actual full executions in this process, not ten. The aggregate pool estimate is consequently about **43.47 MiB/execution**, not 47.82 MiB. The earlier five-iteration profile similarly included a warm-up execution, so its historical 51.72 MiB/run pool estimate should not be used; dividing that aggregate by six gives about 43.10 MiB/run. Heap sampling introduces additional estimation noise.

Benchmark allocation accounting excludes calibration and reports approximately **363.78 MiB/op**, essentially unchanged from the prior profile. These CPU-only changes did not target memory allocation.

Final `inuse_space` is approximately 3.50 MiB, attributed to runtime allocations. This is sampled live heap after teardown/GC, not peak heap or RSS, and does not prove absence of leaks.

## Correctness control

A fresh prebuilt executable was run separately and reported:

```text
orders submitted=1000000 processed=1000000 rejected=0 audited=1000000 units=5500000 gross_cents=27636159434 partitions=333300,333400,333300
runtime edges=10 accepted_elements=5000000 dequeued_elements=5000000 queue_drop_elements=0 pool_drop_elements=0 logged_errors=0 pools_in_use=0
```

All seven groups reported successful affinity placement. Both profiling benchmarks passed. Full and race suites had passed after the implementation change; they were not rerun for this profiling-only task.

## Recommended next experiment

1. Isolate worker pass scanning/dispatch with realistic runner counts and idle/progress mixtures. Investigate eliminating unnecessary per-runner source checks only with lifecycle/Drain semantics protected; no scheduler redesign is justified from cumulative time alone.
2. If focusing on metrics, benchmark the normal startup-initialized path separately from zero-initialized test fixtures before removing the lazy flush-interval fallback. The profile suggests a candidate, not a demonstrated improvement.
3. For full application startup/memory goals, input decoding and the growing valid/invalid slices offer larger allocation targets than the two micro-optimizations. These are application-level experiments and should be treated separately from framework work.

No statistically significant full-showcase speedup has been established by the prior paired launch tests. The new profiles confirm the local hotspots moved, not that overall latency improved.

## Artifacts and evidence limitations

- `/tmp/opencode/showcase-post-timing{,-repeat}.test`
- `/tmp/opencode/showcase-post-timing{,-repeat}-cpu.prof`
- `/tmp/opencode/showcase-post-timing{,-repeat}-mem.prof`
- `/tmp/opencode/showcase-post-timing{,-repeat}-cpu-top.txt`
- Correctness executable: `/tmp/opencode/showcase-post-timing-profile-check`

The graph index generation is 2026-09-02 and is stale/untracked for relevant current files. Findings above rely on fresh pprof symbolization and direct source checks, not the stale graph. Profiles capture aggregate CPU rather than a critical path; asynchronous scheduling, heap sampling, and profiler overhead limit comparisons with older captures. Temporary artifacts may not survive environment cleanup; this report retains their key measurements.
