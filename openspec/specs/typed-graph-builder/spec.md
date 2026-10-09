## Purpose
Define typed graph builder ownership, registration, opaque handles, and generated port families.

## Requirements

### Requirement: Exact builder identity and state contract
The graph package SHALL expose `func NewBuilder() *Builder`, `var ErrInvalidBuilder`, `var ErrBuilderSealed`, and `var ErrNodeIDExhausted`. `NewBuilder` SHALL return an open mutable builder with initialized registration state and a fresh private owner pointer. The pointed-to owner type SHALL have nonzero size so owner pointers from distinct live builders cannot compare equal. The zero-value `Builder` and a nil `*Builder` SHALL be invalid. Builder state SHALL represent invalid, open, and sealed values, but this capability SHALL provide no operation that transitions an open builder to sealed.

Every registration function SHALL return `ErrInvalidBuilder` for invalid state, `ErrBuilderSealed` for sealed state, and `ErrNodeIDExhausted` instead of wrapping node identity. Those failures SHALL leave registration state unchanged.

#### Scenario: Construct builders with distinct owners
- **WHEN** a caller invokes `NewBuilder` twice and keeps both builders live
- **THEN** each builder is open and their private non-zero-sized owner pointers compare unequal

#### Scenario: Reject an uninitialized builder
- **WHEN** a caller registers through a nil builder pointer or a zero-value `Builder`
- **THEN** registration returns `ErrInvalidBuilder` without consuming a node identity or returning valid handles

#### Scenario: Reserve sealed-state rejection
- **WHEN** a same-package test places a builder in the reserved sealed state and attempts registration
- **THEN** registration returns `ErrBuilderSealed` without mutation even though this capability exposes no sealing operation

### Requirement: Zero-invalid stable node and port identities
The exact ID declarations SHALL be `type NodeID uint32`, `const InvalidNodeID NodeID = 0`, `type PortID uint8`, and `const InvalidPortID PortID = 0`. Successful registrations SHALL receive dense stable `NodeID` values beginning at 1 in registration order. Inputs and outputs SHALL use separate node-local `PortID` namespaces; input and ordinary-output positions SHALL use IDs 1 through 8, and the separate typed error output SHALL use output `PortID(9)`. Rejected registrations SHALL NOT consume an ID.

#### Scenario: Assign dense node identities
- **WHEN** three nodes are registered successfully
- **THEN** their node IDs are 1, 2, and 3 in registration order and remain stable

#### Scenario: Assign one-based port identities
- **WHEN** a node with two inputs and two ordinary outputs is registered
- **THEN** its input IDs are 1 and 2, its ordinary-output IDs are 1 and 2, and its typed error-output ID is 9

#### Scenario: Preserve identity after rejection
- **WHEN** one registration fails and the next registration succeeds
- **THEN** the successful registration receives the node ID that was next before the failure

### Requirement: Exact opaque handle APIs
The graph package SHALL define opaque `InPort[T]`, `OutPort[T]`, `ErrorPort[E]`, and `OutputRef[O]` values with private owner and identity fields. `InPort[T]`, `OutPort[T]`, and `ErrorPort[E]` SHALL expose exactly the value-receiver accessors `NodeID() NodeID`, `PortID() PortID`, and `Valid() bool`. `OutputRef[O]` SHALL expose exactly `NodeID() NodeID`, `OutputCount() uint8`, and `Valid() bool`; it SHALL represent all ordinary outputs for one node and SHALL NOT expose a singular `PortID`.

Each singular handle's `Valid` method SHALL require a non-nil owner and nonzero node and port IDs. `OutputRef.Valid` SHALL require a non-nil owner, a nonzero node ID, and an output count from zero through eight. External callers SHALL have no owner constructor or field access, so a zero-value handle remains invalid while preserving its generic type. `ErrorPort[E]` SHALL not be assignable to `OutPort[E]`.

#### Scenario: Inspect registered singular handles
- **WHEN** a caller inspects registered input, ordinary-output, and error-output handles
- **THEN** each handle reports its stable node and port IDs and `Valid` reports true

#### Scenario: Inspect a registered output reference
- **WHEN** a caller inspects an `OutputRef[node.Outputs3[int, string, bool]]` returned by registration
- **THEN** it reports the registered node ID, output count 3, and valid status without exposing a port ID

#### Scenario: Keep an error handle distinct
- **WHEN** external code attempts to use `ErrorPort[int]` where `OutPort[int]` is required
- **THEN** compilation fails without a runtime provenance or type check

