# Order-processing showcase

Run the deterministic, standard-library-only example from the repository root:

```sh
go run ./examples/showcase
go test ./examples/showcase
```

The finite workload contains four valid orders and one invalid order. The
validator reports the invalid record through both its typed error edge and the
optional numeric async logger. To keep a failed `Result` from discarding valid
items in the same handler invocation, valid records are submitted in batches and
invalid records individually.

## Topology

```text
typed ingress -> validator -- broadcast --> audit sink
                              |             -> shard relay -- shard --> partition 0 --┐
                              |                               -> partition 1 ---------+-- fan-in --> aggregate
                              |                               -> partition 2 ---------┘
                              +-- typed validation error --> error sink
```

The graph uses explicit fixed groups and CPU bindings. The requested CPU
affinity may fall back to a locked worker thread on the host; the final report
prints each affinity outcome and warning.

- **Broadcast** creates separate shallow-copy branch batches for the audit and
  relay paths. This example's value records contain no pointer referents.
- **Sharding** routes an order to destination index `CustomerID % 3`, using the
  partition inputs in the order listed in the graph setup. It splits batches
  into bounded branch batches and flushes partial batches at completion.
- **Fan-in** merges partition receipts with one producer-local pool per branch.
  Cross-producer delivery order is unspecified; the collector reports only
  aggregate counts and sums.
- **Metrics** report edge transfers/losses, delivered logger events, and pool
  usage after Drain completes. The sample workload fits the configured bounded
  queues and reports no queue or pool losses.

The summary should report 5 submitted, 4 processed, 1 rejected, 4 audited, 7
units, 1600 gross cents, and partition counts `1,2,1`.
