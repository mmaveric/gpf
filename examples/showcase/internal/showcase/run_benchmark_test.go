package showcase

import (
	"io"
	"testing"
)

func BenchmarkRun(b *testing.B) {
	input := make([]Order, 1_000_000)
	for i := range input {
		input[i] = Order{
			ID:         uint64(i + 1),
			CustomerID: uint64(i%100_000 + 1),
			Units:      uint32(i%10 + 1),
			UnitCents:  uint32(i%1_000 + 1),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := Run(io.Discard, input); err != nil {
			b.Fatal(err)
		}
	}
}
