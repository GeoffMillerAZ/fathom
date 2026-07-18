package investigationmemo_test

import (
	"fmt"
	"testing"

	im "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-memo"
)

func validMemo() im.Memo {
	return im.Memo{
		Key:     "finding-001",
		Summary: "Ownership gap detected in payments service.",
		Detail:  "No CODEOWNERS entry found for pkg/payments. Three recent incidents lacked a clear owner.",
	}
}

// TestPutAndGetHappyPath ensures a stored memo can be retrieved by key.
func TestPutAndGetHappyPath(t *testing.T) {
	var s im.MemoryStore
	m := validMemo()
	if err := s.Put(m); err != nil {
		t.Fatalf("unexpected Put error: %v", err)
	}
	got, ok := s.Get(m.Key)
	if !ok {
		t.Fatal("expected to find memo, got false")
	}
	if got.Summary != m.Summary {
		t.Errorf("Summary mismatch: want %q, got %q", m.Summary, got.Summary)
	}
}

// TestGetMissing ensures Get returns false for an unknown key.
func TestGetMissing(t *testing.T) {
	var s im.MemoryStore
	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("expected false for missing key, got true")
	}
}

// TestPutEmptyKey ensures an empty Key is rejected.
func TestPutEmptyKey(t *testing.T) {
	var s im.MemoryStore
	m := validMemo()
	m.Key = ""
	if err := s.Put(m); err == nil {
		t.Fatal("expected error for empty Key, got nil")
	}
}

// TestPutEmptySummary ensures an empty Summary is rejected.
func TestPutEmptySummary(t *testing.T) {
	var s im.MemoryStore
	m := validMemo()
	m.Summary = ""
	if err := s.Put(m); err == nil {
		t.Fatal("expected error for empty Summary, got nil")
	}
}

// TestPutOverwrite ensures Put overwrites an existing memo with the same key.
func TestPutOverwrite(t *testing.T) {
	var s im.MemoryStore
	m1 := validMemo()
	m2 := validMemo()
	m2.Summary = "Updated finding summary."
	if err := s.Put(m1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.Put(m2); err != nil {
		t.Fatalf("unexpected error on overwrite: %v", err)
	}
	got, ok := s.Get(m1.Key)
	if !ok {
		t.Fatal("memo not found after overwrite")
	}
	if got.Summary != m2.Summary {
		t.Errorf("want updated summary %q, got %q", m2.Summary, got.Summary)
	}
}

// TestListOrder ensures List returns memos in lexicographic key order.
func TestListOrder(t *testing.T) {
	var s im.MemoryStore
	keys := []string{"finding-c", "finding-a", "finding-b"}
	for _, k := range keys {
		m := validMemo()
		m.Key = k
		if err := s.Put(m); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	list := s.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 memos, got %d", len(list))
	}
	want := []string{"finding-a", "finding-b", "finding-c"}
	for i, m := range list {
		if m.Key != want[i] {
			t.Errorf("list[%d]: want key %q, got %q", i, want[i], m.Key)
		}
	}
}

// TestListEmpty ensures List returns nil when the store is empty.
func TestListEmpty(t *testing.T) {
	var s im.MemoryStore
	if got := s.List(); got != nil {
		t.Errorf("expected nil for empty store, got %v", got)
	}
}

// TestStoreInterface ensures MemoryStore satisfies the Store interface at
// compile time.
func TestStoreInterface(t *testing.T) {
	var _ im.Store = (*im.MemoryStore)(nil)
}

// TestConcurrentPut exercises Put under concurrent writers (go test -race).
func TestConcurrentPut(t *testing.T) {
	var s im.MemoryStore
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			m := im.Memo{
				Key:     fmt.Sprintf("key-%d", n),
				Summary: "concurrent-summary",
			}
			_ = s.Put(m)
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
