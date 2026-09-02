// Package tasktree defines the CUE contract for task-tree records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package tasktree

// #Status enumerates the valid lifecycle states of a TaskNode.
#Status: "pending" | "running" | "done" | "failed"

// #TaskNode represents a single node in the task decomposition tree.
#TaskNode: {
	// ID is a non-empty unique identifier for this node.
	ID: string & !=""

	// ParentID is the ID of the parent node, or "" for the root node.
	ParentID: string

	// Title is a human-readable description of the task.
	Title: string & !=""

	// Status is the current lifecycle state of the task.
	Status: #Status
}

// #TaskState is the canonical lifecycle vocabulary for a unit of coordinated
// work, shared across foundations/coordination. The sibling enums (#Status
// here, swarm-orchestrator #TaskStatus, implementation-queue #ItemStatus)
// stay as they are; Go's CanonicalState maps their values onto this one.
// Plan-only: a vocabulary, not a state machine — no transitions are defined.
#TaskState: "pending" | "running" | "input-required" | "completed" | "failed" | "cancelled"

// #TerminalState is the class of #TaskState values from which no transition
// follows.
#TerminalState: #TaskState & ("completed" | "failed" | "cancelled")

// #InterruptedState is the class of #TaskState values paused awaiting
// outside input — neither running nor terminal; resumes to "running".
#InterruptedState: #TaskState & "input-required"
