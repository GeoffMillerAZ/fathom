package reviewqueue_test

import (
	"testing"

	rq "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/review-queue"
)

func TestQueue_EnqueueDequeue_FIFO(t *testing.T) {
	var q rq.Queue

	items := []rq.ReviewItem{
		{ID: "a", Priority: rq.PriorityLow, Payload: "first"},
		{ID: "b", Priority: rq.PriorityLow, Payload: "second"},
		{ID: "c", Priority: rq.PriorityLow, Payload: "third"},
	}
	for _, it := range items {
		if err := q.Enqueue(it); err != nil {
			t.Fatalf("enqueue %q: %v", it.ID, err)
		}
	}

	// Same priority → FIFO order
	for _, want := range []string{"a", "b", "c"} {
		got, err := q.Dequeue()
		if err != nil {
			t.Fatalf("dequeue: %v", err)
		}
		if got.ID != want {
			t.Errorf("expected ID %q, got %q", want, got.ID)
		}
	}
}

func TestQueue_Priority_HighBeforeLow(t *testing.T) {
	var q rq.Queue

	q.Enqueue(rq.ReviewItem{ID: "low", Priority: rq.PriorityLow, Payload: nil})       //nolint
	q.Enqueue(rq.ReviewItem{ID: "high", Priority: rq.PriorityHigh, Payload: nil})     //nolint
	q.Enqueue(rq.ReviewItem{ID: "med", Priority: rq.PriorityMedium, Payload: nil})    //nolint
	q.Enqueue(rq.ReviewItem{ID: "crit", Priority: rq.PriorityCritical, Payload: nil}) //nolint

	wantOrder := []string{"crit", "high", "med", "low"}
	for _, want := range wantOrder {
		got, err := q.Dequeue()
		if err != nil {
			t.Fatalf("dequeue: %v", err)
		}
		if got.ID != want {
			t.Errorf("expected %q, got %q", want, got.ID)
		}
	}
}

func TestQueue_SamePriority_InsertionOrder(t *testing.T) {
	var q rq.Queue

	for _, id := range []string{"x1", "x2", "x3", "x4"} {
		q.Enqueue(rq.ReviewItem{ID: id, Priority: rq.PriorityMedium}) //nolint
	}

	for _, want := range []string{"x1", "x2", "x3", "x4"} {
		got, err := q.Dequeue()
		if err != nil {
			t.Fatalf("dequeue: %v", err)
		}
		if got.ID != want {
			t.Errorf("expected %q, got %q", want, got.ID)
		}
	}
}

func TestQueue_Peek(t *testing.T) {
	var q rq.Queue

	q.Enqueue(rq.ReviewItem{ID: "p1", Priority: rq.PriorityLow})      //nolint
	q.Enqueue(rq.ReviewItem{ID: "p2", Priority: rq.PriorityCritical}) //nolint

	top, err := q.Peek()
	if err != nil {
		t.Fatalf("peek: %v", err)
	}
	if top.ID != "p2" {
		t.Errorf("peek: expected p2, got %q", top.ID)
	}

	// Peek must not remove the item.
	if q.Len() != 2 {
		t.Errorf("peek should not remove items; expected len 2, got %d", q.Len())
	}
}

func TestQueue_DequeueEmpty(t *testing.T) {
	var q rq.Queue
	_, err := q.Dequeue()
	if err == nil {
		t.Error("expected error dequeuing empty queue, got nil")
	}
}

func TestQueue_PeekEmpty(t *testing.T) {
	var q rq.Queue
	_, err := q.Peek()
	if err == nil {
		t.Error("expected error peeking empty queue, got nil")
	}
}

func TestQueue_EmptyIDRejected(t *testing.T) {
	var q rq.Queue
	err := q.Enqueue(rq.ReviewItem{ID: "", Priority: rq.PriorityLow})
	if err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

func TestQueue_Len(t *testing.T) {
	var q rq.Queue
	if q.Len() != 0 {
		t.Fatalf("expected len 0, got %d", q.Len())
	}
	q.Enqueue(rq.ReviewItem{ID: "l1", Priority: rq.PriorityLow}) //nolint
	q.Enqueue(rq.ReviewItem{ID: "l2", Priority: rq.PriorityLow}) //nolint
	if q.Len() != 2 {
		t.Fatalf("expected len 2, got %d", q.Len())
	}
	q.Dequeue() //nolint
	if q.Len() != 1 {
		t.Fatalf("expected len 1 after dequeue, got %d", q.Len())
	}
}

func TestQueue_Deterministic(t *testing.T) {
	// Enqueue the same items twice in the same order; both runs must
	// produce the same dequeue sequence.
	enqueue := func() *rq.Queue {
		var q rq.Queue
		items := []rq.ReviewItem{
			{ID: "d1", Priority: rq.PriorityCritical},
			{ID: "d2", Priority: rq.PriorityLow},
			{ID: "d3", Priority: rq.PriorityHigh},
			{ID: "d4", Priority: rq.PriorityCritical},
		}
		for _, it := range items {
			q.Enqueue(it) //nolint
		}
		return &q
	}

	q1, q2 := enqueue(), enqueue()
	for i := 0; i < 4; i++ {
		a, err1 := q1.Dequeue()
		b, err2 := q2.Dequeue()
		if err1 != nil || err2 != nil {
			t.Fatalf("dequeue error at index %d", i)
		}
		if a.ID != b.ID {
			t.Errorf("index %d: q1 gave %q, q2 gave %q", i, a.ID, b.ID)
		}
	}
}
