## Context

The `node` package currently defines typed handler contracts, while the `generate-typed-node-arities` change will replace the handwritten families with generated explicit-state and closure handler sets. Those handler sets need a small, graph-independent declaration that distinguishes immutable node configuration from mutable state created for one runtime replica.

This change explicitly depends on `generate-typed-node-arities`; its generated handler-set types are the intended arguments for `H`. The change defines node-level data and lifecycle contracts only. There is no graph registration bridge, executable runtime, or graph Build in scope, but future implementations of those components must honor the ownership and non-invocation rules established here.

## Goals / Non-Goals

**Goals:**

- Define exact `StateFactory[S]`, `InitFunc[S]`, `CloseFunc[S]`, and `Stateful[S, H]` public signatures for explicit replica-local state, generated handlers, and optional lifecycle hooks.
- Define `ClosureReplica[H]` and `ClosureFactory[H]` for fresh closure-bound handler sets.
- Establish one-factory-call-per-replica-per-start semantics and precise cleanup ownership.
- Make partial `Init` failure safe by requiring cleanup of every successfully created replica.
- Keep captured configuration immutable and factory invocation deferred until runtime startup.

**Non-Goals:**

- Implement graph `Builder`, ports, registration, `Connect`, or any graph mutation API.
- Implement DAG validation, runtime startup or shutdown, scheduling, replicas, or restart orchestration.
- Implement or modify immutable graph `Definition`.
- Generate handler arities; that work belongs to the prerequisite `generate-typed-node-arities` change.
- Define startup ordering, concurrent-start guarantees, or lifecycle error aggregation policy.

## Decisions

### Keep factory declarations in `node`

The contracts will be declared in `node`, which already owns handler types and does not need to import `graph` or `runtime`. The explicit-state contracts will have these exact public signatures:

```go
type StateFactory[S any] func() (S, error)

type InitFunc[S any] func(*S) error

type CloseFunc[S any] func(*S) error

type Stateful[S, H any] struct {
	Factory  StateFactory[S]
	Handlers H
	Init     InitFunc[S]
	Close    CloseFunc[S]
}
```

`Factory` is required. `Init` and `Close` are optional. A future runtime stores the returned `S` as the replica's state and passes its address to the generated explicit handler set and lifecycle hooks.

The `H` parameter remains unconstrained in this small reusable container. Generated APIs from `generate-typed-node-arities`, and later typed graph registration APIs, bind it to the exact handler-set type for each arity. Adding a broad runtime interface constraint would lose the generated static signatures without providing useful behavior.

Alternatives considered:

- Put factories in `graph`: rejected because state and handler lifecycle are node concerns and would create unnecessary coupling to graph construction.
- Return `*S` from the explicit factory: rejected because returning `S` gives each runtime-owned replica a stable storage location before handlers receive `*S`, without requiring callers to allocate the outer state object.
- Discover `Init` and `Close` through optional interfaces: rejected because explicit typed function fields avoid type assertions and make lifecycle behavior visible in the declaration.

### Represent closure replicas as one factory result

Closure contracts will have these exact public signatures:

```go
type ClosureReplica[H any] struct {
	Handlers H
	Init     func() error
	Close    func() error
}

type ClosureFactory[H any] func() (ClosureReplica[H], error)
```

The closure factory constructs handlers and their lifecycle callbacks together so their shared replica-local captures cannot be mixed with callbacks from another replica. `Init` and `Close` remain optional.

Alternatives considered:

- Accept one closure handler set directly: rejected because an immutable declaration would then reuse mutable captures across replicas and restarts.
- Use separate factories for handlers and lifecycle callbacks: rejected because independently created closures could capture different state and weaken ownership guarantees.

### Transfer lifecycle ownership only on successful factory return

A factory return is successful only when its error is nil. Each successful explicit or closure factory return creates one lifecycle owned by the future runtime. The runtime invokes optional `Init` at most once for that lifecycle and invokes a provided `Close` exactly once before releasing it.

