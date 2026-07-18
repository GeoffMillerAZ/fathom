package operatorelicitation

import "testing"

func TestAsk_AssignsID(t *testing.T) {
	e := NewElicitor()
	id := e.Ask("What format?")
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
	req, ok := e.Get(id)
	if !ok {
		t.Fatal("request not found after Ask")
	}
	if req.Prompt != "What format?" {
		t.Fatalf("unexpected prompt: %q", req.Prompt)
	}
	if req.Resolved {
		t.Fatal("new request should not be resolved")
	}
	if req.Answer != "" {
		t.Fatalf("answer should be empty, got %q", req.Answer)
	}
}

func TestAsk_MonotonicIDs(t *testing.T) {
	e := NewElicitor()
	id1 := e.Ask("Q1")
	id2 := e.Ask("Q2")
	id3 := e.Ask("Q3")
	if id1 == id2 || id2 == id3 || id1 == id3 {
		t.Fatal("IDs should be unique")
	}
}

func TestAnswer_ResolvesRequest(t *testing.T) {
	e := NewElicitor()
	id := e.Ask("preferred output?")

	if err := e.Answer(id, "JSON"); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	req, _ := e.Get(id)
	if !req.Resolved {
		t.Fatal("expected Resolved=true after Answer")
	}
	if req.Answer != "JSON" {
		t.Fatalf("expected answer=JSON, got %q", req.Answer)
	}
}

func TestAnswer_NotFound(t *testing.T) {
	e := NewElicitor()
	if err := e.Answer("ghost", "answer"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAnswer_AlreadyResolved(t *testing.T) {
	e := NewElicitor()
	id := e.Ask("Q")
	_ = e.Answer(id, "first")
	if err := e.Answer(id, "second"); err != ErrAlreadyResolved {
		t.Fatalf("expected ErrAlreadyResolved, got %v", err)
	}
}

func TestPending_Order(t *testing.T) {
	e := NewElicitor()
	id1 := e.Ask("Q1")
	id2 := e.Ask("Q2")
	id3 := e.Ask("Q3")

	// Answer the middle one.
	_ = e.Answer(id2, "A2")

	pending := e.Pending()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}
	if pending[0].ID != id1 {
		t.Fatalf("expected first pending to be %s, got %s", id1, pending[0].ID)
	}
	if pending[1].ID != id3 {
		t.Fatalf("expected second pending to be %s, got %s", id3, pending[1].ID)
	}
}

func TestPending_EmptyWhenAllResolved(t *testing.T) {
	e := NewElicitor()
	id := e.Ask("Q")
	_ = e.Answer(id, "A")
	if len(e.Pending()) != 0 {
		t.Fatal("expected empty pending slice")
	}
}

func TestPending_InitiallyEmpty(t *testing.T) {
	e := NewElicitor()
	if len(e.Pending()) != 0 {
		t.Fatal("expected empty pending on new elicitor")
	}
}

func TestGet_NotFound(t *testing.T) {
	e := NewElicitor()
	_, ok := e.Get("missing")
	if ok {
		t.Fatal("expected not found")
	}
}
