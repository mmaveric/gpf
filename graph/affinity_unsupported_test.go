//go:build !linux || unsupported

package graph

import (
	"errors"
	"testing"
)

func TestUnsupportedAffinityAdapterSelected(t *testing.T) {
	if _, ok := newPlatformAffinity().(unsupportedAffinity); !ok {
		t.Fatalf("platform adapter = %T, want unsupportedAffinity", newPlatformAffinity())
	}
	if err := newPlatformAffinity().bind(0); !errors.Is(err, ErrAffinityUnavailable) {
		t.Fatalf("unsupported bind error = %v", err)
	}
}
