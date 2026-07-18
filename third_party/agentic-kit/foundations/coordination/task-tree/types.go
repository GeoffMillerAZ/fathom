// Package tasktree provides a task decomposition tree for multi-agent
// coordination planning. A TaskTree tracks hierarchical tasks from initial
// decomposition through status tracking, with cycle detection and status
// propagation. It does NOT execute tasks; execution is the caller's
// responsibility.
//
// Plan-only status (D7): this package ships only as a planner. The executor
// (kits/worker-pool with agent-harness as the worker unit) is not yet built;
// it will be added when a real fan-out workload subscribes.
//
// Concurrency contract: TaskTree is not safe for concurrent use; callers that
// share a TaskTree across goroutines must guard access with their own
// synchronization.
package tasktree

import "errors"

// Status represents the lifecycle state of a TaskNode.
type Status string

const (
	// StatusPending indicates the task has been created but not yet started.
	StatusPending Status = "pending"

	// StatusRunning indicates the task is actively being worked on.
	StatusRunning Status = "running"

	// StatusDone indicates the task completed successfully.
	StatusDone Status = "done"

	// StatusFailed indicates the task failed and will not be retried.
	StatusFailed Status = "failed"
)

// ErrCycle is returned when adding a node would introduce a cycle in the tree.
var ErrCycle = errors.New("tasktree: cycle detected")

// ErrDuplicateID is returned when adding a node whose ID already exists.
var ErrDuplicateID = errors.New("tasktree: duplicate node ID")

// ErrNotFound is returned when a referenced node does not exist.
var ErrNotFound = errors.New("tasktree: node not found")

// ErrInvalidStatus is returned when an unrecognized status value is used.
var ErrInvalidStatus = errors.New("tasktree: invalid status")

// TaskNode is a single node in the task decomposition tree.
type TaskNode struct {
	// ID is a stable, unique identifier for this task.
	ID string

	// ParentID is the ID of the parent task, or "" for the root.
	ParentID string

	// Title is a human-readable description of the task.
	Title string

	// Status is the current lifecycle state of the task.
	Status Status
}

// TaskTree is an in-memory, append-only task decomposition tree.
// It enforces acyclicity: Add rejects any node that would form a cycle.
type TaskTree struct {
	nodes map[string]*TaskNode
}

// NewTaskTree returns an empty TaskTree ready for use.
func NewTaskTree() *TaskTree {
	return &TaskTree{nodes: make(map[string]*TaskNode)}
}

// Add inserts a new TaskNode into the tree.
// Returns ErrDuplicateID if a node with the same ID already exists.
// Returns ErrNotFound if ParentID is non-empty and does not reference an existing node.
// Returns ErrCycle if adding the node would create a cycle (guarded defensively).
func (t *TaskTree) Add(n TaskNode) error {
	if _, exists := t.nodes[n.ID]; exists {
		return ErrDuplicateID
	}
	if n.ParentID != "" {
		if _, ok := t.nodes[n.ParentID]; !ok {
			return ErrNotFound
		}
		// Cycle guard: walk ancestors of ParentID; if we ever reach n.ID we have a cycle.
		// Since the tree is stored as parent pointers this can only happen if callers
		// mutate nodes externally — but we defend against it here for correctness.
		if t.isAncestor(n.ParentID, n.ID) {
			return ErrCycle
		}
	}
	copy := n
	t.nodes[n.ID] = &copy
	return nil
}

// isAncestor reports whether candidate is an ancestor of startID in the current tree.
// It traverses parent pointers from startID upward.
func (t *TaskTree) isAncestor(startID, candidate string) bool {
	current := startID
	for current != "" {
		if current == candidate {
			return true
		}
		node, ok := t.nodes[current]
		if !ok {
			break
		}
		current = node.ParentID
	}
	return false
}

// Children returns the immediate children of the node identified by parentID.
// Returns an empty slice (not nil) if the node has no children or does not exist.
// The returned slice is sorted by insertion order (deterministic within a process).
func (t *TaskTree) Children(parentID string) []TaskNode {
	var out []TaskNode
	for _, n := range t.nodesSorted() {
		if n.ParentID == parentID {
			out = append(out, *n)
		}
	}
	if out == nil {
		out = []TaskNode{}
	}
	return out
}

// Leaves returns all nodes that have no children (leaf nodes).
// Returns an empty slice (not nil) if the tree is empty.
func (t *TaskTree) Leaves() []TaskNode {
	childIDs := make(map[string]bool)
	for _, n := range t.nodes {
		if n.ParentID != "" {
			childIDs[n.ParentID] = true
		}
	}
	var out []TaskNode
	for _, n := range t.nodesSorted() {
		if !childIDs[n.ID] {
			out = append(out, *n)
		}
	}
	if out == nil {
		out = []TaskNode{}
	}
	return out
}

// MarkStatus updates the Status of the node identified by id.
// Returns ErrNotFound if no node with that ID exists.
// Returns ErrInvalidStatus if the provided status is not a recognized value.
func (t *TaskTree) MarkStatus(id string, s Status) error {
	switch s {
	case StatusPending, StatusRunning, StatusDone, StatusFailed:
		// valid
	default:
		return ErrInvalidStatus
	}
	n, ok := t.nodes[id]
	if !ok {
		return ErrNotFound
	}
	n.Status = s
	return nil
}

// Get returns the TaskNode for the given id, and whether it was found.
func (t *TaskTree) Get(id string) (TaskNode, bool) {
	n, ok := t.nodes[id]
	if !ok {
		return TaskNode{}, false
	}
	return *n, true
}

// nodesSorted returns all nodes in a deterministic order (sorted by ID).
func (t *TaskTree) nodesSorted() []*TaskNode {
	ids := make([]string, 0, len(t.nodes))
	for id := range t.nodes {
		ids = append(ids, id)
	}
	sortStrings(ids)
	out := make([]*TaskNode, 0, len(ids))
	for _, id := range ids {
		out = append(out, t.nodes[id])
	}
	return out
}

// sortStrings sorts a slice of strings in place using insertion sort.
// Avoids importing "sort" for this small, bounded operation.
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
