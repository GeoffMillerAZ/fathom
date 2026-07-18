// Package swarmorchestrator plans wave batches and tracks task lifecycle over a
// bounded task set. It is a pure planner: it does NOT spawn goroutines,
// processes, or agent workers. Execution is the caller's responsibility.
//
// Plan-only status (D7): this package ships only as a planner. The executor
// (kits/worker-pool with agent-harness as the worker unit) is not yet built;
// it will be added when a real fan-out workload subscribes.
//
// Usage pattern:
//
//  1. Call Plan(tasks, maxConcurrency) to get a deterministic []Wave batching.
//  2. For each wave, call Assign(taskID) to mark tasks in-flight.
//  3. Call Complete(taskID, failed) when a worker finishes.
//
// All operations are deterministic: Plan sorts tasks by ID before batching, so
// the same input always produces the same wave layout.
package swarmorchestrator

import "errors"

// TaskStatus represents the lifecycle state of a swarm task.
type TaskStatus string

const (
	// TaskPending indicates the task has not yet been assigned to a worker.
	TaskPending TaskStatus = "pending"

	// TaskAssigned indicates the task is actively being worked on.
	TaskAssigned TaskStatus = "assigned"

	// TaskDone indicates the task completed successfully.
	TaskDone TaskStatus = "done"

	// TaskFailed indicates the task failed.
	TaskFailed TaskStatus = "failed"
)

// ErrNotFound is returned when a referenced task does not exist.
var ErrNotFound = errors.New("swarmorchestrator: task not found")

// ErrAlreadyComplete is returned when Complete is called on a task that is
// already done or failed.
var ErrAlreadyComplete = errors.New("swarmorchestrator: task already complete")

// ErrNotAssigned is returned when Complete is called on a task that was never
// assigned.
var ErrNotAssigned = errors.New("swarmorchestrator: task not assigned")

// Task is a unit of work managed by the Orchestrator.
type Task struct {
	// ID is a stable, unique identifier.
	ID string

	// Label is a short human-readable description.
	Label string

	// Status is the current lifecycle state.
	Status TaskStatus
}

// Wave is an ordered batch of task IDs scheduled for concurrent execution.
type Wave struct {
	// Index is the zero-based wave number.
	Index int

	// TaskIDs is the set of task IDs scheduled in this wave.
	TaskIDs []string
}

// Plan is the deterministic execution plan produced by Plan().
type Plan struct {
	// Waves is the ordered sequence of execution waves.
	Waves []Wave

	// MaxConcurrency is the bound used when the plan was created.
	MaxConcurrency int
}

// Orchestrator manages a bounded set of concurrent workers over a task set.
// It is not safe for concurrent use from multiple goroutines.
type Orchestrator struct {
	tasks map[string]*Task
}

// NewOrchestrator returns an empty Orchestrator.
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{tasks: make(map[string]*Task)}
}

// Plan takes a slice of tasks and a maxConcurrency bound and returns a
// deterministic []Wave that partitions the tasks into batches of at most
// maxConcurrency. Tasks are sorted by ID before batching so the result is
// stable regardless of input order. The tasks are also registered internally
// so Assign/Complete can track them.
//
// If maxConcurrency < 1 it is treated as 1.
func (o *Orchestrator) Plan(tasks []Task, maxConcurrency int) []Wave {
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}

	// Sort tasks by ID for determinism.
	sorted := make([]Task, len(tasks))
	copy(sorted, tasks)
	sortTasksByID(sorted)

	// Register tasks.
	for i := range sorted {
		t := sorted[i]
		t.Status = TaskPending
		cp := t
		o.tasks[cp.ID] = &cp
	}

	// Batch into waves.
	var waves []Wave
	for i := 0; i < len(sorted); i += maxConcurrency {
		end := i + maxConcurrency
		if end > len(sorted) {
			end = len(sorted)
		}
		batch := sorted[i:end]
		ids := make([]string, len(batch))
		for j, t := range batch {
			ids[j] = t.ID
		}
		waves = append(waves, Wave{Index: len(waves), TaskIDs: ids})
	}
	if waves == nil {
		waves = []Wave{}
	}
	return waves
}

// Assign marks the task with the given ID as TaskAssigned.
// Returns ErrNotFound if the task was not registered via Plan.
func (o *Orchestrator) Assign(taskID string) error {
	t, ok := o.tasks[taskID]
	if !ok {
		return ErrNotFound
	}
	t.Status = TaskAssigned
	return nil
}

// Complete marks the task as done (failed==false) or failed (failed==true).
// Returns ErrNotFound if the task was not registered.
// Returns ErrNotAssigned if the task was never assigned.
// Returns ErrAlreadyComplete if the task is already done or failed.
func (o *Orchestrator) Complete(taskID string, failed bool) error {
	t, ok := o.tasks[taskID]
	if !ok {
		return ErrNotFound
	}
	switch t.Status {
	case TaskDone, TaskFailed:
		return ErrAlreadyComplete
	case TaskPending:
		return ErrNotAssigned
	}
	if failed {
		t.Status = TaskFailed
	} else {
		t.Status = TaskDone
	}
	return nil
}

// Get returns a copy of the Task with the given ID and whether it was found.
func (o *Orchestrator) Get(taskID string) (Task, bool) {
	t, ok := o.tasks[taskID]
	if !ok {
		return Task{}, false
	}
	return *t, true
}

// sortTasksByID sorts a []Task in place by ascending ID using insertion sort.
func sortTasksByID(ts []Task) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j].ID < ts[j-1].ID; j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}
