package main

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mmaveric/gpf/graph"
)

func TestRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		inputs   []string
		numbers  []int
		failures []parseFailure
	}{
		{
			name:     "mixed success and typed failure",
			inputs:   []string{"10", "not-a-number", "30"},
			numbers:  []int{10, 30},
			failures: []parseFailure{{"not-a-number", "not a decimal integer"}},
		},
		{
			name:    "full capacity including zero and negative values",
			inputs:  []string{"0", "-7", "12", "42"},
			numbers: []int{0, -7, 12, 42},
		},
		{
			name:   "all failures reach error sink",
			inputs: []string{"bad", "", "1.5", " 2"},
			failures: []parseFailure{
				{"bad", "not a decimal integer"},
				{"", "not a decimal integer"},
				{"1.5", "not a decimal integer"},
				{" 2", "not a decimal integer"},
			},
		},
		{name: "empty input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// A watchdog, not a completion delay: run drains all accepted work.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := run(ctx, 0, tt.inputs, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.numbers, tt.numbers) {
				t.Errorf("numbers = %v, want %v", got.numbers, tt.numbers)
			}
			if !slices.Equal(got.failures, tt.failures) {
				t.Errorf("failures = %v, want %v", got.failures, tt.failures)
			}
			if len(got.affinity) != 1 || got.affinity[0].CPU != 0 {
				t.Fatalf("affinity = %+v, want one explicit CPU request", got.affinity)
			}
			// Either affinity outcome is valid in containers and on non-Linux hosts.
			switch status := got.affinity[0]; status.Outcome {
			case graph.AffinityOutcomeApplied:
				if status.Warning != nil {
					t.Errorf("applied affinity has warning: %v", status.Warning)
				}
			case graph.AffinityOutcomeLockedThreadFallback:
				if !errors.Is(status.Warning, graph.ErrAffinityUnavailable) {
					t.Errorf("fallback warning = %v, want ErrAffinityUnavailable", status.Warning)
				}
			default:
				t.Errorf("unexpected affinity outcome: %+v", status)
			}
			if len(got.metrics.Pools) == 0 {
				t.Fatal("no pool metrics returned")
			}
			for _, pool := range got.metrics.Pools {
				if pool.InUseSample != 0 {
					t.Errorf("pool still owns batches after drain: %+v", pool)
				}
			}
			if len(got.metrics.Ingresses) != 1 || got.metrics.Ingresses[0].AcceptedBatches != uint64(len(tt.inputs)) {
				t.Errorf("ingress metrics = %+v", got.metrics.Ingresses)
			}
		})
	}
}

func TestRunWritesDOT(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var dot bytes.Buffer
	if _, err := run(ctx, 0, nil, &dot); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"digraph G {",
		`n1 [label="ingress #1"]`,
		`n1 -> n2 [label="data: out1 → in1"]`,
		`n2 -> n3 [label="data: out1 → in1"]`,
		`n2 -> n4 [label="error: error → in1"]`,
	} {
		if !strings.Contains(dot.String(), want) {
			t.Errorf("DOT missing %q:\n%s", want, dot.String())
		}
	}
}

func TestRunRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	t.Run("too many inputs", func(t *testing.T) {
		if _, err := run(context.Background(), 0, make([]string, capacity+1), nil); err == nil {
			t.Fatal("expected bounded input error")
		}
	})
	t.Run("negative CPU", func(t *testing.T) {
		if _, err := run(context.Background(), -1, nil, nil); !errors.Is(err, graph.ErrInvalidCPU) {
			t.Fatalf("error = %v, want ErrInvalidCPU", err)
		}
	})
	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := run(ctx, 0, []string{"10"}, nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})
	t.Run("DOT writer failure", func(t *testing.T) {
		want := errors.New("writer failed")
		if _, err := run(context.Background(), 0, nil, failingWriter{want}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want writer failure", err)
		}
	})
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }
