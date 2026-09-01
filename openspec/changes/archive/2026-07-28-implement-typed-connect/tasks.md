## 1. Typed Queue and Edge Contracts

- [x] 1.1 Verify the generated arity, node-factory, and typed-builder prerequisite changes expose registered builder-owned ports and open-builder mutation hooks required by this change.
- [x] 1.2 Add the exact `type Factory[T any] func(Config) (QueueOps[T], error)` API and document fresh per-runtime construction and caller-owned captures.
- [x] 1.3 Replace placeholder `EdgeConfig` with the exact generic fields `Capacity uint32`, `Policy queue.Policy`, and `Factory queue.Factory[T]`.
- [x] 1.4 Define or integrate the one-based `EdgeID` contract so zero remains invalid and only committed edges advance identity.
- [x] 1.5 Export the seven specified connection sentinels and `EdgeConfigError` with `Config queue.Config`, `Error`, and `Unwrap` matching `ErrInvalidEdgeConfig`.

## 2. Typed Connection Storage

- [x] 2.1 Add builder-local structural edge metadata for source, target, capacity, and policy without adding DAG validation, definition construction, or runtime queue state.
- [x] 2.2 Add input-producer and output-consumer occupancy tracking to the builder's registered endpoint metadata.
- [x] 2.3 Preserve private metadata that distinguishes registered typed error outputs from ordinary outputs without adding error edges.
- [x] 2.4 Add the package-private monomorphized future-binding closure that captures `queue.Factory[T]` without invoking it, erasing `QueueOps[T]` through `any`, using reflection, or requiring a runtime type assertion.
- [x] 2.5 Replace definition-targeted placeholder `Connect[T]` with the exact builder-targeted signature and commit structural metadata, endpoint occupancy, and typed binding together.

## 3. Atomic Local Validation

- [x] 3.1 Add the pure package-private `validateEdgeConfig(queue.Config) error` function for positive capacity and `queue.Drop` or `queue.Wait`, returning `*EdgeConfigError` without accessing builder state or invoking a factory.
- [x] 3.2 Make `Connect` use the shared validator and leave it reusable by later DAG validation against stored non-generic edge metadata.
- [x] 3.3 Implement the exact precedence `ErrBuilderSealed`, invalid output/input, foreign output/input, `ErrErrorRoutingUnavailable`, `ErrInputConnected`, `ErrFanOutUnsupported`, then `ErrInvalidEdgeConfig`.
- [x] 3.4 Reject typed error outputs before cardinality and configuration checks without appending topology, occupying endpoints, retaining or invoking factories, or consuming IDs.
- [x] 3.5 Enforce one producer per ordinary input and one consumer per ordinary output, returning `ErrInputConnected` or `ErrFanOutUnsupported` as specified.
- [x] 3.6 Implement deterministic validate-then-commit behavior so every error returns `EdgeID(0)`, preserves endpoint occupancy and topology, and leaves no gap in dense edge IDs.
- [x] 3.7 Recognize the builder's established sealed state and return `ErrBuilderSealed` without implementing Build or any sealing transition in this change.

## 4. Connection and Compiler Tests

- [x] 4.1 Add table-driven success tests for matching registered ports, both queue policies, default and custom factories, heterogeneous `int` and `string` edges, and consecutive edge IDs.
- [x] 4.2 Add table-driven rejection tests for sealed builders, zero and foreign handles, typed error outputs, zero capacity, unknown policy values, duplicate input producers, and unsupported fan-out.
- [x] 4.3 Test atomic recovery after each rejection, including unchanged existing edges, unoccupied unaffected ports, dormant rejected factories, and dense IDs on the next success.
- [x] 4.4 Test every `errors.Is` category, `errors.As` for `*EdgeConfigError`, and the exact result when multiple invalid conditions exercise deterministic precedence.
- [x] 4.5 Test the pure edge-config validator directly and prove successful and rejected custom factories remain dormant during connection and simulated later Build validation.
- [x] 4.6 Test that stored future-binding metadata remains specialized for the edge element type without retaining live queue operations.
- [x] 4.7 Replace placeholder examples with registered matching heterogeneous ordinary ports and valid generic edge configuration.
- [x] 4.8 Update the compile-fail fixture and Go-toolchain test to prove `OutPort[int]` cannot connect to `InPort[string]`, while matching typed examples compile.

## 5. Documentation and Verification

- [x] 5.1 Update `graph` and `queue` package documentation for exact APIs, deterministic errors, builder-targeted ordinary connections, reserved error outputs, pure config validation, per-runtime factories, dense IDs, and unsupported fan-out.
- [x] 5.2 Run `gofmt` on changed Go files, `go test ./...`, `go test -race ./...`, and `go vet ./...`.
- [x] 5.3 Confirm the change introduces no graph-wide DAG validation, source-to-sink validation, `Definition` construction, Build implementation or sealing transition, error routing, queue execution, or runtime startup behavior.
- [x] 5.4 Run `git diff --check` and `openspec validate implement-typed-connect --strict`.
