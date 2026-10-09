# Future NUMA Placement Design

**Status: preparation only.** No NUMA discovery, memory binding, CPU-to-node
inference, allocator activation, or NUMA-aware dispatch is implemented. The
current metadata change must remain behavior-neutral.

**Research date:** 2026-10-08. Linux source content is Linux man-pages 6.19
(man7.org says fetched 2026-09-09). Microsoft sources were updated 2025-07-01
and 2025-07-16. Verify every platform claim against the project's minimum
supported OS/kernel and Go release before activation.

## Current contracts

`numa.Placement{}` is unspecified; `numa.On(0)` explicitly annotates node zero.
Group, edge, ingress-pool, and batch-pool metadata are copied for inspection.
Typed batch/queue allocator descriptors are retained but never called by
registration, Build, or Start. Execution continues to use the portable Go
allocators and explicit CPU affinity/fallback behavior. An annotation does not
assert that the host has that node, that the process may use it, or that memory
was allocated there.

## Platform facts and design implications

### Linux

`numa(7)` describes NUMA as multiple memory nodes whose access cost depends on
CPU/node distance. Kernel APIs are conditional on `CONFIG_NUMA`, and the manual
recommends the higher-level libnuma interface over direct NUMA system calls.
`mbind(2)` applies a memory policy to a range; normally this affects future
allocations, and anonymous pages follow it when first written. Policy masks
must account for nodes permitted by the current cpuset. `MPOL_BIND` is strict;
preferred/local modes may fall back. Moving existing pages is a separate,
potentially privileged/failing operation. `sched_setaffinity(2)` constrains a
thread's eligible CPUs; it is a separate per-thread operation from memory
policy.

**Implication:** CPU-affinity success cannot prove memory locality. A future
provider must report online, memory-bearing, and process-allowed nodes and detect
or reject stale topology. Policy must state whether it is strict or preferred.

### Windows

The Microsoft NUMA overview states that allocation prefers a node but can fall
back when it lacks pages. `GetNumaHighestNodeNumber` explicitly warns that the
highest ID is not the node count, so IDs may be sparse. Processor groups
complicate mapping above 64 logical processors; modern Windows may associate one
NUMA node with multiple groups. `VirtualAllocExNuma` selects a preferred node
for a virtual allocation, but physical pages are committed on demand and may be
placed on another node if the preferred node is short of memory.

**Implication:** node-to-CPU mapping must be group-aware and cannot assume node
IDs are dense. Windows allocation metadata should be described as a preference
unless the API contract proves stricter placement.

### Go allocation boundary

The Go runtime allocator source documents per-P caches, central span classes,
heap spans, page allocation, and OS interaction. The public `runtime` package
does not expose a stable NUMA placement API. A typed callback returning `[]T`
may still allocate from the ordinary Go heap. Calling an OS placement API around
`make` does not by itself guarantee placement: the relevant pages may already
exist or be first-touched later from a different thread.

**Implication:** current typed descriptors are metadata, not an activation-ready
allocator ABI. Before activation, a descriptor needs explicit typed ownership
and release semantics, alignment/capacity checks, placement outcome reporting,
and a clear rule for pointer-containing `T`. Do not put pointer-bearing values
in memory the Go garbage collector does not scan. Define who first-touches or
initializes pool pages and on which locked worker.

## Candidate future architecture

```text
OS topology snapshot ─── explicit CPU bindings
 (allowed CPUs/nodes,          │
  memory nodes, distances)    │
             └─────────┬──────┘
                       ▼
         physical placement resolver
                       │
       ┌───────────────┴────────────────┐
       ▼                                ▼
 per-worker typed pools          edge/queue storage
 allocation lease + release      allocation lease + release
       │                                │
       └──── batch movement/origin ownership ────┘
```

Potential layers, only after a separate approved change:

1. **Topology provider:** immutable, generation-stamped snapshot of sparse node
   IDs, CPU membership, allowed sets, memory capacity, and distance data. No
   implicit discovery during ordinary Build.
2. **Placement resolver:** maps explicit worker CPU bindings and optional NUMA
   intent to a validated physical plan; stale/unknown IDs are explicit errors.
3. **Typed allocation lease:** allocates and releases storage with requested
   versus achieved placement and fallback status. Preserve origin-pool release
   when a batch crosses worker/group boundaries.
