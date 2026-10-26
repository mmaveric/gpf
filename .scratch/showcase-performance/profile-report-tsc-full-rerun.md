# Fresh application runs and profiles: full TSC handler timing

Date: 2026-10-26. This measurement task did not modify production code or configuration.

## Configuration and comparability

- Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U; benchmark GOMAXPROCS 14.
- Same dataset, SHA-256 `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`.
- Current showcase uses Variant D capacities, group metrics flush interval 2048, logger conditional wrap and worker timing fast path.
- **New relative to the earlier checkpoint:** `examples/showcase/internal/showcase/topology.go:44` enables `TimingFull` with `TimingClockTSC` for handlers. The working tree also includes the hrtime clock implementation and dependency. These existing changes were not altered.
- Handler and worker-pass timing are configured independently (`graph/metrics.go:149–175`). Worker-pass timing remains disabled. `TimingFull` here does NOT cause `beginPassTiming` to execute.
- Standalone remains a separate implementation using per-order Go channels, capacity 256, without framework instrumentation. It is a control, not an equivalent scheduler configuration or an optimization baseline.

## Full prebuilt application runs

Built both executables before timing. Ran 24 pairs sequentially, alternating which application runs first. Python `perf_counter` measures completion of `/usr/bin/time` and the executable; compilation is excluded. RSS is `/usr/bin/time` maximum resident set size. The small wrapper overhead is included equally in both apps.

| Measurement | GPF showcase | Standalone |
|---|---:|---:|
| Median wall time | **0.696395 s** | **0.739571 s** |
| Minimum | 0.667474 s | 0.713325 s |
| Maximum | 0.767707 s | 0.810796 s |
| Median peak RSS | 202,512 KiB | 157,236 KiB |
| Maximum peak RSS | 242,748 KiB | 157,620 KiB |

Showcase's median is **5.84% lower**, and it wins **22/24 pairs**. Exact two-sided sign-test `p=0.0000359`. An exploratory paired bootstrap of the ratio of medians (20,000 resamples, seed 20261026) gives a 95% percentile interval of approximately **−6.98% to −4.34%**. This is evidence of a difference between these two current executables under this run protocol, not evidence that the recent framework optimizations or TSC caused an improvement. Earlier sessions are not a controlled same-configuration baseline.

Showcase median peak RSS is about 28.8% higher. It trades batching/polling and additional buffers against standalone's per-element channels; lower wall time does not imply lower CPU or memory use.

### Correctness

Every one of the 48 runs reported:

```text
orders submitted=1000000 processed=1000000 rejected=0 audited=1000000 units=5500000 gross_cents=27636159434 partitions=333300,333400,333300
```

All showcase runs also reported zero queue/pool drops, zero logged errors, and zero pools in use after Drain. Both profiling benchmarks passed. No full/race test suite was rerun for this measurement-only task.

## CPU and heap capture

One ten-iteration full-process-workflow benchmark capture per application, run sequentially. Input reading and decoding are included. These benchmark profiles exclude process startup and do not replace prebuilt-process latency. In particular, TSC lazy calibration is warmed once per test process, whereas fresh application processes may pay first-use initialization each time.

```sh
go test -o /tmp/opencode/showcase-tsc-full.test -run '^$' \
  -bench '^BenchmarkFullShowcase$' -benchmem -benchtime=10x \
  -cpuprofile=/tmp/opencode/showcase-tsc-full-cpu.prof \
  -memprofile=/tmp/opencode/showcase-tsc-full-mem.prof ./examples/showcase

go test -overlay=/tmp/opencode/standalone-profile-overlay.json \
  -o /tmp/opencode/standalone-tsc-control.test -run '^$' \
  -bench '^BenchmarkFullStandaloneProfile$' -benchmem -benchtime=10x \
  -cpuprofile=/tmp/opencode/standalone-tsc-control-cpu.prof \
  -memprofile=/tmp/opencode/standalone-tsc-control-mem.prof ./examples/standalone-showcase
```

The standalone benchmark is supplied using Go's file overlay, backed by `standalone-profile-benchmark.go.txt` beside this report; production standalone sources are unchanged.

| Profile measurement | Showcase | Standalone |
|---|---:|---:|
| Benchmark time under profiling | 814.929 ms/op | 710.451 ms/op |
| Profile duration | 8.83 s | 7.86 s |
| Total sampled CPU | **20.85 s** | **10.73 s** |
| Allocated bytes/op | 381,455,884 | 197,558,602 |
| Allocated MiB/op | **363.78** | **188.41** |
| Allocations/op | 1,010 | 100 |

The profiled benchmark ranks these applications differently from fresh process runs. That is not contradictory: instrumentation, benchmark process reuse, initialization, GC state and scheduling differ. Do not substitute profiled ns/op for the primary latency measure. CPU samples suggest the showcase consumes substantially more aggregate CPU in this capture, despite its lower fresh-process median. These are single profile captures, not repeated statistical CPU measurements.

## Showcase CPU analysis

Percentages are shares of sampled CPU. Cumulative rows overlap with descendants and must not be summed.

