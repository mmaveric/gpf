## ADDED Requirements

### Requirement: Typed external ingress source
AddIngress[T] SHALL register a dormant typed source with one ordinary output, a NodeID, and an IngressDefinition[T] handle. BindIngress SHALL validate provenance and return a fresh executor-local single-producer endpoint after Start. Invalid, foreign, or stopped bindings SHALL reject. Producer operations SHALL be serialized by one external caller; shutdown admission closure SHALL be allowed concurrently. The adapter SHALL process only accepted input work during drain and SHALL not count framework adapter polling as user handler calls.

#### Scenario: Connect external ingress
- **WHEN** a typed ingress output is connected to a matching input and the definition starts
- **THEN** its bound producer can submit owned or copied batches without boxing T

### Requirement: External ownership and status outcomes
Ingress Acquire SHALL return owned preallocated source storage. Commit SHALL return explicit accepted/full/closed/drop/invalid/oversize statuses, clearing the owned ingress handle only on acceptance or configured Drop. Submit SHALL reject oversize input atomically, copy one bounded batch, and release its private acquisition on nonaccepted outcomes. Uncommitted caller-owned batches SHALL remain releasable after stop. Ingress source pool rows SHALL use their registered source NodeID with EdgeID zero; ordinary edge pools SHALL keep their edge-origin identity.

#### Scenario: Release caller-owned work after stop
- **WHEN** a caller retains an uncommitted ingress acquisition through completed shutdown
- **THEN** Commit rejects without consuming it and Release returns it safely to its retained source pool
