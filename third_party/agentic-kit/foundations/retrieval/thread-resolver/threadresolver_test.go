package threadresolver_test

import (
	"testing"

	threadresolver "github.com/geoffmilleraz/agentic-kit/foundations/retrieval/thread-resolver"
)

func TestResolveGroupsByKey(t *testing.T) {
	t.Parallel()

	msgs := []threadresolver.Message{
		{ThreadKey: "t1", ID: "m1", Body: "first"},
		{ThreadKey: "t2", ID: "m2", Body: "second"},
		{ThreadKey: "t1", ID: "m3", Body: "third"},
	}

	threads := threadresolver.Resolve(msgs)
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}
	if threads[0].Key != "t1" {
		t.Errorf("expected first thread key 't1', got %q", threads[0].Key)
	}
	if len(threads[0].Messages) != 2 {
		t.Errorf("expected 2 messages in t1, got %d", len(threads[0].Messages))
	}
	if threads[0].Messages[0].ID != "m1" || threads[0].Messages[1].ID != "m3" {
		t.Errorf("unexpected message order in t1: %v", threads[0].Messages)
	}
	if threads[1].Key != "t2" {
		t.Errorf("expected second thread key 't2', got %q", threads[1].Key)
	}
}

func TestResolveThreadOrderByFirstOccurrence(t *testing.T) {
	t.Parallel()

	msgs := []threadresolver.Message{
		{ThreadKey: "c", ID: "1"},
		{ThreadKey: "a", ID: "2"},
		{ThreadKey: "b", ID: "3"},
		{ThreadKey: "a", ID: "4"},
		{ThreadKey: "c", ID: "5"},
	}

	threads := threadresolver.Resolve(msgs)
	if len(threads) != 3 {
		t.Fatalf("expected 3 threads, got %d", len(threads))
	}
	keys := []string{threads[0].Key, threads[1].Key, threads[2].Key}
	want := []string{"c", "a", "b"}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("thread[%d].Key = %q, want %q", i, k, want[i])
		}
	}
}

func TestResolvePreservesMessageOrder(t *testing.T) {
	t.Parallel()

	msgs := []threadresolver.Message{
		{ThreadKey: "t1", ID: "z"},
		{ThreadKey: "t1", ID: "a"},
		{ThreadKey: "t1", ID: "m"},
	}

	threads := threadresolver.Resolve(msgs)
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	ids := []string{threads[0].Messages[0].ID, threads[0].Messages[1].ID, threads[0].Messages[2].ID}
	want := []string{"z", "a", "m"}
	for i, id := range ids {
		if id != want[i] {
			t.Errorf("messages[%d].ID = %q, want %q", i, id, want[i])
		}
	}
}

func TestResolveEmptyInputReturnsNonNilEmpty(t *testing.T) {
	t.Parallel()

	threads := threadresolver.Resolve(nil)
	if threads == nil {
		t.Error("expected non-nil slice for nil input")
	}
	if len(threads) != 0 {
		t.Errorf("expected 0 threads for nil input, got %d", len(threads))
	}

	threads = threadresolver.Resolve([]threadresolver.Message{})
	if threads == nil {
		t.Error("expected non-nil slice for empty input")
	}
	if len(threads) != 0 {
		t.Errorf("expected 0 threads for empty input, got %d", len(threads))
	}
}

func TestResolveEmptyThreadKeyGrouped(t *testing.T) {
	t.Parallel()

	msgs := []threadresolver.Message{
		{ThreadKey: "", ID: "m1"},
		{ThreadKey: "t1", ID: "m2"},
		{ThreadKey: "", ID: "m3"},
	}

	threads := threadresolver.Resolve(msgs)
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}
	if threads[0].Key != "" {
		t.Errorf("expected first thread key '', got %q", threads[0].Key)
	}
	if len(threads[0].Messages) != 2 {
		t.Errorf("expected 2 messages in empty-key thread, got %d", len(threads[0].Messages))
	}
}

func TestResolveAllSameKey(t *testing.T) {
	t.Parallel()

	msgs := []threadresolver.Message{
		{ThreadKey: "only", ID: "1"},
		{ThreadKey: "only", ID: "2"},
		{ThreadKey: "only", ID: "3"},
	}

	threads := threadresolver.Resolve(msgs)
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	if len(threads[0].Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(threads[0].Messages))
	}
}

func TestResolveSingleMessage(t *testing.T) {
	t.Parallel()

	msgs := []threadresolver.Message{
		{ThreadKey: "solo", ID: "x", Body: "just me"},
	}
	threads := threadresolver.Resolve(msgs)
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	if threads[0].Messages[0].Body != "just me" {
		t.Errorf("unexpected body: %q", threads[0].Messages[0].Body)
	}
}

func TestResolveIsDeterministic(t *testing.T) {
	t.Parallel()

	msgs := []threadresolver.Message{
		{ThreadKey: "b", ID: "1"},
		{ThreadKey: "a", ID: "2"},
		{ThreadKey: "b", ID: "3"},
		{ThreadKey: "c", ID: "4"},
		{ThreadKey: "a", ID: "5"},
	}

	first := threadresolver.Resolve(msgs)
	for run := 0; run < 50; run++ {
		got := threadresolver.Resolve(msgs)
		if len(got) != len(first) {
			t.Fatalf("run %d: length mismatch", run)
		}
		for i := range got {
			if got[i].Key != first[i].Key {
				t.Fatalf("run %d: thread[%d].Key = %q, want %q", run, i, got[i].Key, first[i].Key)
			}
		}
	}
}
