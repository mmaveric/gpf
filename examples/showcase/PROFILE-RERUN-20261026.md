# Showcase profiles — 2026-10-26 rerun

Freshly rebuilt application executables:

- GPF showcase: `/tmp/opencode/showcase-manual-review-20261026`
- Standalone control: `/tmp/opencode/standalone-manual-review-20261026`
- Dataset SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`
- Go: 1.26.8, linux/amd64, Intel Core Ultra 7 255U

Each executable was run once after building. Both reported exact expected totals for 1,000,000 orders. Showcase additionally reported 5,000,000 accepted/dequeued elements, zero drops/errors/pools in use, and successful placement of all seven groups.

## Profile files in this directory

| App | CPU profile | Heap profile |
|---|---|---|
| GPF showcase | `showcase-rerun-20261026-cpu.prof` | `showcase-rerun-20261026-mem.prof` |
| Standalone | `standalone-rerun-20261026-cpu.prof` | `standalone-rerun-20261026-mem.prof` |

These profiles came from 10-iteration `BenchmarkFullShowcase` / equivalent standalone benchmark runs; reading and JSON decoding are included. The standalone benchmark exists only through the file overlay at `/tmp/opencode/showcase-manual-review-overlay.json`; no standalone source file was modified. The previous files `showcase.cpu.prof` and `showcase.mem.prof` were not overwritten.

Profiles contain symbolized Go function names and can be opened directly without the temporary test binaries:

```sh
go tool pprof examples/showcase/showcase-rerun-20261026-cpu.prof
go tool pprof examples/showcase/showcase-rerun-20261026-mem.prof
go tool pprof examples/showcase/standalone-rerun-20261026-cpu.prof
go tool pprof examples/showcase/standalone-rerun-20261026-mem.prof
```

Useful non-interactive views:

```sh
go tool pprof -top examples/showcase/showcase-rerun-20261026-cpu.prof
go tool pprof -top -sample_index=alloc_space examples/showcase/showcase-rerun-20261026-mem.prof
go tool pprof -top examples/showcase/standalone-rerun-20261026-cpu.prof
go tool pprof -top -sample_index=alloc_space examples/showcase/standalone-rerun-20261026-mem.prof
```

## Summary

### GPF showcase

- Benchmark under profiler: **800.447 ms/op**, **381,455,043 B/op**, **1,011 allocs/op**.
- CPU profile: 8.64 s elapsed, 21.76 s sampled CPU (workers run concurrently).
- `executorWorker.pass`: **10.02% flat**, **43.52% cumulative**. Cumulative time includes runner functions and application work; it is not pure scheduler overhead.
- `recordPassMetrics`: **3.81% flat**.
- `runtime.chanrecv`: **5.19% flat**; `runtime.selectnbrecv`: **1.75% flat**.
- `runtimeLogger.pass`: **0.55% flat**.
- `encoding/json.Unmarshal`: **19.99% cumulative**.
- Heap `alloc_space`: 4,014.15 MiB aggregate, including calibration plus ten measured benchmark operations. Main rows: `reflect.growslice` 1,447.93 MiB (36.07%), `showcase.Run` 1,436.55 MiB flat (35.79%), file reads 627.17 MiB (15.62%), typed pools (order + receipt) 473.35 MiB plus the small ingress pool. Do not add cumulative rows to these flat values.
- `inuse_space` after teardown: 3.01 MiB sampled. This is not peak heap/RSS.

### Standalone

- Benchmark under profiler: **655.651 ms/op**, **197,558,232 B/op**, **98 allocs/op**.
- CPU profile: 7.23 s elapsed, 9.66 s sampled CPU.
- Runtime synchronization: `lock2` **10.14% flat**, `procyieldAsm` **9.83%**, `unlock2` **9.01%**; `chanrecv` 2.48% flat and `chansend` 1.76% flat.
- Heap `alloc_space`: 2,081.55 MiB aggregate, including benchmark calibration. `reflect.growslice` 1,444.70 MiB (69.41%), file reads 627.17 MiB (30.13%).

Profiled benchmark times are not directly comparable to fresh prebuilt process latency: profiling overhead, benchmark process reuse, initialization, GC and scheduling differ. For a more statistically robust full-launch comparison, see `.scratch/showcase-performance/profile-report-indexed-rerun2.md` (24 alternating pairs).

Profiles were parsed with `go tool pprof`; all commands completed successfully. Full tests were not rerun for this profiling-only request. No production source/configuration changes were made.
