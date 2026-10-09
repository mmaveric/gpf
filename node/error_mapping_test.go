package node_test

import (
	"testing"

	"github.com/asharipov/gpf/node"
)

func TestErrorMapperTypedPayload(t *testing.T) {
	integer := node.ErrorMapper[int](func(value int) node.ErrorEvent {
		return node.ErrorEvent{Code: uint32(value), Severity: 2, Attributes: [4]uint64{1, 2, 3, 4}}
	})
	if got := integer.Map(17); got.Code != 17 || got.Severity != 2 || got.Attributes != [4]uint64{1, 2, 3, 4} {
		t.Fatalf("event = %+v", got)
	}
	text := node.ErrorMapper[string](func(value string) node.ErrorEvent { return node.ErrorEvent{Code: uint32(len(value))} })
	if got := text.Map("failure"); got.Code != 7 {
		t.Fatalf("string conversion = %+v", got)
	}
	var empty node.ErrorMapper[struct{ Code uint32 }]
	if got := empty.Map(struct{ Code uint32 }{42}); got != (node.ErrorEvent{}) {
		t.Fatalf("nil mapping = %+v", got)
	}
}

func TestErrorMapperAllocs(t *testing.T) {
	mapper := node.ErrorMapper[int](func(value int) node.ErrorEvent { return node.ErrorEvent{Code: uint32(value)} })
	var event node.ErrorEvent
	if allocations := testing.AllocsPerRun(100, func() { event = mapper.Map(73) }); allocations != 0 {
		t.Fatalf("allocations = %v", allocations)
	}
	if event.Code != 73 {
		t.Fatalf("mapped code = %d", event.Code)
	}
}
