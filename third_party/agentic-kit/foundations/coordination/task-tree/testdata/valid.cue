// Package tasktree — valid test data that must unify with schema.cue.
package tasktree

import "github.com/geoffmilleraz/agentic-kit/foundations/coordination/task-tree:tasktree"

// validRoot is a well-formed root node (ParentID is empty string).
validRoot: tasktree.#TaskNode & {
	ID:       "task-root"
	ParentID: ""
	Title:    "Plan the quarterly report"
	Status:   "pending"
}

// validChild is a well-formed child node referencing the root.
validChild: tasktree.#TaskNode & {
	ID:       "task-child-1"
	ParentID: "task-root"
	Title:    "Gather financial data"
	Status:   "running"
}

// validDone is a completed node.
validDone: tasktree.#TaskNode & {
	ID:       "task-child-2"
	ParentID: "task-root"
	Title:    "Write executive summary"
	Status:   "done"
}

// validFailed is a failed leaf node.
validFailed: tasktree.#TaskNode & {
	ID:       "task-child-3"
	ParentID: "task-child-1"
	Title:    "Fetch external data feed"
	Status:   "failed"
}

// validStatuses verifies all status enum values are accepted.
_s1: tasktree.#Status & "pending"
_s2: tasktree.#Status & "running"
_s3: tasktree.#Status & "done"
_s4: tasktree.#Status & "failed"
