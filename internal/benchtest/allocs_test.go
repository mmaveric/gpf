package benchtest

import "testing"

func TestAssertZeroAllocs(t *testing.T) {
	AssertZeroAllocs(t, 100, func() {})
}
