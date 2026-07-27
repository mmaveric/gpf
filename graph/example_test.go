package graph_test

import "github.com/asharipov/gpf/graph"

func ExampleConnect_heterogeneousPorts() {
	var definition graph.Definition

	var intOutput graph.OutPort[int]
	var intInput graph.InPort[int]
	_ = graph.Connect(&definition, intOutput, intInput, graph.EdgeConfig{})

	var stringOutput graph.OutPort[string]
	var stringInput graph.InPort[string]
	_ = graph.Connect(&definition, stringOutput, stringInput, graph.EdgeConfig{})
}
