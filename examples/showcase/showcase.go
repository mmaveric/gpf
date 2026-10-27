package main

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/graph"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

const (
	groupIngress graph.GroupID = iota + 1
	groupAudit
	groupPartition0
	groupPartition1
	groupPartition2
	groupAggregate
	groupErrors
)

const (
	ingressCapacity      uint32 = 8
	ingressBatchCapacity uint32 = 4
	broadcastCapacity    uint32 = 4
	shardCapacity        uint32 = 4
	shardBatchCapacity   uint32 = 2
	fanInCapacity        uint32 = 8
	loggerCapacity       uint32 = 4
)

type rawOrder struct {
	ID         uint64
	CustomerID uint64
	Units      uint32
	UnitCents  uint32
}

type order struct {
	ID         uint64
	CustomerID uint64
	Units      uint32
	UnitCents  uint32
}

type validationError struct {
	OrderID uint64
	Code    uint32
}

type receipt struct {
	OrderID    uint64
	CustomerID uint64
	Partition  uint32
	Units      uint32
	TotalCents uint64
}

type showcaseStats struct {
	audited    atomic.Uint64
	rejected   atomic.Uint64
	processed  atomic.Uint64
	units      atomic.Uint64
	totalCents atomic.Uint64
	partitions [3]atomic.Uint64
}

type showcaseGraph struct {
	definition     *graph.Definition
	ingress        graph.IngressDefinition[rawOrder]
	broadcastEdges []graph.EdgeID
	shardEdges     []graph.EdgeID
	fanInEdges     []graph.EdgeID
	partitionNodes [3]graph.NodeID
}

func edgeConfig[T any](capacity, batchCapacity uint32) graph.EdgeConfig[T] {
	return graph.EdgeConfig[T]{Capacity: capacity, BatchCapacity: batchCapacity, Policy: queue.Wait}
}

func isValidOrder(value rawOrder) bool {
	return value.ID != 0 && value.Units != 0 && value.UnitCents != 0
}

