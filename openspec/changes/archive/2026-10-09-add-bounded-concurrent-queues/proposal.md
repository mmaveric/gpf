## Why
Stage 4 requires pure-Go rings beyond the existing remote SPSC implementation. Replicas and worker pools need bounded MPSC, SPMC, and MPMC transport with explicit ownership.
## What Changes
- Add preallocated typed MPSC, SPMC, and MPMC queue constructors alongside SPSC.
- Define nonblocking TryPush/TryPop/Len semantics, overflow handling, and concurrency tests.
## Capabilities
### New Capabilities
- `bounded-concurrent-queues`: Portable bounded queue models and allocation-free ownership transfer.
### Modified Capabilities
None.
## Impact
Affects queue internals and stress benchmarks. Depends on `fix-moved-input-ownership`; automatic selection and topology expansion are separate changes.
