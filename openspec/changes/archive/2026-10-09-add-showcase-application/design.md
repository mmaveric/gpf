## Context

See proposal.md. The repository has runnable framework packages and feature-specific tests, but no `examples/` application. The showcase must use only public APIs and should be executable on machines where requested CPU affinity is unavailable; the runtime's locked-thread fallback already supports that case.

## Goals / Non-Goals

**Goals:**
- Provide one deterministic, standard-library-only order pipeline that can be run with `go run ./examples/showcase`.
- Make the topology legible: ingress and validation, an audit broadcast, customer-key sharding, partition processing, fan-in aggregation, typed error delivery, and observable metrics.
- Keep framework resources bounded and stop the executor cleanly after the finite sample workload.
- Allow tests to invoke the application with controlled input/output writers and verify the final summary without depending on worker scheduling order.

**Non-Goals:**
- Add or change framework APIs, queue semantics, or performance claims.
- Add external services, persistent storage, network transports, configuration frameworks, or third-party dependencies.
- Demonstrate every feature or topology combination; in particular, the showcase will not combine sharding/broadcast endpoints with replica or node-worker-pool expansion.

## Decisions

- **Use a finite order-processing workload.** `Order` records carry an order ID, customer ID, unit count, and unit price. A validator converts valid records to the routed order type and returns a typed validation failure for invalid records. This is more concrete than synthetic integer batches while remaining small and deterministic.
- **Keep graph construction in a reusable `Run` function.** Put the runnable entry point under `examples/showcase` and build the graph in a function that accepts writers and the finite input set. `main` supplies the sample orders; tests supply their own buffers. This avoids a second graph API and permits end-to-end assertions.
- **Use one explicit graph for the showcase.** A typed ingress feeds validation. Valid output is connected with `ConnectBroadcast` to an audit sink and a relay node. The relay passes its batch to `ConnectShard`, whose selector routes elements by customer ID to three partition processors. Their typed outputs merge through `ConnectFanIn` into a result collector. Validation errors use a typed error edge; an error mapper and bounded asynchronous logger demonstrate diagnostic mapping separately from the typed error sink.
- **Place fixed workers across CPU-bound groups without expanding endpoints.** Use distinct manually assigned groups for validation/relay, audit, each partition, aggregation, and error handling. Bind explicit nonnegative CPUs in deterministic group order. Affinity fallback is a supported outcome, not an application failure; the example reports it in the final status summary.
- **Use conservative bounded capacities.** Define named constants for ingress, broadcast, shard, fan-in, and logging capacities. Size the finite test workload within ingress admission capacity, and keep shard branch batch capacity smaller than the ingress batch capacity so the example visibly exercises element splitting and partial flushes.
- **Summarize with synchronized application counters.** Partition and audit callbacks run on different workers, so shared totals use standard-library atomics. The output reports accepted/rejected order counts, per-partition results, audit observations, and copied runtime metrics after Drain completes. It does not assert global cross-branch ordering.
- **Keep the example self-documenting.** Add a short `examples/showcase/README.md` with the run command, topology sketch, and explanations of shallow broadcast copies, selector index routing, unordered fan-in, typed failures, and affinity fallback. Add a link from the root README.

## Risks / Trade-offs

- **Demo complexity obscures the core API** → Keep each node handler small, name every group and capacity, and include a topology diagram in the example README.
- **Concurrent callbacks produce nondeterministic output ordering** → Assert aggregate counts and values rather than line order; use atomic counters for shared totals.
- **Host CPU IDs are unavailable or restricted** → Start remains valid through affinity fallback and the example prints the reported status rather than failing.
- **The workload can deadlock if admission exceeds bounded capacity** → Submit a finite number of batches within configured ingress capacity, check every ingress status, and Drain only after successful submissions.

## Migration Plan

No migration is required. The example is additive and uses existing public contracts. Rollback consists of removing `examples/showcase` and its README links; framework packages and APIs are unaffected.