func buildShowcase(stats *showcaseStats) (*showcaseGraph, error) {
	builder := graph.NewBuilder()
	ingress, err := graph.AddIngress[rawOrder](builder, graph.IngressConfig{
		Capacity:      ingressCapacity,
		BatchCapacity: ingressBatchCapacity,
		Policy:        queue.Wait,
	})
	if err != nil {
		return nil, fmt.Errorf("register ingress: %w", err)
	}

	validator, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, rawOrder, node.Outputs1[order], validationError]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, rawOrder, node.Outputs1[order], validationError]{
			MapError: node.ErrorMapper[validationError](func(failure validationError) node.ErrorEvent {
				return node.ErrorEvent{Code: failure.Code, Attributes: [4]uint64{failure.OrderID}}
			}),
			Handle1: func(_ *struct{}, input batch.Batch[rawOrder], outputs *node.Outputs1[order]) node.Result[validationError] {
				for _, value := range input.Items() {
					if !isValidOrder(value) {
						return node.Result[validationError]{Value: validationError{OrderID: value.ID, Code: 1}, Failed: true}
					}
				}
				output, ok := outputs.Out1.Acquire()
				if !ok {
					return node.Result[validationError]{Value: validationError{Code: 2}, Failed: true}
				}
				for index, value := range input.Items() {
					output.Mutable()[index] = order{ID: value.ID, CustomerID: value.CustomerID, Units: value.Units, UnitCents: value.UnitCents}
				}
				output.SetLen(input.Len())
				if !outputs.Out1.Commit(&output) {
					output.Release()
					return node.Result[validationError]{Value: validationError{Code: 2}, Failed: true}
				}
				return node.Result[validationError]{}
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("register validator: %w", err)
	}

	audit, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, order, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, order, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[order], _ *node.Outputs0) node.Result[error] {
			stats.audited.Add(uint64(input.Len()))
			return node.Result[error]{}
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("register audit sink: %w", err)
	}

	shardRelay, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, order, node.Outputs1[order], error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, order, node.Outputs1[order], error]{Handle1: func(_ *struct{}, input batch.Batch[order], outputs *node.Outputs1[order]) node.Result[error] {
			outputs.Out1.Commit(&input)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("register shard relay: %w", err)
	}

	partitionGroups := [3]graph.GroupID{groupPartition0, groupPartition1, groupPartition2}
	partitions := make([]graph.NodePorts[graph.InPorts1[order], node.Outputs1[receipt], error], len(partitionGroups))
	for index := range partitions {
		partition := uint32(index)
		partitions[index], err = graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, order, node.Outputs1[receipt], error]]{
			Factory: func() (struct{}, error) { return struct{}{}, nil },
			Handlers: node.Node1[struct{}, order, node.Outputs1[receipt], error]{Handle1: func(_ *struct{}, input batch.Batch[order], outputs *node.Outputs1[receipt]) node.Result[error] {
				result, ok := outputs.Out1.Acquire()
				if !ok {
					return node.Result[error]{Failed: true, Value: errors.New("showcase result pool exhausted")}
				}
				for itemIndex, value := range input.Items() {
					result.Mutable()[itemIndex] = receipt{
						OrderID:    value.ID,
						CustomerID: value.CustomerID,
						Partition:  partition,
						Units:      value.Units,
						TotalCents: uint64(value.Units) * uint64(value.UnitCents),
					}
				}
				result.SetLen(input.Len())
				if !outputs.Out1.Commit(&result) {
					result.Release()
					return node.Result[error]{Failed: true, Value: errors.New("showcase result commit failed")}
				}
				stats.partitions[partition].Add(uint64(input.Len()))
				return node.Result[error]{}
			}},
		})
		if err != nil {
			return nil, fmt.Errorf("register partition %d: %w", index, err)
		}
	}

	aggregate, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, receipt, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, receipt, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[receipt], _ *node.Outputs0) node.Result[error] {
			for _, value := range input.Items() {
				stats.processed.Add(1)
				stats.units.Add(uint64(value.Units))
				stats.totalCents.Add(value.TotalCents)
			}
			return node.Result[error]{}
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("register aggregate sink: %w", err)
	}

	errorSink, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, validationError, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, validationError, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[validationError], _ *node.Outputs0) node.Result[error] {
			stats.rejected.Add(uint64(input.Len()))
			return node.Result[error]{}
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("register error sink: %w", err)
	}

	placements := []struct {
		node  graph.NodeID
		group graph.GroupID
	}{
		{ingress.NodeID(), groupIngress},
		{validator.NodeID(), groupIngress},
		{shardRelay.NodeID(), groupIngress},
		{audit.NodeID(), groupAudit},
		{partitions[0].NodeID(), groupPartition0},
		{partitions[1].NodeID(), groupPartition1},
		{partitions[2].NodeID(), groupPartition2},
		{aggregate.NodeID(), groupAggregate},
		{errorSink.NodeID(), groupErrors},
	}
	for _, placement := range placements {
		if err := graph.Place(builder, placement.node, placement.group); err != nil {
			return nil, fmt.Errorf("place node %d: %w", placement.node, err)
		}
	}
	for index, group := range []graph.GroupID{groupIngress, groupAudit, groupPartition0, groupPartition1, groupPartition2, groupAggregate, groupErrors} {
		if err := graph.BindCPU(builder, group, index); err != nil {
			return nil, fmt.Errorf("bind group %d CPU: %w", group, err)
		}
	}
	if err := graph.SetLogger(builder, graph.LoggerConfig{Capacity: loggerCapacity, Handle: func(node.ErrorEvent) {}}); err != nil {
		return nil, fmt.Errorf("configure error logger: %w", err)
	}

	if _, err := graph.Connect(builder, ingress.Output(), validator.Inputs().In1, edgeConfig[rawOrder](ingressCapacity, ingressBatchCapacity)); err != nil {
		return nil, fmt.Errorf("connect ingress: %w", err)
	}
	if _, err := graph.ConnectError(builder, validator.Errors(), errorSink.Inputs().In1, edgeConfig[validationError](4, 1)); err != nil {
		return nil, fmt.Errorf("connect validation errors: %w", err)
	}
	broadcastEdges, err := graph.ConnectBroadcast(builder, graph.OutputPorts1(validator.Outputs()).Out1, []graph.InPort[order]{audit.Inputs().In1, shardRelay.Inputs().In1}, edgeConfig[order](broadcastCapacity, ingressBatchCapacity))
	if err != nil {
		return nil, fmt.Errorf("connect audit broadcast: %w", err)
	}
	shardInputs := []graph.InPort[order]{partitions[0].Inputs().In1, partitions[1].Inputs().In1, partitions[2].Inputs().In1}
	shardEdges, err := graph.ConnectShard(builder, graph.OutputPorts1(shardRelay.Outputs()).Out1, shardInputs, func(value order) uint32 {
		return uint32(value.CustomerID % uint64(len(partitions)))
	}, edgeConfig[order](shardCapacity, shardBatchCapacity))
	if err != nil {
		return nil, fmt.Errorf("connect customer shards: %w", err)
	}
	partitionOutputs := make([]graph.OutPort[receipt], len(partitions))
	for index, partition := range partitions {
		partitionOutputs[index] = graph.OutputPorts1(partition.Outputs()).Out1
	}
	fanInEdges, err := graph.ConnectFanIn(builder, partitionOutputs, aggregate.Inputs().In1, edgeConfig[receipt](fanInCapacity, shardBatchCapacity))
	if err != nil {
		return nil, fmt.Errorf("connect result fan-in: %w", err)
	}
	definition, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("build showcase graph: %w", err)
	}
	return &showcaseGraph{
		definition:     definition,
		ingress:        ingress,
		broadcastEdges: broadcastEdges,
		shardEdges:     shardEdges,
		fanInEdges:     fanInEdges,
		partitionNodes: [3]graph.NodeID{partitions[0].NodeID(), partitions[1].NodeID(), partitions[2].NodeID()},
	}, nil
}

