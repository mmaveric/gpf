## ADDED Requirements

### Requirement: Exact validation entry point and builder-state behavior
The graph package SHALL expose `func (b *Builder) Validate() (*ValidationReport, error)`. Callers SHALL construct usable builders with `NewBuilder`; a nil receiver or uninitialized zero-value receiver SHALL return a nil report and a `*ValidationError` containing exactly one `ValidationCodeInvalidBuilder` issue. An initialized builder containing no nodes SHALL return a non-nil report with empty topology snapshots and a `*ValidationError` containing exactly one `ValidationCodeEmptyGraph` issue.

Every non-empty initialized builder SHALL return a non-nil report. It SHALL return nil error exactly when that report has no issues and otherwise SHALL return a `*ValidationError` whose issues equal the report issues. Validation SHALL accept both open builders and builders sealed by a later build stage; sealed state alone SHALL NOT produce an issue or `ErrBuilderSealed`. Validation SHALL preserve the receiver's existing open or sealed state.

#### Scenario: Reject nil and zero-value builders
- **WHEN** `Validate` is called on a nil `*Builder` or an uninitialized zero-value `Builder`
- **THEN** it returns a nil report and one `ValidationCodeInvalidBuilder` issue without panicking

#### Scenario: Reject an initialized empty builder
- **WHEN** `Validate` is called on a builder returned by `NewBuilder` before any node is registered
- **THEN** it returns a non-nil empty report and one `ValidationCodeEmptyGraph` issue

#### Scenario: Validate a non-empty open builder
- **WHEN** `Validate` is called on a non-empty initialized open builder
- **THEN** it returns a point-in-time report and either nil error or a matching `ValidationError` according to that report's issues

#### Scenario: Validate a sealed builder
- **WHEN** `Validate` is called on a builder sealed by the later immutable build stage
- **THEN** it analyzes the fixed topology without returning `ErrBuilderSealed` or changing sealed state

### Requirement: Reusable deterministic validation report
`ValidationReport` SHALL have unexported backing storage and SHALL expose defensive-copy `Issues() []ValidationIssue`, `TopologicalOrder() []NodeID`, `ForwardAdjacency() []ValidationAdjacency`, and `ReverseAdjacency() []ValidationAdjacency` methods. `ValidationAdjacency` SHALL contain `NodeID` and `Edges []ValidationAdjacentEdge`; `ValidationAdjacentEdge` SHALL contain `EdgeID` and `AdjacentNodeID`.

Each adjacency snapshot SHALL contain one row per registered node, including an empty row for a node with no edges, in ascending `NodeID` order. Forward rows SHALL list destination nodes and reverse rows SHALL list source nodes, with each row sorted by `AdjacentNodeID` then `EdgeID`. Snapshots SHALL contain only ordinary edges and SHALL own their row and edge storage independently of builder metadata.

For every acyclic non-empty topology, `TopologicalOrder` SHALL contain every node exactly once in canonical Kahn order even when other validation issues exist. For cyclic topology it SHALL be empty rather than exposing a partial Kahn prefix. The report SHALL contain sorted issues, and its accessors SHALL deep-copy issues, cycle witnesses, topological order, adjacency rows, and adjacency edges. It SHALL be a point-in-time analysis unaffected by later mutation of an open builder and SHALL provide the complete graph analysis that a later immutable `Build` can copy without rerunning validation algorithms.

#### Scenario: Snapshot an acyclic invalid graph
- **WHEN** an initialized non-empty graph is acyclic but has static, connectivity, edge-configuration, or reachability issues
- **THEN** its report contains those sorted issues, complete adjacency snapshots, and a full canonical topological order

#### Scenario: Suppress a partial cyclic order
- **WHEN** Kahn processing leaves residual cyclic nodes
- **THEN** the report retains complete adjacency snapshots but returns an empty topological order

#### Scenario: Keep a report independent of builder mutation
- **WHEN** a caller validates an open builder and then mutates that builder
- **THEN** the previously returned report's issues, order, and adjacency snapshots remain unchanged

#### Scenario: Protect nested report storage
- **WHEN** a caller changes any issue, witness, order, adjacency row, or edge slice returned by a report accessor
- **THEN** subsequent accessors and formatting of the associated validation error remain unchanged

### Requirement: Structured aggregate validation diagnostics
The graph package SHALL expose exported `ValidationCode`, `ValidationIssue`, and `ValidationError` types. `ValidationIssue` SHALL contain exported `Code ValidationCode`, `NodeID NodeID`, `PortID PortID`, `EdgeID EdgeID`, and `Cycle []NodeID` fields; its code SHALL determine which entity fields apply, and unused fields SHALL be zero. `ValidationError` SHALL expose `Issues() []ValidationIssue`, be discoverable with `errors.As`, contain every independently discoverable issue, and deep-copy issue and cycle storage. Its structured data and text SHALL remain identical across repeated validation of unchanged topology.

