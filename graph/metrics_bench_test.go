package graph

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkGroupMetricsPass(b *testing.B) {
	for _, every := range []uint64{1, 8, 32, 64, 256} {
		b.Run(fmt.Sprintf("flush-every-%d", every), func(b *testing.B) {
			worker := &executorWorker{metrics: &groupCounters{flushEvery: every}}
			var sources atomic.Bool

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				worker.pass(&sources)
			}
		})
	}
}
