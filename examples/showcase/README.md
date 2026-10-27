# Order-processing showcase

A bounded GPF pipeline demonstrates typed ingress, validation, broadcast,
customer sharding, fan-in, drain, runtime metrics, and handler timing. For a
goroutine/channel control implementation, see [standalone-showcase](../standalone-showcase/).
Commands below run from the repository root with Go 1.22 or newer.

## Run and generate input

Ordinary tests generate their inputs in memory or in test-owned temporary files;
they do not need the large JSON fixtures:

```sh
go test ./examples/showcase/...
```

For applications and file-loading benchmarks, generate the optional fixtures
once. **The generator refuses existing destinations, including symlinks.** If
you already have these local files, leave them in place and skip generation.

```sh
go run ./examples/testdata/generate -valid-out examples/data.json
go run ./examples/testdata/generate -mixed-out examples/testdata/orders-5pct-invalid.json
go run ./examples/showcase
go run ./examples/showcase -data examples/testdata/orders-5pct-invalid.json
```

Each file has one million records. Defaults search `examples/data.json`, then
`../data.json`, relative to the working directory. `-data PATH` selects one
explicit file with no fallback; this also works for a prebuilt executable from
another directory. `-h` describes the flags. No environment variables alter data
or timing configuration.

For zero-based record `i`, generation uses `id=i+1`, `customerId=i%10000+1`,
`units=i%10+1`, and `unitCents=50+(i*37)%9951`. Mixed generation invalidates record
`20*g+g%20` in each block `g`, cycling the zeroed field through `id`, `units`, and
`unitCents`. Files contain compact records separated by `,\n`, enclosed by
`[\n` and `\n]\n`. Generator tests pin these SHA-256 hashes:

| Fixture | SHA-256 |
|---|---|
| Valid | `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460` |
| 5% invalid | `e77a1fcf1b062915b644308422ca53601e83550d547c63de4cd453ad783a15f0` |

Output directories must already exist. If generation fails while writing, a
partial new file may remain; existing files are never replaced. The two output
options are independent, not an atomic two-file transaction. Large generated
JSON files and local profiles are not source prerequisites.

## Topology and validation

```text
typed ingress -> validator -- valid --> broadcast --> audit sink
                   |                       `--> shard relay --> shard by CustomerID % 3
                   |                                              |--> partition 0 --┐
                   |                                              |--> partition 1 --+--> fan-in --> aggregate
                   |                                              `--> partition 2 --┘
                   `-- invalid (ordinary typed output) --> error sink
