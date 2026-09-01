## Context

Stage 0 established public contracts and benchmark placeholders. `batch.Batch[T]` currently wraps an unowned slice and `Pool[T]` is a zero-sized declaration, so no path can demonstrate the plan's allocation-free data-plane requirement. The next roadmap stage is a typed, preallocated memory subsystem; typed graph ports, ingress, queues, and the executor remain unavailable.

## Goals / Non-Goals

**Goals:**

- Provide a typed fixed-capacity `Pool[T]` that completes all storage allocation during construction.
- Provide an owned `Batch[T]` handle whose logical length is explicitly controlled and whose populated data can be read without allocation.
- Support allocation-free acquire, move, and release operations after pool construction.
- Make pool exhaustion and clearing behavior deterministic and testable.
- Activate the existing pool benchmark and zero-allocation assertion after setup.

**Non-Goals:**

- Implement graph builders, typed ports, ingress, `Commit`, copying `Submit`, local edges, queues, scheduling, or metrics.
- Make pool acquisition or release safe for concurrent callers; Stage 1 pools are owned by one goroutine.
- Add runtime ownership checks, fallback allocation, pooling of arbitrary slices, or unsafe optimizations.
- Support dynamic resize or reconfiguration after construction.

## Decisions

### Allocate a single typed backing array during pool construction

`NewPool[T](PoolConfig)` validates nonzero slot and batch capacities, checks size arithmetic, allocates all slots and their typed backing storage, and initializes a free-slot stack. Every slot receives a fixed-capacity view into preallocated storage. Constructor errors occur off the data path; `Acquire` never allocates.

Alternatives considered:

- Allocate each batch slice on acquisition: rejected because it violates the zero-allocation path.
- Allocate an independent backing slice per slot: viable, but a contiguous backing array makes the allocation boundary and clearing scope clearer.

### Keep logical length in Batch and expose separate writable and readable views

`Batch[T]` tracks the slot and logical length. `Mutable()` returns the full fixed-capacity storage with `len == cap`, so producers fill by index rather than `append`. `SetLen(n)` accepts only values within capacity. `Items()` returns storage limited to the logical length. This avoids accidental capacity growth and makes the valid element range explicit.

Alternatives considered:

- Return a zero-length slice and require `append`: rejected because exceeding capacity can silently allocate.
- Expose only an append method: rejected because the user explicitly selected direct `[]T` access and because indexed filling is simpler to benchmark.

### Transfer ownership explicitly and clear handles on move or release

`Move` transfers a batch handle and zeroes the source. `Pool.Release` accepts a batch pointer, returns its slot to the free stack, and zeroes the caller's handle. Correct callers release exactly once; this stage does not add runtime tracking for double release, stale use, or release to a different pool. Future handlers can move a compatible input before automatic release.

Alternatives considered:

- Keep `Release` value-based: rejected because it cannot invalidate the caller's handle.
- Add atomic ownership flags: rejected because checks add data-plane overhead and the established contract deliberately leaves ownership correctness to API use, tests, and fuzzing.

### Clear all reusable storage when configured

With `ClearOnRelease`, release zeroes the full slot capacity, not merely the current logical length. A previous batch can have populated elements beyond a later batch's logical length, so clearing only `Items()` could retain pointers. The default remains configurable because full clearing trades CPU time for prompt reference release.

### Keep the initial pool single-goroutine-owned

The free-slot stack uses ordinary non-atomic metadata and assumes one owner goroutine for acquire and release. This matches the single-group execution path that follows and avoids inventing cross-group pool recycling before queue topology is designed. A later hybrid-runtime change must introduce the needed concurrency model explicitly.

### Measure only the executable pool path

The reserved `pool-acquire-release` benchmark becomes executable and reports allocations after setup. Its test uses `testing.AllocsPerRun` around repeated acquire/fill/release work. Other benchmark entries remain skipped until their owning stages exist.

## Risks / Trade-offs

- [A caller writes after `SetLen` or after release] → Document ownership boundaries, clear handles during move/release, and cover correct lifecycle sequences with tests and fuzzing; runtime misuse detection remains out of scope.
- [Large capacity multiplication overflows or overcommits memory] → Validate configuration arithmetic before allocation and reject invalid sizes at construction.
- [Full clearing adds CPU cost] → Keep it opt-in per pool and benchmark both clear and no-clear paths.
- [Single-owner pools cannot support later cross-group release] → Treat pool concurrency as a later design decision, rather than adding locks or atomics prematurely.
- [Returning `[]T` enables capacity misuse] → `Mutable` returns fixed-length storage and `SetLen` validates the externally declared logical length; documentation forbids append beyond the provided capacity.

## Migration Plan

1. Replace the contract-only batch and pool declarations with the executable API and update their documentation.
2. Add deterministic lifecycle, clearing, exhaustion, and allocation tests; enable the pool benchmark gate.
3. Update future graph and node implementations to use `Batch.Move` and `Pool.Release` rather than retaining raw storage.

No persisted data or deployed runtime exists. Reverting the change restores the declaration-only batch package.

## Open Questions

- Should a later concurrent pool design use a lock-free free list, return queues to pool owners, or per-worker pools with transfer accounting?
- Should pool-level usage and exhaustion counters be introduced with the metrics stage rather than the batch stage?
