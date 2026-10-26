# Current validator showcase vs standalone

Date: 2026-10-26. Measurement-only task; no production code/configuration changes.

## Setup

- Both executables freshly built from the current working tree using Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U. Benchmark GOMAXPROCS: 14.
- Dataset: `examples/data.json`, SHA-256 `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`, one million valid orders.
- Showcase includes the filtering validator, batched ingress without whole-input valid/invalid slices, full TSC handler timing, fixed CPU placements, metrics, and preallocated pools.
- Standalone uses per-order channels of capacity 256, without framework instrumentation or equivalent placement configuration. These are two current implementations, not a controlled isolation of framework overhead.
- Showcase SHA-256: `7af87d07413e3dc04e655f5ee54abb804b688989f194bb868e4c1d433b8d1748` (matches the preceding validator rerun binary).
- Standalone SHA-256: `6b4f64be1bd5eb43f12101d249d170e76d0b5564e00934cb202a5afcb59aa965`.

## Fresh process launches

24 serial pairs with alternating starting order; compilation excluded, full file load/decode included. Wall time uses Python perf_counter around GNU time and the executable; GNU time measures peak RSS and aggregate user+system CPU. No profiling/tests ran concurrently.

| Metric | Showcase | Standalone | Showcase vs standalone |
|---|---:|---:|---:|
| Median wall time | 634.352 ms | 704.554 ms | −9.96% |
| Min–max wall time | 614.936–655.414 ms | 683.958–754.992 ms | |
| Median peak RSS | 173,900 KiB | 157,236 KiB | +10.60% |
| Median aggregate process CPU | 1.140 s | 0.930 s | +22.58% |

Showcase wins 24/24 pairs; exact two-sided sign-test p = 1.1921e-7. Lower launch latency does not mean lower total resource consumption. GNU time CPU readings have hundredth-second resolution and are not utilization/energy metrics.

Every run in both applications had identical order totals:

```text
orders submitted=1000000 processed=1000000 rejected=0 audited=1000000 units=5500000 gross_cents=27636159434 partitions=333300,333400,333300
```

All showcase runs also reported 5,000,000 accepted/dequeued edge elements, zero queue/pool drops, zero logged errors and zero pools in use. This workload does not measure mixed/invalid throughput.

## Fresh profiles

Two independent CPU+heap captures per implementation, ten measured full-workflow benchmark iterations each plus calibration. Execution order: showcase p1, standalone p1, standalone p2, showcase p2. All measurements serial. Standalone benchmark supplied via Go file overlay from the existing `.scratch/showcase-performance/standalone-profile-benchmark.go.txt`; no source file added to the standalone package.

| Capture | Profiled ms/op | B/op | Allocs/op | Total sampled CPU |
|---|---:|---:|---:|---:|
| Showcase p1 | 807.660 | 244,018,492 | 966 | 21.17 s |
| Showcase p2 | 772.502 | 244,017,515 | 963 | 16.79 s |
| Standalone p1 | 659.095 | 197,558,201 | 96 | 9.60 s |
| Standalone p2 | 662.005 | 197,558,622 | 98 | 9.70 s |

**Profiled benchmark order is reversed:** standalone is faster in these reused, profiled benchmark processes. This differs from fresh-process launches. Profiling overhead, process reuse, GC, startup and TSC initialization differ between protocols; these samples do not establish steady-state unprofiled performance. Report the launch result as full-launch latency, not as a universal throughput win.

Allocations: showcase 232.71 MiB/op vs standalone 188.41 MiB/op (+23.52%). After removing whole-input classification buffers, extra fixed graph/pool storage accounts for approximately 44.31 MiB/op of allocation difference.

### CPU

- Showcase: worker pass self CPU 7.92–11.67%; cumulative 35.91–44.07% includes application runners/handlers. Worker loop cumulative 46.87–56.35%, overlapping pass and nested runtime work. JSON Unmarshal cumulative 30.18–40.38%.
- Standalone: channel send cumulative 20.52–20.73%; channel receive and channel locking/spinning also visible. `runtime.lock2` self CPU 9.59–11.25% (cumulative 19.07–20.00%). JSON Unmarshal cumulative 57.32–57.40%.
- Cumulative CPU percentages overlap and must not be summed. Different denominators mean JSON's larger percentage in standalone does not imply slower decoding there.
- Profiles suggest graph dispatch/polling and per-record channel synchronization are the main respective orchestration costs; they do not alone prove which mechanism causes the launch difference.

### Allocation space

Showcase p1: JSON-related `reflect.growslice` 56.13%, file buffer 24.50%, order pools 12.64%, receipt pools 5.58%. Standalone p1: slice growth 69.40%, file buffer 30.20%, with no comparable large graph pools. Profile volume is cumulative across calibration and measured operations, not peak memory.

## Artifacts

`/tmp/opencode/validator-vs-standalone-20261026/` contains both application/test binaries, launch `*.out`/`*.time`, `launches.json`, overlay configuration, and `{showcase,standalone}-p{1,2}-{cpu,mem}.prof`, benchmark text and top tables. Launch runner: `/tmp/opencode/validator-standalone-compare.py`.

```sh
go tool pprof -http=127.0.0.1:8080 /tmp/opencode/validator-vs-standalone-20261026/showcase.test /tmp/opencode/validator-vs-standalone-20261026/showcase-p1-cpu.prof
go tool pprof -top -alloc_space /tmp/opencode/validator-vs-standalone-20261026/standalone.test /tmp/opencode/validator-vs-standalone-20261026/standalone-p1-mem.prof
```
