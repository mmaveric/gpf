package main

import (
	"io"
	"testing"

	"github.com/mmaveric/gpf/examples/showcase/internal/showcase"
)

func BenchmarkFullShowcase(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Loading is timed because the primary workflow includes reading and decoding the input file.
		orders, err := loadOrders()
		if err != nil {
			b.Fatal(err)
		}
		if err := showcase.Run(io.Discard, orders); err != nil {
			b.Fatal(err)
		}
	}
}