```

An order is valid when its ID, units and unit price are nonzero; customer ID zero
is allowed. The validator separates valid and invalid records **within each
batch**, preserving valid records alongside invalid ones. Expected invalid data
does not fail the handler, use `ConnectError`, or emit numeric logger events.
The configured async logger remains available for mapped handler failures, but
invalid orders are not logged by this example. A failed handler invocation can
discard its staged outputs; returning failure is not a per-record filter.

- Broadcast creates separate shallow-copy branch batches. Order records contain
  no pointer referents.
- Sharding uses the partition input order shown above, splitting batches and
  flushing partial branch batches. Cross-producer fan-in order is unspecified;
  the aggregate counts and sums are order-independent.
- Seven fixed groups request CPUs 0–6. Affinity support and allowed CPUs depend
  on the host; the report prints each placement outcome and warning.
- Submission retries temporary ingress fullness/exhaustion with `Gosched`.
  Edges use `queue.Wait`, not drop policies. These finite workloads are checked
  for exact totals, zero queue/pool drops and zero pools in use after drain.
  This is not a guarantee for arbitrary workloads, nor a cancellation-aware API.

Configured capacity is in batch slots; batch capacity is the maximum number of
items per batch. Shared broadcast/shard/merge plans have topology-specific pool
ownership, so these products are not peak heap/RSS estimates.

| Transport | Capacity | Batch capacity |
|---|---:|---:|
| Ingress source and ingress edge | 256 | 64 |
| Validator invalid output | 256 | 64 |
| Valid broadcast | 256 | 64 |
| Customer shard | 12,288 | 32 |
| Receipt fan-in | 4,096 | 32 |

The logger capacity is 4 events. Group metrics flush every 2,048 passes and
finally at drain. Reports include edge transfers/losses, delivered logger events,
pool usage, handler timing and affinity outcomes.

## Timing: standard by default, explicit TSC option

Full handler timing measures every handler invocation with Go's standard
monotonic clock by default. The report says `handler_timing_clock=standard`.
It measures handler intervals, not end-to-end latency or pure scheduler cost.

```sh
go run ./examples/showcase -clock standard
go run ./examples/showcase -clock tsc
```

`-clock tsc` explicitly requests approximate calibrated TSC timing. Unsupported
processors return a timing configuration error; there is no silent fallback.
The internal API's `Run` and zero-value `Config` use standard timing;
`RunWithConfig(..., Config{Clock: "tsc"})` opts in. Existing profile reports are
historical measurements of older source/configuration, not current baselines.

## Expected totals

Default valid input:

```text
orders submitted=1000000 processed=1000000 rejected=0 audited=1000000 units=5500000 gross_cents=27636159434 partitions=333300,333400,333300
```

The runtime reports 10 edges, 5,000,000 accepted/dequeued edge elements,
`queue_drop_elements=0 pool_drop_elements=0 logged_errors=0 pools_in_use=0`.

Mixed input:

```text
orders submitted=1000000 processed=950000 rejected=50000 audited=950000 units=5225000 gross_cents=26253957313 partitions=317300,315400,317300
```

Mixed input also has zero drops, logged handler errors and pools in use after
drain. Timing values, measured-call counts and affinity details can vary.

## Reproducible benchmark boundaries

Run applications/benchmarks sequentially on the same machine and toolchain.
Build/test cache writes are normal. Neither app needs a file overlay.

| Benchmark | Timed work | Fixture |
|---|---|---|
| `BenchmarkFullShowcase` | Read file, decode JSON, construct/start pipeline, process, drain/join, format report to `io.Discard` | Same generated JSON for both apps |
| `BenchmarkLoadOrders` | Read file and decode JSON only | Same generated JSON for both apps |
| Internal `BenchmarkRun` | Construct/start pipeline, process, drain, format report | Prebuilt synthetic million valid orders; no JSON loading |

Full workflow does not include process launch or compilation. Allocation counts
cover the same boundary as time. Showcase includes its metrics, timing, affinity
and reporting overhead; the standalone has none of those framework facilities.
This is a comparison of the two implementations, not an isolated framework tax.

```sh
go test -run='^$' -bench='^BenchmarkFullShowcase$' -benchmem -benchtime=3x -count=10 ./examples/showcase
go test -run='^$' -bench='^BenchmarkFullShowcase$' -benchmem -benchtime=3x -count=10 ./examples/standalone-showcase
go test -run='^$' -bench='^BenchmarkLoadOrders$' -benchmem ./examples/showcase
go test -run='^$' -bench='^BenchmarkLoadOrders$' -benchmem ./examples/standalone-showcase
```

Showcase's full benchmark names the selected clock in a sub-benchmark. Its
test-only `-showcase-clock` flag defaults to `standard`, independent of CLI flags
and environment. Explicit TSC measurement:

```sh
go test -run='^$' -bench='^BenchmarkFullShowcase$' -benchmem -benchtime=3x ./examples/showcase -args -showcase-clock=tsc
```

Both full/load benchmark packages accept `-args -showcase-data=/absolute/path`
to select another file, including mixed input. Default benchmark paths resolve
from each package directory. Both compute-only benchmarks instead use identical
synthetic formulas: customer IDs cycle through 100,000, units through 10, prices
through 1,000 (starting at 1). Those totals differ from the JSON dataset. Internal
Showcase `BenchmarkRun` always uses standard timing:

```sh
go test -run='^$' -bench='^BenchmarkRun$' -benchmem ./examples/showcase/internal/showcase
go test -run='^$' -bench='^BenchmarkRun$' -benchmem ./examples/standalone-showcase
```

Use repeated samples for analysis; a one-iteration smoke test establishes only
that a benchmark runs. Profile into new external paths (for example under
`/tmp/opencode`) rather than overwriting historical `.prof` files. Profiling
changes timings; do not treat profiled results as unprofiled latency evidence.
The local `topology.svg` was not regenerated during this source stabilization
and is not authoritative for current routing or capacities.
