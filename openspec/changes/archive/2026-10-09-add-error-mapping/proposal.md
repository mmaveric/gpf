## Why
Stage 5's typed errors are routable but cannot be converted into fixed logging events without application-specific adapters. A typed mapper establishes the boundary needed by asynchronous logging.
## What Changes
- Define fixed-size ErrorEvent values and allocation-free ErrorMapper[E] function contracts.
- Add typed mapping configuration and preserve original E routing independently.
## Capabilities
### New Capabilities
- `typed-error-mapping`: Typed conversion to fixed numeric logging events.
### Modified Capabilities
- `typed-pipeline-contracts`: Make typed mapper declarations and explicit conversion available while keeping automatic logging execution dormant without a destination.
## Impact
Affects node/graph configuration and generated failure bindings. Depends on `fix-moved-input-ownership`; transport and logger execution belong to `add-async-error-logger`.
