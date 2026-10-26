# OpenSpec implementation roadmap

Source: [Go pipeline framework plan](go-pipeline-framework-plan.md), section 12, stages 0–8. Status snapshot: 2026-10-10.

Scope selected by the user: one separate change for each unfinished implementation point; reuse existing changes and map completed points to archives. Section 13 testing is included in each feature's tasks, not duplicated into a second set of testing changes.

The original plan produced 23 new changes and 2 existing active changes. All implementation and verification changes have since been archived; there are no active OpenSpec changes. `verify-replica-state-isolation` and `plan-numa-placement` are verification/documentation-only and explicitly skip specs.

The dependency map below records the original implementation order. It is retained as historical planning context; it is no longer an execution queue.

## Current status

Stages 0–7 are implemented and archived. Stage 8 produced two distinct outcomes: platform-specific fast paths received a documented no-go decision, while NUMA support remains inactive metadata and a placement design, not runtime NUMA allocation. Panic recovery remains intentionally outside the runtime contract. See the linked archive entries and `docs/numa-placement-design.md` for details.

## Every implementation point

Archive links describe the delivered capabilities and decisions. The table reflects the current status, including work completed after the original roadmap snapshot.

| Stage | Original implementation point | Coverage / owner |
|---|---|---|
| 0 | Ownership, move, drop, wait, error semantics | [Contracts archive][contracts], [ownership fix][ownership], [typed errors archive][errors] |
| 0 | Public generic signatures | [Contracts archive][contracts], [arity archive][arities] |
| 0 | Node/pool/local/remote/end-to-end harness | [Complete benchmark matrix][matrix] |
| 0 | AllocsPerRun gates | [Batch archive][batch], [single-group archive][single], [multi-group archive][multi], [Go 1.26 fix][allocs]; new paths extend gates in their own tasks |
| 1 | Batch[T] | [Batch archive][batch] |
| 1 | Preallocated Pool[T] | [Batch archive][batch] |
| 1 | Per-pool clearing policy | [Batch archive][batch] |
| 1 | Move and automatic release | [Ownership correction][ownership] |
| 1 | Producer Acquire/Commit and copying Submit | [add-typed-ingress][ingress] |
| 2 | Typed port handles | [Builder archive][builder] |
| 2 | Arity generator 0–8 | [Arity archive][arities] |
| 2 | Separate input handlers | [Arity archive][arities], [single-group archive][single] |
| 2 | Node factory, explicit and closure state | [Factory archive][factories], [builder archive][builder] |
| 2 | Connect[T] | [Typed connect archive][connect] |
| 2 | DAG validation and immutable definition | [Validation archive][validation], [definition archive][definition] |
| 3 | Typed local edges | [Single-group archive][single] |
| 3 | Interface-free runnable | [Single-group archive][single] |
| 3 | Ready mask and input selector | [Single-group archive][single] |
| 3 | Sink handler | [Single-group archive][single] |
| 3 | Processing/drop metrics | [Runtime metrics][metrics] |
| 4 | QueueOps[T] | [Contracts archive][contracts], [multi-group archive][multi] |
| 4 | Bounded pure-Go rings | SPSC baseline [multi-group archive][multi] and [concurrent queue models][queues] |
| 4 | Automatic SPSC/MPSC/SPMC/MPMC selection | [add-automatic-queue-selection][selection] |
| 4 | Busy-spin Wait and Drop | [Single-group archive][single], [multi-group archive][multi], [typed errors archive][errors] |
| 4 | Execution groups | [Multi-group archive][multi] |
| 4 | Explicit CPUs, thread locking, affinity | [CPU affinity archive][affinity] |
| 4 | Graceful drain with timeout | [Drain deadline support][drain] |
| 5 | Result[E] | [Contracts archive][contracts] |
| 5 | Separate typed error port | [Typed errors archive][errors] |
| 5 | ErrorMapper[E] | [add-error-mapping][mapping] |
| 5 | Bounded async logger | [add-async-error-logger][logging] |
| 5 | Full metrics set | [Base metrics][metrics], [extended metrics][extended], and [processing timing][timing] |
| 5 | Full and sampled timing | [add-processing-timing][timing] |
| 5 | Custom timestamp function | [add-custom-timestamp-source][clock] |
| 6 | Group replicas | [add-group-replicas][replicas] |
| 6 | Node worker pools | [add-node-worker-pools][workers] |
| 6 | Separate node state per replica | Existing [factory contract][factories]; runtime state is implemented by replica/pool changes and verified by [verify-replica-state-isolation][isolation] |
| 6 | Element-level sharding | [add-element-sharding][sharding] |
| 6 | Broadcast copying | [add-broadcast-fan-out][broadcast] |
| 6 | Unordered fan-in | [add-unordered-fan-in][fanin] |
| 7 | Manual placement as hard constraint | [add-auto-grouping-constraints][constraints] |
| 7 | Minimize remote edges | [add-remote-edge-minimization][minimize] |
| 7 | Group plan before runtime creation | [add-automatic-group-planning][planning] |
| 7 | Explicit CPUs for automatic groups | [add-auto-group-cpu-bindings][autocpu] |
| 8 | Evidence-backed platform fast paths | [assess-platform-fast-paths][fastpaths] |
| 8 | Preserve pure-Go fallback | Existing [portable affinity baseline][affinity]; future optimized-path conformance belongs to [fast-path tasks][fastpaths] |
| 8 | NUMA IDs and dormant allocator hooks | [add-numa-allocation-metadata][numameta] |
| 8 | Prepare future NUMA placement stage | [plan-numa-placement][numaplan] |