func Run(output io.Writer, input []rawOrder) (runErr error) {
	if output == nil {
		return errors.New("showcase output writer is nil")
	}
	stats := &showcaseStats{}
	compiled, err := buildShowcase(stats)
	if err != nil {
		return err
	}
	executor, err := compiled.definition.Start()
	if err != nil {
		return fmt.Errorf("start showcase: %w", err)
	}
	stopped := false
	defer func() {
		if !stopped {
			runErr = errors.Join(runErr, executor.Close())
		}
	}()
	ingress, err := graph.BindIngress(executor, compiled.ingress)
	if err != nil {
		return fmt.Errorf("bind showcase ingress: %w", err)
	}
	var valid, invalid []rawOrder
	for _, value := range input {
		if isValidOrder(value) {
			valid = append(valid, value)
		} else {
			invalid = append(invalid, value)
		}
	}
	for offset := 0; offset < len(valid); offset += int(ingressBatchCapacity) {
		end := min(offset+int(ingressBatchCapacity), len(valid))
		if status := ingress.Submit(valid[offset:end]); status != graph.IngressAccepted {
			return fmt.Errorf("submit valid orders: status %d", status)
		}
	}
	for _, value := range invalid {
		if status := ingress.Submit([]rawOrder{value}); status != graph.IngressAccepted {
			return fmt.Errorf("submit invalid order %d: status %d", value.ID, status)
		}
	}
	if err := executor.Drain(); err != nil {
		return fmt.Errorf("drain showcase: %w", err)
	}
	stopped = true
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
	if _, err := fmt.Fprintf(output, "orders submitted=%d processed=%d rejected=%d audited=%d units=%d gross_cents=%d partitions=%d,%d,%d\n",
		len(input), stats.processed.Load(), stats.rejected.Load(), stats.audited.Load(), stats.units.Load(), stats.totalCents.Load(),
		stats.partitions[0].Load(), stats.partitions[1].Load(), stats.partitions[2].Load()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "runtime edges=%d accepted_elements=%d dequeued_elements=%d queue_drop_elements=%d pool_drop_elements=%d logged_errors=%d pools_in_use=%d\n",
		len(metrics.Edges), acceptedElements, dequeuedElements, queueDropped, poolDropped, deliveredLogs, poolsInUse); err != nil {
		return err
	}
	for _, status := range executor.AffinityStatus() {
		if _, err := fmt.Fprintf(output, "affinity group=%d cpu=%d outcome=%d warning=%v\n", status.GroupID, status.CPU, status.Outcome, status.Warning); err != nil {
			return err
		}
	}
	return nil
}

func sampleOrders() []rawOrder {
	return []rawOrder{
		{ID: 1, CustomerID: 10, Units: 2, UnitCents: 125},
		{ID: 2, CustomerID: 11, Units: 1, UnitCents: 700},
		{ID: 3, CustomerID: 12, Units: 3, UnitCents: 200},
		{ID: 4, CustomerID: 10, Units: 1, UnitCents: 50},
		{ID: 5, CustomerID: 13, Units: 0, UnitCents: 100},
	}
}
