package showcase

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/mmaveric/gpf/graph"
)

func TestRunBoundariesAndMixedBatches(t *testing.T) {
	tests := []struct {
		name  string
		count int
		mixed bool
	}{
		{name: "empty"},
		{name: "one", count: 1},
		{name: "before batch boundary", count: 63, mixed: true},
		{name: "exact batch", count: 64, mixed: true},
		{name: "batch plus remainder", count: 65, mixed: true},
		{name: "multiple mixed batches", count: 257, mixed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]Order, tt.count)
			for i := range input {
				input[i] = Order{ID: uint64(i + 1), CustomerID: uint64(i % 3), Units: 2, UnitCents: 125}
				if tt.mixed && i%5 == 0 {
					switch (i / 5) % 3 {
					case 0:
						input[i].ID = 0
					case 1:
						input[i].Units = 0
					case 2:
						input[i].UnitCents = 0
					}
				}
			}
			assertRunSummary(t, input)
		})
	}
}

func TestRunAllInvalidAndSkewedCustomers(t *testing.T) {
	tests := []struct {
		name  string
		input []Order
	}{
		{
			name: "all invalid",
			input: []Order{
				{CustomerID: 1, Units: 2, UnitCents: 50},
				{ID: 2, CustomerID: 2, UnitCents: 100},
				{ID: 3, CustomerID: 3, Units: 1},
			},
		},
		{name: "single partition", input: make([]Order, 257)},
	}
	for i := range tests[1].input {
		tests[1].input[i] = Order{ID: uint64(i + 1), CustomerID: 0, Units: 1, UnitCents: 75}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assertRunSummary(t, tt.input) })
	}
}

func assertRunSummary(t *testing.T, input []Order) {
	t.Helper()
	var valid, rejected, units, cents uint64
	var partitions [3]uint64
	for _, order := range input {
		if order.ID == 0 || order.Units == 0 || order.UnitCents == 0 {
			rejected++
			continue
		}
		valid++
		units += uint64(order.Units)
		cents += uint64(order.Units) * uint64(order.UnitCents)
		partitions[order.CustomerID%3]++
	}
	want := fmt.Sprintf(
		"orders submitted=%d processed=%d rejected=%d audited=%d units=%d gross_cents=%d partitions=%d,%d,%d\n",
		len(input), valid, rejected, valid, units, cents, partitions[0], partitions[1], partitions[2],
	)
	var output bytes.Buffer
	if err := Run(&output, input); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), want) {
		t.Fatalf("summary want %q, got:\n%s", want, output.String())
	}
	for _, text := range []string{
		"queue_drop_elements=0 pool_drop_elements=0 logged_errors=0 pools_in_use=0",
		"handler_timing_clock=standard measured_calls=",
	} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("report missing %q:\n%s", text, output.String())
		}
	}
}

func TestRunRejectsNilOutput(t *testing.T) {
	if err := Run(nil, nil); err == nil {
		t.Fatal("Run(nil, nil) error = nil")
	}
}

var errReportWrite = errors.New("test report write failure")

type failingReportWriter struct {
	remaining int
}

func (w *failingReportWriter) Write(data []byte) (int, error) {
	if w.remaining == 0 {
		return 0, errReportWrite
	}
	w.remaining--
	return len(data), nil
}

func TestRunReturnsReportWriteErrorsAfterDrain(t *testing.T) {
	for remaining := range 4 {
		t.Run(fmt.Sprint(remaining), func(t *testing.T) {
			writer := &failingReportWriter{remaining: remaining}
			input := []Order{{ID: 1, CustomerID: 1, Units: 1, UnitCents: 1}}
			if err := Run(writer, input); !errors.Is(err, errReportWrite) {
				t.Fatalf("report error = %v", err)
			}
		})
	}
}

func TestRunWithConfigClockSelection(t *testing.T) {
	for _, clock := range []string{"", "standard", "tsc", "unknown"} {
		name := clock
		if name == "" {
			name = "zero value"
		}
		t.Run(name, func(t *testing.T) {
			config := Config{Clock: clock}
			var output bytes.Buffer
			input := []Order{{ID: 1, CustomerID: 1, Units: 1, UnitCents: 50}}
			err := RunWithConfig(&output, input, config)
			if clock == "unknown" {
				if err == nil || config.Validate() == nil {
					t.Fatal("unknown clock was accepted")
				}
				if output.Len() != 0 {
					t.Fatal("invalid config produced a report")
				}
				return
			}
			if clock == "tsc" && errors.Is(err, graph.ErrUnsupportedTimingClock) {
				return // Explicit TSC must fail, not silently fall back, on unsupported CPUs.
			}
			if err != nil {
				t.Fatal(err)
			}
			if clock == "" {
				clock = "standard"
			}
			if !strings.Contains(output.String(), "handler_timing_clock="+clock+" ") {
				t.Fatalf("wrong reported clock:\n%s", output.String())
			}
			line := strings.Split(output.String(), "\n")[2]
			var reportedClock string
			var calls uint64
			_, err = fmt.Sscanf(line, "handler_timing_clock=%s measured_calls=%d", &reportedClock, &calls)
			if err != nil || calls == 0 {
				t.Fatalf("no measured calls in %q: %v", line, err)
			}
		})
	}
	if err := RunWithConfig(io.Discard, nil, Config{Clock: "standard"}); err != nil {
		t.Fatal(err)
	}
}
