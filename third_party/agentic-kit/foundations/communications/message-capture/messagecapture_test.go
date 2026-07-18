package messagecapture_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	messagecapture "github.com/geoffmilleraz/agentic-kit/foundations/communications/message-capture"
)

// ── Message.Validate ─────────────────────────────────────────────────────────

var baseTime = time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)

func TestMessage_Validate_valid(t *testing.T) {
	m := messagecapture.Message{
		ID:      "msg-001",
		Channel: "general",
		Author:  "alice",
		Text:    "Hello!",
		At:      baseTime,
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestMessage_Validate_emptyText_valid(t *testing.T) {
	// Text may be empty (attachment-only posts).
	m := messagecapture.Message{
		ID:      "msg-002",
		Channel: "uploads",
		Author:  "bob",
		Text:    "",
		At:      baseTime,
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("empty Text should be valid, got: %v", err)
	}
}

func TestMessage_Validate_invalid(t *testing.T) {
	base := messagecapture.Message{
		ID:      "msg-001",
		Channel: "general",
		Author:  "alice",
		Text:    "hi",
		At:      baseTime,
	}

	cases := []struct {
		name    string
		mutate  func(*messagecapture.Message)
		wantErr string
	}{
		{
			name:    "empty ID",
			mutate:  func(m *messagecapture.Message) { m.ID = "" },
			wantErr: "ID must be non-empty",
		},
		{
			name:    "whitespace ID",
			mutate:  func(m *messagecapture.Message) { m.ID = "   " },
			wantErr: "ID must be non-empty",
		},
		{
			name:    "empty Channel",
			mutate:  func(m *messagecapture.Message) { m.Channel = "" },
			wantErr: "Channel must be non-empty",
		},
		{
			name:    "empty Author",
			mutate:  func(m *messagecapture.Message) { m.Author = "" },
			wantErr: "Author must be non-empty",
		},
		{
			name:    "zero At",
			mutate:  func(m *messagecapture.Message) { m.At = time.Time{} },
			wantErr: "At must be a non-zero time",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := base
			tc.mutate(&msg)
			err := msg.Validate()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ── Store ────────────────────────────────────────────────────────────────────

func TestStore_Add_dedup(t *testing.T) {
	var s messagecapture.Store

	m1 := messagecapture.Message{ID: "a", Channel: "c", Author: "u", At: baseTime}
	m2 := messagecapture.Message{ID: "a", Channel: "c", Author: "u", At: baseTime} // duplicate

	if !s.Add(m1) {
		t.Fatal("first Add should return true")
	}
	if s.Add(m2) {
		t.Fatal("duplicate Add should return false")
	}
	if s.Len() != 1 {
		t.Fatalf("expected len 1, got %d", s.Len())
	}
}

func TestStore_All_ordered(t *testing.T) {
	var s messagecapture.Store

	msgs := []messagecapture.Message{
		{ID: "x", Channel: "c", Author: "u", At: baseTime},
		{ID: "y", Channel: "c", Author: "u", At: baseTime},
		{ID: "z", Channel: "c", Author: "u", At: baseTime},
	}
	for _, m := range msgs {
		s.Add(m)
	}

	all := s.All()
	if len(all) != len(msgs) {
		t.Fatalf("expected %d messages, got %d", len(msgs), len(all))
	}
	for i, m := range all {
		if m.ID != msgs[i].ID {
			t.Errorf("position %d: got ID %q, want %q", i, m.ID, msgs[i].ID)
		}
	}
}

func TestStore_All_returnsCopy(t *testing.T) {
	var s messagecapture.Store
	s.Add(messagecapture.Message{ID: "a", Channel: "c", Author: "u", At: baseTime})

	a := s.All()
	a[0].ID = "mutated"

	b := s.All()
	if b[0].ID == "mutated" {
		t.Fatal("All() should return a copy, not a reference to the internal slice")
	}
}

// ── Capture ──────────────────────────────────────────────────────────────────

// staticChannel is a Channel stub that returns a fixed set of messages.
type staticChannel struct {
	msgs []messagecapture.Message
	err  error
}

func (sc *staticChannel) Poll() ([]messagecapture.Message, error) {
	return sc.msgs, sc.err
}

func TestCapture_Run_basic(t *testing.T) {
	ch := &staticChannel{msgs: []messagecapture.Message{
		{ID: "1", Channel: "c", Author: "alice", At: baseTime},
		{ID: "2", Channel: "c", Author: "bob", At: baseTime},
	}}

	var cap messagecapture.Capture
	added, err := cap.Run(ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 2 {
		t.Fatalf("expected 2 added, got %d", added)
	}
	if cap.Len() != 2 {
		t.Fatalf("expected Len 2, got %d", cap.Len())
	}
}

func TestCapture_Run_dedup_acrossPolls(t *testing.T) {
	ch := &staticChannel{msgs: []messagecapture.Message{
		{ID: "1", Channel: "c", Author: "alice", At: baseTime},
	}}

	var cap messagecapture.Capture
	cap.Run(ch) //nolint:errcheck
	// Second poll returns same message (re-delivered).
	added, err := cap.Run(ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 0 {
		t.Fatalf("expected 0 new messages on second run, got %d", added)
	}
	if cap.Len() != 1 {
		t.Fatalf("expected total Len 1, got %d", cap.Len())
	}
}

func TestCapture_Run_pollError(t *testing.T) {
	ch := &staticChannel{err: errors.New("network timeout")}

	var cap messagecapture.Capture
	_, err := cap.Run(ch)
	if err == nil {
		t.Fatal("expected error from Poll, got nil")
	}
	if cap.Len() != 0 {
		t.Fatal("store must not be modified on Poll error")
	}
}

func TestCapture_Messages_order(t *testing.T) {
	ids := []string{"alpha", "beta", "gamma"}
	var msgs []messagecapture.Message
	for _, id := range ids {
		msgs = append(msgs, messagecapture.Message{ID: id, Channel: "c", Author: "u", At: baseTime})
	}

	var cap messagecapture.Capture
	cap.Run(&staticChannel{msgs: msgs}) //nolint:errcheck

	got := cap.Messages()
	for i, m := range got {
		if m.ID != ids[i] {
			t.Errorf("Messages()[%d].ID = %q, want %q", i, m.ID, ids[i])
		}
	}
}

// ── Channel interface conformance (compile-time check) ────────────────────────

var _ messagecapture.Channel = (*staticChannel)(nil)
