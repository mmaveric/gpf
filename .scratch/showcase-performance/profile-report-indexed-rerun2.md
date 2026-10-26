# Application rerun and profiles after indexed pass traversal

Captured 2026-10-26. No production code/configuration was modified in this measurement session.

## Configuration

- Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U; benchmark GOMAXPROCS 14.
- Same dataset SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`.
- Showcase: current Variant D capacities, flush interval 2048, full TSC **handler** timing, disabled worker-pass timing, optimized logger cursor, indexed runner traversal.
- Standalone: unchanged per-order Go channels, capacity 256, no framework instrumentation.
- Freshly built executable hashes match the previous indexed showcase and standalone binaries exactly:
  - Showcase: `f966a27aa39f86829d64174c6509d5b54ec2b7b5e3e7fb873975ae81b3e556b3`.
  - Standalone: `6b4f64be1bd5eb43f12101d249d170e76d0b5564e00934cb202a5afcb59aa965`.

These are comparisons of two applications, not an isolated test of indexed traversal or TSC. Lower absolute times than prior sessions cannot be attributed to new code: the executable bytes are unchanged.

## Prebuilt runs: 24 alternating pairs

Compilation is excluded. Applications run serially; starting order alternates each pair. Wall time is Python perf_counter around `/usr/bin/time` plus executable; RSS and user/system CPU are measured by GNU time (`%e %M %U %S`). No profiling or test suite runs concurrently with measurements.

| Measurement | Showcase | Standalone |
|---|---:|---:|
| Median wall | **0.657016 s** | **0.688890 s** |
| Minimum wall | 0.638383 s | 0.675471 s |
| Maximum wall | 0.682787 s | 0.729520 s |
| Median peak RSS | **202,386 KiB** | **157,172 KiB** |
| Maximum peak RSS | 234,932 KiB | 157,760 KiB |
| Median user + system CPU per process | **1.455 s** | **0.910 s** |

Showcase median wall time is **4.63% lower** and it wins **24/24 pairs**. Exact two-sided sign-test p = **1.1921e-7**. Exploratory paired bootstrap of the ratio of medians (20,000 resamples, seed 20261026): 95% percentile interval **−5.58% to −3.37%**. Statistical evidence applies to this run protocol/environment, not arbitrary deployments.

Median peak RSS is approximately **28.8% higher**, and median aggregate process CPU approximately **59.9% higher**, for showcase. GNU time CPU values are coarse (hundredths of a second), but they independently support the profile observation that lower latency does not imply lower CPU consumption. These metrics are not CPU utilization percentages or energy measurements.

Every run returned the exact expected order count, audit count, units, gross cents and partition totals. All showcase runs additionally reported accepted/dequeued elements 5,000,000, zero queue/pool drops, zero logged errors, zero pools in use, and 140,625 measured handler calls. Duration/max timing fields are nondeterministic and were not required to be byte-identical.

## Profile method and results

Two independent CPU+heap captures per application, each with ten measured full benchmark iterations. Execution order was showcase p1, standalone p1, showcase p2, standalone p2. Loading/decoding input is included. Standalone uses the existing test-only file overlay, not a production source change.

```sh
go test -o PREFIX.test -run '^$' -bench '^BenchmarkFullShowcase$' \
  -benchmem -benchtime=10x -cpuprofile=PREFIX-cpu.prof \
  -memprofile=PREFIX-mem.prof ./examples/showcase

go test -overlay=/tmp/opencode/indexed-rerun2-overlay.json \
  -o PREFIX.test -run '^$' -bench '^BenchmarkFullStandaloneProfile$' \
  -benchmem -benchtime=10x -cpuprofile=PREFIX-cpu.prof \
  -memprofile=PREFIX-mem.prof ./examples/standalone-showcase
