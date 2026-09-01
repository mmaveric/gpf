## Context

The current `graph` package contains declaration-only `Builder`, `Definition`, `InPort[T]`, and `OutPort[T]` types. Its zero-value ports have no graph, node, or port identity, and node definitions cannot be registered. The earlier monolithic `implement-typed-graph-definition` artifacts describe the full construction and validation direction, but this change extracts only mutable graph identity and typed node registration.

Implementation is ordered after `generate-typed-node-arities` and then `add-node-factory-contracts`. The first dependency supplies sealed `node.OutputSet` shapes and generated handler sets for arities zero through eight. The second supplies explicit-state and closure factory contracts. This change consumes those APIs without redefining them.

Go 1.22 has neither generic methods nor variadic generics. The graph must preserve concrete input, output, and error types without reflection, runtime type assertions, or heterogeneous `any` storage. Package dependencies must remain acyclic while later connection, validation, and runtime changes reuse the identities introduced here.

## Goals / Non-Goals

**Goals:**

- Construct a valid mutable builder with a private graph identity through `NewBuilder`.
- Define exact zero-invalid and one-based assigned conventions for `NodeID` and `PortID`.
- Return opaque builder-owned typed input, ordinary-output, and error-output handles from node registration.
- Generate graph port bundles, output expansion, and explicit-state and closure registrations for arities zero through eight.
- Store structural node metadata, static validation masks, and dormant concretely typed normalization closures for future validation/runtime stages.
- Preserve the package direction `node <- graph` and compiler-checked element types throughout registration.

**Non-Goals:**

- Implement or alter executable `Connect` behavior, edge identity, edge metadata, cardinality, or fan-out policy.
- Validate DAG structure, connectivity, handlers, selectors, factories, or lifecycle callbacks.
- Implement `Connect`, `Build`, the transition that seals a builder, or an immutable `Definition`.
- Invoke state factories, closure factories, handlers, selectors, or lifecycle hooks.
- Add runtime state, queues, batches, scheduling, replicas, execution groups, or CPU placement.

## Decisions

### Implement prerequisites in an explicit order

Implementation SHALL begin only after `generate-typed-node-arities` and then `add-node-factory-contracts` are complete. The builder's generated signatures refer to the output shapes and handler/factory families supplied by those changes, so implementing in another order would require temporary duplicate contracts.

Alternative considered: define temporary graph-local node contracts. Rejected because they would create churn and risk a public API that diverges from the prerequisite changes.

### Keep graph construction downstream of node contracts

The `node` package owns output shapes, handler sets, selectors, and explicit/closure factories. The `graph` package imports `node` and owns graph identity, handles, registration results, and metadata. The `node` package never imports `graph`.

```text
batch <- node <- graph
```

Alternative considered: place graph handles in generated node declarations. Rejected because graph registration consumes node contracts and the reverse import would create a cycle.

### Give each builder an unforgeable owner token

`NewBuilder() *Builder` returns initialized mutable state with a freshly allocated private `*graphOwner`. `graphOwner` SHALL contain at least one byte; it must not be `struct{}` or another zero-sized type because Go permits pointers to distinct zero-sized allocations to compare equal. Registration APIs accept `*Builder`; a nil pointer or a zero-value `Builder` is invalid. Every handle contains the owner pointer, a node identity, a node-local port identity where applicable, and a phantom generic marker. Fields and constructors remain private, so external callers can create only invalid zero values.

The token is identity, not a user-visible numeric graph ID. It is shared by all handles from one builder and distinct `NewBuilder` calls are guaranteed to receive pointers that compare unequal. Later connection and definition changes can compare owner pointers without exposing mutation or relying on global counters.

Alternative considered: use a process-global numeric graph counter. Rejected because it adds synchronization, overflow, and externally observable allocation order without improving ownership checks.

### Use zero-invalid, one-based stable IDs

The exact declarations are `type NodeID uint32`, `const InvalidNodeID NodeID = 0`, `type PortID uint8`, and `const InvalidPortID PortID = 0`. Successful registrations receive dense `NodeID` values beginning at one in registration order. Registration failure is atomic and does not advance the next ID.

Inputs and outputs use separate node-local namespaces. Input and ordinary-output positions use `PortID` values one through eight. The typed error output always uses `PortID(9)`, outside the ordinary eight-output range. Zero-value handles therefore contain invalid node and port IDs naturally. `OutputRef[O]` identifies the owner and node as a whole and has no singular `PortID`.