4. **Owner-thread initialization:** either allocate and first-touch on the
   target locked worker or use a platform allocation mechanism that gives an
   equivalent guarantee. Share the existing shutdown deadline with failures
   and rollback.
5. **Copied observability:** report requested/achieved placement and fallback;
   never present a request as a measured result.

## Decisions required before activation

- Is NUMA intent strict binding, preferred placement with fallback, or a set of
  candidate nodes? What does unspecified mean at every API boundary?
- Which source supplies CPU/node mappings, allowed cpusets/cgroups, memory-bearing
  nodes, and distances? How is staleness detected after hotplug or container
  policy changes?
- What does an allocator lease own and release? Can it return GC-scanned Go
  memory, or only pointer-free storage? How are alignment, clearing, and reuse
  guaranteed?
- Which thread allocates and first-touches a pool? How does this interact with
  locked workers, startup rollback, custom factories, and sequential restarts?
- When a batch is moved remotely, is memory retained at the origin, copied, or
  migrated? Who pays and accounts for migration costs? Where should a shared
  MPSC/MPMC ring live?
- What is the behavior when a node is absent, disallowed, memory-starved, or
  unsupported by the kernel/API: fail startup, portable fallback, or per-pool
  fallback status?
- Which Linux kernels/configurations, Windows processor-group versions,
  architectures, and CGO settings are supported? What is the forced-portable
  build/runtime selection?
- How will current `Placement{Node, Present}` and dormant allocator descriptors
  evolve without changing default allocation or existing CPU affinity semantics?

## Activation gates and benchmark plan

Any later implementation proposal should require:

1. **Correctness:** tests for unique ownership, no loss/duplication, origin-pool
   release, partial-start rollback, Drain/Close cleanup, and local/remote edges.
2. **Topology:** sparse IDs, non-NUMA systems, restricted cpusets, no-memory
   nodes, Windows processor groups, and stale topology have deterministic
   tested outcomes.
3. **Placement verification:** use OS evidence (e.g. Linux
   `/proc/<pid>/numa_maps`) to distinguish requested from achieved placement;
   affinity status alone is not sufficient.
4. **Controlled benchmarks:** portable vs preferred/strict/fallback on
   representative local and cross-node pipelines. Cover batch sizes, graph
   depth, replicas/pools, queue model, memory footprint, pointer-free and
   pointer-bearing `T`, and ownership/release cycles. Pin CPUs and record CPU,
   memory topology, OS/kernel, Go version, and policy.
5. **Repeated measurements:** serial repeated trials on the same machine;
   report distributions, significance, allocations, throughput, tail latency,
   startup cost, and footprint. Include remote-memory baselines and placement
   verification. A single microbenchmark cannot justify activation.
6. **Fallback/reversal:** no-CGO and unsupported targets pass the same semantic
   ownership suite with portable allocation. Activation has an explicit
   reversible config/build selection.

The hypothesis is falsifiable: local placement must improve a representative
memory-bound workload beyond run-to-run noise without unacceptable cross-node,
valid outcome.

## Primary references

Read 2026-10-08. Linux man-page content is version 6.19 (source page fetched
2026-09-09); Microsoft pages list update dates in their metadata.

- Linux `numa(7)`: <https://man7.org/linux/man-pages/man7/numa.7.html>
- Linux `mbind(2)`: <https://man7.org/linux/man-pages/man2/mbind.2.html>
- Linux `set_mempolicy(2)`: <https://man7.org/linux/man-pages/man2/set_mempolicy.2.html>
- Linux `sched_setaffinity(2)`: <https://man7.org/linux/man-pages/man2/sched_setaffinity.2.html>
- Go 1.26.0 runtime allocator source: <https://cs.opensource.google/go/go/+/go1.26.0:src/runtime/malloc.go>
- Go `runtime` package documentation (no public NUMA placement API): <https://pkg.go.dev/runtime>
- Microsoft NUMA support overview: <https://learn.microsoft.com/en-us/windows/win32/procthread/numa-support>
- Microsoft `VirtualAllocExNuma`: <https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualallocexnuma>
- Microsoft `GetNumaHighestNodeNumber`: <https://learn.microsoft.com/en-us/windows/win32/api/systemtopologyapi/nf-systemtopologyapi-getnumahighestnodenumber>
