package main

import (
	"flag"
	"io"
	"testing"
)

var benchmarkData = flag.String("showcase-data", "", "orders JSON path for load/full benchmarks (default: shared fixture)")

// BenchmarkFullShowcase matches Showcase's load/decode/run/report boundary.
func BenchmarkFullShowcase(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		orders, err := loadOrdersFrom(*benchmarkData)
		if err != nil {
			b.Fatal(err)
		}
		if err := run(io.Discard, orders); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadOrders(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		if _, err := loadOrdersFrom(*benchmarkData); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRun uses the same synthetic valid fixture as internal/showcase's
// BenchmarkRun. Fixture construction and JSON loading are outside timing.
func BenchmarkRun(b *testing.B) {
	orders := make([]order, 1_000_000)
	for i := range orders {
		orders[i] = order{
			ID:         uint64(i + 1),
			CustomerID: uint64(i%100_000 + 1),
			Units:      uint32(i%10 + 1),
			UnitCents:  uint32(i%1_000 + 1),
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := run(io.Discard, orders); err != nil {
			b.Fatal(err)
		}
	}
}
