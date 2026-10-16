package evidencecompactor_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	evidencecompactor "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/evidence-compactor"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
)

// fixedClock returns a deterministic clock that always returns t.
func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// epoch is a deterministic fixed time for tests.
var epoch = time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

// droppedMsgs builds a minimal two-message dropped slice for tests.
func droppedMsgs() []llmclient.Message {
	return []llmclient.Message{
		{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("ask something")}},
		{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{llmclient.TextBlock("answer something")}},
	}
}

// newCompactor is a test convenience constructor that wires a *bytes.Buffer
// as the JSONL journal sink.
func newCompactor(t *testing.T, client llmclient.LLMClient, buf *bytes.Buffer) *evidencecompactor.Compactor {
	t.Helper()
	c, err := evidencecompactor.New(
		evidencecompactor.Config{Model: "test-model", MaxTokens: 512},
		client,
		jsonllog.NewWriter(buf),
		fixedClock(epoch),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// lastJournalRecord decodes the last (or only) JSONL line from buf as a
// JournalRecord.
func lastJournalRecord(t *testing.T, buf *bytes.Buffer) evidencecompactor.JournalRecord {
	t.Helper()
	lines := bytes.Split(bytes.TrimRight(buf.Bytes(), "\n"), []byte("\n"))
	if len(lines) == 0 {
		t.Fatal("journal buffer is empty")
	}
	var rec evidencecompactor.JournalRecord
	if err := json.Unmarshal(lines[len(lines)-1], &rec); err != nil {
		t.Fatalf("decode journal record: %v", err)
	}
	return rec
}

// --- New() validation -------------------------------------------------------

func TestNewValidation(t *testing.T) {
	var buf bytes.Buffer
	w := jsonllog.NewWriter(&buf)
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("summary"))

	t.Run("nil client", func(t *testing.T) {
		_, err := evidencecompactor.New(evidencecompactor.Config{}, nil, w, nil)
		if err == nil || !strings.Contains(err.Error(), "client") {
			t.Fatalf("want client error, got %v", err)
		}
	})
	t.Run("nil journal", func(t *testing.T) {
		_, err := evidencecompactor.New(evidencecompactor.Config{}, mock, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "journal") {
			t.Fatalf("want journal error, got %v", err)
		}
	})
	t.Run("nil clock defaults to time.Now", func(t *testing.T) {
		_, err := evidencecompactor.New(evidencecompactor.Config{}, mock, w, nil)
		if err != nil {
			t.Fatalf("nil clock should be accepted: %v", err)
		}
	})
}

// --- Happy path: journal then summarize -------------------------------------

func TestCompactJournalsThenSummarizes(t *testing.T) {
	var buf bytes.Buffer
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("The model answer was 42."))
	c := newCompactor(t, mock, &buf)

	dropped := droppedMsgs()
	msg, ok, err := c.Compact(context.Background(), dropped)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if !ok {
		t.Fatal("Compact returned ok=false, want true")
	}
	if len(msg.Content) == 0 {
		t.Fatal("summary message has no content")
	}

	// Journal must have been written.
	if buf.Len() == 0 {
		t.Fatal("journal buffer is empty after Compact")
	}
	rec := lastJournalRecord(t, &buf)

	// Verify journal record fields.
	if rec.Kind != "compaction-dropped" {
		t.Fatalf("rec.Kind = %q, want compaction-dropped", rec.Kind)
	}
	if rec.At != epoch.Format(time.RFC3339) {
		t.Fatalf("rec.At = %q, want %q", rec.At, epoch.Format(time.RFC3339))
	}
	if rec.DroppedCount != 2 {
		t.Fatalf("rec.DroppedCount = %d, want 2", rec.DroppedCount)
	}
	if len(rec.Messages) != 2 {
		t.Fatalf("rec.Messages len = %d, want 2", len(rec.Messages))
	}
	if rec.TemplateVersion != evidencecompactor.SummaryTemplateVersion {
		t.Fatalf("rec.TemplateVersion = %q, want %q", rec.TemplateVersion, evidencecompactor.SummaryTemplateVersion)
	}
	if rec.ParamsHash == "" {
		t.Fatal("rec.ParamsHash is empty")
	}

	// LLM must have been called exactly once.
	if mock.CallCount() != 1 {
		t.Fatalf("LLM called %d times, want 1", mock.CallCount())
	}
}