#### Scenario: Inspect zero-value handles
- **WHEN** a caller inspects zero-value `InPort[int]`, `OutPort[string]`, `ErrorPort[error]`, and `OutputRef[node.Outputs0]` values
- **THEN** singular handles report invalid IDs, the output reference reports an invalid node ID, and every `Valid` call reports false

### Requirement: Exact NodePorts API
The graph package SHALL define opaque `NodePorts[I, O, E]`, with `O` constrained by `node.OutputSet`, and exactly these value-receiver accessors: `NodeID() NodeID`, `Inputs() I`, `Outputs() OutputRef[O]`, and `Errors() ErrorPort[E]`. A successful registration SHALL return one `NodePorts` whose four accessors refer to one owner and node. A zero-value `NodePorts` SHALL return invalid component handles and `InvalidNodeID`.

#### Scenario: Access a heterogeneous registration result
- **WHEN** registration succeeds for two differently typed inputs, three differently typed ordinary outputs, and a typed error
- **THEN** `NodeID`, `Inputs`, `Outputs`, and `Errors` return the matching typed values for one stable node identity

#### Scenario: Access zero-value NodePorts
- **WHEN** a caller invokes every accessor on a zero-value `NodePorts`
- **THEN** `NodeID` is invalid and the returned input, output reference, and error port are invalid without a panic

### Requirement: Complete generated graph port and expansion families
The shared arity generator SHALL generate and commit graph-owned `InPorts0` through `InPorts8` with fields named `In1` through `In8` as applicable and `OutPorts0` through `OutPorts8` with fields named `Out1` through `Out8` as applicable. The generated expansion function names SHALL be exactly `OutputPorts0`, `OutputPorts1`, through `OutputPorts8`. `OutputPortsN` SHALL accept only `OutputRef[node.OutputsN[...]]` and return the matching `OutPortsN[...]`.

Registration SHALL obtain ordinary output arity by evaluating the zero value of `O node.OutputSet` and calling its prerequisite `OutputCount() uint8` method, without reflection or type assertion. It SHALL store that count in `OutputRef`. `OutputPortsN` SHALL require the stored count to equal N, preserve owner and node identity, and derive ordinary-output IDs 1 through N; a count mismatch or invalid reference SHALL return a zero invalid bundle.

#### Scenario: Compile all exact generated names
- **WHEN** compile tests instantiate every `InPortsN`, `OutPortsN`, and `OutputPortsN` declaration from zero through eight
- **THEN** all exact names compile and each nonzero field has its declared concrete element type

#### Scenario: Expand heterogeneous outputs
- **WHEN** `OutputPorts3` expands a valid `OutputRef[node.Outputs3[int, string, bool]]`
- **THEN** it returns `Out1 OutPort[int]`, `Out2 OutPort[string]`, and `Out3 OutPort[bool]` with port IDs 1, 2, and 3 for the reference owner and node

#### Scenario: Reject an invalid expansion reference
- **WHEN** an `OutputPortsN` function receives an invalid reference or a same-package test supplies a stored count other than N
- **THEN** it returns a zero `OutPortsN` containing no valid handles

### Requirement: Exact generated registration families
The graph package SHALL generate package functions named exactly `AddNode0` through `AddNode8` and `AddClosureNode0` through `AddClosureNode8`. `AddNodeN` SHALL accept `*Builder` and the matching prerequisite `node.Stateful[S, node.NodeN[...]]`; `AddClosureNodeN` SHALL accept `*Builder` and the matching `node.ClosureFactory[node.ClosureNodeN[...]]`. Each SHALL return `(NodePorts[InPortsN[...], O, E], error)`, with `O` independently constrained by `node.OutputSet`. Arity zero SHALL omit input type parameters and return `InPorts0`. Each generated registration SHALL construct `ErrorPort[E]{owner: builder owner, nodeID: registered node, portID: 9}` only after successful registration.

The generated families SHALL vary only by input arity. Ordinary output arity SHALL be recovered later through the exact `OutputPortsN` functions, and the generator SHALL NOT emit an input/output arity cross-product.

#### Scenario: Register explicit state
- **WHEN** a caller passes a prerequisite explicit-state definition to the `AddNodeN` function matching its input arity
- **THEN** the function returns typed `NodePorts` with stable input, ordinary output-reference, and distinct error-port identities

#### Scenario: Register a closure factory
- **WHEN** a caller passes a prerequisite closure factory to the `AddClosureNodeN` function matching its input arity
- **THEN** the function returns the same graph-owned result shape with a distinct error-port identity and records closure registration kind

