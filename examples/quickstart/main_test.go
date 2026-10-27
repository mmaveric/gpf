package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestExecute(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	if err := execute(0, "", &stdout); err != nil {
		t.Fatal(err)
	}
	const want = "numbers: [10 30]\nrejected \"not-a-number\": not a decimal integer\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestExecuteOutputFailure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		failAt  int
		context string
	}{
		{name: "numbers", failAt: 1, context: "write numbers: "},
		{name: "rejection", failAt: 2, context: "write rejection: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want := errors.New("output failed")
			stdout := &failOnWrite{failAt: tt.failAt, err: want}
			err := execute(0, "", stdout)
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want wrapped output failure", err)
			}
			if !strings.HasPrefix(err.Error(), tt.context) {
				t.Errorf("error = %q, want context %q", err, tt.context)
			}
			if stdout.calls != tt.failAt {
				t.Errorf("write calls = %d, want %d", stdout.calls, tt.failAt)
			}
		})
	}
}

type failOnWrite struct {
	failAt int
	calls  int
	err    error
}

func (w *failOnWrite) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		return 0, w.err
	}
	return len(p), nil
}
