// Package queue defines typed bounded queue contracts.
//
// Future queues use non-blocking TryPush and TryPop operations. Wait retries
// publication with busy-spinning; Drop releases the batch and records a drop.
// No queue implementation exists in this stage.
package queue
