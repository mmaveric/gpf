package main

import (
	"flag"
	"io"
	"testing"

	"github.com/mmaveric/gpf/examples/showcase/internal/showcase"
)

var benchmarkClock = flag.String("showcase-clock", "standard", "showcase benchmark timing clock: standard or tsc")
var benchmarkData = flag.String("showcase-data", "", "orders JSON path for load/full benchmarks (default: shared fixture)")

func BenchmarkFullShowcase(b *testing.B) {
	config := showcase.Config{Clock: *benchmarkClock}
	if err := config.Validate(); err != nil {
		b.Fatal(err)
	}
	b.Run("clock="+*benchmarkClock, func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			// Loading is timed because the workflow includes reading and decoding the input file.
			orders, err := loadOrdersFrom(*benchmarkData)
			if err != nil {
				b.Fatal(err)
			}
			if err := showcase.RunWithConfig(io.Discard, orders, config); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkLoadOrders(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		if _, err := loadOrdersFrom(*benchmarkData); err != nil {
			b.Fatal(err)
		}
	}
}