```

| Profile | Benchmark ms/op | B/op | Allocs/op | Total sampled CPU |
|---|---:|---:|---:|---:|
| Showcase p1 | 832.739 | 381,455,555 | 1,009 | 21.30 s |
| Showcase p2 | 886.739 | 381,455,219 | 1,012 | 25.63 s |
| Standalone p1 | 666.408 | 197,559,039 | 102 | 9.81 s |
| Standalone p2 | 668.960 | 197,558,184 | 98 | 9.78 s |

Profile durations: showcase 8.96/9.51 s, standalone 7.34/7.39 s. Sampled CPU exceeds wall because execution is concurrent. Two captures are descriptive, not sufficient for a statistical claim about CPU change.

The profiled benchmark order differs from fresh process latency. Benchmark process reuse, clock initialization, profiling and GC/scheduling conditions differ. Treat ordinary prebuilt wall times as the primary application metric. In particular, first-use TSC calibration occurs once per benchmark process, unlike repeated fresh executables. Profile ns/op should not replace the 24-pair launch result.

## Showcase CPU analysis

Percentages refer to total sampled CPU; cumulative rows include children and overlap.

| Function | p1 flat / cumulative | p2 flat / cumulative |
|---|---:|---:|
| `executorWorker.pass` | **11.41% / 41.97%** | **10.53% / 44.87%** |
| `recordPassMetrics` | **2.96% / 3.15%** | **2.89% / 2.97%** |
| `executorWorker.loop` | 2.35% / 53.15% | 3.16% / 58.60% |
| `runtime.chanrecv` | 5.16% / 7.09% | 5.46% / 8.47% |
| `runtime.selectnbrecv` | 1.78% / 8.87% | 2.30% / 10.77% |
| `runtimeLogger.pass` | **0.61% / 0.80%** | **0.59% / 0.59%** |
| `encoding/json.Unmarshal` | 0% / 31.50% | 0% / 26.06% |

The indexed pass remains a major scheduler self-time contributor, but its cumulative total includes runners/handlers. The previous value-copy and subsequent indexed profiles are different runs with varying sample totals; changes in percentage are not proof of end-to-end improvement. The prior controlled 24-pair indexed-vs-value-loop experiment still stands: estimated −0.40%, p=0.839, no significant full-run benefit.

The worker loop checks its command channel on every pass and yields only after 2^20 consecutive no-progress passes (`graph/executor.go:654–672`). This is consistent with persistent channel polling and scan cost. It does not prove all polling is wasted or identify which worker contributes most; per-group idle/progress counts or a trace are needed before changing scheduling/backoff.

Logger is consistently under 1% self CPU. Worker timing helper has no attributed samples, consistent with independently disabled worker timing. Unpruned timing/hrtime-focused profiles capture only 30 ms each (0.12–0.14%) across timing paths. Sparse timing samples do not quantify the full instrumentation overhead, which includes branches and atomics attributed to callers.

## Standalone CPU analysis

| Function | p1 flat / cumulative | p2 flat / cumulative |
|---|---:|---:|
| `runtime.lock2` | 10.50% / 19.57% | 9.10% / 19.53% |
| `runtime.unlock2` | 9.48% / 11.21% | 9.00% / 10.22% |
| `runtime.procyieldAsm` | 8.36% / 8.36% | 9.41% / 9.41% |
| `runtime.chansend` | 1.94% / 20.59% | 1.84% / 20.35% |
| `runtime.chanrecv` | 2.24% / 17.64% | 2.25% / 16.67% |
| `encoding/json.Unmarshal` | 0% / **56.07%** | 0% / **56.75%** |

Standalone retains the channel synchronization/lock/spin signature. Its lower CPU consumption is observed in both the ordinary runs and profiles. CPU profiles do not measure blocked wall time; a block profile/trace is needed to separate off-CPU channel waiting from on-CPU synchronization.

## Memory analysis

Allocation volume is stable: showcase about **363.78 MiB/op**, standalone **188.41 MiB/op**. The indexed traversal changed neither pool sizing nor application slice growth, so no meaningful volume reduction was expected.

Showcase sampled alloc-space totals **4,011.23–4,018.36 MiB** across calibration plus measured executions:

- `showcase.Run` flat allocations: **1,442.93–1,446.98 MiB**, approximately 36%, principally valid/invalid slice growth.
- JSON-driven `reflect.growslice`: **1,441.73–1,447.45 MiB**, approximately 36%.
- File contents: **627.17 MiB**, approximately 15.6%.
- All three typed `NewPool` rows total **476.69–479.28 MiB**, approximately 11.9%. With eleven actual executions (one calibration plus ten measured), this is about **43.34–43.57 MiB/execution**, subject to heap sampling noise.

Standalone sampled alloc-space totals **2,082.33–2,085.26 MiB**: about 69.5% decoded input slice growth and 30.1% file contents. Its processing/channel allocations are small compared with input loading.

Each benchmark heap profile includes the one-iteration calibration. Do not divide aggregate allocations by ten; use reported B/op for measured per-operation allocation. `Definition.Start` cumulative rows overlap pool allocations and must not be added to them.

Final sampled live heap is **3.00–4.51 MiB** for showcase and about **4.00 MiB** for standalone. These post-teardown samples are not peak RSS and do not establish absence of leaks.

## Recommended next evidence

1. For framework CPU: collect per-group pass/idle ratios and relate them to lifecycle phases before selecting polling/backoff or source-free specialization. Logger and clock reads are not priority targets in these profiles.
2. For startup and memory: isolate the growing valid/invalid slices. Current `Run` starts workers before building those slices, so moving input preparation before worker startup is another testable application-level hypothesis; no gain is claimed and routing/lifecycle semantics must be checked.
3. Keep the indexed-loop change's claim local: eliminated copying and better synthetic dispatch do not yet demonstrate lower complete-launch median against the prior loop.

## Artifacts and limitations

- `/tmp/opencode/indexed-rerun2-app-runs.json`: 48 run records, including CPU/RSS.
- `/tmp/opencode/indexed-rerun2-{showcase,standalone}-*.out`: validated outputs.
- `/tmp/opencode/indexed-rerun2-sha256.json`: source and binary hashes.
- `/tmp/opencode/indexed-rerun2-{showcase,standalone}-p{1,2}.test` and matching `-cpu.prof`, `-mem.prof`, `-bench.txt`.
- Matching `-cpu-top.txt`, `-alloc_space-top.txt`, `-inuse_space-top.txt`; showcase additionally has `-workers.txt` and `-timing.txt`.

The graph index generation is stale (2026-09-02); coverage flags the relevant current paths as changed/untracked. Findings use fresh profiles and current source checks. Temporary artifacts may not survive cleanup; the main measurements are preserved in this report. Full/race tests passed after the loop implementation; no full/race suite was rerun for this measurement-only task.
