## 1. Prerequisite and Contract Alignment

- [x] 1.1 Complete and integrate `generate-typed-node-arities` before implementing this change, then confirm the generated explicit-state and closure handler-set names used as `H`.
- [x] 1.2 Review the generated handler signatures and preserve their `*S` explicit-state binding without changing generator output, graph APIs, or arity coverage in this change.

## 2. Node Factory Data Contracts

- [x] 2.1 Add the exact named types `StateFactory[S] func() (S, error)`, `InitFunc[S] func(*S) error`, and `CloseFunc[S] func(*S) error` to `node`.
- [x] 2.2 Add `Stateful[S, H]` with fields `Factory StateFactory[S]`, `Handlers H`, `Init InitFunc[S]`, and `Close CloseFunc[S]`, documenting the factory as required and lifecycle hooks as optional.
- [x] 2.3 Add `ClosureReplica[H]` with `Handlers H`, `Init func() error`, and `Close func() error` fields, plus `ClosureFactory[H] func() (ClosureReplica[H], error)`.
- [x] 2.4 Document on the public declarations that factory success transfers one lifecycle, factory failure transfers none, and a provided `Close` is required exactly once after successful construction on normal shutdown or partial-`Init` rollback.
- [x] 2.5 Document one factory call per logical replica per start, fresh mutable state for every successful call, and immutable captured configuration with no framework deep-copy guarantee.

## 3. Focused Contract Tests

- [x] 3.1 Add compile-time tests for the exact `StateFactory`, `InitFunc`, `CloseFunc`, `Stateful`, `ClosureReplica`, and `ClosureFactory` signatures with representative generated handler sets and no graph imports.
- [x] 3.2 Add direct data-contract tests covering required factory fields, nil optional lifecycle hooks, and preservation of typed handlers and callbacks in explicit and closure declarations.
- [x] 3.3 Add node-level examples that call representative factories directly for multiple simulated replicas and starts and verify independent explicit state and closure environments.
- [x] 3.4 Add direct lifecycle contract examples for successful factory results, failed factory results, and partial-`Init` rollback, preserving the rule that a provided `Close` runs once after each successful factory return and never after a failed return.
- [x] 3.5 Add an inert-data test proving that constructing, copying, and inspecting node factory declarations does not invoke their factories.

## 4. Deferred Graph Integration Boundary

- [x] 4.1 Document that the future graph registration bridge records factory presence and other statically visible validation facts without invoking either explicit state or closure factories.
- [x] 4.2 Document that future graph Build consumes the recorded static facts without invoking factories and that implementation and executable tests for this behavior belong to the next graph bridge change.
- [x] 4.3 Verify this change adds no graph Build tests or Builder, ports, registration, `Connect`, DAG, or `Definition` code.

## 5. Verification

- [x] 5.1 Run `gofmt` on changed Go files and `go test ./node` after the prerequisite generated arities are present.
- [x] 5.2 Run `go test ./...`, `go test -race ./...`, and `go vet ./...` to confirm the additive contracts do not regress existing packages.
- [x] 5.3 Run `git diff --check` and `openspec validate add-node-factory-contracts --strict`.
