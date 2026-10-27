package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
			if (err != nil) != tt.wantErr {
				t.Fatalf("load error = %v", err)
			}
			if len(orders) != tt.wantLen {
				t.Fatalf("orders = %d, want %d", len(orders), tt.wantLen)
			}
		})
	}
	for _, path := range []string{filepath.Join(dir, "missing.json"), dir} {
		if _, err := loadOrdersFrom(path); err == nil {
			t.Fatalf("read %s succeeded", path)
		}
	}
}

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
	for _, want := range []string{
		"orders submitted=2 processed=1 rejected=1 audited=1 units=2 gross_cents=100 partitions=0,1,0",
		"handler_timing_clock=standard ",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q:\n%s", want, output.String())
		}
	}
	if err := runCommand([]string{"-h"}, io.Discard, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v", err)
	}
	for _, args := range [][]string{
		{"-clock", "unknown", "-data", path},
		{"-data", path, "unexpected"},
		{"-unknown"},
		{"-data", path + ".missing"},
	} {
		if err := runCommand(args, io.Discard, io.Discard); err == nil {
			t.Fatalf("arguments %v accepted", args)
		}
	}
}