Issues SHALL sort by fixed validation-code order, then ascending `NodeID`, node-local `PortID`, and `EdgeID`, with any cycle witness compared lexicographically as a final key. `ValidationCode` SHALL reserve an invalid zero value that validation does not emit, followed in sort order by `ValidationCodeInvalidBuilder`, `ValidationCodeEmptyGraph`, `ValidationCodeMissingFactory`, `ValidationCodeMissingHandler`, `ValidationCodeMissingSelector`, `ValidationCodeUnconnectedInput`, `ValidationCodeUnconnectedOutput`, `ValidationCodeInvalidEdgeCapacity`, `ValidationCodeInvalidEdgePolicy`, `ValidationCodeCycle`, `ValidationCodeUnreachableFromSource`, and `ValidationCodeCannotReachSink`.

#### Scenario: Aggregate independently discoverable failures
- **WHEN** a builder contains multiple missing static contracts, disconnected ordinary ports, and topology failures
- **THEN** `Validate` returns one structured `ValidationError` containing all independently discoverable issues in the specified deterministic order

#### Scenario: Repeat validation without diagnostic drift
- **WHEN** `Validate` is called repeatedly without an intervening builder mutation
- **THEN** every call returns equivalent issue fields, cycle witnesses, ordering, and error text

#### Scenario: Match report and error issues
- **WHEN** an initialized builder returns a report with one or more issues
- **THEN** the returned `ValidationError` contains exactly the same sorted issue values and owns separate backing storage

### Requirement: Registration-produced static validation descriptors
Every generated graph registration path SHALL store a non-generic static validation descriptor containing `uint16` required and present masks in node structural metadata. Bit 0 SHALL represent the factory, bit 1 SHALL represent the selector, and bits 2 through 9 SHALL represent handler positions zero through seven. Source handler position zero SHALL represent the source handler; for a node with inputs, position `i` SHALL represent input handler `i`.

Explicit-state registration SHALL require its factory, every source or input handler, and a selector only when ordinary input arity exceeds one. Closure registration SHALL require only its factory because its handlers and selector do not exist until the factory runs. Generated generic registration SHALL determine present bits directly from statically typed function fields. Validation SHALL derive missing requirements from the stored masks and SHALL NOT use reflection, runtime type assertions, or `any` callback storage to rediscover them.

Validation SHALL report one missing-factory, missing-handler, or missing-selector issue for each applicable missing bit. It SHALL NOT invoke a state factory, closure factory, handler, selector, lifecycle callback, or other user callback. Callbacks that can exist only in a future closure-factory result SHALL NOT be inspected during builder validation.

#### Scenario: Report all missing static node contracts
- **WHEN** registered nodes have nil required factories, nil statically visible handlers, or a missing statically visible multi-input selector
- **THEN** `Validate` returns one entity-specific issue for every missing required static contract

#### Scenario: Accept absent single-input selector
- **WHEN** a node has zero or one ordinary input and all of its other static contracts are present
- **THEN** `Validate` does not report a missing-selector issue for that node

#### Scenario: Keep factories and callbacks dormant
- **WHEN** validation examines explicit-state and closure-based nodes whose factories or callbacks have observable side effects
- **THEN** validation completes without executing any factory or callback

#### Scenario: Validate from homogeneous static metadata
- **WHEN** validation examines heterogeneous registered node types and arities
- **THEN** it reports static contract issues solely from registration-produced masks without reflection, type assertions, or `any` callback storage

### Requirement: Ordinary connectivity and edge configuration validation
Validation SHALL require exactly one connection for every declared ordinary input and every declared ordinary output under the existing single-consumer connection model. A node SHALL be allowed to declare zero ordinary inputs or zero ordinary outputs. Typed error outputs SHALL remain non-connectable under the prior connection contract and SHALL be excluded from connectivity counts, adjacency snapshots, degree calculations, topological ordering, cycle detection, and source-to-sink reachability.

`Connect` and validation SHALL call the same unexported non-generic pure edge-configuration validator for capacity and queue-full policy; validation SHALL NOT duplicate those predicates. Validation SHALL map every violation to the associated edge identity and validation code. The shared helper SHALL NOT mutate topology or invoke an optional typed queue factory.

#### Scenario: Report disconnected ordinary ports
- **WHEN** one or more declared ordinary inputs or ordinary outputs have no connection
- **THEN** `Validate` reports a separate issue identifying each disconnected node and node-local port

#### Scenario: Exclude typed error outputs
- **WHEN** validation analyzes a node's typed error output, which cannot be passed to the prior ordinary `Connect` API
- **THEN** the error output contributes no connectivity issue, edge, degree, adjacency entry, cycle, or reachability path

