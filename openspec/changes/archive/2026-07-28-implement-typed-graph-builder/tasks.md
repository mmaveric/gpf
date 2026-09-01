## 1. Prerequisite and Scope Gate

- [x] 1.1 Verify `generate-typed-node-arities` is implemented before `add-node-factory-contracts`, and verify both changes are complete before modifying graph registration APIs.
- [x] 1.2 Confirm the prerequisite node output-shape, handler-set, explicit-state, and closure-factory names used by this change, updating only this change's graph signatures when aligning with those completed contracts.
- [x] 1.3 Record focused implementation boundaries in graph package documentation: registration is mutable, callbacks are dormant, and connection behavior, validation passes, the sealing transition, `Build`, and immutable definitions remain unavailable.

## 2. Builder Identity and Opaque Handles

- [x] 2.1 Define the exact `NodeID uint32` and `PortID uint8` APIs with zero invalid, dense node IDs from 1, separate one-based input/output namespaces, and error-output `PortID(9)`.
- [x] 2.2 Implement `NewBuilder() *Builder` with a private owner type containing at least one byte, then test that owner pointers from two live builders compare unequal.
- [x] 2.3 Add private invalid/open/sealed builder states and exact `ErrInvalidBuilder`, `ErrBuilderSealed`, and `ErrNodeIDExhausted` sentinels, without adding a sealing transition.
- [x] 2.4 Implement opaque builder-owned `InPort[T]` and `OutPort[T]` with exactly `NodeID`, `PortID`, and `Valid` accessors, plus `OutputRef[O]` with exactly `NodeID`, `OutputCount`, and `Valid` accessors.
- [x] 2.5 Add exact private-field `NodePorts[I, O, E]` with `NodeID`, `Inputs`, `Outputs`, and `Errors` value-receiver accessors and safe zero-value behavior.
- [x] 2.6 Add focused tests for invalid builder forms, reserved sealed rejection, invalid zero-value handles/results, stable dense IDs, one-based port IDs, ID exhaustion, and atomic ID allocation after rejection.

## 3. Generated Graph Port Families

- [x] 3.1 Extend the shared deterministic arity model with graph `InPorts0` through `InPorts8` fields `In1` through `In8` and `OutPorts0` through `OutPorts8` fields `Out1` through `Out8`.
- [x] 3.2 Generate functions named exactly `OutputPorts0` through `OutputPorts8`, restricted to matching `OutputRef[node.OutputsN[...]]` shapes and deriving one-based ordinary-output IDs.
- [x] 3.3 Use `O node.OutputSet` and its zero value's `OutputCount() uint8` to populate `OutputRef` and structural metadata without reflection or type assertions.
- [x] 3.4 Make output expansion preserve owner/node identity and return a zero invalid bundle for an invalid reference or stored-count mismatch.
- [x] 3.5 Add a graph-specific generated-output manifest covering graph port/bundle and registration files, with standard headers and no graph output owned by the node manifest.
- [x] 3.6 Add a graph-specific stability test that regenerates only its manifest, compares listed files byte-for-byte, and rejects missing, changed, or unexpected graph-generated files.
- [x] 3.7 Add compile tests for every exact bundle and expansion name, heterogeneous outputs, and compile-fail coverage for a mismatched output shape.

## 4. Explicit and Closure Registration

- [x] 4.1 Add private `nodeMetadata` with node ID, registration kind, input/output counts, `staticValidation`, and one dormant `nodeNormalizer`, without edge, topology, build-result, or definition state.
- [x] 4.2 Define private `validationMask uint16` with factory bit 0, source bit 1, input-handler bits 2 through 9, selector bit 10, and exact `required`, `present`, and `deferred` fields in `staticValidation`.
- [x] 4.3 Implement explicit-state mask population so required covers factory, source/input handlers, and multi-input selector, present reflects static nil checks, and deferred is empty.
- [x] 4.4 Implement closure mask population so required/present cover the visible factory and deferred covers factory-produced source/input handlers and multi-input selector.
- [x] 4.5 Define `nodeNormalizer func(*normalizationTarget) error` and a private target placeholder with no dynamically typed fields, then capture each concrete contract through a monomorphized generic path without `any`, reflection, or runtime type assertions.
- [x] 4.6 Generate exact `AddNode0` through `AddNode8` signatures for prerequisite explicit-state contracts, varying only input arity and returning matching `NodePorts`.
- [x] 4.7 Generate exact `AddClosureNode0` through `AddClosureNode8` signatures for prerequisite closure factories, varying only input arity and returning matching `NodePorts`.
- [x] 4.8 Keep output shape independent from input arity and verify generated APIs contain no input/output arity cross-product.
- [x] 4.9 Add tests for explicit and closure registration at arities zero, one, and eight, heterogeneous middle arities, stable kind/count metadata, typed error port 9, and exact validation masks.
- [x] 4.10 Add callback-counter tests proving registration invokes neither the dormant normalizer nor state factories, closure factories, handlers, selectors, initialization hooks, or close hooks.

## 5. Architecture and Verification

- [x] 5.1 Verify `graph` imports `node`, `node` does not import `graph`, and all graph handles, bundles, registration results, and registration functions remain in `graph`.
- [x] 5.2 Audit handwritten and generated registration code for heterogeneous `any` or `interface{}` storage or parameters, reflection imports/calls, and runtime type assertions.
- [x] 5.3 Verify this change adds no edge/connection behavior, validation pass, sealing transition, `Build` method, topology, build result, or immutable-definition behavior.
- [x] 5.4 Run graph-manifest regeneration, `go generate ./...`, and a second unchanged regeneration, then run `gofmt` over handwritten Go files.
- [x] 5.5 Run focused graph tests, compile/compile-fail fixtures, `go test ./...`, `go test -race ./...`, and `go vet ./...`.
- [x] 5.6 Run `git diff --check` and `openspec validate --all --strict`.
