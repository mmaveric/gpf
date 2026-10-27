# Quickstart: ingress → transform → sink

Run a small graph that parses decimal integers and routes invalid input to a
separate **typed error sink**. No input files, network services, or extra CLI
dependencies are needed.

```text
string ingress (#1) → parser (#2) → integer sink (#3)
                         └─ parseFailure → error sink (#4)
```

## Run and test

From the repository root, using the Go version required by `go.mod` or newer:

```sh
go run ./examples/quickstart
go test ./examples/quickstart
go test -race ./examples/quickstart
```

Expected standard output:

```text
numbers: [10 30]
rejected "not-a-number": not a decimal integer
```

The failure is an expected data result, not an executable failure. Setup,
publication, DOT-writing, or shutdown errors exit nonzero. The tests cover mixed
results, all-success and all-failure inputs, empty input, rejection paths, DOT
edges, and zero outstanding pool batches after drain. They do not use sleeps or
assume which CPU-affinity outcome the host permits.

All nodes share the default execution group and one worker. The builder requires
an explicit CPU binding, so `-cpu` requests CPU 0 by default:

```sh
go run ./examples/quickstart -cpu 2
```

Choose an ID allowed by your host/container if you want affinity applied. CPU 0
is **not** assumed to be available. If the platform cannot apply the requested
affinity, GPF continues on its locked OS thread and the example prints a warning
to standard error. This portable fallback is not CPU pinning. Negative CPU IDs
are configuration errors.

## Export the topology

```sh
go run ./examples/quickstart -dot /tmp/quickstart.dot
dot -Tsvg /tmp/quickstart.dot -o /tmp/quickstart.svg
```

Only rendering requires Graphviz (`dot`); generating DOT does not. `-dot` creates
or truncates the named file. It calls `Definition.WriteDOT` before starting the
executor. Labels identify node kinds/IDs and distinguish data from error edges;
this is logical topology, not a runtime timing or placement visualization.

## Follow the code

- [`main.go`](main.go) supplies `10`, `not-a-number`, and `30`, prints results,
  handles Ctrl-C, and optionally opens the DOT file.
- [`pipeline.go`](pipeline.go) registers ingress and closure nodes, uses
  `Connect` for data and `ConnectError` for failures, binds a CPU, builds and
  starts the definition, publishes input, and drains the executor.
- The parser returns `node.Result[parseFailure]{Failed: true, Value: ...}` on
  invalid input. The `Failed` flag triggers routing; the value is concrete
  `parseFailure` data, not an untyped `error` payload. Valid strings produce an
  output batch of `int` values instead.

Each handler invocation sees one element. The finite demo accepts at most four
strings, so it can enqueue its entire input even before the worker runs. This is
an ownership/lifecycle example, not a throughput benchmark or streaming producer.

## Batch ownership and bounds

`Capacity: 4` bounds each queue in **batches**. `BatchCapacity: 1` bounds each
batch in **elements**. These are different limits. The ingress source pool is
also bounded; enlarging only a queue is not a general solution to pool pressure.

- `Ingress.Submit` copies caller data into an acquired ingress batch. The caller
  keeps its original slice; `Submit` transfers or releases its internal batch on
  every outcome. This example checks for `IngressAccepted` on every submission.
- If using `Ingress.Acquire`/`Commit` instead, `IngressAccepted` and
  `IngressDropped` consume and clear the batch handle. `IngressFull` and
  `IngressClosed` leave it caller-owned: retry or `Release` it. Do not lose a
  rejected handle, use it after transfer, or release it twice.
- The parser's `Output.Acquire` returns one owned output batch. It fills the
  storage, calls `SetLen`, and commits it. A successful output `Commit` moves
  the handle for post-handler publication; it is not an immediate queue push.
  The example releases the batch if commit fails.