#### Scenario: Recheck invalid stored edge configuration
- **WHEN** builder metadata contains an edge with non-positive capacity or an unsupported queue-full policy
- **THEN** the shared pure validator produces the same rejection used by `Connect`, and validation reports the applicable code and edge identity without invoking its queue factory

### Requirement: Deterministic DAG ordering and cycle witness
Validation SHALL use the report's forward adjacency snapshot to run Kahn's algorithm with the smallest available `NodeID` selected at each step and outgoing edges traversed by ascending destination `NodeID` then `EdgeID`. For an acyclic graph, the report SHALL contain all nodes exactly once in canonical order, and every edge SHALL lead from an earlier node to a later node.

If Kahn's algorithm leaves residual nodes, validation SHALL report a cycle issue containing one deterministic closed witness. Residual depth-first search SHALL start nodes in ascending `NodeID`, traverse outgoing residual edges by destination `NodeID` then `EdgeID`, and use the first back edge; every consecutive witness pair SHALL be an edge, the first and last node SHALL match, and a self-loop witness SHALL be `[N, N]`.

#### Scenario: Compute canonical order for a DAG
- **WHEN** multiple nodes are simultaneously ready in a valid acyclic graph
- **THEN** the validation report orders the smallest ready `NodeID` first and produces an order respected by every edge

#### Scenario: Report a stable cycle witness
- **WHEN** Kahn's algorithm cannot consume every node
- **THEN** `Validate` reports one cycle issue whose closed witness follows the deterministic residual DFS rules

#### Scenario: Report a self-loop witness
- **WHEN** a node has a stored edge to itself
- **THEN** the cycle issue carries that node twice as the complete witness

### Requirement: Bidirectional source-to-sink reachability
A structural source SHALL be a node with zero declared ordinary inputs, and a structural sink SHALL be a node with zero declared ordinary outputs. Validation SHALL traverse the report's forward snapshot from every structural source and its reverse snapshot from every structural sink. It SHALL report an unreachable-from-source issue for each node not found by the forward traversal and an unable-to-reach-sink issue for each node not found by the reverse traversal, including when cycle issues also exist.

Source and sink classification SHALL use declared ordinary arity rather than current edge degree. Typed error outputs are non-connectable and SHALL be absent from both traversals. A node with zero ordinary inputs and zero ordinary outputs SHALL be both a source and a sink and SHALL be valid in isolation when its static contracts are complete.

#### Scenario: Reject a component without a structural source
- **WHEN** a component's nodes are not reachable from any zero-input node
- **THEN** validation reports an unreachable-from-source issue for every node in that component

#### Scenario: Reject a component without a structural sink
- **WHEN** a component's nodes cannot reach any zero-output node
- **THEN** validation reports an unable-to-reach-sink issue for every node in that component

#### Scenario: Accept an isolated zero-arity node
- **WHEN** a statically complete node declares zero ordinary inputs and zero ordinary outputs and has no edges
- **THEN** validation treats it as both source and sink and reports no connectivity, reachability, or cycle issue

### Requirement: Validation does not mutate or seal the builder
Validation SHALL use only report-owned snapshots and local scratch state and SHALL NOT add, remove, or reorder nodes, ports, or edges; consume identities; cache topology results; open or seal the builder; or alter an existing sealed state. This SHALL hold after both successful and failed validation. Subsequent valid registration and connection operations on an open builder SHALL retain the same identity allocation behavior they would have had without validation.

#### Scenario: Correct topology after failed validation
- **WHEN** validation fails and the caller adds valid missing topology
- **THEN** the same builder remains mutable and can validate successfully after correction

#### Scenario: Continue building after successful validation
- **WHEN** validation succeeds on an open builder and the caller performs another otherwise valid builder mutation
- **THEN** the mutation is accepted and a later validation evaluates the updated topology

#### Scenario: Preserve sealed state
- **WHEN** validation succeeds or fails on an already sealed builder
- **THEN** it performs no topology mutation and the builder remains sealed by the preexisting build operation

#### Scenario: Preserve identity allocation
- **WHEN** a caller validates before adding the next node or edge
- **THEN** validation consumes no identity and the mutation receives the same identity it would have received without validation

### Requirement: Validation robustness under generated topologies
Validation SHALL terminate without panic for nil, zero-value, empty, open, sealed, and every topology constructible through the builder API. For unchanged initialized topology it SHALL return deterministic reports and diagnostics, and whenever it reports no issue its canonical report order SHALL contain every node once and respect every ordinary edge.

#### Scenario: Validate generated operation sequences
- **WHEN** fuzz or property tests generate arbitrary valid builder operation sequences and repeatedly validate the resulting topology
- **THEN** validation terminates without panic and produces deterministic diagnostics or a complete edge-respecting canonical order