| Path | Flat | Cumulative | Interpretation |
|---|---:|---:|---|
| `executorWorker.pass` | **13.14%** | **42.73%** | Scan, admission checks and dispatch; cumulative includes handlers |
| Receipt-producing generated `normalizeExplicit1` runner | 4.70% | 11.22% | Framework plumbing mixed with application work |
| `recordPassMetrics` | **2.64%** | 2.73% | Local pass accounting/checks remain visible |
| Shard output binding | 2.83% | 4.65% | Includes application selector work |
| `executorWorker.loop` | 2.21% | 51.89% | Worker orchestration, including nested pass costs |
| `runtime.chanrecv` | 3.36% | 5.47% | Runtime channel/polling support |
| `runtime.selectnbrecv` | 1.87% | 7.34% | Overlaps channel receive support |
| `runtimeLogger.pass` | **0.43%** | 0.53% | No longer a major hotspot |
| `encoding/json.Unmarshal` | 0% | **31.70%** | Input decoding, outside framework processing |

With default pprof pruning, TSC timing functions do not appear in the top table. Repeating with `-nodefraction=0 -edgefraction=0` and a timing/hrtime focus reveals **30 ms (0.14%) cumulative** through `timingClock.func1`, including 10 ms each in `rdtscAsm` and `Count.ApproxDuration`. This is very sparse sampling and **not a measurement of total instrumentation overhead**: branches, modulo, counter atomics and inlined instructions may be attributed to callers. It cannot establish that TSC is faster than the standard clock without a controlled comparison.

No attributed `beginPassTiming` samples occur, consistent with independently disabled worker timing. The key remaining framework target is still pass scanning/dispatch, not logger cursor arithmetic or clock reads.

## Standalone CPU analysis

| Path | Flat | Cumulative |
|---|---:|---:|
| `runtime.procyieldAsm` | **10.44%** | 10.44% |
| `runtime.lock2` | **10.07%** | 21.90% |
| `runtime.unlock2` | **8.20%** | 9.51% |
| `runtime.chansend` | 1.68% | 22.83% |
| `runtime.chanrecv` | 2.14% | 16.78% |
| `encoding/json.Unmarshal` | 0% | **54.24%** |

The standalone implementation sends individual orders/receipts over channels. The profile is consistent with channel synchronization and spin/lock costs plus input decoding. It uses less aggregate CPU and memory in these captures, but its fresh-process latency is higher in the 24 paired runs. Lock/send/receive rows overlap and should not be added as independent totals. CPU samples alone do not quantify off-CPU waiting; block/trace evidence would be needed for that.

## Memory analysis

Showcase alloc-space profile totals **4,016.07 MiB**, chiefly:

- `showcase.Run`: **1,449.48 MiB flat (36.09%)**, predominantly growing valid/invalid slices.
- `reflect.growslice`: **1,447.10 MiB (36.03%)**, chiefly decoded input slice growth.
- File contents: **627.17 MiB (15.62%)**.
- Internal order/receipt pools visible in the top table: **326.23 + 139.14 MiB**, with additional smaller ingress pool allocations. `Definition.Start` cumulative allocations overlap pools and must not be added.

Standalone alloc-space profile totals **2,077.62 MiB**, with **1,440.86 MiB (69.35%)** in slice growth and **627.17 MiB (30.19%)** in file contents. Almost all volume is input preparation rather than channel construction.

Each heap profile includes the one-iteration benchmark calibration plus ten measured iterations: **eleven full executions**. Do not divide aggregate heap totals by ten. Benchmark B/op already excludes calibration and is the preferred per-operation figure.

The approximately **175.38 MiB/op extra showcase allocation** is consistent with the additional valid/invalid slice growth plus batch pool storage. The unchanged broad allocation pattern suggests these application-level slices are a larger memory target than clock/logger changes.

Post-teardown sampled live heap is about 4.51 MiB (showcase) and 4.00 MiB (standalone). This is not peak RSS or proof of no leaks.

## Next experiments

1. Framework: benchmark pass scanning/dispatch with realistic runner counts and idle/work mixtures; preserve source admission and Drain/Close behavior. Cumulative samples alone do not justify a scheduler redesign.
2. Application memory/startup: avoid repeated growth/copies of the valid-order slice while preserving routing and ordering. This is separate from framework optimization.
3. If comparing timing clocks, build controlled variants differing only in handler clock/config and measure fresh-process latency as well as timed-handler microbenchmarks. The current session does not prove an effect of TSC.

## Artifacts and evidence

- `/tmp/opencode/tsc-full-app-runs.json`: all wall times and RSS values.
- `/tmp/opencode/tsc-full-{showcase,standalone}-*.out`: validated per-run output.
- `/tmp/opencode/tsc-full-source-sha256.json`: hashes of relevant source/config files.
- `/tmp/opencode/{showcase-tsc-full,standalone-tsc-control}-{cpu,mem}.prof` and corresponding `.test` binaries.
- `/tmp/opencode/{showcase-tsc-full,standalone-tsc-control}-{cpu,alloc_space,inuse_space}-top.txt`.
- `/tmp/opencode/showcase-tsc-full-{runners,workers}-list.txt`.

Executable SHA-256:

- Showcase: `d07ab41955cdcc8b3ce6cbacfbc6fd1102752287c921b8f16e07df4b982464e5`.
- Standalone: `6b4f64be1bd5eb43f12101d249d170e76d0b5564e00934cb202a5afcb59aa965`.

The graph index remains stale (2026-09-02); coverage checks flag changed/untracked paths. Timing configuration and runtime behavior claims were checked against current source and fresh profiles, not inferred from that index. Temporary artifacts may be cleaned later; key results are preserved here.
