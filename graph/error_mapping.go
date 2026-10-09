package graph

import "github.com/asharipov/gpf/node"

// The typed mapper is captured once during normalization. Sink resolution is a
// control-plane operation; worker failures perform no group lookup or boxing.
func bindErrorMapper[E any](target *normalizationTarget, id NodeID, mapper node.ErrorMapper[E]) func(E) {
	if target.eventSink == nil {
		return nil
	}
	emit := target.eventSink(target.currentGroup)
	if emit == nil {
		return nil
	}
	group := target.currentGroup
	replica := target.currentReplica
	if target.metrics != nil && int(id) <= len(target.metrics.nodes) {
		identity := target.metrics.nodes[id-1].identity
		id, group = identity.NodeID, identity.GroupID
	}
	return func(value E) {
		event := mapper.Map(value)
		event.NodeID, event.GroupID = uint32(id), uint32(group)
		event.Replica, event.Worker = replica, 0
		emit(event)
	}
}
