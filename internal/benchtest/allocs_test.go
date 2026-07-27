package benchtest

import "testing"

func TestAssertZeroAllocs(t *testing.T) {
	t.Parallel()

	AssertZeroAllocs(t, 100, func() {})
}
