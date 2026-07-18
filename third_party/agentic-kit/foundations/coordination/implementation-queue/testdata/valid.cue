// Package implementationqueue — valid test data that must unify with schema.cue.
package implementationqueue

import "github.com/geoffmilleraz/agentic-kit/foundations/coordination/implementation-queue:implementationqueue"

// validPendingItem is a well-formed pending queue item.
validPendingItem: implementationqueue.#Item & {
	Key:     "impl-001"
	Payload: "refactor payment handler"
	Status:  "pending"
}

// validInProgressItem is an item currently being processed.
validInProgressItem: implementationqueue.#Item & {
	Key:     "impl-002"
	Payload: "add validation layer"
	Status:  "in_progress"
}

// validDoneItem is a completed item.
validDoneItem: implementationqueue.#Item & {
	Key:     "impl-003"
	Payload: "write unit tests"
	Status:  "done"
}

// validFailedItem is a failed item.
validFailedItem: implementationqueue.#Item & {
	Key:     "impl-004"
	Payload: "deploy to staging"
	Status:  "failed"
}

// validStatuses verifies all status enum values are accepted.
_s1: implementationqueue.#ItemStatus & "pending"
_s2: implementationqueue.#ItemStatus & "in_progress"
_s3: implementationqueue.#ItemStatus & "done"
_s4: implementationqueue.#ItemStatus & "failed"
