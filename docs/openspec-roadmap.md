# OpenSpec implementation roadmap

Source: [Go pipeline framework plan](go-pipeline-framework-plan.md), section 12, stages 0–8. Snapshot: 2026-09-30.

Scope selected by the user: one separate change for each unfinished implementation point; reuse existing changes and map completed points to archives. Section 13 testing is included in each feature's tasks, not duplicated into a second set of testing changes.

There are **23 new changes** and **2 existing active changes**. Every new change has proposal, design, and tasks; behavioral changes also have delta specs. `verify-replica-state-isolation` and `plan-numa-placement` are verification/documentation-only and explicitly skip specs.

Planning-complete means the artifacts are ready, not that prerequisite implementations are complete. Dependencies below are execution gates recorded in each proposal and first task; OpenSpec does not automatically enforce these cross-change dependencies. Apply a named change only after its prerequisites are satisfied.

## Immediate next step

```text
/opsx-apply fix-moved-input-ownership
```

Then resume `/opsx-apply add-runtime-metrics`. Metrics is partially implemented (1/19 tasks checked at this snapshot) and blocked by the forwarded-input ownership regression. Preserve that worktree while applying the prerequisite fix.

## Every implementation point

Completed archive links describe delivered baseline capabilities; where a later regression/gap exists, the active corrective change is also linked.

| Stage | Original implementation point | Coverage / owner |
|---|---|---|
| 0 | Ownership, move, drop, wait, error semantics | [Contracts archive][contracts], [ownership fix][ownership], [typed errors archive][errors] |
| 0 | Public generic signatures | [Contracts archive][contracts], [arity archive][arities] |
| 0 | Node/pool/local/remote/end-to-end harness | Baseline [contracts archive][contracts]; remaining skipped cases: [complete-benchmark-matrix][matrix] |
| 0 | AllocsPerRun gates | [Batch archive][batch], [single-group archive][single], [multi-group archive][multi], [Go 1.26 fix][allocs]; new paths extend gates in their own tasks |
| 1 | Batch[T] | [Batch archive][batch] |
| 1 | Preallocated Pool[T] | [Batch archive][batch] |
| 1 | Per-pool clearing policy | [Batch archive][batch] |
| 1 | Move and automatic release | Baseline [batch archive][batch]; outstanding correction: [fix-moved-input-ownership][ownership] |
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
| 3 | Processing/drop metrics | Existing [add-runtime-metrics][metrics] |
| 4 | QueueOps[T] | [Contracts archive][contracts], [multi-group archive][multi] |
| 4 | Bounded pure-Go rings | SPSC baseline [multi-group archive][multi]; remaining models: [add-bounded-concurrent-queues][queues] |
| 4 | Automatic SPSC/MPSC/SPMC/MPMC selection | [add-automatic-queue-selection][selection] |
| 4 | Busy-spin Wait and Drop | [Single-group archive][single], [multi-group archive][multi], [typed errors archive][errors] |
| 4 | Execution groups | [Multi-group archive][multi] |
| 4 | Explicit CPUs, thread locking, affinity | [CPU affinity archive][affinity] |
| 4 | Graceful drain with timeout | Drain baseline [multi-group archive][multi]; remaining deadline: [add-drain-timeout][drain] |
| 5 | Result[E] | [Contracts archive][contracts] |
| 5 | Separate typed error port | [Typed errors archive][errors] |
| 5 | ErrorMapper[E] | [add-error-mapping][mapping] |
| 5 | Bounded async logger | [add-async-error-logger][logging] |
| 5 | Full metrics set | Existing [base metrics][metrics], remaining [extended metrics][extended]; timing/log fields are owned by their changes |
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

## Shared contract integration

Most new features have additive capability specs to avoid 20+ stale copies of the same broad `Queue and execution semantics contract` requirement. Some current main capabilities explicitly defer these features. During each feature's implementation, its completion task requires reconciling those deferrals and adding full MODIFIED requirement blocks to that change from the then-current main specs. Preserve all previously enabled capabilities; do not replace a shared requirement using an older parallel proposal's snapshot. Add the corresponding Modified Capabilities entries to the proposal when those deltas are captured.

This integration step is required before archive, not permission to silently leave contradictory deferrals in main specs. Main specs remain the implemented baseline until each feature is actually delivered and synced.

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
[ownership]: ../openspec/changes/fix-moved-input-ownership/
[metrics]: ../openspec/changes/add-runtime-metrics/
[matrix]: ../openspec/changes/complete-benchmark-matrix/
[ingress]: ../openspec/changes/add-typed-ingress/
[queues]: ../openspec/changes/add-bounded-concurrent-queues/
[selection]: ../openspec/changes/add-automatic-queue-selection/
[drain]: ../openspec/changes/add-drain-timeout/
[mapping]: ../openspec/changes/add-error-mapping/
[logging]: ../openspec/changes/add-async-error-logger/
[extended]: ../openspec/changes/add-extended-runtime-metrics/
[timing]: ../openspec/changes/add-processing-timing/
[clock]: ../openspec/changes/add-custom-timestamp-source/
[replicas]: ../openspec/changes/add-group-replicas/
[workers]: ../openspec/changes/add-node-worker-pools/
[isolation]: ../openspec/changes/verify-replica-state-isolation/
[sharding]: ../openspec/changes/add-element-sharding/
[broadcast]: ../openspec/changes/add-broadcast-fan-out/
[fanin]: ../openspec/changes/add-unordered-fan-in/
[constraints]: ../openspec/changes/add-auto-grouping-constraints/
[minimize]: ../openspec/changes/add-remote-edge-minimization/
[planning]: ../openspec/changes/add-automatic-group-planning/
[autocpu]: ../openspec/changes/add-auto-group-cpu-bindings/
[fastpaths]: ../openspec/changes/assess-platform-fast-paths/
[numameta]: ../openspec/changes/add-numa-allocation-metadata/
[numaplan]: ../openspec/changes/plan-numa-placement/
