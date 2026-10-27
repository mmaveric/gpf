package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orders.json")
	data := `[{"id":1,"customerId":1,"units":2,"unitCents":50},{"id":2,"units":0,"unitCents":50}]`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runCommand([]string{"-data", path}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	want := "orders submitted=2 processed=1 rejected=1 audited=1 units=2 gross_cents=100 partitions=0,1,0"
	if !strings.Contains(output.String(), want) {
		t.Fatalf("missing %q:\n%s", want, output.String())
	}
	if err := runCommand([]string{"-h"}, io.Discard, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v", err)
	}
	for _, args := range [][]string{
		{"-data", path, "unexpected"},
		{"-unknown"},
		{"-data", path + ".missing"},
	} {
		if err := runCommand(args, io.Discard, io.Discard); err == nil {
			t.Fatalf("arguments %v accepted", args)
		}
	}
}

func TestLoadOrdersFrom(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		data    string
		wantLen int
		wantErr bool
	}{
		{name: "valid", data: `[{"id":1,"customerId":0,"units":2,"unitCents":50}]`, wantLen: 1},
		{name: "empty", data: `[]`},
		{name: "malformed", data: `[`, wantErr: true},
		{name: "wrong shape", data: `{}`, wantErr: true},
		{name: "numeric overflow", data: `[{"units":4294967296}]`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name+".json")
			if err := os.WriteFile(path, []byte(tt.data), 0o600); err != nil {
				t.Fatal(err)
			}
			orders, err := loadOrdersFrom(path)
			if (err != nil) != tt.wantErr || len(orders) != tt.wantLen {
				t.Fatalf("loaded %d orders, error %v", len(orders), err)
			}
		})
	}
	for _, path := range []string{filepath.Join(dir, "missing.json"), dir} {
		if _, err := loadOrdersFrom(path); err == nil {
			t.Fatalf("read %s succeeded", path)
		}
	}
}

func TestRun(t *testing.T) {
	orders := []order{
		{ID: 1, CustomerID: 10, Units: 2, UnitCents: 125},
		{ID: 2, CustomerID: 11, Units: 1, UnitCents: 700},
		{ID: 3, CustomerID: 12, Units: 3, UnitCents: 200},
		{ID: 4, CustomerID: 10, Units: 1, UnitCents: 50},
		{ID: 5, CustomerID: 13, Units: 0, UnitCents: 100},
	}
	var output bytes.Buffer
	if err := run(&output, orders); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"orders submitted=5 processed=4 rejected=1 audited=4 units=7 gross_cents=1600 partitions=1,2,1",
		"runtime mode=standalone framework=none",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("output missing %q:\n%s", expected, output.String())
		}
	}
}

func TestRunRejectsNilOutput(t *testing.T) {
	if err := run(nil, nil); err == nil {
		t.Fatal("run(nil, nil) error = nil, want error")
	}
}

func TestRunRoutesInvalidOrdersAndKeepsValidOrders(t *testing.T) {
	orders := []order{
		{ID: 1, CustomerID: 1, Units: 2, UnitCents: 50},
		{ID: 2, CustomerID: 2, Units: 0, UnitCents: 100},
		{ID: 3, CustomerID: 2, Units: 1, UnitCents: 75},
	}
	var output bytes.Buffer
	if err := run(&output, orders); err != nil {
		t.Fatal(err)
	}
	want := "orders submitted=3 processed=2 rejected=1 audited=2 units=3 gross_cents=175 partitions=0,1,1"
	if !strings.Contains(output.String(), want) {
		t.Fatalf("output missing %q:\n%s", want, output.String())
	}
}

func TestRunBoundariesAndMixedInput(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		invalid bool
		mixed   bool
	}{
		{name: "empty"},
		{name: "one", count: 1},
		{name: "all invalid", count: 65, invalid: true},
		{name: "before showcase batch boundary", count: 63, mixed: true},
		{name: "exact showcase batch", count: 64, mixed: true},
		{name: "showcase batch plus remainder", count: 65, mixed: true},
		{name: "before channel boundary", count: 255, mixed: true},
		{name: "exact channel capacity", count: 256, mixed: true},
		{name: "channel plus remainder", count: 257, mixed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders := make([]order, tt.count)
			var processed, rejected, units, cents uint64
			var partitions [3]uint64
			for i := range orders {
				value := order{ID: uint64(i + 1), CustomerID: uint64(i % 3), Units: 2, UnitCents: 125}
				if tt.invalid || (tt.mixed && i%5 == 0) {
					switch (i / 5) % 3 {
					case 0:
						value.ID = 0
					case 1:
						value.Units = 0
					case 2:
						value.UnitCents = 0
					}
					rejected++
				} else {
					processed++
					units += 2
					cents += 250
					partitions[value.CustomerID%3]++
				}
				orders[i] = value
			}
			var output bytes.Buffer
			if err := run(&output, orders); err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf(
				"orders submitted=%d processed=%d rejected=%d audited=%d units=%d gross_cents=%d partitions=%d,%d,%d\n",
				len(orders), processed, rejected, processed, units, cents, partitions[0], partitions[1], partitions[2],
			)
			if !strings.HasPrefix(output.String(), want) {
				t.Fatalf("summary want %q, got:\n%s", want, output.String())
			}
		})
	}
}

var errReportWrite = errors.New("test report write failure")

type failingWriter struct {
	remaining int
}

func (w *failingWriter) Write(data []byte) (int, error) {
	if w.remaining == 0 {
		return 0, errReportWrite
	}
	w.remaining--
	return len(data), nil
}

func TestRunReturnsReportWriteErrors(t *testing.T) {
	for remaining := range 2 {
		t.Run(fmt.Sprint(remaining), func(t *testing.T) {
			writer := &failingWriter{remaining: remaining}
			orders := []order{{ID: 1, CustomerID: 0, Units: 1, UnitCents: 1}}
			if err := run(writer, orders); !errors.Is(err, errReportWrite) {
				t.Fatalf("report error = %v", err)
			}
		})
	}
}