IDs identify registration structure only. They do not imply edges, topological order, scheduling order, or runtime placement.

Alternative considered: use zero-based valid port IDs and reserve 255 as invalid. Rejected because making zero invalid for both public ID types gives every zero-value handle consistent invalid semantics and avoids special-case accessors.

### Reserve builder lifecycle state without implementing sealing

`Builder` contains a private state with exactly three meanings: invalid for the zero value, open for a value returned by `NewBuilder`, and sealed for a value transitioned by a later build change. The public sentinel errors are exactly `ErrInvalidBuilder`, `ErrBuilderSealed`, and `ErrNodeIDExhausted`. Every registration function checks state before mutation, returns `ErrInvalidBuilder` for invalid state, returns `ErrBuilderSealed` for sealed state, and returns `ErrNodeIDExhausted` rather than wrapping the next `NodeID` to zero.

This change initializes only open builders and provides no public or private operation that performs the open-to-sealed transition. Reserving the state and error now lets later registration, connection, and build changes share one rejection contract without claiming that `Build` exists in this capability.

### Separate input registration from output expansion

Generated `InPorts0` through `InPorts8` expose `In1` through `In8` as applicable. Generated `OutPorts0` through `OutPorts8` expose `Out1` through `Out8` as applicable. `NodePorts[I, O, E]` has private fields and exactly these value-receiver accessors: `NodeID() NodeID`, `Inputs() I`, `Outputs() OutputRef[O]`, and `Errors() OutPort[E]`. Only graph registration can construct a valid value.

`OutputRef[O]` has private owner, node ID, and output-count fields and exactly these value-receiver accessors: `NodeID() NodeID`, `OutputCount() uint8`, and `Valid() bool`. The prerequisite `node.OutputSet` contract exposes `OutputCount() uint8`. Registration obtains the count by evaluating `var outputs O` and calling `outputs.OutputCount()`, validates the range zero through eight, and stores the count without reflection or a type assertion.

Generated function names are exactly `OutputPorts0`, `OutputPorts1`, through `OutputPorts8`. Each accepts only the matching `OutputRef[node.OutputsN[...]]` and returns `OutPortsN[...]`; for example, `OutputPorts2[O1, O2](OutputRef[node.Outputs2[O1, O2]]) OutPorts2[O1, O2]`. Expansion verifies the reference's stored count equals N, derives one-based output IDs, and otherwise returns a zero invalid bundle. It never creates graph identity.

`InPort[T]` and `OutPort[T]` have exactly `NodeID() NodeID`, `PortID() PortID`, and `Valid() bool` value-receiver accessors. `Valid` requires a non-nil owner and nonzero node and port IDs. `OutputRef.Valid` requires a non-nil owner, a nonzero node ID, and an output count no greater than eight.

Generated `AddNode0` through `AddNode8` and `AddClosureNode0` through `AddClosureNode8` vary only by input arity. Their output-shape parameter remains independent and constrained by `node.OutputSet`; callers select the matching `OutputPortsN` helper to recover ordinary outputs. This produces linear generated families instead of an 81-member input/output cross-product.

The generated signatures use the following exact parameter ordering and result shape. Arity zero and arity two illustrate the mechanically repeated family:

```go
func AddNode0[S, E any, O node.OutputSet](
	b *Builder,
	definition node.Stateful[S, node.Node0[S, O, E]],
) (NodePorts[InPorts0, O, E], error)

func AddClosureNode0[E any, O node.OutputSet](
	b *Builder,
	factory node.ClosureFactory[node.ClosureNode0[O, E]],
) (NodePorts[InPorts0, O, E], error)

func AddNode2[S, I1, I2, E any, O node.OutputSet](
	b *Builder,
	definition node.Stateful[S, node.Node2[S, I1, I2, O, E]],
) (NodePorts[InPorts2[I1, I2], O, E], error)

func AddClosureNode2[I1, I2, E any, O node.OutputSet](
	b *Builder,
	factory node.ClosureFactory[node.ClosureNode2[I1, I2, O, E]],
) (NodePorts[InPorts2[I1, I2], O, E], error)
```

`AddNode1` through `AddNode8` and `AddClosureNode1` through `AddClosureNode8` repeat the arity-two form with exactly N ordered input parameters. These are package functions, not generic methods. Parameter names are `b`, `definition`, and `factory`; no graph-specific wrapper config is introduced.

