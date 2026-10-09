## ADDED Requirements

### Requirement: Dormant typed logging mapper declarations
Generated explicit-state and closure handler families SHALL expose optional typed `MapError ErrorMapper[E]` configuration matching the handler's concrete failure type. Typed mapper declarations and explicit conversion SHALL be available independently of asynchronous logging. Registration and Build SHALL not invoke the mapper, and Start without a logging destination SHALL leave runtime conversion dormant. Existing deferral of runtime logging/error-mapping execution SHALL not prohibit these typed declarations or explicit conversion helpers. Configured callbacks SHALL use immutable captures and SHALL be documented as nonblocking/nonallocating when invoked by a worker.

#### Scenario: Configure without logging activation
- **WHEN** a caller supplies a mapper matching a node's concrete E and starts the definition without a logger
- **THEN** failure routing remains typed and the mapper callback remains uncalled

#### Scenario: Reject a mismatched mapper declaration
- **WHEN** a caller assigns an ErrorMapper[string] to an int-failure node's MapError field
- **THEN** Go compilation rejects the mismatched function type before graph registration
