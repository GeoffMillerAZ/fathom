package reviewqueue

import (
	"fmt"
	"sort"
)

// Queue is a priority queue of ReviewItems. Items are returned in priority
// order (highest first) with insertion order as the tiebreaker, making
// dequeue deterministic for items of equal priority.
type Queue struct {
	items []entry
	seq   int // monotonic insertion counter for tiebreaking
}

// entry wraps a ReviewItem with its insertion sequence number.
type entry struct {
	item ReviewItem
	seq  int
}

// Enqueue adds an item to the queue. Returns an error if ID is empty.
func (q *Queue) Enqueue(item ReviewItem) error {
	if item.ID == "" {
		return fmt.Errorf("reviewqueue: ReviewItem.ID must not be empty")
	}
	q.items = append(q.items, entry{item: item, seq: q.seq})
	q.seq++
	// Keep the slice sorted: highest priority first, lowest seq first for ties.
	sort.SliceStable(q.items, func(i, j int) bool {
		if q.items[i].item.Priority != q.items[j].item.Priority {
			return q.items[i].item.Priority > q.items[j].item.Priority
		}
		return q.items[i].seq < q.items[j].seq
	})
	return nil
}

// Dequeue removes and returns the highest-priority item (FIFO within a
// priority tier). Returns an error if the queue is empty.
func (q *Queue) Dequeue() (ReviewItem, error) {
	if len(q.items) == 0 {
		return ReviewItem{}, fmt.Errorf("reviewqueue: queue is empty")
	}
	item := q.items[0].item
	q.items = q.items[1:]
	return item, nil
}

// Peek returns the next item that would be dequeued without removing it.
// Returns an error if the queue is empty.
func (q *Queue) Peek() (ReviewItem, error) {
	if len(q.items) == 0 {
		return ReviewItem{}, fmt.Errorf("reviewqueue: queue is empty")
	}
	return q.items[0].item, nil
}

// Len returns the number of items currently in the queue.
func (q *Queue) Len() int {
	return len(q.items)
}