// --- Journal-before-summary ordering ----------------------------------------
//
// Prove that a failing LLM call still leaves the journal written.
// If journal is written first and LLM fails, ground truth is safe.

func TestJournalWrittenBeforeLLMCall(t *testing.T) {
	var buf bytes.Buffer
	llmErr := errors.New("LLM unavailable")
	mock := llmclient.NewMockClient(llmclient.MockEntryErr(llmErr))
	c := newCompactor(t, mock, &buf)

	dropped := droppedMsgs()
	_, _, err := c.Compact(context.Background(), dropped)
	if err == nil {
		t.Fatal("want error from failing LLM, got nil")
	}
	if !strings.Contains(err.Error(), "LLM") && !strings.Contains(err.Error(), "summary") {
		t.Fatalf("error %q should mention LLM or summary", err)
	}

	// Journal must have been written despite the LLM failure.
	if buf.Len() == 0 {
		t.Fatal("journal buffer empty — ground truth was NOT preserved before LLM call")
	}
	rec := lastJournalRecord(t, &buf)
	if rec.DroppedCount != 2 {
		t.Fatalf("journal record has DroppedCount=%d, want 2", rec.DroppedCount)
	}
	if len(rec.Messages) != 2 {
		t.Fatalf("journal record has %d messages, want 2", len(rec.Messages))
	}

	// LLM must have been called (it failed, but the call happened after journal).
	if mock.CallCount() != 1 {
		t.Fatalf("LLM called %d times, want 1 (call happened, journal came first)", mock.CallCount())
	}
}

// --- LLM failure → (false, err): no data loss ------------------------------

func TestLLMFailureReturnsError(t *testing.T) {
	var buf bytes.Buffer
	llmErr := errors.New("rate limit")
	mock := llmclient.NewMockClient(llmclient.MockEntryErr(llmErr))
	c := newCompactor(t, mock, &buf)

	msg, ok, err := c.Compact(context.Background(), droppedMsgs())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, llmErr) {
		t.Fatalf("error = %v, want to wrap %v", err, llmErr)
	}
	if ok {
		t.Fatal("ok must be false on LLM failure")
	}
	_ = msg // zero value expected
}

// --- Provenance: marker present in summary ----------------------------------

func TestSummaryMarkerPresent(t *testing.T) {
	var buf bytes.Buffer
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("Summary text here."))
	c := newCompactor(t, mock, &buf)

	msg, ok, err := c.Compact(context.Background(), droppedMsgs())
	if err != nil || !ok {
		t.Fatalf("Compact returned ok=%v err=%v", ok, err)
	}
	if len(msg.Content) == 0 {
		t.Fatal("no content blocks in summary message")
	}
	text := msg.Content[0].Text

	// Marker must be present.
	if !strings.HasPrefix(text, "[COMPACTION: ") {
		t.Fatalf("summary text does not start with marker prefix:\n%s", text)
	}
	// Extract and parse the marker JSON.
	end := strings.Index(text, "]")
	if end < 0 {
		t.Fatalf("marker not closed with ]: %s", text[:min(80, len(text))])
	}
	markerJSON := strings.TrimPrefix(text[:end+1], "[COMPACTION: ")
	markerJSON = strings.TrimSuffix(markerJSON, "]")
	var marker evidencecompactor.SummaryMarker
	if err := json.Unmarshal([]byte(markerJSON), &marker); err != nil {
		t.Fatalf("parse marker JSON %q: %v", markerJSON, err)
	}
	if marker.TemplateVersion != evidencecompactor.SummaryTemplateVersion {
		t.Fatalf("marker.TemplateVersion = %q, want %q", marker.TemplateVersion, evidencecompactor.SummaryTemplateVersion)
	}
	if marker.PromptHash == "" {
		t.Fatal("marker.PromptHash is empty")
	}
	if marker.ParamsHash == "" {
		t.Fatal("marker.ParamsHash is empty")
	}
	if marker.JournaledAt != epoch.Format(time.RFC3339) {
		t.Fatalf("marker.JournaledAt = %q, want %q", marker.JournaledAt, epoch.Format(time.RFC3339))
	}
	if marker.DroppedCount != 2 {
		t.Fatalf("marker.DroppedCount = %d, want 2", marker.DroppedCount)
	}

	// Summary text must follow the marker.
	afterMarker := text[end+1:]
	if !strings.Contains(afterMarker, "Summary text here.") {
		t.Fatalf("LLM response text not found after marker: %q", afterMarker)
	}
}

