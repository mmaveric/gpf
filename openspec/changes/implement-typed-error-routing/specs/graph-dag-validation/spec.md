## MODIFIED Requirements

### Requirement: Reusable deterministic validation report
`ValidationReport` SHALL have unexported backing storage and SHALL expose defensive-copy `Issues() []ValidationIssue`, `TopologicalOrder() []NodeID`, `ForwardAdjacency() []ValidationAdjacency`, and `ReverseAdjacency() []ValidationAdjacency` methods. `ValidationAdjacency` SHALL contain `NodeID` and `Edges []ValidationAdjacentEdge`; `ValidationAdjacentEdge` SHALL contain `EdgeID` and `AdjacentNodeID`.

Each adjacency snapshot SHALL contain one row per registered node, including an empty row for a node with no edges, in ascending `NodeID` order. Forward rows SHALL list destination nodes and reverse rows SHALL list source nodes, with each row sorted by `AdjacentNodeID` then `EdgeID`. Snapshots SHALL contain ordinary and connected error edges and SHALL own their row and edge storage independently of builder metadata.

For every acyclic non-empty topology, `TopologicalOrder` SHALL contain every node exactly once in canonical Kahn order even when other validation issues exist. For cyclic topology it SHALL be empty rather than exposing a partial Kahn prefix. The report SHALL contain sorted issues, and its accessors SHALL deep-copy issues, cycle witnesses, topological order, adjacency rows, and adjacency edges. It SHALL be a point-in-time analysis unaffected by later mutation of an open builder and SHALL provide the complete graph analysis that a later immutable `Build` can copy without rerunning validation algorithms.

#### Scenario: Snapshot a connected error edge
- **WHEN** an error port is connected to a matching ordinary input
- **THEN** forward and reverse adjacency contain that edge in canonical order and the report remains independent of later builder mutation

#### Scenario: Snapshot an acyclic invalid graph
- **WHEN** an initialized non-empty graph is acyclic but has static, connectivity, edge-configuration, or reachability issues
- **THEN** its report contains those sorted issues, complete adjacency snapshots, and a full canonical topological order

#### Scenario: Suppress a partial cyclic order
- **WHEN** ordinary and error edges together leave residual cyclic nodes in Kahn processing
- **THEN** the report retains complete adjacency snapshots and returns an empty topological order

#### Scenario: Keep a report independent of builder mutation
- **WHEN** a caller validates an open builder and then mutates that builder
- **THEN** the previously returned report's issues, order, and adjacency snapshots remain unchanged

#### Scenario: Protect nested report storage
- **WHEN** a caller changes any issue, witness, order, adjacency row, or edge slice returned by a report accessor
- **THEN** subsequent accessors and formatting of the associated validation error remain unchanged

### Requirement: Ordinary connectivity and edge configuration validation
Validation SHALL require exactly one connection for every declared ordinary input and every declared ordinary output under the existing single-consumer connection model. An ordinary input connected from either an ordinary or error output SHALL satisfy its one-producer requirement. A node SHALL be allowed to declare zero ordinary inputs or zero ordinary outputs. Error outputs SHALL be optional and SHALL not produce an unconnected-output issue.

`Connect`, `ConnectError`, and validation SHALL call the same unexported non-generic pure edge-configuration validator for capacity, batch capacity, and queue-full policy; validation SHALL NOT duplicate those predicates. Validation SHALL map every violation to the associated edge identity and validation code. The shared helper SHALL NOT mutate topology or invoke an optional typed queue factory.

#### Scenario: Accept an input fed by an error edge
- **WHEN** a declared ordinary input has exactly one matching error-edge producer
- **THEN** validation reports no unconnected-input issue for that input

#### Scenario: Report disconnected ordinary ports
- **WHEN** one or more declared ordinary inputs or ordinary outputs have no connection
- **THEN** `Validate` reports a separate issue identifying each disconnected node and node-local port

#### Scenario: Exclude typed error outputs
- **WHEN** validation analyzes an unconnected typed error output
- **THEN** the error output contributes no connectivity issue, edge, degree, adjacency entry, cycle, or reachability path

#### Scenario: Permit an unconnected error output
- **WHEN** one or more registered error outputs have no consumer
- **THEN** validation reports no connectivity issue for those error ports

#### Scenario: Recheck invalid stored edge configuration
- **WHEN** builder metadata contains an ordinary edge with invalid capacity, batch capacity, or policy
- **THEN** the shared pure validator reports the applicable code and edge identity without invoking its queue factory

#### Scenario: Recheck invalid stored error-edge configuration
- **WHEN** builder metadata contains an error edge with invalid capacity, batch capacity, or policy
- **THEN** the shared pure validator reports the applicable code and edge identity without invoking its queue factory

### Requirement: Bidirectional source-to-sink reachability
A structural source SHALL be a node with zero declared ordinary inputs, and a structural sink SHALL be a node with zero declared ordinary outputs. Validation SHALL traverse the report's forward snapshot from every structural source and its reverse snapshot from every structural sink. Because the snapshot includes connected error edges, an error consumer reachable only through a potential failed result SHALL participate in reachability. It SHALL report an unreachable-from-source issue for each node not found by the forward traversal and an unable-to-reach-sink issue for each node not found by the reverse traversal, including when cycle issues also exist.

Source and sink classification SHALL use declared ordinary arity rather than current edge degree. Unconnected error outputs SHALL be absent from both traversals.

#### Scenario: Reach an error-only consumer
- **WHEN** a producer's error port connects to a consumer whose only input is that error edge
- **THEN** validation treats the consumer as reachable from the producer and includes it in source-to-sink analysis

#### Scenario: Reject a component without a structural source
- **WHEN** a component's nodes are not reachable from any zero-input node
- **THEN** validation reports an unreachable-from-source issue for every node in that component

#### Scenario: Reject a component without a structural sink
- **WHEN** a component's nodes cannot reach any zero-output node
- **THEN** validation reports an unable-to-reach-sink issue for every node in that component

#### Scenario: Accept an isolated zero-arity node
- **WHEN** a statically complete node declares zero ordinary inputs and zero ordinary outputs and has no edges
- **THEN** validation treats it as both source and sink and reports no connectivity, reachability, or cycle issue
