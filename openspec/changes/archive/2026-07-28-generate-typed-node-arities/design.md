## Context

The `node` package currently handwrites `Outputs0` through `Outputs8` and explicit `Node0` through `Node8`. Their output parameter is unconstrained, the handler names encode arity rather than role, closure handler sets do not exist, and multi-input readiness has no typed contract. This repetition is already a source of drift risk and will be consumed by the next staged change, `add-node-factory-contracts`.

Go 1.22 has neither variadic generics nor generic methods, so fixed arities remain necessary. This change is dependency 1 of 6 and must stabilize only node-level type contracts before factories and graph construction are introduced.

## Goals / Non-Goals

**Goals:**

- Generate and commit complete output-shape and handler-set families for arities zero through eight.
- Constrain every handler set to a sealed ordinary-output shape.
- Preserve a distinct compile-time element type and handler for each input.
- Define explicit-state and state-capturing closure handler sets with typed readiness selection for multi-input nodes.
- Make generation standard-library-only, deterministic, formatted, and protected by byte-stability tests.
- Leave a stable contract for `add-node-factory-contracts` to consume.

**Non-Goals:**

- Define state factories, closure factories, lifecycle hooks, replicas, or runtime state creation.
- Define graph builders, input or output port handles, node registration, or topology identities.
- Implement `Connect`, edge configuration, fan-out, DAG validation, builder sealing, or `Definition`.
- Execute handlers, select ready inputs at runtime, publish output batches, route errors, or alter batch ownership semantics.
- Generate graph-package declarations or input/output arity cross-products.

## Decisions

### Keep stable primitives handwritten and generate repetitive families

Handwritten node source will own `Result[E]`, sealed `OutputSet` with exact method `OutputCount() uint8`, `SourceHandler`, `InputHandler`, `ReadyMask`, `PortIndex`, selectors, and their documentation. Generated source will own only `Outputs0` through `Outputs8`, explicit `Node0` through `Node8`, and closure `ClosureNode0` through `ClosureNode8`.

Separating semantic primitives from repetitive declarations keeps the generator small and makes contract changes reviewable. Generating all declarations, including stable primitives, was rejected because it would hide design decisions inside templates and create unnecessary generated churn.

### Model input and output arity independently

Each generated explicit handler set has one state type `S`, one distinct type parameter per input, one `O OutputSet`, and one error type `E`. `Node0` carries a `SourceHandler`; `Node1` through `Node8` carry one `InputHandler` field per input. Each generated closure handler set omits `S`; its function fields have equivalent signatures without the explicit `*S` argument so captured state remains possible without defining a factory in this change.

`Outputs0` through `Outputs8` independently model ordinary output arity and implement `OutputSet`. This produces nine output shapes and eighteen handler sets rather than an input/output cross-product.

Generating every input/output combination was rejected because it would create 81 primary combinations per handler style without adding static safety. Leaving `O` unconstrained was rejected because arbitrary types could masquerade as framework output shapes.

### Seal ordinary output shapes in the node package

`OutputSet` contains an unexported marker plus exported `OutputCount() uint8`. Only generated output shapes in `node` can implement the marker, and each shape returns its constant arity from zero through eight. Output element types remain phantom generic information; handlers continue to receive `*O` without boxing values or using reflection.

The error type `E` remains the parameter of `Result[E]` and is not part of `OutputSet`. This change removes only the node-package placeholder `ErrorPort` handle and its fields from handler sets. The main contract continues to reserve `E` so later graph registration exposes one separate typed error output beyond the eight ordinary outputs.

An open marker interface was rejected because external output shapes would prevent exhaustive arity handling. Including error output in `OutputsN` was rejected because it would consume or exceed the ordinary-output limit.

### Give every input its own typed handler

`SourceHandler[S, O, E]` replaces the source-specific arity name. `InputHandler[S, I, O, E]` is reused independently for every explicit input field, with a different `I` at each position. Closure handler fields mirror these signatures without `S`. This allows a node to consume, for example, `batch.Batch[int]` and `batch.Batch[string]` without type erasure.

