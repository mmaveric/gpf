package graph

import (
	"testing"

	"github.com/mmaveric/gpf/node"
)

func BenchmarkRuntimeLoggerPass(b *testing.B) {
	groups := make([]groupPlan, 7)
	for i := range groups {
		groups[i].id = GroupID(i + 1)
	}
	logger := newRuntimeLogger(groups, LoggerConfig{
		Capacity: 1,
		Handle:   func(node.ErrorEvent) {},
	}, &metricStore{})

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		logger.pass()
	}
}
