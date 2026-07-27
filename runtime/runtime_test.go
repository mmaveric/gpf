package runtime_test

import (
	"testing"
	"time"

	"github.com/asharipov/gpf/runtime"
)

func TestConfigDeclaration(t *testing.T) {
	t.Parallel()

	config := runtime.Config{
		Groups: []runtime.GroupConfig{
			{
				ID:       1,
				CPU:      2,
				NUMA:     0,
				Replicas: 1,
			},
		},
		DrainTimeout: time.Second,
	}
	if len(config.Groups) != 1 || config.DrainTimeout != time.Second {
		t.Fatalf("Config declaration was not preserved: %+v", config)
	}
}
