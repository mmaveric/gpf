# Mixed dataset: showcase vs standalone

Date: 2026-10-26. Measurement only; no application or dataset changes.

## Protocol

- Both applications freshly built with Go 1.26.8, linux/amd64, on the same environment as the earlier all-valid comparison (Intel Core Ultra 7 255U).
- Dataset: `examples/testdata/orders-5pct-invalid.json`, one million orders, exactly 50,000 invalid.
- Dataset SHA-256: `e77a1fcf1b062915b644308422ca53601e83550d547c63de4cd453ad783a15f0`.
- 24 serial paired launches, alternating starting order. No simultaneous profiling/benchmarking. Compilation excluded; reading and decoding JSON included.
- Run from an isolated working directory whose `examples/data.json` symlink targets the mixed fixture. Default repository data paths/files were not changed.
- Python perf_counter measures wrapper+process elapsed time; GNU time measures peak RSS and user+system process CPU.
- Showcase binary SHA-256: `7af87d07413e3dc04e655f5ee54abb804b688989f194bb868e4c1d433b8d1748`.
- Standalone binary SHA-256: `6b4f64be1bd5eb43f12101d249d170e76d0b5564e00934cb202a5afcb59aa965`.
- Both binary hashes match the preceding all-valid comparison. Showcase includes full TSC handler timing, fixed CPU placements, metrics and fixed pools; standalone uses per-record channels without equivalent framework instrumentation/placements. This compares current applications, not isolated framework overhead.

## Results

| Metric | Showcase | Standalone | Showcase relative to standalone |
|---|---:|---:|---:|
| Median full-launch wall time | 623.823 ms | 689.029 ms | −9.46% |
| Minimum | 611.476 ms | 683.230 ms | |
| Maximum | 666.691 ms | 725.869 ms | |
| Median peak RSS | 173,928 KiB | 157,044 KiB | +10.75% |
| Median aggregate process CPU | 1.170 s | 0.890 s | +31.46% |

Showcase was faster in all 24 pairs; exact two-sided sign-test p = 1.1921e-7. This supports lower full-launch latency under this protocol, not a universal steady-state throughput advantage. GNU time CPU resolution is hundredths of a second; CPU totals are not utilization/energy readings.

In the earlier, separately measured all-valid dataset comparison, showcase's median was 634.352 ms vs standalone 704.554 ms (−9.96%). The relative advantage remains similar with 5% invalid input (−9.46%). Do not attribute changes in absolute times solely to invalid records: datasets were measured in separate time windows, and rejected orders skip downstream work.

## Correctness

All 48 runs matched:

```text
orders submitted=1000000 processed=950000 rejected=50000 audited=950000 units=5225000 gross_cents=26253957313 partitions=317300,315400,317300
```

Every showcase run also reported:

```text
runtime edges=10 accepted_elements=4850000 dequeued_elements=4850000 queue_drop_elements=0 pool_drop_elements=0 logged_errors=0 pools_in_use=0
```

Affinity reports were consistent across all showcase runs. The error sink receives ordinary validation-output batches; invalid records do not imply logged handler failures.

## Artifacts

- `/tmp/opencode/mixed-showcase-vs-standalone-20261026/launches.json`: summary and every wall/RSS/CPU sample.
- Same directory: freshly built binaries, per-run stdout and GNU time files, and isolated `workdir/examples/data.json` symlink.
- Runner: `/tmp/opencode/mixed-showcase-standalone-compare.py`.

No fresh CPU/heap profiles or allocation benchmarks were collected in this comparison. No source changes were made and the test suite was not rerun; all measured launches were validated against exact expected totals and no-loss metrics.
