package showcase

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPreservesMillionDeterministicOrdersAcrossRepeatedRuns(t *testing.T) {
	input := make([]Order, 1_000_000)
	for i := range input {
		input[i] = Order{
			ID:         uint64(i + 1),
			CustomerID: uint64(i%100_000 + 1),
			Units:      uint32(i%10 + 1),
			UnitCents:  uint32(i%1_000 + 1),
		}
	}
	wants := []string{
		"orders submitted=1000000 processed=1000000 rejected=0 audited=1000000 units=5500000 gross_cents=2761000000 partitions=333330,333340,333330",
		"queue_drop_elements=0 pool_drop_elements=0 logged_errors=0",
	}
	for run := range 10 {
		var output bytes.Buffer
		if err := Run(&output, input); err != nil {
			t.Fatalf("run %d: %v", run+1, err)
		}
		for _, want := range wants {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("run %d missing %q:\n%s", run+1, want, output.String())
			}
		}
	}
}
