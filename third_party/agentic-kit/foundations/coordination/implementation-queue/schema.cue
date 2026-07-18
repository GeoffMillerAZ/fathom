// Package implementationqueue defines the CUE contract for the implementation queue.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package implementationqueue

// #ItemStatus enumerates the lifecycle states of a queue item.
#ItemStatus: "pending" | "in_progress" | "done" | "failed"

// #Item is a single work item in the queue.
#Item: {
	// Key is the stable dedup identifier for this item.
	Key: string & !=""

	// Payload is the string-encoded work descriptor.
	Payload: string & !=""

	// Status is the current lifecycle state.
	Status: #ItemStatus
}
