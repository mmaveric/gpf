## MODIFIED Requirements

### Requirement: Structured aggregate validation diagnostics
The graph package SHALL expose exported `ValidationCode`, `ValidationIssue`, and `ValidationError` types. `ValidationIssue` SHALL contain exported `Code ValidationCode`, `NodeID NodeID`, `PortID PortID`, `EdgeID EdgeID`, `GroupID GroupID`, `CPU int`, and `Cycle []NodeID` fields; its code SHALL determine which entity fields apply, and unused fields SHALL be zero. `ValidationError` SHALL expose `Issues() []ValidationIssue`, be discoverable with `errors.As`, contain every independently discoverable issue, and deep-copy issue and cycle storage. Its structured data and text SHALL remain identical across repeated validation of unchanged topology.

Issues SHALL sort by fixed validation-code order, then ascending `NodeID`, node-local `PortID`, `EdgeID`, `GroupID`, and `CPU`, with any cycle witness compared lexicographically as a final key. `ValidationCode` SHALL reserve an invalid zero value that validation does not emit, followed in sort order by `ValidationCodeInvalidBuilder`, `ValidationCodeEmptyGraph`, `ValidationCodeMissingFactory`, `ValidationCodeMissingHandler`, `ValidationCodeMissingSelector`, `ValidationCodeUnconnectedInput`, `ValidationCodeUnconnectedOutput`, `ValidationCodeInvalidEdgeCapacity`, `ValidationCodeInvalidEdgePolicy`, `ValidationCodeCycle`, `ValidationCodeUnreachableFromSource`, `ValidationCodeCannotReachSink`, `ValidationCodeInvalidGroupPlacement`, `ValidationCodeRemoteQueueFactory`, `ValidationCodeMissingGroupCPU`, `ValidationCodeInvalidGroupCPU`, `ValidationCodeDuplicateGroupCPU`, and `ValidationCodeIneffectiveGroupCPU`.

#### Scenario: Aggregate independently discoverable failures
- **WHEN** a builder contains multiple missing static contracts, disconnected ordinary ports, topology failures, and CPU-binding failures
- **THEN** Validate returns one structured ValidationError containing all independently discoverable issues in the specified deterministic order

#### Scenario: Repeat validation without diagnostic drift
- **WHEN** Validate is called repeatedly without an intervening builder mutation
- **THEN** every call returns equivalent issue fields, cycle witnesses, ordering, and error text

#### Scenario: Match report and error issues
- **WHEN** an initialized builder returns a report with one or more issues
- **THEN** the returned ValidationError contains exactly the same sorted issue values and owns separate backing storage

#### Scenario: Identify a CPU-binding failure
- **WHEN** validation finds a missing, invalid, duplicate, or ineffective CPU binding
- **THEN** its issue contains the affected GroupID and applicable CPU value without using a NodeID, PortID, EdgeID, or cycle witness
