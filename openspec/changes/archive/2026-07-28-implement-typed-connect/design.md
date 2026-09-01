## Context

The current graph contract exposes phantom typed ports, a non-generic `EdgeConfig`, and a `Connect[T]` placeholder that accepts `*Definition` and always returns `ErrUnavailable`. The prerequisite changes `generate-typed-node-arities`, `add-node-factory-contracts`, and `implement-typed-graph-builder` are assumed to provide registered builder-owned ports, node and port identities, and an open mutable `Builder`.

This change turns connection creation into executable control-plane mutation while preserving compile-time element compatibility. It must retain a typed path from each edge to a queue that a future runtime can create, but it must not construct mutable queue state now. The monolithic `implement-typed-graph-definition` artifacts are context only; graph-wide validation and immutable-definition sealing remain separate changes.

## Goals / Non-Goals

**Goals:**

- Define per-runtime `queue.Factory[T]` and generic `EdgeConfig[T]` contracts.
- Connect matching registered ports on one open builder and return dense stable edge IDs.
- Define stable sentinel and typed errors with deterministic validation precedence.
- Reject invalid provenance, reserved error outputs, invalid configuration, duplicate producers, and duplicate consumers without partial mutation.
- Reject fan-out explicitly until a later capability defines its delivery semantics.
- Centralize edge-configuration checks in one pure validator reusable by later graph-wide validation.
- Preserve the edge's element type in a dormant future-binding closure without reflection, runtime type assertions, or an `any` data-plane registry.
- Cover successful heterogeneous connections and compiler rejection of mismatched element types.

**Non-Goals:**

- Build, validate, topologically sort, or perform the transition that seals a graph definition.
- Detect cycles, disconnected nodes, missing graph-wide connectivity, or source-to-sink reachability.
- Implement queue storage, execute factories, run nodes, publish batches, or start runtimes.
- Define broadcast, distribution, sharding, or any other fan-out behavior.
- Change generated arities, node factories, or typed builder registration APIs supplied by prerequisite changes.

## Decisions

### Make queue construction generic and runtime-local

The public queue and graph APIs are exactly:

```go
package queue

type Factory[T any] func(Config) (QueueOps[T], error)
```

```go
package graph

type EdgeConfig[T any] struct {
	Capacity uint32
	Policy   queue.Policy
	Factory  queue.Factory[T]
}

func Connect[T any](
	builder *Builder,
	output OutPort[T],
	input InPort[T],
	config EdgeConfig[T],
) (EdgeID, error)
```

A future runtime calls `Factory[T]` separately for each runtime instance of an edge, allowing queue allocation or setup to fail without sharing mutable queue operations across restarts. A nil `EdgeConfig.Factory` selects the future built-in queue implementation rather than making the edge invalid.

`Connect` converts capacity and policy to `queue.Config` and passes that value to one package-private pure function with the exact signature `func validateEdgeConfig(queue.Config) error`. The validator accepts `Capacity > 0` and only the declared `queue.Drop` and `queue.Wait` policies. It has no builder access, performs no mutation, and never receives or invokes the typed factory. The later DAG-validation change reuses this function against stored edge metadata rather than duplicating configuration rules.

Alternatives considered:

- Store a `QueueOps[T]` in edge configuration: rejected because it would share live mutable queue state across future runtimes.
- Use an untyped factory: rejected because it would lose the compiler-checked relationship between the port element type and queue operations.
- Keep `WaitForSpace bool`: rejected because `queue.Policy` already names the supported behavior and permits explicit validation.

### Mutate only an open builder and assign one-based dense edge IDs

`Connect[T]` delegates builder lifecycle checking to the state supplied by `implement-typed-graph-builder`. A sealed state is part of the builder contract now so mutators have one stable error behavior, but this change introduces no `Build` operation and never transitions a builder to sealed. The later successful Build implementation performs that transition.

`EdgeID(0)` is invalid. The first successful connection receives ID 1, and each later success receives the next ID. Errors return the zero ID. Since the next ID derives from committed edge count rather than attempted calls, failures cannot create gaps.

Alternatives considered:

- Continue accepting `*Definition`: rejected because topology must be collected before any future definition can become immutable.
- Allocate an ID before validation and roll it back: rejected because validate-then-commit is simpler and makes atomicity explicit.

### Publish stable connection errors and validate before one commit

The graph package exports these sentinel errors:

```go
var (
	ErrBuilderSealed          = errors.New("graph builder is sealed")
	ErrInvalidPort            = errors.New("invalid graph port")
	ErrForeignPort            = errors.New("graph port belongs to another builder")
	ErrInputConnected         = errors.New("graph input already has a producer")
	ErrFanOutUnsupported      = errors.New("graph output fan-out is unsupported")
	ErrInvalidEdgeConfig      = errors.New("invalid graph edge configuration")
	ErrErrorRoutingUnavailable = errors.New("graph error routing is unavailable")
)

type EdgeConfigError struct {
	Config queue.Config
}

func (e *EdgeConfigError) Error() string
func (e *EdgeConfigError) Unwrap() error
```

`EdgeConfigError.Unwrap` returns `ErrInvalidEdgeConfig`, so callers test the category with `errors.Is` and inspect rejected capacity and policy with `errors.As`. Other local failures return their sentinel directly. Every failure returns `EdgeID(0)`.

Connection validation runs without mutating builder state and uses this deterministic precedence:

1. Sealed builder: `ErrBuilderSealed`.
2. Zero, malformed, or unknown output, then input: `ErrInvalidPort`.
3. Output, then input owned by another builder: `ErrForeignPort`.
4. A valid registered typed error output: `ErrErrorRoutingUnavailable`.
5. Input already having a producer: `ErrInputConnected`.
6. Output already having a consumer: `ErrFanOutUnsupported`.
7. Invalid capacity or policy from `validateEdgeConfig`: `*EdgeConfigError` wrapping `ErrInvalidEdgeConfig`.

Only after every check succeeds does one commit append edge metadata, mark both endpoints connected, store the typed binding closure, and expose the new ID. This precedence is observable and table-tested, including cases where more than one condition is invalid.

An occupied input reports the one-producer violation. An occupied output reports a distinct unsupported-fan-out error because multiple consumers require explicit broadcast or distribution semantics. No validation failure changes endpoint occupancy, appends metadata, retains the rejected factory, or consumes an ID.

Alternatives considered:

- Permit multiple consumers in metadata and defer semantics: rejected because a later executor could not determine whether to copy, share, broadcast, or distribute batch ownership.
- Return only a general invalid-connection error: rejected because callers and focused tests need to distinguish unsupported fan-out from malformed handles and configuration.

### Reserve typed error outputs without adding error edges

Node registration from the prerequisite changes returns a typed error `OutPort[E]` outside the ordinary output arity. Its private handle metadata marks it as an error output even though its public generic port type remains `OutPort[E]`. `Connect` recognizes that role after validity and ownership checks and returns `ErrErrorRoutingUnavailable` before endpoint cardinality or edge-configuration checks.

No error edge is appended, no endpoint is occupied, no factory is retained or invoked, and no edge ID is consumed. Error outputs remain reserved and unconnected until a dedicated error-routing change defines routing, ownership, and runtime behavior.

Alternatives considered:

- Treat error output as an ordinary output: rejected because doing so would silently commit topology whose failure and ownership semantics are undefined.
- Omit error output handles until routing exists: rejected because node factory and registration contracts already reserve their static typed identity.

### Preserve the element type through a future-binding closure

The builder's structural edge record stores only non-generic IDs and validated queue configuration. `Connect[T]` also creates a package-private closure in its generic instantiation that captures `queue.Factory[T]` and the edge identity. A later runtime-binding change can extend the package-private continuation target and invoke the closure to install `QueueOps[T]` directly into the corresponding typed producer and consumer path.

The closure remains dormant in this change. Neither `Connect`, `validateEdgeConfig`, nor any graph construction or future Build validation invokes the factory. The closure does not return a type-erased queue, place `QueueOps[T]` in `any`, use reflection, or require a runtime assertion. Captured user configuration is required to remain safe for later invocation, and each future runtime invokes the factory to obtain its own queue operations.

Alternatives considered:

- Store heterogeneous queue factories in `any`: rejected because recovery would require runtime assertions and weaken the typed contract.
- Invoke the factory during `Connect`: rejected because connection construction is not a runtime start and would retain mutable state prematurely.

### Test public construction paths and compiler enforcement

Runtime tests use registered ports returned by the prerequisite builder API rather than zero-value handles. Table-driven cases cover valid matching connections, exact `errors.Is` and `errors.As` behavior, deterministic multi-error precedence, positive capacity and policy checks through the shared pure validator, zero and foreign handles, reserved error outputs, sealed state, occupied inputs, occupied outputs, dormant custom factories, and dense IDs across intervening failures. Matching `int` and `string` examples demonstrate heterogeneous ordinary edges in one builder.

The compile-fail fixture attempts one `Connect` call with `OutPort[int]` and `InPort[string]`; a test invokes the Go toolchain and requires type checking to fail. This complements ordinary compiling examples because runtime tests cannot represent an ill-typed call.

## Risks / Trade-offs

- [The future-binding target is not executable yet] -> Keep the closure package-private and test that factories remain dormant; the runtime change will supply the invocation target without changing the public generic contracts.
- [Function values can capture mutable caller state] -> Document that factory captures are caller-owned and that each invocation must produce fresh runtime-local queue operations.
- [Rejecting all fan-out limits topology] -> Return a specific error and leave both existing topology and ID allocation unchanged until explicit fan-out semantics are specified.
- [Validation precedence can affect observed errors] -> Publish and table-test the exact order while guaranteeing atomicity for every error.
- [Prerequisite API details may move] -> Limit integration to builder openness, handle provenance, endpoint identity, and edge append hooks promised by the three named dependencies.

## Migration Plan

1. Add `queue.Factory[T]` without implementing a queue.
2. Replace the placeholder edge configuration and definition-targeted `Connect` signature with the exact generic builder-targeted API and stable error contract.
3. Add the pure shared configuration validator, atomic local validation, error-output rejection, endpoint occupancy, dense edge storage, and the dormant typed binding closure.
4. Replace placeholder examples and tests with registered ordinary-port success cases, deterministic error checks, and compile-fail verification.

There is no executable graph runtime to migrate. Rollback restores the placeholder declarations and removes connection metadata written only by the new builder API. The later Build change owns the sealing transition and can reuse `ErrBuilderSealed` and `validateEdgeConfig` without revising these public contracts.

## Open Questions

- What package-private runtime binding target will invoke the stored typed closure when queue execution is implemented?