## Dependency map

The dependency list is authoritative for application order; archive each completed prerequisite with synced specs before dependent implementation where practical. Preserve original factory/ownership/Go 1.22 contracts throughout.

| Change | Required implemented changes |
|---|---|
| `fix-moved-input-ownership` | Existing typed runtime baseline |
| `add-runtime-metrics` | `fix-moved-input-ownership` |
| `complete-benchmark-matrix` | `fix-moved-input-ownership` |
| `add-typed-ingress` | `fix-moved-input-ownership`, `add-runtime-metrics` |
| `add-bounded-concurrent-queues` | `fix-moved-input-ownership` |
| `add-automatic-queue-selection` | `add-bounded-concurrent-queues` |
| `add-drain-timeout` | `fix-moved-input-ownership`, `add-runtime-metrics` |
| `add-error-mapping` | `fix-moved-input-ownership` |
| `add-async-error-logger` | `add-error-mapping`, `add-runtime-metrics`, `add-drain-timeout` |
| `add-extended-runtime-metrics` | `add-runtime-metrics`, `add-processing-timing` |
| `add-processing-timing` | `add-runtime-metrics` |
| `add-custom-timestamp-source` | `add-processing-timing` |
| `add-group-replicas` | `fix-moved-input-ownership`, `add-automatic-queue-selection`, `add-runtime-metrics`, `add-typed-ingress`, `add-extended-runtime-metrics`, `add-async-error-logger`, `add-error-mapping` |
| `add-node-worker-pools` | `fix-moved-input-ownership`, `add-automatic-queue-selection`, `add-runtime-metrics` |
| `verify-replica-state-isolation` | `add-group-replicas`, `add-node-worker-pools` |
| `add-element-sharding` | `fix-moved-input-ownership`, `add-runtime-metrics` |
| `add-broadcast-fan-out` | `fix-moved-input-ownership`, `add-runtime-metrics` |
| `add-unordered-fan-in` | `fix-moved-input-ownership`, `add-automatic-queue-selection`, `add-runtime-metrics` |
| `add-auto-grouping-constraints` | `add-group-replicas`, `add-node-worker-pools` |
| `add-remote-edge-minimization` | `add-auto-grouping-constraints` |
| `add-automatic-group-planning` | `add-remote-edge-minimization`, `add-automatic-queue-selection` |
| `add-auto-group-cpu-bindings` | `add-automatic-group-planning` |
| `assess-platform-fast-paths` | `complete-benchmark-matrix`, `add-bounded-concurrent-queues`, `add-runtime-metrics` |
| `add-numa-allocation-metadata` | `add-auto-group-cpu-bindings` |
| `plan-numa-placement` | `add-numa-allocation-metadata` |

```text
ownership fix ──┬── base metrics ──┬── ingress / deadline drain
                │                 ├── timing ──┬── extended metrics
                │                 │           └── custom clock
                │                 └── broadcast / sharding
                ├── error mapping ── async logger (+ metrics + drain)
                ├── benchmark completion ── optional fast paths (+ queues + metrics)
                └── concurrent queues ── queue selection
                                           ├── fan-in (+ ownership + metrics)
                                           └── replicas / node pools (+ ownership + metrics)
                                                ├── state isolation verification
                                                └── auto constraints ── minimization
                                                     ── plan preview ── CPU binding
                                                          ── NUMA metadata ── future NUMA design
```

## Shared contract integration (historical)

