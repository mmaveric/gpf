package main

import (
	"io"
	"testing"

	"github.com/asharipov/gpf/examples/showcase/internal/showcase"
)

func BenchmarkFullShowcase(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orders, err := loadOrders()
		if err != nil {
			b.Fatal(err)
		}
		if err := showcase.Run(io.Discard, orders); err != nil {
			b.Fatal(err)
		}
	}
}
