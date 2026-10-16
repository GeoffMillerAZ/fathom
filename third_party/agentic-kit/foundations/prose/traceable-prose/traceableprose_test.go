package traceableprose_test

import (
	"testing"
	"time"

	tp "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/prose/traceable-prose"
)

// ---- helpers ---------------------------------------------------------------

func validNode() tp.TraceableNode {
	return tp.TraceableNode{
		ID:         "node-001",
		Text:       "The payments service processes 10 000 requests per second.",
		SourceRefs: []string{"span-perf-001"},
	}
}

func validEvent() tp.RegenerationEvent {
	return tp.RegenerationEvent{
		TriggeredBy:     "correction-007",
		AffectedNodeIDs: []string{"node-001"},
		OccurredAt:      time.Now(),
		Note:            "evidence updated",
	}
}

// ---- TraceableNode.Validate ------------------------------------------------

func TestTraceableNode_ValidateHappyPath(t *testing.T) {
	if err := validNode().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestTraceableNode_RequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		mutFn func(*tp.TraceableNode)
	}{
		{"empty-ID", func(n *tp.TraceableNode) { n.ID = "" }},
		{"empty-Text", func(n *tp.TraceableNode) { n.Text = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := validNode()
			tc.mutFn(&n)
			if err := n.Validate(); err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

func TestTraceableNode_RedactedRequiresReason(t *testing.T) {
	n := validNode()
	n.Redacted = true
	n.RedactionReason = ""
	if err := n.Validate(); err == nil {
		t.Fatal("expected error: Redacted=true with empty RedactionReason")
	}
}

func TestTraceableNode_RedactedWithReason(t *testing.T) {
	n := validNode()
	n.Redacted = true
	n.RedactionReason = "DLP policy triggered"
	if err := n.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestTraceableNode_EmptySourceRef(t *testing.T) {
	n := validNode()
	n.SourceRefs = []string{"span-001", ""}
	if err := n.Validate(); err == nil {
		t.Fatal("expected error for empty SourceRef element")
	}
}

// ---- TraceableNode.MarkRedacted --------------------------------------------

func TestMarkRedacted_HappyPath(t *testing.T) {
	n := validNode()
	if err := n.MarkRedacted("DLP triggered"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !n.Redacted {
		t.Error("expected Redacted=true")
	}
	if n.RedactionReason != "DLP triggered" {
		t.Errorf("expected reason 'DLP triggered', got %q", n.RedactionReason)
	}
}

func TestMarkRedacted_EmptyReasonRejected(t *testing.T) {
	n := validNode()
	if err := n.MarkRedacted(""); err == nil {
		t.Fatal("expected error for empty reason, got nil")
	}
}

// ---- RegenerationEvent.Validate --------------------------------------------

func TestRegenerationEvent_ValidateHappyPath(t *testing.T) {
	if err := validEvent().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRegenerationEvent_RequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		mutFn func(*tp.RegenerationEvent)
	}{
		{"empty-TriggeredBy", func(e *tp.RegenerationEvent) { e.TriggeredBy = "" }},
		{"empty-AffectedNodeIDs", func(e *tp.RegenerationEvent) { e.AffectedNodeIDs = nil }},
		{"zero-OccurredAt", func(e *tp.RegenerationEvent) { e.OccurredAt = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := validEvent()
			tc.mutFn(&ev)
			if err := ev.Validate(); err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

func TestRegenerationEvent_EmptyNodeID(t *testing.T) {
	ev := validEvent()
	ev.AffectedNodeIDs = []string{"node-001", ""}
	if err := ev.Validate(); err == nil {
		t.Fatal("expected error for empty node ID in AffectedNodeIDs")
	}
}

// ---- RegenerationLog -------------------------------------------------------

func TestRegenerationLog_AppendAndRetrieve(t *testing.T) {
	var log tp.RegenerationLog
	if err := log.Append(validEvent()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if log.Len() != 1 {
		t.Errorf("expected Len=1, got %d", log.Len())
	}
	events := log.Events()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

func TestRegenerationLog_AppendRejectsInvalidEvent(t *testing.T) {
	var log tp.RegenerationLog
	bad := tp.RegenerationEvent{} // zero value — TriggeredBy empty
	if err := log.Append(bad); err == nil {
		t.Fatal("expected error for invalid event, got nil")
	}
	if log.Len() != 0 {
		t.Errorf("log should remain empty after failed Append, got Len=%d", log.Len())
	}
}

func TestRegenerationLog_EventsReturnsCopy(t *testing.T) {
	var log tp.RegenerationLog
	_ = log.Append(validEvent())
	ev := log.Events()
	ev[0].Note = "mutated"
	if log.Events()[0].Note == "mutated" {
		t.Error("Events() should return a copy; log was mutated through returned slice")
	}
}

func TestRegenerationLog_MultipleAppends(t *testing.T) {
	var log tp.RegenerationLog
	for i := 0; i < 3; i++ {
		_ = log.Append(validEvent())
	}
	if log.Len() != 3 {
		t.Errorf("expected Len=3, got %d", log.Len())
	}
}

// ---- MarkNodesForRegeneration ----------------------------------------------

func TestMarkNodesForRegeneration_MarksMatching(t *testing.T) {
	nodes := []tp.TraceableNode{
		{ID: "n1", Text: "A"},
		{ID: "n2", Text: "B"},
		{ID: "n3", Text: "C"},
	}
	count := tp.MarkNodesForRegeneration(nodes, []string{"n1", "n3"})
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
	if !nodes[0].NeedsRegeneration {
		t.Error("n1 should be marked")
	}
	if nodes[1].NeedsRegeneration {
		t.Error("n2 should not be marked")
	}
	if !nodes[2].NeedsRegeneration {
		t.Error("n3 should be marked")
	}
}

func TestMarkNodesForRegeneration_NoMatch(t *testing.T) {
	nodes := []tp.TraceableNode{{ID: "n1", Text: "A"}}
	count := tp.MarkNodesForRegeneration(nodes, []string{"n999"})
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}
	if nodes[0].NeedsRegeneration {
		t.Error("n1 should not be marked")
	}
}

func TestMarkNodesForRegeneration_EmptyAffected(t *testing.T) {
	nodes := []tp.TraceableNode{{ID: "n1", Text: "A"}}
	count := tp.MarkNodesForRegeneration(nodes, nil)
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}
}