- The runtime releases handler input when the handler returns unless the
  handler moves or explicitly releases it. These handlers only read inputs.
  Sinks copy values into result slices; they do not retain `Items()` slices
  backed by pooled memory.
- `DrainContext` closes ingress admission, drains accepted work, and joins the
  worker before the caller reads results. A deferred `Close` is the early-error
  safety net; unlike a successful drain, `Close` discards pending work.

All edges and ingress use `queue.Wait`. On a full **graph edge**, Wait retains
pending output for scheduler retry; Drop releases it instead. On **external
ingress**, Wait does not block the producer: `Commit`/`Submit` returns
`IngressFull`, so a streaming producer must implement context-aware retry.
Drop returns `IngressDropped` on a full ingress queue and consumes the batch.
Pool exhaustion is a separate condition (`IngressExhausted` for `Submit`), not
proof that the queue is full. Wait does not create unlimited storage or eliminate
the need to check acquisition/publication results.

## When to add workers

Start with one group, as here. Measure a bottleneck before adding parallelism;
more workers also introduce cross-worker transport and more in-flight storage.

| Need | Choose | Why / boundary |
| --- | --- | --- |
| Pipeline stages should progress independently | Manual stage groups with `Place` and `BindCPU` | Separate workers for stages; does not duplicate a stage. |
| Scale an entire downstream subpipeline | `ReplicateGroup` | Copies every node in that group; keep ingress outside a group replicated more than once. |
| Only one expensive node needs more workers | `ReplicateNode` (node worker pool) | Keeps neighbors unchanged; shared input queues distribute batches to workers. |
| Route elements by key or another explicit choice | `ConnectShard` to separate branches | Selector chooses the destination per element; replicas/pools alone do not provide key affinity. |

