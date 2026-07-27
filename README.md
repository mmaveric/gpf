# Go Pipeline Framework

This repository is building a typed, batch-oriented Go pipeline framework. The
current stage defines public contracts and measurement infrastructure only;
there is no executable graph, pool, scheduler, queue, or runtime yet.

Future steady-state data paths use concrete generic types and typed function
fields. They do not use reflection, runtime type assertions, or `interface{}`
values; closures remain permitted when they preserve those constraints.

The module path is `github.com/asharipov/gpf` and the minimum supported Go
version is Go 1.22.

## Benchmarks

Run the benchmark matrix with:

```bash
go test -run '^$' -bench . -benchmem ./...
```

Current data-plane benchmark entries are skipped until their corresponding
implementation exists. Future implementations must call `b.ReportAllocs()` and
use the allocation assertion after setup completes.

The benchmark matrix reserves these stable names:

- `node-call/batch=64`
- `pool-acquire-release/batch=64`
- `local-edge/batch=64/group=1`
- `remote-queue/batch=64/groups=2`
- `end-to-end-dag/batch=64/groups=2/depth=3`

Throughput and latency results are recorded for manual comparison rather than
enforced as merge-blocking thresholds. A passing zero-allocation assertion does
not rule out instruction-count, cache-miss, throughput, or tail-latency
regressions.

## Typed Connection Check

Matching ports connect through `graph.Connect[T]`. The positive example is in
`graph/example_test.go`. To inspect the intentional compile failure for
mismatched ports, see `graph/testdata/compilefail/connect_mismatched.go`.
