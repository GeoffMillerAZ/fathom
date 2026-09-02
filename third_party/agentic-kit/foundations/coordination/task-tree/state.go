package tasktree

import (
	"errors"
	"fmt"
)

// TaskState is the canonical lifecycle vocabulary for a unit of coordinated
// work, shared across foundations/coordination: pending, running,
// input-required, completed, failed, cancelled. It is a CUE #TaskState in
// schema.cue. The three sibling enums that predate it — this package's
// Status, swarm-orchestrator's TaskStatus and implementation-queue's
// ItemStatus — stay as they are; CanonicalState maps any of their values
// onto a TaskState so a planner can read all three in one vocabulary.
// Plan-only (I20): this is a mapping, not a state machine — nothing here
// transitions, spawns or executes anything.
type TaskState string

const (
	// StatePending is a task created but not yet started.
	StatePending TaskState = "pending"

	// StateRunning is a task actively being worked on (also "assigned" and
	// "in_progress" in the sibling enums).
	StateRunning TaskState = "running"

	// StateInputRequired is a task paused until something outside it
	// answers — a human, a credential, a missing input. Interrupted, not
	// terminal: it resumes to StateRunning.
	StateInputRequired TaskState = "input-required"

	// StateCompleted is a task that finished successfully ("done" in the
	// sibling enums).
	StateCompleted TaskState = "completed"

	// StateFailed is a task that ended in error and will not be retried.
	StateFailed TaskState = "failed"

	// StateCancelled is a task stopped on request before it finished.
	StateCancelled TaskState = "cancelled"
)

// ErrUnknownState is returned by CanonicalState for a string no
// coordination enum defines. Unknown never maps to a state: a planner that
// guessed would report a task as pending or done that is neither.
var ErrUnknownState = errors.New("tasktree: unknown task state")

// Terminal reports whether s is a final state — completed, failed or
// cancelled — from which no transition follows (CUE #TerminalState).
func (s TaskState) Terminal() bool {
	switch s {
	case StateCompleted, StateFailed, StateCancelled:
		return true
	}
	return false
}

// Interrupted reports whether s is paused awaiting outside input —
// input-required — which is neither running nor terminal (CUE
// #InterruptedState).
func (s TaskState) Interrupted() bool {
	return s == StateInputRequired
}

// canonical maps every status string the coordination packages emit onto
// the canonical vocabulary. Keys are the raw enum values of tasktree.Status
// (pending/running/done/failed), swarmorchestrator.TaskStatus
// (pending/assigned/done/failed), implementationqueue.ItemStatus
// (pending/in_progress/done/failed) and TaskState itself. The test reads the
// three schema.cue enums and asserts every value is a key here.
var canonical = map[string]TaskState{
	"pending":        StatePending,
	"running":        StateRunning,
	"assigned":       StateRunning,
	"in_progress":    StateRunning,
	"input-required": StateInputRequired,
	"done":           StateCompleted,
	"completed":      StateCompleted,
	"failed":         StateFailed,
	"cancelled":      StateCancelled,
}

// CanonicalState maps a status string from any foundations/coordination
// enum — tasktree.Status, swarmorchestrator.TaskStatus,
// implementationqueue.ItemStatus, or a TaskState — onto the canonical
// TaskState. Matching is exact: no case folding, no trimming, no aliases
// beyond the values those enums define. An unknown string returns
// ErrUnknownState (wrapped with the offending value) and the zero TaskState.
func CanonicalState(s string) (TaskState, error) {
	if st, ok := canonical[s]; ok {
		return st, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownState, s)
}
