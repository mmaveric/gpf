package benchtest

import "testing"

func BenchmarkDataPlane(b *testing.B) {
	cases := []struct {
		name string
	}{
		{name: "node-call/batch=64"},
		{name: "pool-acquire-release/batch=64"},
		{name: "local-edge/batch=64/group=1"},
		{name: "remote-queue/batch=64/groups=2"},
		{name: "end-to-end-dag/batch=64/groups=2/depth=3"},
	}

	for _, benchmarkCase := range cases {
		b.Run(benchmarkCase.name, func(b *testing.B) {
			b.ReportAllocs()
			b.Skip("data-plane path is not implemented")
		})
	}
}