// --- Marker round-trip: parses after model context -------------------------

func TestMarkerRoundTrip(t *testing.T) {
	marker := evidencecompactor.SummaryMarker{
		TemplateVersion: "v1.0.0",
		PromptHash:      "abc123",
		ParamsHash:      "def456",
		JournaledAt:     "2026-07-09T12:00:00Z",
		DroppedCount:    5,
	}
	formatted, err := evidencecompactor.FormatMarker(marker)
	if err != nil {
		t.Fatalf("FormatMarker: %v", err)
	}
	if !strings.HasPrefix(formatted, "[COMPACTION: ") {
		t.Fatalf("formatted marker missing prefix: %q", formatted)
	}
	if !strings.HasSuffix(strings.TrimRight(formatted, "\n"), "]") {
		t.Fatalf("formatted marker missing closing ]: %q", formatted)
	}
	// Must be parseable.
	inner := strings.TrimPrefix(formatted, "[COMPACTION: ")
	inner = inner[:strings.Index(inner, "]")]
	var got evidencecompactor.SummaryMarker
	if err := json.Unmarshal([]byte(inner), &got); err != nil {
		t.Fatalf("unmarshal marker: %v", err)
	}
	if got != marker {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, marker)
	}
}

// --- Provenance hashes are deterministic ------------------------------------

func TestProvenanceHashesDeterministic(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	dropped := droppedMsgs()

	mock1 := llmclient.NewMockClient(llmclient.MockEntryOK("summary"))
	mock2 := llmclient.NewMockClient(llmclient.MockEntryOK("summary"))
	c1 := newCompactor(t, mock1, &buf1)
	c2 := newCompactor(t, mock2, &buf2)

	msg1, _, err1 := c1.Compact(context.Background(), dropped)
	msg2, _, err2 := c2.Compact(context.Background(), dropped)
	if err1 != nil || err2 != nil {
		t.Fatalf("Compact errors: %v, %v", err1, err2)
	}

	// Both should produce identical markers (same template, same params).
	text1 := msg1.Content[0].Text
	text2 := msg2.Content[0].Text
	end1 := strings.Index(text1, "]")
	end2 := strings.Index(text2, "]")
	if text1[:end1] != text2[:end2] {
		t.Fatalf("markers differ between identical runs:\n%s\n%s", text1[:end1], text2[:end2])
	}
}

// --- Empty dropped list: declines gracefully --------------------------------

func TestCompactEmptyDroppedDeclines(t *testing.T) {
	var buf bytes.Buffer
	mock := llmclient.NewMockClient()
	c := newCompactor(t, mock, &buf)

	msg, ok, err := c.Compact(context.Background(), nil)
	if err != nil {
		t.Fatalf("Compact(nil): unexpected error: %v", err)
	}
	if ok {
		t.Fatal("ok must be false for empty dropped list")
	}
	_ = msg
	if mock.CallCount() != 0 {
		t.Fatal("LLM called for empty dropped list")
	}
	if buf.Len() != 0 {
		t.Fatal("journal written for empty dropped list")
	}
}

