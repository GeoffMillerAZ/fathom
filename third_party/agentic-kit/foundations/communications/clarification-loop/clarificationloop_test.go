package clarificationloop_test

import (
	"strings"
	"testing"

	cl "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/communications/clarification-loop"
)

// ── Question.Validate ─────────────────────────────────────────────────────────

func TestQuestion_Validate_valid(t *testing.T) {
	q := cl.Question{ID: "q-1", Text: "What is the scope?"}
	if err := q.Validate(); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestQuestion_Validate_resolvedWithAnswer(t *testing.T) {
	q := cl.Question{ID: "q-1", Text: "What is the scope?", Resolved: true, Answer: "global"}
	if err := q.Validate(); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestQuestion_Validate_invalid(t *testing.T) {
	cases := []struct {
		name    string
		q       cl.Question
		wantErr string
	}{
		{
			name:    "empty ID",
			q:       cl.Question{ID: "", Text: "hi"},
			wantErr: "ID must be non-empty",
		},
		{
			name:    "empty Text",
			q:       cl.Question{ID: "q-1", Text: ""},
			wantErr: "Text must be non-empty",
		},
		{
			name:    "resolved with empty Answer",
			q:       cl.Question{ID: "q-1", Text: "hi", Resolved: true, Answer: ""},
			wantErr: "Answer must be non-empty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.q.Validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ── Loop.Ask ──────────────────────────────────────────────────────────────────

func TestLoop_Ask_basic(t *testing.T) {
	var l cl.Loop
	if err := l.Ask("q-1", "What is the deadline?"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l.Len() != 1 {
		t.Fatalf("expected Len 1, got %d", l.Len())
	}
}

func TestLoop_Ask_duplicateID_rejected(t *testing.T) {
	var l cl.Loop
	l.Ask("q-1", "first") //nolint:errcheck
	err := l.Ask("q-1", "duplicate")
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error %q should mention already exists", err.Error())
	}
}

func TestLoop_Ask_emptyID_rejected(t *testing.T) {
	var l cl.Loop
	if err := l.Ask("", "some text"); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestLoop_Ask_emptyText_rejected(t *testing.T) {
	var l cl.Loop
	if err := l.Ask("q-1", ""); err == nil {
		t.Fatal("expected error for empty text")
	}
}

// ── Loop.Answer ───────────────────────────────────────────────────────────────

func TestLoop_Answer_basic(t *testing.T) {
	var l cl.Loop
	l.Ask("q-1", "What is the scope?") //nolint:errcheck
	if err := l.Answer("q-1", "global"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q, ok := l.Get("q-1")
	if !ok {
		t.Fatal("question q-1 not found")
	}
	if !q.Resolved {
		t.Error("question should be resolved")
	}
	if q.Answer != "global" {
		t.Errorf("Answer: got %q, want %q", q.Answer, "global")
	}
}

func TestLoop_Answer_notFound(t *testing.T) {
	var l cl.Loop
	err := l.Answer("nonexistent", "some answer")
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error %q should say not found", err.Error())
	}
}

func TestLoop_Answer_alreadyResolved(t *testing.T) {
	var l cl.Loop
	l.Ask("q-1", "?")        //nolint:errcheck
	l.Answer("q-1", "first") //nolint:errcheck
	err := l.Answer("q-1", "second")
	if err == nil {
		t.Fatal("expected error when answering already-resolved question")
	}
	if !strings.Contains(err.Error(), "already resolved") {
		t.Fatalf("error %q should say already resolved", err.Error())
	}
}

func TestLoop_Answer_emptyAnswer_rejected(t *testing.T) {
	var l cl.Loop
	l.Ask("q-1", "?") //nolint:errcheck
	if err := l.Answer("q-1", ""); err == nil {
		t.Fatal("expected error for empty answer")
	}
}

// ── Loop.Open ─────────────────────────────────────────────────────────────────

func TestLoop_Open_returnsOnlyUnresolved(t *testing.T) {
	var l cl.Loop
	l.Ask("q-1", "first")  //nolint:errcheck
	l.Ask("q-2", "second") //nolint:errcheck
	l.Ask("q-3", "third")  //nolint:errcheck
	l.Answer("q-2", "ans") //nolint:errcheck

	open := l.Open()
	if len(open) != 2 {
		t.Fatalf("expected 2 open, got %d", len(open))
	}
	for _, q := range open {
		if q.ID == "q-2" {
			t.Error("q-2 is resolved, should not be in Open()")
		}
	}
}

func TestLoop_Open_emptyLoop(t *testing.T) {
	var l cl.Loop
	open := l.Open()
	if open == nil {
		t.Fatal("Open() on empty loop should return non-nil empty slice")
	}
	if len(open) != 0 {
		t.Fatalf("expected 0 open, got %d", len(open))
	}
}

func TestLoop_Open_allResolved(t *testing.T) {
	var l cl.Loop
	l.Ask("q-1", "?")     //nolint:errcheck
	l.Answer("q-1", "ok") //nolint:errcheck
	open := l.Open()
	if len(open) != 0 {
		t.Fatalf("expected 0 open, got %d", len(open))
	}
}

// ── Loop.All and insertion order ──────────────────────────────────────────────

func TestLoop_All_insertionOrder(t *testing.T) {
	var l cl.Loop
	ids := []string{"q-3", "q-1", "q-2"}
	for _, id := range ids {
		l.Ask(id, "text") //nolint:errcheck
	}
	all := l.All()
	for i, id := range ids {
		if all[i].ID != id {
			t.Errorf("All()[%d].ID = %q, want %q", i, all[i].ID, id)
		}
	}
}

func TestLoop_All_returnsCopy(t *testing.T) {
	var l cl.Loop
	l.Ask("q-1", "?") //nolint:errcheck
	all := l.All()
	all[0].Text = "mutated"
	// Verify the loop is unaffected.
	q, _ := l.Get("q-1")
	if q.Text == "mutated" {
		t.Fatal("All() should return a copy")
	}
}