Alternative considered: generate a registration function for every input/output arity pair. Rejected because it expands the API quadratically without adding static safety.

### Bridge registration to later validation with static masks

Each successful registration appends one private `nodeMetadata` containing `NodeID`, registration kind, input count, ordinary output count, a `staticValidation` descriptor, and one dormant `nodeNormalizer` closure. Port IDs are derivable from the counts and one-based convention and need not be duplicated.

`validationMask` is a private `uint16`. Bit 0 is factory, bit 1 is source handler, bits 2 through 9 are input handlers 1 through 8, and bit 10 is selector. `staticValidation` contains three `validationMask` fields named `required`, `present`, and `deferred`. Explicit-state registration marks its factory, source/input handlers, and multi-input selector as required; it marks each statically non-nil function as present and leaves deferred empty. Closure registration marks its closure factory required and present when non-nil, while source/input handler and multi-input selector bits are deferred because they cannot be observed until the factory runs. Single-input and zero-input nodes do not require selectors. A later validator can report `required &^ present` without invoking user code, while a later runtime normalization stage handles `deferred` bits after factory invocation.

The exact private bridge type is `type nodeNormalizer func(*normalizationTarget) error`. `normalizationTarget` is a private control-plane placeholder with no dynamically typed fields. A generated registration function creates the normalizer while concrete type parameters are in scope, so the closure captures the complete explicit-state or closure-factory contract at its concrete type and calls a monomorphized generic normalization helper when a later stage supplies the target. This change stores but never invokes the closure.

The descriptor contains only masks and counts. The registration and normalization bridge does not place handlers, factories, inputs, outputs, errors, or data-plane values into `any` or `interface{}` fields or parameters, does not use reflection, and does not use runtime type assertions. Static nil-presence recording does not invoke callbacks.

Alternative considered: retain registered definitions in `[]any` and recover them with type assertions. Rejected because it moves compiler guarantees into runtime checks and violates the typed control-plane contract.

### Extend the shared generator with a graph-owned manifest

This change may extend the shared standard-library-only arity generator introduced by `generate-typed-node-arities`; doing so does not move graph declarations into `node`. The generator has a graph-specific manifest that owns exactly the generated graph port/bundle file and generated graph registration file. A graph-specific stability test regenerates only that manifest into a temporary directory and compares every listed file byte-for-byte with committed output, while also failing for missing or unexpected generated graph files.

Alternative considered: add a second graph-only generator. Rejected because one arity model avoids drift, while target-specific manifests and tests preserve clear ownership.

### Keep executable topology outside this change

This change mutates registration state only. It defines no edge model or connection behavior, no DAG or connectivity validation pass, no sealing transition, no `Build` method, and no immutable-definition behavior. The reserved sealed state, `ErrBuilderSealed`, and static validation descriptors are inputs to later capabilities, not partial implementations of them.

## Risks / Trade-offs

- [Prerequisite APIs change before implementation] -> Implement the named changes in order and generate graph signatures directly from their final contracts.
- [Generated API surface becomes repetitive] -> Generate only linear graph families, list every graph output in its own manifest, and verify that manifest deterministically.
- [Uniform stored closures appear type-erased] -> Keep their parameters and results private and non-generic, capture concrete values lexically, and prohibit `any`, reflection, and assertions in the registration path.
- [Builder copying could alias mutable state] -> Expose construction and mutation through `*Builder`, document that builders must not be copied, and keep identity/state private.
- [Port namespaces can be confused during later edge work] -> Define separate one-based input and output namespaces and reserve output port 9 for errors.
- [Closure-produced callbacks cannot be checked statically] -> Record them in the deferred mask and leave invocation-time checks to the later normalization stage.

## Migration Plan

1. Complete `generate-typed-node-arities`.
2. Complete `add-node-factory-contracts`.
3. Add the non-zero-sized owner token, builder state/errors, one-based ID conventions, opaque handles, and private validation metadata.
4. Extend the shared deterministic arity generator with a graph-specific manifest covering graph bundles, output expansion, and registration functions, then commit generated output.
5. Add focused builder registration examples without adding connection, build, or definition behavior.

There is no deployed executable graph builder. Rollback removes the new registration files and generated graph declarations.

## Open Questions

None for this change. Edge IDs, connection policy, validation diagnostics, the sealing transition, and public definition inspection are intentionally assigned to later changes.