A single union-like handler was rejected because Go cannot express heterogeneous input batches without boxing or runtime assertions. One handler receiving all input types was rejected because runtime dispatch is one ready input per invocation.

### Define readiness as an eight-bit positional contract

`ReadyMask` uses `uint8`; `PortIndex` identifies zero-based input positions; and `Port1` through `Port8` have values zero through seven. Bit `1 << PortN` indicates that input is ready. Explicit `Node2` through `Node8` include `Selector[S]`, while closure counterparts include `ClosureSelector`; zero- and one-input handler sets have no selector field.

Selectors are contractually called only when multiple inputs are ready. When called, a selector MUST return a `PortIndex` that is within the handler set's declared input arity and whose bit is set in the supplied `ReadyMask`. This change defines and documents that obligation but does not execute selectors; a future runtime will invoke them and enforce the returned-index contract. One-based stored values were rejected because they would waste one of the eight mask bits or require conversion on every mask operation.

### Generate from one ordered standard-library model

`internal/genarity` will enumerate integer arities zero through eight in ascending slices and render fixed templates with `text/template`. Its generation model will support an ordered collection of output targets so the later graph-builder change can add graph templates without replacing the generator. This phase registers and emits node targets only; it will format source with `go/format`, prepend the standard generated-code header, and write the same node-output manifest entries for every run. Templates will not use map iteration, timestamps, absolute paths, environment-dependent data, or external modules.

A `go:generate` directive in `node` will invoke only the node targets. Generated output shapes and handler sets will be committed so normal builds and downstream changes do not require generation. Extensibility is an internal generator boundary, not permission for this phase to emit graph declarations. Hand-maintained copies were rejected because synchronized explicit and closure families are exactly the repetition this change removes. Third-party generation tools were rejected because the required model is small and the repository requires deterministic standard-library generation.

### Verify public shape and byte stability

Contract tests will instantiate all output, explicit, and closure arities; assert exact `OutputCount() uint8` results and port-bit positions; exercise selectors that satisfy the ready, in-range return contract; and compile heterogeneous per-input handler assignments. A compile-fail fixture will demonstrate that an external arbitrary type cannot satisfy `OutputSet`.

A generation-stability test will run the node targets into a temporary directory and compare the ordered node-output manifest paths and every corresponding byte against committed node files. The manifest defines outputs owned by this phase, not a permanent global generator file set; later graph outputs can have their own manifest and checks without weakening node stability. This checks both omitted node arities and nondeterministic formatting without rewriting the worktree.

Snapshotting selected snippets was rejected because it can miss changes elsewhere in generated files. Regenerating directly over committed files in the test was rejected because tests must not mutate the source tree.

## Risks / Trade-offs

- [Generated public API is broad] -> Limit generation to three linear arity families from zero through eight and test every member.
- [A sealed interface uses interface syntax] -> Use it only as a generic compile-time constraint; do not store or dispatch data-plane values through interface values.
- [Handwritten primitives and templates can drift] -> Keep template inputs minimal and compile every generated family against the handwritten constraints.
- [Generated output can vary across edits or environments] -> Exclude variable metadata, format with `go/format`, and compare temporary regeneration byte-for-byte in tests.
- [Closure handler fields cannot be factory-validated yet] -> Define only their static signatures here and defer fresh-instance and lifecycle guarantees to `add-node-factory-contracts`.

## Migration Plan

1. Introduce the handwritten sealed output, handler, readiness, and selector primitives.
2. Add the standard-library generator and `go:generate` entry point, then generate and commit the output-shape and handler-set files.
3. Remove the handwritten arity declarations and legacy handler names once generated equivalents compile.
4. Add contract, compile-fail, and generation-stability tests, then update node documentation.

There is no released graph runtime or compatibility promise. Rollback restores the handwritten declarations and removes the generator and generated files.

## Open Questions

None for this dependency. Factory ownership and lifecycle semantics are intentionally deferred to `add-node-factory-contracts`.
