# typed-error-mapping Specification

## Purpose
Specify typed conversion of node failures into fixed-size events suitable for bounded logging transport.

## Requirements

### Requirement: Typed fixed-size mapping contract
ErrorMapper[E] SHALL convert concrete E to an ErrorEvent containing numeric code/severity and fixed numeric attributes without boxed E. Configuration SHALL remain immutable and Build SHALL not invoke the mapper. A standalone conversion helper SHALL preserve the configured numeric payload.
#### Scenario: Convert a typed failure
- **WHEN** a configured mapper converts a concrete failure value
- **THEN** the result carries the expected fixed numeric payload without a runtime type assertion

### Requirement: Mapping is independent of error-edge delivery
Mapping hooks SHALL permit one event per failed return when an asynchronous destination is installed, regardless of error-edge pool exhaustion, Drop, or absence. With no logging destination installed, runtime mapping SHALL remain dormant. Callback allocation/blocking obligations SHALL be documented rather than guaranteed by the framework.
#### Scenario: Keep mapping dormant
- **WHEN** a definition with mapper configuration builds and starts without a logger destination
- **THEN** construction and processing do not call the dormant mapper merely to inspect its type or payload
