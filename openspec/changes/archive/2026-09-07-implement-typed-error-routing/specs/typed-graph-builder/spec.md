## MODIFIED Requirements

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
