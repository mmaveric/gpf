# Select a target after polling results

Type: grilling
Mode: HITL
Status: resolved
Blocked by: 09

## Question

Which path should be the next optimization target after the idle-yield experiment failed the full-process speed criterion?

The full-workflow allocation profile attributes 31.07% flat allocation space to `reflect.growslice` and 13.48% to `os.readFileContents`; the current loader reads the JSON file and unmarshals into a growing order slice. The idle-yield change improved `BenchmarkFullShowcase` but had no statistically significant effect on prebuilt full-process launches. Choose a next target or explicitly stop the speed search, while keeping exact outputs and the strict full-process acceptance metric.

One distinct candidate in the same full CPU profile is the generated one-input runner `graph.normalizeExplicit1` specialization, at 15.42% cumulative time. Its runner calls `input1.ready()` before `input1.pop()`; for the showcase, readiness checks queue length and pop attempts the dequeue. A narrow hypothesis is to remove the readiness precheck for single-input runners while preserving readiness masks/selectors for multi-input nodes. This is a candidate to measure, not an established speedup.

## Answer

The user selected the **single-input direct-pop experiment**: bypass the separate `ready()` queue-length check before `pop()` for one-input runners, preserve multi-input readiness-mask/selector behavior, and retain the change only if full-process launches improve significantly.
