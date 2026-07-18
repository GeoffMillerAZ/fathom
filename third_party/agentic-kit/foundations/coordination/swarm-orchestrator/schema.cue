// Package swarmorchestrator defines the CUE contract for swarm orchestration records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package swarmorchestrator

// #TaskStatus enumerates the lifecycle states of a swarm task.
#TaskStatus: "pending" | "assigned" | "done" | "failed"

// #Task is a unit of work managed by the Orchestrator.
#Task: {
	// ID is a stable, non-empty identifier for this task.
	ID: string & !=""

	// Label is a short human-readable description of the task.
	Label: string & !=""

	// Status is the current lifecycle state of the task.
	Status: #TaskStatus
}

// #Wave is an ordered batch of task IDs scheduled for concurrent execution.
#Wave: {
	// Index is the zero-based wave number (determines execution order).
	Index: >=0 & int

	// TaskIDs is the set of task IDs scheduled in this wave.
	TaskIDs: [...string]
}

// #Plan is the deterministic execution plan produced by Plan().
#Plan: {
	// Waves is the ordered sequence of execution waves.
	Waves: [...#Wave]

	// MaxConcurrency is the bound used when the plan was created.
	MaxConcurrency: >=1 & int
}