#### Scenario: Register explicit state with an error handle
- **WHEN** a caller passes a prerequisite explicit-state definition to the `AddNodeN` function matching its input arity
- **THEN** the function returns typed `NodePorts` with stable input, ordinary output-reference, and distinct error-port identities

#### Scenario: Combine arities independently
- **WHEN** compile tests combine representative input arities with zero-, one-, and eight-output `node.OutputSet` shapes and typed error connections
- **THEN** all combinations compile through the linear registration and expansion families without an input/output arity cross-product

### Requirement: Precise static validation bridge
Every successful registration SHALL atomically append one private `nodeMetadata` record containing node ID, explicit or closure registration kind, input count, ordinary output count, one `staticValidation` descriptor, and one dormant `nodeNormalizer` closure. `validationMask` SHALL be a private `uint16`: bit 0 SHALL represent factory, bit 1 source handler, bits 2 through 9 input handlers 1 through 8, and bit 10 selector. `staticValidation` SHALL contain `required`, `present`, and `deferred` fields of that mask type.

Explicit-state registration SHALL mark its factory and source or input handlers as required, plus selector when input count exceeds one; it SHALL mark every statically non-nil required function as present and SHALL leave deferred empty. Closure registration SHALL mark its factory required and present when non-nil; it SHALL mark its factory-produced source or input handlers and multi-input selector as deferred because their presence is not statically observable. A later validator SHALL be able to compute missing statically known functions as `required &^ present` without invoking user code.

The exact private bridge type SHALL be `type nodeNormalizer func(*normalizationTarget) error`. `normalizationTarget` SHALL be private control-plane state with no dynamically typed fields. The generated generic registration function SHALL construct `nodeNormalizer` while all concrete types are in scope. The closure SHALL capture the complete concrete explicit-state or closure-factory contract and retain a monomorphized path to that future target. Registration SHALL store but SHALL NOT invoke the normalizer or any factory, handler, selector, initialization hook, or close hook.

Neither `nodeMetadata`, `staticValidation`, `nodeNormalizer`, nor their registration path SHALL use `any` or `interface{}` fields or parameters, reflection, or runtime type assertions to retain or recover registered values.

#### Scenario: Record explicit static presence
- **WHEN** an explicit two-input registration has a non-nil factory and first handler but nil second handler and selector
- **THEN** required marks factory, both handlers, and selector; present marks only factory and first handler; deferred is empty; and no callback runs

#### Scenario: Record closure deferred presence
- **WHEN** a non-nil two-input closure factory is registered
- **THEN** required and present mark the factory, deferred marks both produced handlers and selector, and the factory is not invoked

#### Scenario: Audit dynamic mechanisms
- **WHEN** handwritten and generated graph registration code is inspected
- **THEN** registration metadata and normalization contain no dynamic storage, reflection, or runtime type assertions

### Requirement: Graph-specific generated output ownership
The shared standard-library-only arity generator MAY be extended with graph templates and SHALL define a graph-specific output manifest listing every generated graph port/bundle and registration file. A graph-specific stability test SHALL regenerate only that manifest into a temporary directory, compare every listed file byte-for-byte with committed output, and fail for a missing or unexpected generated graph file. Generated graph declarations SHALL remain in package `graph` and SHALL carry the standard generated-code header.

#### Scenario: Regenerate the graph manifest
- **WHEN** the shared generator runs for its graph target with unchanged templates
- **THEN** the graph-specific stability test reports every manifested file unchanged and no extra graph-generated output

#### Scenario: Detect graph output drift
- **WHEN** a manifested graph file differs, is absent, or an unmanifested generated graph file appears
- **THEN** the graph-specific stability test fails independently of node generated-output checks

### Requirement: Acyclic package and stage boundary
The `graph` package SHALL import and consume `node` contracts, while `node` SHALL NOT import `graph`. Graph handles, bundles, registration results, and registration functions SHALL remain in `graph`. This capability SHALL add no edge model or connection behavior, no DAG or connectivity validation pass, no builder-sealing transition, no `Build` method, and no immutable-definition behavior. Reserved sealed state, `ErrBuilderSealed`, static masks, and dormant normalization SHALL be extension points only.

#### Scenario: Verify package direction
- **WHEN** package dependencies are inspected and compiled
- **THEN** dependency direction is `node <- graph` with no import cycle or graph handle in `node`

#### Scenario: Verify focused builder state
- **WHEN** nodes are registered successfully
- **THEN** the builder contains owner, lifecycle state, node identities, static validation descriptors, and dormant normalizers but no edges, topological order, build result, or immutable definition
