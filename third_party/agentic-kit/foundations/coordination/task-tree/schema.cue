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
