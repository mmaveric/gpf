package graph_test

import (
	"errors"
	"testing"

	"github.com/asharipov/gpf/graph"
)

func TestConnect_ReturnsUnavailableUntilGraphBuildExists(t *testing.T) {
	t.Parallel()

	var definition graph.Definition
	err := graph.Connect(
		&definition,
		graph.OutPort[int]{},
		graph.InPort[int]{},
		graph.EdgeConfig{},
	)
	if !errors.Is(err, graph.ErrUnavailable) {
		t.Fatalf("Connect() error = %v, want ErrUnavailable", err)
	}
}