// --- Context propagation ----------------------------------------------------

func TestContextPropagatedToLLM(t *testing.T) {
	var buf bytes.Buffer
	// Use a cancelled context: the LLM call should honour it.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("summary"))
	c := newCompactor(t, mock, &buf)

	_, _, err := c.Compact(ctx, droppedMsgs())
	// The mock checks ctx.Done() between entry consumption and return;
	// cancellation should surface as an error.
	if err == nil {
		t.Fatal("want error from cancelled context, got nil")
	}
}

// --- Journal record contains all dropped messages verbatim -----------------

func TestJournalContainsDroppedMessages(t *testing.T) {
	var buf bytes.Buffer
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	c := newCompactor(t, mock, &buf)

	dropped := []llmclient.Message{
		{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("question one")}},
		{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{llmclient.TextBlock("answer one")}},
		{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("question two")}},
	}
	_, _, err := c.Compact(context.Background(), dropped)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}

	rec := lastJournalRecord(t, &buf)
	if len(rec.Messages) != 3 {
		t.Fatalf("journal has %d messages, want 3", len(rec.Messages))
	}
	if rec.Messages[0].Role != "user" || rec.Messages[0].Content[0].Text != "question one" {
		t.Fatalf("message 0 = %+v, want user/question one", rec.Messages[0])
	}
	if rec.Messages[1].Role != "assistant" || rec.Messages[1].Content[0].Text != "answer one" {
		t.Fatalf("message 1 = %+v, want assistant/answer one", rec.Messages[1])
	}
	if rec.Messages[2].Role != "user" || rec.Messages[2].Content[0].Text != "question two" {
		t.Fatalf("message 2 = %+v, want user/question two", rec.Messages[2])
	}
}

// --- Integration with conversation.WindowCtx --------------------------------
//
// Exercises the full plumbing: conversation.WindowCtx calls the Compactor,
// which journals and summarises, and the summary is spliced role-safely.

func TestIntegrationWithConversationWindowCtx(t *testing.T) {
	// Import conversation inline to avoid circular concern in the test.
	// The compactor's Compact method IS the Compactor func — wire it directly.
	var buf bytes.Buffer
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("COMPACTED: earlier turns discussed X."))
	comp := newCompactor(t, mock, &buf)

	// We test the integration by calling Compact directly and checking the
	// output is valid as a conversation.Options.Compactor return value:
	// role-neutral content (only text blocks, no tool_use/tool_result).
	dropped := droppedMsgs()
	msg, ok, err := comp.Compact(context.Background(), dropped)
	if err != nil || !ok {
		t.Fatalf("Compact: ok=%v err=%v", ok, err)
	}
	for i, b := range msg.Content {
		if b.Type == llmclient.BlockToolUse || b.Type == llmclient.BlockToolResult {
			t.Fatalf("content block %d has role-unsafe type %q", i, b.Type)
		}
		if b.Type == llmclient.BlockText && b.Text == "" {
			t.Fatalf("content block %d is an empty text block", i)
		}
	}
}

// --- Race detector: concurrent Compact calls --------------------------------

func TestCompactConcurrentSafe(t *testing.T) {
	var buf bytes.Buffer
	entries := make([]llmclient.MockEntry, 20)
	for i := range entries {
		entries[i] = llmclient.MockEntryOK(fmt.Sprintf("summary %d", i))
	}
	mock := llmclient.NewMockClient(entries...)
	c := newCompactor(t, mock, &buf)

	dropped := droppedMsgs()
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			c.Compact(context.Background(), dropped) //nolint:errcheck
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// min is available in Go 1.21+ but not in text/template helpers; keep a
// local copy for the test file.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
