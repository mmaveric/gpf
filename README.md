# Go Pipeline Framework

This repository is building a typed, batch-oriented Go pipeline framework. The
typed batch pool is executable, and fixed node arities are generated as public
contracts. Graph builders validate immutable topology definitions, and a valid
definition starts one locked worker per fixed manually placed CPU-bound group through `Definition.Start`.

Steady-state paths use concrete generic batches and generated typed function
closures. They do not use reflection, runtime type assertions, or boxed
data-plane values.

The module path is `github.com/asharipov/gpf` and the minimum supported Go
version is Go 1.22.

## Generated Node Contracts

The `node` package provides sealed `Outputs0` through `Outputs8`, explicit-state
`Node0` through `Node8`, and closure-based `ClosureNode0` through
`ClosureNode8`. Every input has its own typed handler. Multi-input selectors use
zero-based `Port1` through `Port8` positions and must choose a ready input when
called.

Generated node files are committed. Regenerate and verify them with:

```bash
go generate ./...
go test ./internal/genarity ./node
```

`Outputs1` through `Outputs8` expose typed `Out1` through `Out8` values.
Handlers use `Acquire` and `Commit` to produce or move batches. An uncommitted
acquisition is released automatically. A failed `Result` discards ordinary
outputs and un-moved inputs before optionally publishing one typed error batch.

## Benchmarks

Run the benchmark matrix with:

```bash
go test -run '^$' -bench . -benchmem ./...
```

The pool, remote-queue, and two-group end-to-end entries execute after setup
and report allocations. Throughput and latency are non-blocking comparison
signals; interpret them alongside allocation results rather than as thresholds.

The benchmark matrix reserves these stable names:

- `node-call/batch=64`
- `pool-acquire-release/batch=64`
- `local-edge/batch=64/group=1`
- `remote-queue/batch=64/groups=2`
- `local-error/batch=64/groups=1`
- `remote-error/batch=64/groups=2`
- `end-to-end-dag/batch=64/groups=2/depth=3`

Throughput and latency results are recorded for manual comparison rather than
enforced as merge-blocking thresholds. A passing zero-allocation assertion does
not rule out instruction-count, cache-miss, throughput, or tail-latency
regressions.

## Typed Connection Check

Matching ordinary ports connect through `graph.Connect[T]`; a registered typed
failure port connects only through `graph.ConnectError[E]`:

```go
_, err := graph.ConnectError(builder, producer.Errors(), errorSink.Inputs().In1, config)
```

An error output is optional. An unconnected failure, an exhausted error pool,
or a full Drop edge discards the failure value. A Wait edge retains one typed
error batch and pauses only that producing runner until it publishes. Error
batches have length one, retain their originating pool through local and remote
transport, and can be moved by a consumer to a compatible ordinary output.

## Fixed-Group Execution

`graph.Builder.Build` validates an open graph, then returns a cached immutable
definition on success. The definition reports node and edge counts and a copied
canonical topological order. Successful Build seals registration and connection;
failed validation leaves the builder open for correction.

Build retains state, closure, and queue factory configuration without invoking
factories, handlers, or lifecycle callbacks. `Definition.Start` materializes
fresh state, pools, and queues for one worker per fixed group. `graph.Place`
assigns a registered node once to a nonzero opaque group ID; unplaced nodes use
`graph.DefaultGroupID` (1), preserving the existing one-worker graph shape.
Before Build, every effective group must receive one unique nonnegative CPU with
`graph.BindCPU`; this is graph-construction metadata, not Start configuration.
Every ordinary or error edge requires both `Capacity` and `BatchCapacity`. Same-group edges use
the built-in local ring or a fresh custom `QueueOps`; cross-group edges use the
built-in typed bounded SPSC ring, and Build rejects a remote custom factory.
`Drop` releases full-edge outputs and `Wait` retries one pending output on later
scheduler passes without blocking a consumer group.

`Executor.Drain` uses a deterministic all-worker pause barrier, disables
sources, and processes accepted local and remote work to global quiescence.
`Executor.Close` discards queued and pending work from that barrier. Both close
node lifecycles in reverse canonical order on their locked owner workers, and
definitions can be reused sequentially after a stop. Linux attempts per-worker
CPU affinity with a pure-Go syscall. Unsupported platforms and rejected Linux
requests keep the worker locked and report the fallback through the copied,
ascending `Executor.AffinityStatus` result; affinity failure does not fail
Start. Automatic grouping, CPU discovery or CPU-set policy, NUMA placement,
replicas, MPSC/SPMC/MPMC queues, fan-out, sharding, metrics, logging, error
mapping, retries, panic recovery, error fan-out/distribution, ingress, and
concurrent starts remain deferred.
