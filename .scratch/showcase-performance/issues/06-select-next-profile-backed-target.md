# Select the next profile-backed target

Type: grilling
Mode: HITL
Status: resolved
Blocked by: 05

## Question

The scheduler-metrics candidate did not produce a statistically significant speedup and was reverted. The full-workflow profile now shows `reflect.growslice` at 31.07% flat allocation space, `showcase.Run` at 30.98%, `batch.NewPool` at 14.33%, and `os.readFileContents` at 13.48%. CPU samples still show `groupCounters.recordPass` at 21.74% flat; `executorWorker.pass` is 62.06% cumulative.

Which single cost center should be the next optimization target?

Choose a target that can materially lower the full-application median while preserving exact million-order totals, the live metrics contract, and the demonstrated topology. Weigh the expected speed impact against memory use; identify whether the work belongs in the showcase or a narrowly scoped GPF core path. Record the chosen hypothesis and evidence before implementation.

## Answer

The user selected **`showcase.Run` input buffering**. First test whether the all-valid workload can submit slices from the original input without building a second million-element `valid` slice. Preserve the existing mixed-validity/error-routing behavior and output. Keep the change only if repeated end-to-end measurements show a statistically significant speedup; compare allocations and peak RSS as secondary metrics.
