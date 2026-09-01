## 1. Handwritten Node Primitives

- [x] 1.1 Define sealed `OutputSet` with its private marker and exact exported `OutputCount() uint8` method while preserving typed `Result[E]`.
- [x] 1.2 Replace legacy arity-based handler names with explicit-state `SourceHandler[S, O, E]` and `InputHandler[S, I, O, E]` constrained by `O OutputSet`.
- [x] 1.3 Define `ReadyMask`, zero-based `PortIndex`, and `Port1` through `Port8` with the documented `1 << PortN` bit mapping.
- [x] 1.4 Define explicit `Selector[S]` and state-capturing `ClosureSelector` signatures and document that, whenever called for multiple ready inputs, they must return an in-range `PortIndex` marked ready in the supplied mask; defer invocation and enforcement to execution.
- [x] 1.5 Remove only the node-package placeholder `ErrorPort` declaration and handler-set fields while retaining `E` through `Result[E]` and reserving it for later graph registration's separate typed error output beyond eight ordinary outputs.

## 2. Standard-Library Arity Generator

- [x] 2.1 Create `internal/genarity` with one ordered zero-through-eight arity model, an extensible ordered output-target model for later graph templates, command-line temporary-output support, and no dependencies outside the standard library.
- [x] 2.2 Add deterministic `text/template` generation for `Outputs0` through `Outputs8`, including phantom output types, the private `OutputSet` marker, and exact `OutputCount() uint8` methods.
- [x] 2.3 Add deterministic template generation for explicit `Node0` through `Node8`, using one separately typed handler per input and selectors only on arities two through eight.
- [x] 2.4 Add deterministic template generation for `ClosureNode0` through `ClosureNode8`, using state-capturing handler signatures and closure selectors only on arities two through eight.
- [x] 2.5 Format every rendered file with `go/format`, add the standard generated-code header, and exclude timestamps, absolute paths, map iteration, and environment-derived content.
- [x] 2.6 Define this phase's ordered node-output manifest and add a `go:generate` entry point in `node` that renders only those node targets without generating factories or graph-package declarations.

## 3. Generated Contract Replacement

- [x] 3.1 Generate and retain the formatted output-shape and handler-set files in version control for normal builds without a generation prerequisite.
- [x] 3.2 Remove the superseded handwritten `Outputs0` through `Outputs8`, `Node0` through `Node8`, `Handler0`, `Handler1`, and node-package placeholder `ErrorPort` declaration and fields.
- [x] 3.3 Verify generated explicit and closure families cover every arity exactly once, constrain `O` to `OutputSet`, and preserve distinct `I1` through `I8` types without an input/output cross-product.
- [x] 3.4 Verify the resulting `node` package contains no state factory, closure factory, lifecycle, graph builder, graph port, registration, connection, validation, or definition API.

## 4. Contract And Generation Tests

- [x] 4.1 Add contract tests that instantiate `Outputs0` through `Outputs8`, assert exact `OutputCount() uint8` results, and compile them as `OutputSet` constraints.
- [x] 4.2 Add contract tests that instantiate every `Node0` through `Node8` and assign heterogeneous per-input `SourceHandler` or `InputHandler` functions with typed `Result[E]` outcomes.
- [x] 4.3 Add contract tests that instantiate every `ClosureNode0` through `ClosureNode8` and verify their handlers preserve heterogeneous input types without an explicit state parameter.
- [x] 4.4 Add tests for the zero-based `Port1` through `Port8` values, readiness-bit mapping, and explicit and closure selectors that return an in-range ready index when called with multiple ready inputs.
- [x] 4.5 Add and execute a compile-fail fixture proving an external arbitrary type cannot satisfy sealed `OutputSet` or instantiate a generated handler set.
- [x] 4.6 Add a generation-stability test that regenerates node targets into a temporary directory and compares every ordered node-output manifest path and byte with committed node files, without asserting that the generator's global output set can never grow or modifying the worktree.

## 5. Documentation And Verification

- [x] 5.1 Update node package documentation for sealed ordinary outputs, separate typed input handlers, explicit versus closure handler sets, readiness positions, the selector return obligation, and the separately reserved typed error output.
- [x] 5.2 Document that this change is dependency 1 of 6, downstream `add-node-factory-contracts` consumes these contracts, and the later graph-builder change may extend `internal/genarity` while factory and graph APIs remain unavailable here.
- [x] 5.3 Run `go generate ./...` and the generation-stability test, confirming regeneration leaves every committed file in this phase's node-output manifest unchanged.
- [x] 5.4 Run `gofmt`, `go test ./...`, `go test -race ./...`, and `go vet ./...` across the completed contract change.
- [x] 5.5 Run `git diff --check` and `openspec validate generate-typed-node-arities --strict` before handing the generated contracts to `add-node-factory-contracts`.