Most feature changes used additive capability specs to avoid duplicating the broad `Queue and execution semantics contract` requirement. During implementation, their archive tasks reconciled earlier deferrals and synced the changed requirements into the main specs. This section records the integration rule used while the roadmap was active.

The main specs now describe the implemented baseline. Preserve all enabled capabilities when making future spec changes; do not replace a shared requirement from an older proposal snapshot.

## Design limits carried from the plan

- DAG topology, fixed parallelism, no dynamic scaling, and no panic recovery remain constraints.
- Drain deadlines are cooperative with user handlers/queue/logger callbacks; they do not promise a goroutine kill mechanism.
- Group minimization guarantees deterministic legal adjacent coalescing and quotient acyclicity, not global partition optimality.
- Broadcast copies element values shallowly into distinct slots; it does not deep-copy pointer referents.
- Mapping/logging transport and timing framework paths retain zero-allocation obligations; user callbacks have documented nonallocating/nonblocking obligations.
- Fast-path work may produce a no-go decision. Pure Go remains a valid final result.
- NUMA annotations/descriptors remain inactive. Future placement preparation is documentation, not activation.

[contracts]: ../openspec/changes/archive/2026-07-27-establish-pipeline-contracts/
[batch]: ../openspec/changes/archive/2026-07-28-implement-batch-memory-subsystem/
[arities]: ../openspec/changes/archive/2026-07-28-generate-typed-node-arities/
[factories]: ../openspec/changes/archive/2026-07-28-add-node-factory-contracts/
[builder]: ../openspec/changes/archive/2026-07-28-implement-typed-graph-builder/
[connect]: ../openspec/changes/archive/2026-07-28-implement-typed-connect/
[validation]: ../openspec/changes/archive/2026-07-28-add-graph-dag-validation/
[definition]: ../openspec/changes/archive/2026-07-28-build-immutable-graph-definition/
[single]: ../openspec/changes/archive/2026-08-20-implement-single-group-executor/
[multi]: ../openspec/changes/archive/2026-08-20-implement-manual-multigroup-runtime/
[affinity]: ../openspec/changes/archive/2026-08-20-add-cpu-affinity-runtime/
[allocs]: ../openspec/changes/archive/2026-09-01-fix-go126-allocation-tests/
[errors]: ../openspec/changes/archive/2026-09-07-implement-typed-error-routing/
[ownership]: ../openspec/changes/archive/2026-10-09-fix-moved-input-ownership/
[metrics]: ../openspec/changes/archive/2026-10-09-add-runtime-metrics/
[matrix]: ../openspec/changes/archive/2026-10-09-complete-benchmark-matrix/
[ingress]: ../openspec/changes/archive/2026-10-09-add-typed-ingress/
[queues]: ../openspec/changes/archive/2026-10-09-add-bounded-concurrent-queues/
[selection]: ../openspec/changes/archive/2026-10-09-add-automatic-queue-selection/
[drain]: ../openspec/changes/archive/2026-10-09-add-drain-timeout/
[mapping]: ../openspec/changes/archive/2026-10-09-add-error-mapping/
[logging]: ../openspec/changes/archive/2026-10-09-add-async-error-logger/
[extended]: ../openspec/changes/archive/2026-10-09-add-extended-runtime-metrics/
[timing]: ../openspec/changes/archive/2026-10-09-add-processing-timing/
[clock]: ../openspec/changes/archive/2026-10-09-add-custom-timestamp-source/
[replicas]: ../openspec/changes/archive/2026-10-09-add-group-replicas/
[workers]: ../openspec/changes/archive/2026-10-09-add-node-worker-pools/
[isolation]: ../openspec/changes/archive/2026-10-09-verify-replica-state-isolation/
[sharding]: ../openspec/changes/archive/2026-10-09-add-element-sharding/
[broadcast]: ../openspec/changes/archive/2026-10-09-add-broadcast-fan-out/
[fanin]: ../openspec/changes/archive/2026-10-09-add-unordered-fan-in/
[constraints]: ../openspec/changes/archive/2026-10-09-add-auto-grouping-constraints/
[minimize]: ../openspec/changes/archive/2026-10-09-add-remote-edge-minimization/
[planning]: ../openspec/changes/archive/2026-10-09-add-automatic-group-planning/
[autocpu]: ../openspec/changes/archive/2026-10-09-add-auto-group-cpu-bindings/
[fastpaths]: ../openspec/changes/archive/2026-10-09-assess-platform-fast-paths/
[numameta]: ../openspec/changes/archive/2026-10-09-add-numa-allocation-metadata/
[numaplan]: ../openspec/changes/archive/2026-10-09-plan-numa-placement/