Worker counts are fixed at build time and require complete, distinct CPU bindings.
Each factory must create fresh mutable instance state; captures are not deep-copied.
Parallel paths promise no global output order. Shard endpoints cannot themselves
be replica/node-pool expanded. See [physical parallelism and its limits](../../spec/03-topologies-transport.md#physical-parallelism)
and [element sharding](../../spec/03-topologies-transport.md#element-sharding).

## Practical capacity tuning

Keep this four-string demo unchanged; tune a real workload with representative
payloads, arrival bursts, and a latency target, changing one limit at a time.

| Knob / signal | Practical interpretation |
| --- | --- |
| Batch size | Larger batches can amortize work; if the producer waits to fill them, latency rises. Smaller batches can reduce that wait but increase per-batch overhead. There is no universal best size. |
| Queue capacity | More slots can absorb a burst, not fix a sustained slow consumer; queued work also increases latency and memory use. |
| Pool capacity / acquisitions | Queued, pending, and downstream-held batches remain in flight. A free queue slot does not imply an available pool slot; budget storage for the whole ownership path, not just queue depth. |
| Pressure / loss | Inspect queue depth/high-water, pool exhaustion, Wait retries, and drop counters separately. Check latency and throughput together before keeping a change. |

Wait backpressure does not prevent acquisition failure. Verify every acquire and
publication result; see [metric fields](../../graph/metrics.go) for distinct loss
boundaries rather than treating every pressure counter as dropped data.

## Troubleshoot construction and validation

Check each registration/connection error first. Before starting a graph, call
`builder.Validate()` and inspect `report.Issues()` for the issue code and relevant
node, port, edge, group, or CPU; `Build` also validates the graph.

| Symptom | What to check |
| --- | --- |
| Generic type mismatch while compiling | Connect matching `OutPort[T]` and `InPort[T]`; use `ConnectError` for an `ErrorPort[E]`, not ordinary `Connect`. |
| Unconnected port or missing CPU | Wire every required ordinary input/output and bind each effective group. Error outputs may remain unconnected. Expanded workers need their own complete CPU assignments. |
| Missing factory, handler, or selector | Supply a factory and all applicable handlers; nodes with more than one input also require a selector. Explicit declarations are checked statically; closure-produced handlers/selectors are checked at `Start`, after the factory exists. |
| Second ordinary connection to an occupied destination | Ordinary inputs accept one producer. Use explicit `ConnectFanIn` for multiple producers rather than connecting repeatedly to the same input. |
| Invalid capacities or policy | Both capacities must be positive; choose `queue.Wait` or `queue.Drop`. Keep submitted batches within `BatchCapacity`; that runtime rejection is distinct from builder validation. |

Use the [diagnostic contract](../../spec/01-typed-graph.md#validation-and-diagnostics)
and [validation source](../../graph/validation.go) for exact structured diagnostics,
not compiler-version-dependent wording or guessed error strings.

## Cooperative shutdown

The command uses a five-second context and cancels it on Ctrl-C. Publication
checks that context, and `DrainContext` uses it as a cooperative shutdown budget.
Cancellation can discard work not yet delivered; it does not promise complete
results. No results are printed if the run returns an error.

This is **not a hard execution deadline**: cancellation cannot interrupt a user
handler, queue callback, or lifecycle callback that never returns. Keep callbacks
short and make your own blocking operations cancellation-aware. Do not call
`Drain` and `Close` concurrently; this example calls them sequentially.

## When to add more workers

Start with one group when a pipeline is small or its stages are cheap. Add manual
groups when you have independent work that benefits from separate workers and
can justify bounded cross-worker queues. Each physical worker requires an
explicit, unique nonnegative CPU binding; affinity may still fall back, and
parallel delivery does not promise global ordering.

Choose the explicit parallel operation that matches the work:

| Need | Use | Important distinction |
| --- | --- | --- |
| Run a whole pipeline partition on fixed workers | Group replicas | Each accepted boundary batch goes to one replica; this is distribution, not broadcast. |
| Run one hot node on several fixed workers | Node worker pool | Inputs are shared bounded queues; node state is separate per worker. |
| Copy every batch to each destination | Broadcast | Branches own distinct pooled storage; copying pointer-containing elements is shallow. |
| Send each element to one selected destination | Element sharding | The selector runs per element; parallel branch order is not global. |
| Combine compatible producers at one input | Unordered fan-in | Delivery has no cross-producer order guarantee. |

These expansions have compatibility boundaries; they cannot be layered onto
every endpoint. Check the [topology contract](../../spec/03-topologies-transport.md)
before changing an ordinary connection into a parallel topology. Fixed replicas
and worker pools do not dynamically resize.

## Choosing capacities and diagnosing graph errors

`Capacity` is queue slots measured in **batches**; `BatchCapacity` is elements
per batch. They are not total in-flight element budgets. Pools are separate,
bounded resources, so increasing a queue alone does not fix pool exhaustion. There
is no universally correct batch size: larger batches can reduce per-batch
overhead but may increase latency and retained memory. Start with capacities
appropriate to the workload, then measure drops, pool utilization, queue pressure,
and latency under representative traffic. The showcase READMEs show their own
fixed benchmark setups; do not treat their numbers as universal recommendations.

Some construction mistakes fail at different stages:

- Connecting ports with different Go element types is a **compile-time** error;
  an ordinary output and a typed error output use different connection APIs.
- Missing required ports, invalid capacities/policies, and incomplete CPU
  bindings are reported by graph validation/Build. `Validate` returns structured
  issues; use those rather than parsing diagnostic text.
- A zero-value or foreign-builder port handle, or a repeated connection, is
  rejected without consuming an edge ID. Ordinary `Connect` does not provide
  implicit fan-out or fan-in; register those topologies explicitly.
- Explicit-state handlers and selectors can be checked from registration data.
  Closure-produced handlers are checked when `Start` invokes and normalizes the
  factory, so startup can fail even after graph validation succeeded.

See [typed graph construction and diagnostics](../../spec/01-typed-graph.md)
for the full API, validation codes, precedence, and atomicity guarantees.
