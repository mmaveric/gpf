## Why
Stage 4 calls for graceful drain with timeout, while the executor only exposes an unbounded Drain. Applications need a deadline-triggered transition to forced cleanup.
## What Changes
- Add context-aware drain while retaining existing Drain behavior.
- On cancellation, close admission, stop further drain passes, reclaim remaining work, and report the cause.
## Capabilities
### New Capabilities
- `deadline-drain`: Deadline-aware graceful-to-forced shutdown.
### Modified Capabilities
- `single-group-executor`: Add cooperative context-aware lifecycle stopping and retain retries for already committed source outputs after disabling new source invocations.
## Impact
Affects executor lifecycle and error reporting. Depends on `fix-moved-input-ownership` and `add-runtime-metrics`; later ingress and logger changes must participate in the same shutdown budget.
