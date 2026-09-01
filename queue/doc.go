// Package queue defines typed bounded queue contracts.
//
// Local ordinary and error edges use non-blocking TryPush and TryPop operations on one group
// worker. Wait retains a full-edge batch for a later pass so downstream work can
// progress; Drop releases it. A Factory constructs fresh operations only for
// each started local edge and receives Capacity, BatchCapacity, and Policy.
// Cross-group ordinary and error edges use graph's built-in typed SPSC ring; Build rejects a
// custom factory for them without invocation. Callers own factory captures and
// custom queue resources. Error mapping, retries, and observability remain
// unavailable alongside richer queue models and concurrent starts.
package queue
