package graph_test

import (
	"fmt"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/graph"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func ExampleConnect_heterogeneousPorts() {
	builder := graph.NewBuilder()
	intSource, _ := graph.AddNode0(builder, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs1[int], error]]{})
	intTarget, _ := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{})
	_, _ = graph.Connect(builder, graph.OutputPorts1(intSource.Outputs()).Out1, intTarget.Inputs().In1, graph.EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop})

	stringSource, _ := graph.AddNode0(builder, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs1[string], error]]{})
	stringTarget, _ := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, string, node.Outputs0, error]]{})
	_, _ = graph.Connect(builder, graph.OutputPorts1(stringSource.Outputs()).Out1, stringTarget.Inputs().In1, graph.EdgeConfig[string]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait})
}

func ExampleBuilder_Build() {
	builder := graph.NewBuilder()
	source, _ := graph.AddNode0(builder, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs1[int], error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs1[int], error]{
			Handle: func(*struct{}, *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} },
		},
	})
	sink, _ := graph.AddNode1(builder, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{
			Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} },
		},
	})
	_, _ = graph.Connect(builder, graph.OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, graph.EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop})
	_ = graph.BindCPU(builder, graph.DefaultGroupID, 0)

	definition, err := builder.Build()
	fmt.Println(err, definition.NodeCount(), definition.EdgeCount(), definition.TopologicalOrder())
	// Output:
	// <nil> 2 1 [1 2]
}
