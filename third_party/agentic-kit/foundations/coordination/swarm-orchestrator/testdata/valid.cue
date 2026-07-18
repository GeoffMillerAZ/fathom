// Package swarmorchestrator — valid test data that must unify with schema.cue.
package swarmorchestrator

import "github.com/geoffmilleraz/agentic-kit/foundations/coordination/swarm-orchestrator:swarmorchestrator"

// validTask is a well-formed pending task.
validTask: swarmorchestrator.#Task & {
	ID:     "task-001"
	Label:  "Fetch external data"
	Status: "pending"
}

// validTaskAssigned is a task currently being worked on.
validTaskAssigned: swarmorchestrator.#Task & {
	ID:     "task-002"
	Label:  "Process records"
	Status: "assigned"
}

// validTaskDone is a completed task.
validTaskDone: swarmorchestrator.#Task & {
	ID:     "task-003"
	Label:  "Write output"
	Status: "done"
}

// validTaskFailed is a failed task.
validTaskFailed: swarmorchestrator.#Task & {
	ID:     "task-004"
	Label:  "Upload results"
	Status: "failed"
}

// validWave is a well-formed wave with two task IDs.
validWave: swarmorchestrator.#Wave & {
	Index:   0
	TaskIDs: ["task-001", "task-002"]
}

// validPlan is a well-formed two-wave plan.
validPlan: swarmorchestrator.#Plan & {
	MaxConcurrency: 2
	Waves: [
		{Index: 0, TaskIDs: ["task-001", "task-002"]},
		{Index: 1, TaskIDs: ["task-003"]},
	]
}

// validStatuses verifies all status enum values are accepted.
_s1: swarmorchestrator.#TaskStatus & "pending"
_s2: swarmorchestrator.#TaskStatus & "assigned"
_s3: swarmorchestrator.#TaskStatus & "done"
_s4: swarmorchestrator.#TaskStatus & "failed"
