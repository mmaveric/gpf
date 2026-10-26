# Decide whether to prototype wake-driven workers

Type: grilling
Mode: HITL
Status: resolved
Blocked by: 12

## Question

Should the next experiment replace or supplement busy worker polling with internal wake-driven idling?

Profiles still show substantial time in `executorWorker.pass`, but counter updates, a higher idle-yield threshold, direct-pop, and physical-core CPU placement have not produced a statistically significant prebuilt-launch speedup. A wake-driven design could reduce repeated empty-queue polling, but requires queue-to-worker notifications and careful preservation of Drain, live metrics, and custom queue behavior.

Choose whether to authorize an isolated prototype with a polling fallback for queues that cannot signal, keep the current architecture and stop this speed search, or specify a different bounded next target. No public API or latency-contract change is assumed without a separate decision.

## Answer

The user said to continue after the recommended wake-driven direction. Proceed with an isolated internal prototype that keeps the public API, live metrics, Drain behavior, and polling fallback for custom queues. Retain it only if full-process launches show a statistically significant speedup; otherwise revert it.
