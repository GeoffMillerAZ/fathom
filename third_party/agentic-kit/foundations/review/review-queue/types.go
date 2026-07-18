// Package reviewqueue provides a FIFO/priority queue of items needing review,
// ordered by severity (critical first) then by insertion order for determinism.
//
// Concurrency: Queue is not safe for concurrent use; guard with your own synchronisation.
package reviewqueue

// ReviewPriority determines the ordering within the queue.
// Higher numeric value = higher priority (dequeued first).
type ReviewPriority int

const (
	// PriorityLow is the lowest review priority.
	PriorityLow ReviewPriority = 1
	// PriorityMedium is a moderate review priority.
	PriorityMedium ReviewPriority = 2
	// PriorityHigh is a high review priority.
	PriorityHigh ReviewPriority = 3
	// PriorityCritical is the highest review priority; items are dequeued first.
	PriorityCritical ReviewPriority = 4
)

// ReviewItem is an entry in the review queue.
type ReviewItem struct {
	// ID is a stable unique identifier for the item.
	ID string

	// Priority determines the item's position in the queue.
	Priority ReviewPriority

	// Payload carries the opaque data associated with the item.
	Payload interface{}
}
