# Select the next speed target

Type: grilling
Mode: HITL
Status: resolved
Blocked by: 07

## Question

Which measured path should be explored next after the scheduler-counter and input-buffer candidates failed to produce a statistically significant end-to-end speedup?

The current full-workflow profile still attributes 21.74% flat CPU samples to `graph.(*groupCounters).recordPass` and 62.06% cumulative time to `graph.(*executorWorker).pass`. The input-buffer candidate reduced allocations and RSS substantially but did not improve time significantly. Choose one next path while preserving exact totals, live metrics semantics, and the current topology; record the evidence and hypothesis before implementation.

## Answer

The user selected **worker idle polling/scheduling**, specifically the `executorWorker.pass`/idle loop path, as the next target. Keep the live metrics and Drain behavior intact; retain a change only if repeated end-to-end measurements show a statistically significant speedup.
