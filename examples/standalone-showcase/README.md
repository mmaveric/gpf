# Standalone order-processing showcase

This standard-library-only control implementation mirrors
[Showcase](../showcase/) using bounded goroutine/channel transport, without
importing GPF. Commands below run from the repository root with Go 1.22 or newer.

## Run and generate input

Tests use small in-memory or test-owned temporary inputs; the large JSON files
are optional and not needed for correctness tests:

```sh
go test ./examples/standalone-showcase
```

Generate files only when the destinations do not already exist. The generator
refuses overwrites and symlinks, so existing local datasets remain unchanged:

```sh
go run ./examples/testdata/generate -valid-out examples/data.json
go run ./examples/testdata/generate -mixed-out examples/testdata/orders-5pct-invalid.json
go run ./examples/standalone-showcase
go run ./examples/standalone-showcase -data examples/testdata/orders-5pct-invalid.json
```

Default loading searches `examples/data.json`, then `../data.json`, relative to
the working directory. `-data PATH` selects one explicit path with no fallback;
use an absolute path for a binary launched elsewhere. `-h` prints help. No
environment variable selects the fixture. Output directories must exist; write
failures may leave a partial new output. Existing outputs are never replaced.
For exact formulas, encoding and pinned hashes, see
[Showcase's generation instructions](../showcase/#run-and-generate-input).

## Pipeline and validation

The producer sends each order through an ingress channel. Validation sends valid
records to an audit worker and to partition `CustomerID % 3`; invalid records
increment the rejected count and do not enter either valid path. ID, units and
unit price must be nonzero; customer ID zero is valid. Three partition workers
produce receipts for a fan-in aggregator. Pipeline transport channels each hold
256 **individual records**, unlike Showcase's batch-slot capacities. Completion
channels hold one result (three for the partition results).

The producer closes ingress; the validator closes downstream inputs; the caller
joins partition workers before closing fan-in and then receives aggregate
totals. Cross-partition ordering is unspecified. Backpressure blocks on full
channels; this example has no drop mode or cancellation API. Report-write errors
are returned only after workers have completed.

It intentionally does not implement GPF's runtime metrics, CPU affinity, typed
error transport, numeric logging, pooling or per-handler timing. Its final line
is `runtime mode=standalone framework=none`.

Default input:

```text
orders submitted=1000000 processed=1000000 rejected=0 audited=1000000 units=5500000 gross_cents=27636159434 partitions=333300,333400,333300
```

Mixed input:

```text
orders submitted=1000000 processed=950000 rejected=50000 audited=950000 units=5225000 gross_cents=26253957313 partitions=317300,315400,317300
```

These summaries match Showcase. Expected invalid records are ordinary data
routing there, not failed handler invocations or logged errors.

## Benchmarks without overlays

The checked-in benchmarks match Showcase's boundaries:

- `BenchmarkFullShowcase`: file read, JSON decoding, pipeline creation/start,
  processing, worker completion and report formatting to `io.Discard`.
- `BenchmarkLoadOrders`: file read and JSON decoding only.
- `BenchmarkRun`: pipeline creation/start, processing, worker completion and
  reporting over prebuilt synthetic input. Fixture generation/loading is not
  timed. Its million-order synthetic fixture matches internal Showcase's
  `BenchmarkRun`, not the generated JSON: customer IDs cycle through 100,000,
  units through 10, and prices through 1,000, starting at 1.

```sh
go test -run='^$' -bench='^BenchmarkFullShowcase$' -benchmem -benchtime=3x -count=10 ./examples/showcase
go test -run='^$' -bench='^BenchmarkFullShowcase$' -benchmem -benchtime=3x -count=10 ./examples/standalone-showcase
go test -run='^$' -bench='^BenchmarkLoadOrders$' -benchmem ./examples/standalone-showcase
go test -run='^$' -bench='^BenchmarkRun$' -benchmem ./examples/standalone-showcase
```

Both full/load benchmark packages accept
`-args -showcase-data=/absolute/path/to/orders.json`; defaults use the shared
generated valid file. No source edits, environment overrides, or temporary
overlays are needed. Showcase's full benchmark defaults to **standard** monotonic
handler timing; TSC is an explicit separate experiment with
`-args -showcase-clock=tsc`. Standalone has no handler-clock flag or timing.

Run comparisons serially on the same host/toolchain with the same input.
Full workflow includes neither process launch nor compilation; allocations cover
the same scope as time. Showcase includes metrics/affinity/timing/reporting that
this control omits, so the difference is not a pure framework overhead estimate.
Single-iteration runs are smoke checks, not statistical performance evidence.
Existing profile reports describe historical source/configuration and do not
establish a baseline for the new standard-clock default. Save fresh profiles to
new paths outside the repository instead of overwriting local artifacts.
