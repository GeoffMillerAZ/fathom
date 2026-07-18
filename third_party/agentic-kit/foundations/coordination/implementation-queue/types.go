// Package implementationqueue provides a FIFO work queue with deduplication and
// status tracking. It is a pure data structure: it does NOT execute or dispatch
// work items.
//
// Plan-only status (D7): this package ships as a planner primitive. The worker
// executor (kits/worker-pool with agent-harness as the worker unit) is not yet
// built; it will be added when a real fan-out workload subscribes.
//
// Items are keyed by a stable string Key. Pushing an item whose Key already
// exists in the queue (regardless of status) is a no-op — the original entry
// is preserved. Pop removes and returns the front of the queue. Peek returns
// the front without removing it.
//
// All operations are deterministic and safe for single-goroutine use.
//
// Concurrency contract: Queue is not safe for concurrent use; callers that
// share a Queue across goroutines must guard access with their own
// synchronization.
package implementationqueue

import "errors"

// ItemStatus represents the lifecycle state of a queue item.
type ItemStatus string

const (
	// StatusPending indicates the item is waiting to be processed.
	StatusPending ItemStatus = "pending"

	// StatusInProgress indicates the item is actively being processed.
	StatusInProgress ItemStatus = "in_progress"

	// StatusDone indicates the item completed successfully.
	StatusDone ItemStatus = "done"

	// StatusFailed indicates the item failed.
	StatusFailed ItemStatus = "failed"
)

// ErrEmpty is returned when Pop or Peek is called on an empty queue.
var ErrEmpty = errors.New("implementationqueue: queue is empty")

// ErrNotFound is returned when MarkStatus references a key not in the queue.
var ErrNotFound = errors.New("implementationqueue: item not found")

// ErrInvalidStatus is returned when an unrecognized status value is used.
var ErrInvalidStatus = errors.New("implementationqueue: invalid status")

// Item is a single work item in the queue.
type Item struct {
	// Key is the stable dedup identifier.
	Key string

	// Payload is the string-encoded work descriptor.
	Payload string

	// Status is the current lifecycle state.
	Status ItemStatus
}

// Queue is a FIFO work queue with key-based deduplication and status tracking.
// It is not safe for concurrent use from multiple goroutines.
type Queue struct {
	// order preserves insertion order for FIFO behaviour.
	order []string
	// items stores all known items (including dequeued ones) keyed by Key.
	items map[string]*Item
}

// NewQueue returns an empty Queue.
func NewQueue() *Queue {
	return &Queue{items: make(map[string]*Item)}
}

// Push appends an item to the back of the queue.
// If an item with the same Key already exists the call is a no-op (dedup).
// Returns true if the item was added, false if it was a duplicate.
func (q *Queue) Push(key, payload string) bool {
	if _, exists := q.items[key]; exists {
		return false
	}
	item := &Item{Key: key, Payload: payload, Status: StatusPending}
	q.items[key] = item
	q.order = append(q.order, key)
	return true
}

// Pop removes and returns the front pending item in the queue.
// Returns ErrEmpty if there are no pending items.
// The item's status is advanced to StatusInProgress.
func (q *Queue) Pop() (Item, error) {
	for len(q.order) > 0 {
		key := q.order[0]
		q.order = q.order[1:]
		item, ok := q.items[key]
		if !ok {
			continue
		}
		if item.Status != StatusPending {
			// Already popped or completed — skip.
			continue
		}
		item.Status = StatusInProgress
		return *item, nil
	}
	return Item{}, ErrEmpty
}

// Peek returns a copy of the front pending item without removing it.
// Returns ErrEmpty if there are no pending items.
func (q *Queue) Peek() (Item, error) {
	for _, key := range q.order {
		item, ok := q.items[key]
		if !ok {
			continue
		}
		if item.Status == StatusPending {
			return *item, nil
		}
	}
	return Item{}, ErrEmpty
}

// Len returns the number of pending items still in the queue.
func (q *Queue) Len() int {
	count := 0
	for _, key := range q.order {
		item, ok := q.items[key]
		if ok && item.Status == StatusPending {
			count++
		}
	}
	return count
}

// MarkStatus sets the status of the item with the given key.
// Returns ErrNotFound if no item with that key exists.
// Returns ErrInvalidStatus if the status value is not recognized.
func (q *Queue) MarkStatus(key string, s ItemStatus) error {
	switch s {
	case StatusPending, StatusInProgress, StatusDone, StatusFailed:
		// valid
	default:
		return ErrInvalidStatus
	}
	item, ok := q.items[key]
	if !ok {
		return ErrNotFound
	}
	item.Status = s
	return nil
}

// Get returns a copy of the item with the given key and whether it was found.
func (q *Queue) Get(key string) (Item, bool) {
	item, ok := q.items[key]
	if !ok {
		return Item{}, false
	}
	return *item, true
}
