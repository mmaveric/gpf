package showcase

import (
	"fmt"
	"io"

	"github.com/mmaveric/gpf/graph"
)

func writeReport(output io.Writer, inputCount int, stats *showcaseStats, executor *graph.Executor, clock string) error {
	metrics := executor.Metrics()
	var acceptedElements, dequeuedElements, queueDropped, poolDropped, deliveredLogs, poolsInUse uint64
	for _, edge := range metrics.Edges {
		acceptedElements += edge.AcceptedElements
		dequeuedElements += edge.DequeuedElements
		queueDropped += edge.QueueDroppedElements
		poolDropped += edge.PoolDroppedElements
	}
	for _, logger := range metrics.Loggers {
		deliveredLogs += logger.DeliveredEvents
	}
	for _, pool := range metrics.Pools {
		poolsInUse += uint64(pool.InUseSample)
	}
	var measuredCalls, measuredNanoseconds, maxMeasuredNanoseconds uint64
	for _, node := range metrics.Nodes {
		measuredCalls += node.MeasuredCalls
		measuredNanoseconds += node.MeasuredNanoseconds
		maxMeasuredNanoseconds = max(maxMeasuredNanoseconds, node.MaxMeasuredNanoseconds)
	}
	if _, err := fmt.Fprintf(output, "orders submitted=%d processed=%d rejected=%d audited=%d units=%d gross_cents=%d partitions=%d,%d,%d\n",
		inputCount, stats.processed.Load(), stats.rejected.Load(), stats.audited.Load(), stats.units.Load(), stats.totalCents.Load(), stats.partitions[0].Load(), stats.partitions[1].Load(), stats.partitions[2].Load()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "runtime edges=%d accepted_elements=%d dequeued_elements=%d queue_drop_elements=%d pool_drop_elements=%d logged_errors=%d pools_in_use=%d\n",
		len(metrics.Edges), acceptedElements, dequeuedElements, queueDropped, poolDropped, deliveredLogs, poolsInUse); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "handler_timing_clock=%s measured_calls=%d measured_nanoseconds=%d max_nanoseconds=%d\n",
		clock, measuredCalls, measuredNanoseconds, maxMeasuredNanoseconds); err != nil {
		return err
	}
	for _, status := range executor.AffinityStatus() {
		if _, err := fmt.Fprintf(output, "affinity group=%d cpu=%d outcome=%d warning=%v\n", status.GroupID, status.CPU, status.Outcome, status.Warning); err != nil {
			return err
		}
	}
	return nil
}
