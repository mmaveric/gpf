# Measure core-aware CPU placement

Type: task
Mode: AFK
Status: resolved
Blocked by: 11

## Question

Does assigning the seven fixed worker groups to distinct physical cores improve full-process wall time compared with the current sequential CPU mapping?

### Work

- Keep the node/edge topology and worker-group count unchanged. Compare the current CPU mapping 0–6 with one distinct CPU per physical core on this host (candidate: 0, 2, 4, 5, 6, 7, 8); confirm every affinity assignment succeeds.
- Build each binary once and run baseline/candidate launches serially in paired or alternating order against the same `examples/data.json`, for at least ten runs per mapping.
- Require exact output and zero queue/pool drops and logged errors. Record wall time, peak RSS, CPU mapping, affinity outcomes, Go/OS/hardware, and dataset hash.
- Keep the candidate mapping only if the end-to-end speed improvement is statistically significant. Otherwise restore the current mapping and record the no-change result.
- Do not change framework code, topology, queue capacities, or worker-group count.

### Resolution evidence

Attach paired raw launch samples and statistical comparison, environment/affinity records, output checks, and the final mapping or no-change result.

## Answer

- Compared the current CPUs 0–6 mapping with one logical CPU per physical core, `[0,2,4,5,6,7,8]`, using ten serial alternating pairs.
- Candidate mean wall time was 1.259 s versus 1.271 s baseline (estimated 0.94% speedup), but the paired two-sided t-test was not significant (`p=0.619`, 95% CI −3.21% to +5.10%). No mapping change was retained; the example remains on CPUs 0–6.
- All 20 outputs had exact totals, zero queue/pool drops, zero logged errors, and successful affinity. Peak RSS means were 234,532 KB baseline and 230,554 KB candidate.
- The mapping experiment made no code changes or commits. Raw data and analysis are in `/tmp/opencode/core-aware-cpu-placement/README.txt`.