If `Init` fails, the lifecycle is still owned because the factory already succeeded. Startup rollback therefore invokes its `Close`, as well as `Close` for every other successfully created lifecycle being rolled back. If the factory returns an error, no lifecycle transfers and no callback from that result is invoked. The same close-once rule applies to normal shutdown. Cleanup ordering and aggregation of `Init` or `Close` errors remain runtime design concerns.

Alternatives considered:

- Close only replicas whose `Init` completed: rejected because a failing `Init` can partially acquire resources.
- Invoke `Close` after a failed factory return when the result appears populated: rejected because an error result does not establish a valid lifecycle contract; factories remain responsible for cleaning up their own failed construction.

### Require fresh mutable state and immutable captures

A future runtime calls the applicable factory exactly once for every logical node replica on every start. Every successful call must produce a fresh logical mutable state instance; mutable maps, slices, pointers, and closure environments must not be shared with another successful call. The framework cannot reliably deep-copy or inspect generic state, so factory authors are responsible for this freshness.

Factories, handlers, and lifecycle callbacks may capture configuration, but callers must treat that captured configuration as immutable after handing the declaration to the framework and for as long as any build or start can use it. Per-replica mutation belongs only in the state returned by `Stateful.Factory` or in the environment created inside a `ClosureFactory` call.

Alternatives considered:

- Deep-copy captured configuration: rejected because Go function captures are opaque and arbitrary generic values cannot be copied safely.
- Permit synchronized shared mutable captures: rejected because it weakens replica isolation and makes sequential restart behavior depend on state outside the factory result.

### Defer the graph bridge while preserving inert Build semantics

Future graph registration will adapt the typed declarations into graph-owned metadata without invoking factories. It records only static validation facts available from the declaration, including whether the required factory function is nil and the statically visible explicit handler-set facts. For a closure declaration, registration can record factory presence but cannot inspect the handlers or lifecycle callbacks that the factory will later return.

Future graph Build consumes those recorded static facts and must not invoke `Stateful.Factory` or a `ClosureFactory`; only runtime startup may do so. This preserves Build as a control-plane operation and prevents resource acquisition, mutable replica creation, or user side effects before a start.

This change will not add graph code or executable Build tests. It establishes the node contract and integration handoff; the next change that owns typed graph registration and its Build bridge must implement and test that behavior.

Alternatives considered:

- Invoke factories during Build to validate returned handlers: rejected because it creates state too early, introduces side effects, and risks reusing a Build-time replica at runtime.
- Invoke and immediately close a validation replica: rejected because user factory calls may be externally observable and need not be safe or cheap outside startup.

## Risks / Trade-offs

- [Freshness cannot be mechanically verified for arbitrary generic state] -> Document it as a factory-author obligation and test representative explicit and closure factories across replicas and starts.
- [Captured configuration cannot be frozen by the framework] -> Require immutability contractually and direct all mutable runtime data into factory results.
- [Closure-produced handlers cannot be inspected before invocation] -> Validate only that the closure factory is non-nil before startup; later runtime startup owns validation of its returned replica.
- [Build non-invocation cannot be integration-tested in this node-only change] -> Test the factory data contracts directly here and require the next graph bridge change to test registration and Build without executing stored factories.
- [Lifecycle error ordering is unspecified] -> Preserve the close-once ownership invariant here and defer ordering and error aggregation to runtime design.

## Migration Plan

1. Complete and integrate `generate-typed-node-arities`.
2. Add `StateFactory`, `InitFunc`, `CloseFunc`, `Stateful`, `ClosureReplica`, and `ClosureFactory` plus lifecycle documentation to `node` without changing graph APIs.
3. Add compile-time declarations and focused contract examples for generated explicit and closure handler sets.
4. Hand the static-fact recording and Build non-invocation requirements to the next graph bridge change, and hand factory invocation and cleanup behavior to the runtime change.

There is no executable node runtime to migrate. Rollback removes the additive contracts and their documentation/tests.

## Open Questions

None for this change. Startup ordering, concurrent starts, and lifecycle error aggregation are intentionally deferred to the runtime change.
