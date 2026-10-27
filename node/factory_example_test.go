package node_test

import (
	"fmt"

	"github.com/mmaveric/gpf/node"
)

func ExampleStateful() {
	declaration := node.Stateful[int, node.Node0[int, node.Outputs0, error]]{
		Factory: func() (int, error) { return 1, nil },
	}

	first, _ := declaration.Factory()
	second, _ := declaration.Factory()
	fmt.Println(first, second)
	// Output: 1 1
}

func ExampleClosureFactory() {
	factory := func() (node.ClosureReplica[node.ClosureNode0[node.Outputs0, error]], error) {
		count := 0
		return node.ClosureReplica[node.ClosureNode0[node.Outputs0, error]]{
			Handlers: node.ClosureNode0[node.Outputs0, error]{
				Handle: func(*node.Outputs0) node.Result[error] {
					count++
					return node.Result[error]{
						Value: fmt.Errorf("calls: %d", count),
					}
				},
			},
		}, nil
	}

	first, _ := factory()
	second, _ := factory()
	first.Handlers.Handle(&node.Outputs0{})
	result := second.Handlers.Handle(&node.Outputs0{})
	fmt.Println(result.Value)
	// Output: calls: 1
}
