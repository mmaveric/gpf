## ADDED Requirements

### Requirement: Deterministic node arity generation
The repository SHALL use a standard-library-only generator to produce and commit formatted node output shapes, explicit handler sets, and closure handler sets for every arity from zero through eight. This phase SHALL register and emit node files only through an ordered node-output manifest. Generated node files SHALL carry the standard generated-code marker and SHALL be byte-for-byte reproducible from an unchanged generator without timestamps, absolute paths, map-order dependence, or other environment-dependent content. `internal/genarity` SHALL permit a later graph-builder change to add separate graph output targets without changing the node-output manifest's stability contract. This phase SHALL NOT produce state factories, graph builders, graph ports, node registration, connections, DAG validation, or graph definitions.

#### Scenario: Regenerate committed node declarations
- **WHEN** the arity generator runs with unchanged source and templates
- **THEN** every path in the ordered node-output manifest has byte-for-byte identical temporary and committed content

#### Scenario: Generate every supported family member
- **WHEN** the generated node package is compiled
- **THEN** output shapes, explicit handler sets, and closure handler sets are available at every arity from zero through eight without an input/output cross-product

#### Scenario: Keep generation scoped to node contracts
- **WHEN** the generated declarations are inspected
- **THEN** they contain no factory, graph-construction, connection, validation, or definition APIs

#### Scenario: Extend the generator in a later change
- **WHEN** a later graph-builder change adds separate graph output targets to `internal/genarity`
- **THEN** the node-output manifest and its byte-stability checks remain node-specific rather than rejecting additions to a permanent global file set

## MODIFIED Requirements

### Requirement: Node and error result contract
The node contract SHALL define sealed `OutputSet` shapes `Outputs0` through `Outputs8` for zero through eight ordinary typed outputs, and every shape SHALL implement exact exported method `OutputCount() uint8`. It SHALL define explicit-state handler sets `Node0` through `Node8` and state-capturing closure handler sets `ClosureNode0` through `ClosureNode8`. `Node0` SHALL use a typed `SourceHandler`; every input of `Node1` through `Node8` SHALL have a separate `InputHandler` whose batch element type matches that input, and closure handler fields SHALL preserve the equivalent per-input types without an explicit state parameter. Multi-input explicit and closure handler sets SHALL respectively expose typed selectors over `ReadyMask`; `ReadyMask` SHALL represent eight input positions through zero-based `PortIndex` constants `Port1` through `Port8`. Whenever called for multiple ready inputs, a selector MUST return a `PortIndex` that is both within its handler set's declared input range and marked ready in the supplied mask; runtime invocation and enforcement SHALL remain deferred to the execution phase. Every handler SHALL return `Result[E]`. The node-package placeholder error handle SHALL be removed, while the main contract SHALL reserve `E` for later graph registration to expose as one separate typed error output beyond the eight ordinary outputs. The declarations SHALL NOT require reflection, runtime type assertions, or boxed data-plane values.

#### Scenario: Declare typed handler outcome
- **WHEN** a developer declares an explicit or closure handler set with error type `E`
- **THEN** every source or input handler returns `Result[E]` without boxing its inputs, outputs, or error value

#### Scenario: Compile all handler-set arities
- **WHEN** a developer instantiates explicit and closure handler sets at each supported input arity
- **THEN** `Node0` through `Node8` and `ClosureNode0` through `ClosureNode8` compile against any generated `OutputSet`

#### Scenario: Report exact ordinary output count
- **WHEN** a developer calls `OutputCount()` on any generated output shape
- **THEN** the call returns that shape's arity from zero through eight as `uint8`

#### Scenario: Preserve heterogeneous input types
- **WHEN** a two-input node declares `int` as its first input and `string` as its second input
- **THEN** its first handler accepts `batch.Batch[int]`, its second handler accepts `batch.Batch[string]`, and both share the same output-shape and error types

#### Scenario: Reject arbitrary output shape
- **WHEN** external code attempts to instantiate a handler set with a type outside `Outputs0` through `Outputs8`
- **THEN** compilation fails because the type cannot implement the sealed `OutputSet` contract

#### Scenario: Address ready inputs by mask position
- **WHEN** a selector is called with multiple ready bits in a handler set's readiness mask
- **THEN** it returns a zero-based `Port1` through `Port8` index that is within that handler set's input arity and whose bit is ready

#### Scenario: Keep error outcome outside ordinary outputs
- **WHEN** a handler uses `Outputs8` and returns a typed error outcome
- **THEN** all eight ordinary output types remain available, `Result[E]` carries the error type without a node-package placeholder handle, and `E` is reserved for later graph registration to expose as a separate typed error output
