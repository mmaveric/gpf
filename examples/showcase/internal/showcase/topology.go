package showcase

import (
	"errors"
	"fmt"

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
	ingressCapacity        uint32 = 256
	ingressBatchCapacity   uint32 = 64
	broadcastCapacity      uint32 = 256
	shardCapacity          uint32 = 12288
	shardBatchCapacity     uint32 = 32
	fanInCapacity          uint32 = 4096
	groupMetricsFlushEvery uint64 = 2048
	loggerCapacity         uint32 = 4
)

func edgeConfig[T any](capacity, batchCapacity uint32) graph.EdgeConfig[T] {
	return graph.EdgeConfig[T]{Capacity: capacity, BatchCapacity: batchCapacity, Policy: queue.Wait}
}

func isValidOrder(value Order) bool {
	return value.ID != 0 && value.Units != 0 && value.UnitCents != 0
}

func buildShowcase(stats *showcaseStats, config Config) (*showcaseGraph, error) {
	clock, err := config.clockSource()
	if err != nil {
		return nil, err
	}
	builder := graph.NewBuilder()
	if err := graph.SetTiming(builder, graph.TimingConfig{Mode: graph.TimingFull, ClockSource: clock}); err != nil {
		return nil, fmt.Errorf("configure %s handler timing: %w", config.Clock, err)
	}
	if err := graph.SetGroupMetricsFlushEvery(builder, groupMetricsFlushEvery); err != nil {
		return nil, fmt.Errorf("configure group metrics flush interval: %w", err)
	}
	ingress, err := graph.AddIngress[Order](builder, graph.IngressConfig{Capacity: ingressCapacity, BatchCapacity: ingressBatchCapacity, Policy: queue.Wait})
	if err != nil {
		return nil, fmt.Errorf("register ingress: %w", err)
	}
	validator, err := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, Order, node.Outputs2[order, validationError], error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, Order, node.Outputs2[order, validationError], error]{
			Handle1: func(_ *struct{}, input batch.Batch[Order], outputs *node.Outputs2[order, validationError]) node.Result[error] {
				valid, validOK := outputs.Out1.Acquire()
				invalid, invalidOK := outputs.Out2.Acquire()
				if !validOK || !invalidOK {
					valid.Release()
					invalid.Release()
					return node.Result[error]{Failed: true, Value: errors.New("showcase validation pool exhausted")}
				}
				var validLen, invalidLen int
				for _, value := range input.Items() {
					if isValidOrder(value) {
						valid.Mutable()[validLen] = order{ID: value.ID, CustomerID: value.CustomerID, Units: value.Units, UnitCents: value.UnitCents}
						validLen++
					} else {
						invalid.Mutable()[invalidLen] = validationError{OrderID: value.ID, Code: 1}
						invalidLen++
					}
				}
				valid.SetLen(validLen)
				invalid.SetLen(invalidLen)
				if validLen > 0 && !outputs.Out1.Commit(&valid) {
					valid.Release()
					invalid.Release()
					return node.Result[error]{Failed: true, Value: errors.New("showcase valid-order commit failed")}
				}
				if invalidLen > 0 && !outputs.Out2.Commit(&invalid) {
					invalid.Release()
					return node.Result[error]{Failed: true, Value: errors.New("showcase invalid-order commit failed")}
				}
				return node.Result[error]{}
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
					result.Mutable()[itemIndex] = receipt{OrderID: value.ID, CustomerID: value.CustomerID, Partition: partition, Units: value.Units, TotalCents: uint64(value.Units) * uint64(value.UnitCents)}
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
		{ingress.NodeID(), groupIngress}, {validator.NodeID(), groupIngress}, {shardRelay.NodeID(), groupIngress},
		{audit.NodeID(), groupAudit}, {partitions[0].NodeID(), groupPartition0}, {partitions[1].NodeID(), groupPartition1},
		{partitions[2].NodeID(), groupPartition2}, {aggregate.NodeID(), groupAggregate}, {errorSink.NodeID(), groupErrors},
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
	if _, err := graph.Connect(builder, ingress.Output(), validator.Inputs().In1, edgeConfig[Order](ingressCapacity, ingressBatchCapacity)); err != nil {
		return nil, fmt.Errorf("connect ingress: %w", err)
	}
	if _, err := graph.Connect(builder, graph.OutputPorts2(validator.Outputs()).Out2, errorSink.Inputs().In1, edgeConfig[validationError](ingressCapacity, ingressBatchCapacity)); err != nil {
		return nil, fmt.Errorf("connect validation errors: %w", err)
	}
	broadcastEdges, err := graph.ConnectBroadcast(builder, graph.OutputPorts2(validator.Outputs()).Out1, []graph.InPort[order]{audit.Inputs().In1, shardRelay.Inputs().In1}, edgeConfig[order](broadcastCapacity, ingressBatchCapacity))
	if err != nil {
		return nil, fmt.Errorf("connect audit broadcast: %w", err)
	}
	shardInputs := []graph.InPort[order]{partitions[0].Inputs().In1, partitions[1].Inputs().In1, partitions[2].Inputs().In1}
	shardEdges, err := graph.ConnectShard(builder, graph.OutputPorts1(shardRelay.Outputs()).Out1, shardInputs, func(value order) uint32 { return uint32(value.CustomerID % uint64(len(partitions))) }, edgeConfig[order](shardCapacity, shardBatchCapacity))
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
	return &showcaseGraph{definition: definition, ingress: ingress, broadcastEdges: broadcastEdges, shardEdges: shardEdges, fanInEdges: fanInEdges,
		partitionNodes: [3]graph.NodeID{partitions[0].NodeID(), partitions[1].NodeID(), partitions[2].NodeID()}}, nil
}
