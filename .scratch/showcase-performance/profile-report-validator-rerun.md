# Showcase validator rerun and profiles

Date: 2026-10-26. Measurement only: production code/configuration unchanged.

## Configuration and baseline

- Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U; benchmark GOMAXPROCS 14.
- Dataset SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460` (one million valid orders).
- Current executable freshly built from the working tree: `/tmp/opencode/showcase-validator-rerun-20261026`.
- Baseline: preserved `/tmp/opencode/showcase-pass-indexed-after`, from the earlier indexed-scheduler experiment. Unlike the previous conversation's ten-pair comparison against `showcase-tsc-full-rerun`, this baseline already includes indexed runner traversal. Both configurations use full TSC handler timing and the same capacities/metrics interval.
- Baseline executable SHA-256: `f966a27aa39f86829d64174c6509d5b54ec2b7b5e3e7fb873975ae81b3e556b3`.
- Current executable SHA-256: `7af87d07413e3dc04e655f5ee54abb804b688989f194bb868e4c1d433b8d1748`.
- Baseline is a preserved binary, not a newly reconstructed source snapshot. Results compare these executables; attribution of every difference to one source edit is not independently established.

## Full application: 24 alternating serial pairs

Compilation excluded. Python perf_counter wraps GNU time and executable startup/completion. GNU time records peak RSS and user+system process CPU. No profiles/tests run concurrently with launches.

| Metric | Before | After |
|---|---:|---:|
| Median wall time | 675.505 ms | 626.552 ms |
| Minimum wall time | 651.335 ms | 609.785 ms |
| Maximum wall time | 703.884 ms | 641.731 ms |
| Median peak RSS | 201,326 KiB | 174,258 KiB |
| Median aggregate process CPU | 1.525 s | 1.120 s |

Wall time is **7.25% lower**, with after winning **24/24** pairs; exact two-sided sign-test p = **1.1921e-7**. This supports a difference under this protocol/environment, not arbitrary deployments. Median RSS is 13.44% lower; median aggregate CPU is 26.56% lower. CPU totals use coarse hundredth-second GNU time readings, and are not utilization or energy measurements.

All 48 executions matched semantic output (excluding nondeterministic handler timing):

```text
orders submitted=1000000 processed=1000000 rejected=0 audited=1000000 units=5500000 gross_cents=27636159434 partitions=333300,333400,333300
runtime edges=10 accepted_elements=5000000 dequeued_elements=5000000 queue_drop_elements=0 pool_drop_elements=0 logged_errors=0 pools_in_use=0
```

These data exercise the all-valid path, not mixed/invalid throughput.

## Fresh CPU/heap captures

Two independent captures per variant, each ten measured BenchmarkFullShowcase iterations, plus Go's calibration invocation. Runs were serial: before p1, after p1, before p2, after p2. Full workflow includes reading/decoding the dataset. Baseline test executable: `/tmp/opencode/indexed-rerun2-showcase-p1.test`; current test executable was freshly compiled with `go test -c`.

| Variant | Capture | Profiled ms/op | B/op | Allocs/op | Sampled CPU total |
|---|---|---:|---:|---:|---:|
| Before | p1 | 858.207 | 381,457,390 | 1,014 | 23.98 s |
| Before | p2 | 869.028 | 381,456,696 | 1,009 | 23.74 s |
| After | p1 | 805.833 | 244,019,283 | 968 | 19.47 s |
| After | p2 | 760.163 | 244,018,690 | 965 | 16.08 s |

Allocation volume falls from approximately **363.79 to 232.71 MiB/op (−36.03%)**. Two profiled samples do not establish a statistically significant benchmark timing/CPU delta; use unprofiled application launches for latency. Benchmark reuse, profiling and clock initialization differ from fresh-process execution.

### Allocation findings

In after p1 `alloc_space`:

- `reflect.growslice` via JSON array decoding: **56.23%**.
- File-content allocation: **24.36%**.
- Order pools: **12.77%**; receipt pools: **5.48%**.
- `Run` no longer has the previous huge flat allocation row (before p1: 36.22%). It still cumulatively includes graph/pool initialization.

Allocation profile totals aggregate calibration plus measured runs; do not read the roughly 1.45 GB grow-slice row as per-operation memory. Input JSON decoding still grows its own output slice even though Run's second million-order slice has been removed.

Final after `inuse_space` samples are about 5.51/3.00 MiB. These are teardown snapshots, not peak memory or proof of no leaks.

### CPU findings

| Current path | p1 | p2 | Meaning |
|---|---:|---:|---|
| executorWorker.pass flat | 9.40% | 8.46% | Scan/dispatch self CPU |
| executorWorker.pass cumulative | 41.40% | 36.57% | Includes runners/handlers, not pure overhead |
| executorWorker.loop cumulative | 53.16% | 45.34% | Overlaps pass and nested runtime work |
| encoding/json.Unmarshal cumulative | 33.95% | 41.67% | Application input decoding |
| receipt-producing normalized runner cumulative | 12.43% | 11.01% | Framework runner plus application work |
| recordPassMetrics flat | 3.03% | 1.80% | Pass accounting |
| runtimeLogger.pass flat | 0.46% | 0.31% | Not a leading target |

Percentages are shares of aggregate CPU samples, not wall latency; cumulative rows overlap. JSON Unmarshal cumulative absolute CPU is 6.61/6.70 s across the captures, while worker CPU varies more. Source confirms worker command polling each pass and yielding only after 2^20 no-progress passes (`graph/executor.go:654–672`). Profiles alone do not quantify wasted idle CPU or show which worker is responsible.

Next measurement candidates, not implemented here:

1. For full-launch memory/latency, isolate JSON slice growth/loading: input loading accounts for about 80.6% of after-p1 allocation space.
2. For framework CPU, isolate worker scan/poll/runner dispatch, measuring progress/idle ratios before choosing scheduling changes.
3. Avoid prioritizing logger or clock changes on this evidence; sparse clock samples do not measure total timing instrumentation cost.

## Artifacts and reproducibility

Raw launch outputs, timings and summary: `/tmp/opencode/validator-profile-rerun-20261026/{before,after}-NN.{out,time}` and `launches.json`.

Fresh profiles: `/tmp/opencode/validator-profile-rerun-20261026/{before,after}-p{1,2}-{cpu,mem}.prof`, corresponding `-bench.txt`, `-cpu-top.txt`, `-alloc_space-top.txt`, `-inuse_space-top.txt`. Old profile files were not overwritten.

```sh
go tool pprof -http=127.0.0.1:8080 /tmp/opencode/validator-profile-rerun-20261026/after.test /tmp/opencode/validator-profile-rerun-20261026/after-p1-cpu.prof
go tool pprof -top -alloc_space /tmp/opencode/validator-profile-rerun-20261026/after.test /tmp/opencode/validator-profile-rerun-20261026/after-p1-mem.prof
```

Graph index generation is stale (2026-09-02); coverage flags current paths as changed/untracked. Material findings use fresh profiles and current source fallback. No production edits or commits were made during this rerun.
