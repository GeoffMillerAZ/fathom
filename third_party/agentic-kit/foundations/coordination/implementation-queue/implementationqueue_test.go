package implementationqueue

import "testing"

func TestPush_Dedup(t *testing.T) {
	q := NewQueue()
	if !q.Push("k1", "payload-1") {
		t.Fatal("first push should return true")
	}
	if q.Push("k1", "payload-1-dup") {
		t.Fatal("second push of same key should return false (dedup)")
	}
	if q.Len() != 1 {
		t.Fatalf("expected Len=1 after dedup push, got %d", q.Len())
	}
}

func TestPop_FIFOOrder(t *testing.T) {
	q := NewQueue()
	q.Push("k1", "first")
	q.Push("k2", "second")
	q.Push("k3", "third")

	item, err := q.Pop()
	if err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if item.Key != "k1" {
		t.Fatalf("expected k1, got %s", item.Key)
	}
	if item.Status != StatusInProgress {
		t.Fatalf("expected StatusInProgress after Pop, got %v", item.Status)
	}

	item2, _ := q.Pop()
	if item2.Key != "k2" {
		t.Fatalf("expected k2, got %s", item2.Key)
	}
}

func TestPop_Empty(t *testing.T) {
	q := NewQueue()
	_, err := q.Pop()
	if err != ErrEmpty {
		t.Fatalf("expected ErrEmpty, got %v", err)
	}
}

func TestPeek_DoesNotRemove(t *testing.T) {
	q := NewQueue()
	q.Push("k1", "payload")

	item, err := q.Peek()
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if item.Key != "k1" {
		t.Fatalf("expected k1, got %s", item.Key)
	}
	if q.Len() != 1 {
		t.Fatal("Peek should not remove item")
	}
}

func TestPeek_Empty(t *testing.T) {
	q := NewQueue()
	_, err := q.Peek()
	if err != ErrEmpty {
		t.Fatalf("expected ErrEmpty, got %v", err)
	}
}

func TestLen(t *testing.T) {
	q := NewQueue()
	if q.Len() != 0 {
		t.Fatalf("expected Len=0, got %d", q.Len())
	}
	q.Push("k1", "a")
	q.Push("k2", "b")
	if q.Len() != 2 {
		t.Fatalf("expected Len=2, got %d", q.Len())
	}
	q.Pop()
	if q.Len() != 1 {
		t.Fatalf("expected Len=1 after Pop, got %d", q.Len())
	}
}

func TestMarkStatus_AllValues(t *testing.T) {
	cases := []struct {
		name   string
		status ItemStatus
	}{
		{"pending", StatusPending},
		{"in_progress", StatusInProgress},
		{"done", StatusDone},
		{"failed", StatusFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := NewQueue()
			q.Push("k1", "p1")
			if err := q.MarkStatus("k1", tc.status); err != nil {
				t.Fatalf("MarkStatus(%v): %v", tc.status, err)
			}
			item, _ := q.Get("k1")
			if item.Status != tc.status {
				t.Fatalf("expected %v, got %v", tc.status, item.Status)
			}
		})
	}
}

func TestMarkStatus_NotFound(t *testing.T) {
	q := NewQueue()
	if err := q.MarkStatus("ghost", StatusDone); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMarkStatus_Invalid(t *testing.T) {
	q := NewQueue()
	q.Push("k1", "p1")
	if err := q.MarkStatus("k1", ItemStatus("cancelled")); err != ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestGet_NotFound(t *testing.T) {
	q := NewQueue()
	_, ok := q.Get("ghost")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestPeek_SkipsNonPending(t *testing.T) {
	q := NewQueue()
	q.Push("k1", "first")
	q.Push("k2", "second")
	// Pop k1 (advances to in_progress) then re-peek should show k2
	q.Pop()
	item, err := q.Peek()
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if item.Key != "k2" {
		t.Fatalf("expected k2, got %s", item.Key)
	}
}
