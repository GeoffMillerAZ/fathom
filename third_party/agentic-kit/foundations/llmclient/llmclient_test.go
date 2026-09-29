package llmclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// ---------------------------------------------------------------------------
// Ladder tests
// ---------------------------------------------------------------------------

func TestLadder_StartsAtRungZero(t *testing.T) {
	l, err := llmclient.NewLadder(
		llmclient.Rung{Model: "haiku", ContextLimit: 200_000, CostWeight: 0.25, QualityThreshold: 0.7},
		llmclient.Rung{Model: "sonnet", ContextLimit: 200_000, CostWeight: 1.0, QualityThreshold: 0.85},
		llmclient.Rung{Model: "opus", ContextLimit: 200_000, CostWeight: 3.0, QualityThreshold: 0.95},
	)
	if err != nil {
		t.Fatalf("NewLadder: %v", err)
	}
	if got := l.Current().Model; got != "haiku" {
		t.Errorf("expected rung 0 model haiku, got %q", got)
	}
	if got := l.CurrentIndex(); got != 0 {
		t.Errorf("expected index 0, got %d", got)
	}
}

func TestLadder_ClimbAdvancesOnEscalate(t *testing.T) {
	l, _ := llmclient.NewLadder(
		llmclient.Rung{Model: "haiku"},
		llmclient.Rung{Model: "sonnet"},
		llmclient.Rung{Model: "opus"},
	)

	// No escalation — stays at rung 0.
	rung, advanced := l.Climb(llmclient.Signal{Escalate: false})
	if advanced {
		t.Error("expected no advance when Escalate=false")
	}
	if rung.Model != "haiku" {
		t.Errorf("expected haiku, got %q", rung.Model)
	}
	if l.CurrentIndex() != 0 {
		t.Errorf("index should still be 0, got %d", l.CurrentIndex())
	}

	// Escalate → rung 1.
	rung, advanced = l.Climb(llmclient.Signal{Escalate: true, Reason: "quality too low"})
	if !advanced {
		t.Error("expected advance when Escalate=true")
	}
	if rung.Model != "sonnet" {
		t.Errorf("expected sonnet, got %q", rung.Model)
	}
	if l.CurrentIndex() != 1 {
		t.Errorf("expected index 1, got %d", l.CurrentIndex())
	}

	// Escalate again → rung 2.
	rung, advanced = l.Climb(llmclient.Signal{Escalate: true})
	if !advanced {
		t.Error("expected advance to rung 2")
	}
	if rung.Model != "opus" {
		t.Errorf("expected opus, got %q", rung.Model)
	}
}

func TestLadder_StopsAtTop(t *testing.T) {
	l, _ := llmclient.NewLadder(
		llmclient.Rung{Model: "haiku"},
		llmclient.Rung{Model: "sonnet"},
	)

	// Climb to top.
	l.Climb(llmclient.Signal{Escalate: true})

	// Attempt to go beyond top.
	rung, advanced := l.Climb(llmclient.Signal{Escalate: true})
	if advanced {
		t.Error("should not advance beyond top rung")
	}
	if rung.Model != "sonnet" {
		t.Errorf("expected sonnet at top, got %q", rung.Model)
	}
	if l.CurrentIndex() != 1 {
		t.Errorf("index should stay at 1, got %d", l.CurrentIndex())
	}
}

func TestLadder_SingleRung(t *testing.T) {
	l, _ := llmclient.NewLadder(llmclient.Rung{Model: "only"})
	rung, advanced := l.Climb(llmclient.Signal{Escalate: true})
	if advanced {
		t.Error("single-rung ladder must never advance")
	}
	if rung.Model != "only" {
		t.Errorf("expected 'only', got %q", rung.Model)
	}
}

func TestLadder_RequiresAtLeastOneRung(t *testing.T) {
	_, err := llmclient.NewLadder()
	if err == nil {
		t.Error("expected error for empty rung list")
	}
}

func TestLadder_Reset(t *testing.T) {
	l, _ := llmclient.NewLadder(
		llmclient.Rung{Model: "haiku"},
		llmclient.Rung{Model: "sonnet"},
	)
	l.Climb(llmclient.Signal{Escalate: true})
	l.Reset()
	if l.CurrentIndex() != 0 {
		t.Errorf("after Reset, expected index 0, got %d", l.CurrentIndex())
	}
}

// ---------------------------------------------------------------------------
// MockClient tests
// ---------------------------------------------------------------------------

func TestMockClient_ScriptedResponses(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryOK("first"),
		llmclient.MockEntryOK("second"),
	)

	ctx := context.Background()

	r1, err := mc.Complete(ctx, llmclient.Request{Prompt: "p1"})
	if err != nil || r1.Text != "first" {
		t.Errorf("unexpected r1: %v / %v", r1, err)
	}

	r2, err := mc.Complete(ctx, llmclient.Request{Prompt: "p2"})
	if err != nil || r2.Text != "second" {
		t.Errorf("unexpected r2: %v / %v", r2, err)
	}
}

func TestMockClient_ScriptedError(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryErr(&llmclient.RateLimitError{Message: "slow down"}),
	)
	_, err := mc.Complete(context.Background(), llmclient.Request{})
	var rle *llmclient.RateLimitError
	if !errors.As(err, &rle) {
		t.Errorf("expected RateLimitError, got %T: %v", err, err)
	}
}

func TestMockClient_FallbackWhenExhausted(t *testing.T) {
	mc := llmclient.NewMockClient(llmclient.MockEntryOK("only"))
	mc.Complete(context.Background(), llmclient.Request{}) // consume only entry

	r, err := mc.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Errorf("fallback should not error, got %v", err)
	}
	if r.Text != "mock: no more entries" {
		t.Errorf("unexpected fallback text: %q", r.Text)
	}
}

func TestMockClient_CallCount(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryOK("a"),
		llmclient.MockEntryOK("b"),
	)
	mc.Complete(context.Background(), llmclient.Request{})
	mc.Complete(context.Background(), llmclient.Request{})
	if mc.CallCount() != 2 {
		t.Errorf("expected 2 calls, got %d", mc.CallCount())
	}
}

func TestMockClient_Stream(t *testing.T) {
	mc := llmclient.NewMockClient(llmclient.MockEntryOK("streamed text"))
	ch, err := mc.Stream(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	var got string
	for chunk := range ch {
		got += chunk.TextDelta
	}
	if got != "streamed text" {
		t.Errorf("unexpected stream content: %q", got)
	}
}

// ---------------------------------------------------------------------------
// WithRetry tests
// ---------------------------------------------------------------------------

func deterministicRetryConfig(maxAttempts int, baseDelay time.Duration) llmclient.RetryConfig {
	// Fixed seed → deterministic jitter in tests.
	src := rand.New(rand.NewSource(42))
	return llmclient.RetryConfig{
		MaxAttempts:  maxAttempts,
		BaseDelay:    baseDelay,
		Jitter:       true,
		JitterSource: src,
	}
}

func TestWithRetry_SucceedsOnFirstAttempt(t *testing.T) {
	mc := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmclient.WithRetry(mc, deterministicRetryConfig(3, 0))

	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "ok" {
		t.Errorf("unexpected text: %q", resp.Text)
	}
	if resp.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", resp.Attempts)
	}
}

func TestWithRetry_RetriesRateLimitAndSucceeds(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryRateLimit("too fast"),
		llmclient.MockEntryRateLimit("still too fast"),
		llmclient.MockEntryOK("finally"),
	)
	client := llmclient.WithRetry(mc, deterministicRetryConfig(5, 0))

	resp, err := client.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if resp.Text != "finally" {
		t.Errorf("unexpected text: %q", resp.Text)
	}
	if resp.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", resp.Attempts)
	}
}

func TestWithRetry_FailsFastOnInvalidRequest(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryInvalid("bad prompt"),
		llmclient.MockEntryOK("should not reach this"),
	)
	client := llmclient.WithRetry(mc, deterministicRetryConfig(5, 0))

	resp, err := client.Complete(context.Background(), llmclient.Request{})
	if err == nil {
		t.Fatal("expected error for invalid request")
	}
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Errorf("expected InvalidRequestError, got %T: %v", err, err)
	}
	// Only one attempt should have been made (fail fast, no retry).
	if resp.Attempts != 1 {
		t.Errorf("expected 1 attempt (fail fast), got %d", resp.Attempts)
	}
	if mc.CallCount() != 1 {
		t.Errorf("expected 1 call to mock, got %d", mc.CallCount())
	}
}

func TestWithRetry_ExhaustsMaxAttempts(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryRateLimit("slow"),
		llmclient.MockEntryRateLimit("slow"),
		llmclient.MockEntryRateLimit("slow"),
	)
	client := llmclient.WithRetry(mc, deterministicRetryConfig(3, 0))

	resp, err := client.Complete(context.Background(), llmclient.Request{})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	var rle *llmclient.RateLimitError
	if !errors.As(err, &rle) {
		t.Errorf("expected RateLimitError, got %T", err)
	}
	if resp.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", resp.Attempts)
	}
}

func TestWithRetry_AttemptsReflectedCorrectly(t *testing.T) {
	// Two rate-limit errors, then success → Attempts == 3.
	mc := llmclient.NewMockClient(
		llmclient.MockEntryRateLimit("a"),
		llmclient.MockEntryRateLimit("b"),
		llmclient.MockEntryOK("done"),
	)
	client := llmclient.WithRetry(mc, deterministicRetryConfig(10, 0))

	resp, err := client.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", resp.Attempts)
	}
}

func TestWithRetry_ContextCancellationHonoured(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryRateLimit("slow"),
	)
	// Use a non-zero BaseDelay so the retry actually waits.
	src := rand.New(rand.NewSource(99))
	cfg := llmclient.RetryConfig{
		MaxAttempts:  5,
		BaseDelay:    10 * time.Second,
		Jitter:       false,
		JitterSource: src,
	}
	client := llmclient.WithRetry(mc, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Complete(ctx, llmclient.Request{})
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestWithRetry_StreamRetriesRateLimitAndSucceeds(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryRateLimit("too fast"),
		llmclient.MockEntryOK("streamed"),
	)
	client := llmclient.WithRetry(mc, deterministicRetryConfig(5, 0))

	ch, err := client.Stream(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	var got string
	for chunk := range ch {
		got += chunk.TextDelta
	}
	if got != "streamed" {
		t.Errorf("unexpected stream text: %q", got)
	}
}

// ---------------------------------------------------------------------------
// Error type tests
// ---------------------------------------------------------------------------

func TestErrorTypes(t *testing.T) {
	rle := &llmclient.RateLimitError{Message: "slow down", RetryAfter: 5 * time.Second}
	if rle.Error() == "" {
		t.Error("RateLimitError.Error() should not be empty")
	}
	if !strings.Contains(rle.Error(), "5s") {
		t.Errorf("RateLimitError.Error() should mention the hint, got %q", rle.Error())
	}
	noHint := &llmclient.RateLimitError{Message: "slow down"}
	if strings.Contains(noHint.Error(), "retry after") {
		t.Errorf("zero RetryAfter must not print a hint, got %q", noHint.Error())
	}

	svrErr := &llmclient.ServerError{StatusCode: 503, Message: "unavailable"}
	if svrErr.Error() == "" {
		t.Error("ServerError.Error() should not be empty")
	}

	invErr := &llmclient.InvalidRequestError{Message: "bad param"}
	if invErr.Error() == "" {
		t.Error("InvalidRequestError.Error() should not be empty")
	}

	authErr := &llmclient.AuthError{Message: "forbidden"}
	if authErr.Error() == "" {
		t.Error("AuthError.Error() should not be empty")
	}
}

// ---------------------------------------------------------------------------
// v2 contract tests: content blocks, tool use, streaming, retry hints, ladder
// ---------------------------------------------------------------------------

// drainStream collects all chunks, failing the test if the channel does not
// close within the guard timeout.
func drainStream(t *testing.T, ch <-chan llmclient.StreamChunk) []llmclient.StreamChunk {
	t.Helper()
	var chunks []llmclient.StreamChunk
	guard := time.After(5 * time.Second)
	for {
		select {
		case c, ok := <-ch:
			if !ok {
				return chunks
			}
			chunks = append(chunks, c)
		case <-guard:
			t.Fatalf("stream did not close within guard timeout (%d chunks so far)", len(chunks))
		}
	}
}

func TestContentBlockConstructors(t *testing.T) {
	tests := []struct {
		name string
		got  llmclient.ContentBlock
		want llmclient.ContentBlock
	}{
		{
			name: "text",
			got:  llmclient.TextBlock("hello"),
			want: llmclient.ContentBlock{Type: llmclient.BlockText, Text: "hello"},
		},
		{
			name: "image",
			got:  llmclient.ImageBlock("image/png", "aGVsbG8="),
			want: llmclient.ContentBlock{Type: llmclient.BlockImage, MediaType: "image/png", Data: "aGVsbG8="},
		},
		{
			name: "tool_use",
			got:  llmclient.ToolUseBlock("tu_1", "search", json.RawMessage(`{"q":"x"}`)),
			want: llmclient.ContentBlock{Type: llmclient.BlockToolUse, ID: "tu_1", Name: "search", Input: json.RawMessage(`{"q":"x"}`)},
		},
		{
			name: "tool_result",
			got:  llmclient.ToolResultBlock("tu_1", "42 results", true),
			want: llmclient.ContentBlock{Type: llmclient.BlockToolResult, ToolUseID: "tu_1", Content: "42 results", IsError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Errorf("got %+v, want %+v", tt.got, tt.want)
			}
		})
	}
}

func TestResponse_ToolCallsAndAsMessage(t *testing.T) {
	unknown := llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"..."}`)}
	resp := llmclient.Response{
		Text: "let me look that up",
		Content: []llmclient.ContentBlock{
			unknown,
			llmclient.TextBlock("let me look that up"),
			llmclient.ToolUseBlock("tu_1", "search", json.RawMessage(`{"q":"go"}`)),
			llmclient.ToolUseBlock("tu_2", "fetch", json.RawMessage(`{"url":"u"}`)),
		},
		StopReason: llmclient.StopToolUse,
	}

	calls := resp.ToolCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].ID != "tu_1" || calls[0].Name != "search" {
		t.Errorf("unexpected first call: %+v", calls[0])
	}
	if calls[1].ID != "tu_2" || calls[1].Name != "fetch" {
		t.Errorf("unexpected second call: %+v", calls[1])
	}

	msg := resp.AsMessage()
	if msg.Role != llmclient.RoleAssistant {
		t.Errorf("AsMessage role = %q, want %q", msg.Role, llmclient.RoleAssistant)
	}
	// Verbatim round-trip, including the unknown block.
	if !reflect.DeepEqual(msg.Content, resp.Content) {
		t.Errorf("AsMessage content not verbatim:\ngot  %+v\nwant %+v", msg.Content, resp.Content)
	}
	// Appending to the message must not mutate the response's content slice.
	msg.Content = append(msg.Content, llmclient.TextBlock("extra"))
	if len(resp.Content) != 4 {
		t.Errorf("response content mutated by append: %d blocks", len(resp.Content))
	}
}

func TestResponse_AsMessageEmptyContent(t *testing.T) {
	msg := llmclient.Response{}.AsMessage()
	if msg.Role != llmclient.RoleAssistant {
		t.Errorf("role = %q, want assistant", msg.Role)
	}
	if msg.Content != nil {
		t.Errorf("empty response must yield nil content, got %v", msg.Content)
	}
}

func TestMockClient_ToolUseEntry(t *testing.T) {
	input := json.RawMessage(`{"q":"weather"}`)
	mc := llmclient.NewMockClient(
		llmclient.MockEntryToolUse("tu_9", "lookup", input),
		llmclient.MockEntryOK("final answer"),
	)

	resp, err := mc.Complete(context.Background(), llmclient.Request{Prompt: "?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StopReason != llmclient.StopToolUse {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, llmclient.StopToolUse)
	}
	calls := resp.ToolCalls()
	if len(calls) != 1 || calls[0].ID != "tu_9" || calls[0].Name != "lookup" {
		t.Fatalf("unexpected tool calls: %+v", calls)
	}
	if string(calls[0].Input) != string(input) {
		t.Errorf("Input = %s, want %s", calls[0].Input, input)
	}

	// Multi-turn sequence: the next entry is the final text turn.
	resp2, err := mc.Complete(context.Background(), llmclient.Request{
		Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("?")}},
			resp.AsMessage(),
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.ToolResultBlock("tu_9", "sunny", false)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2.Text != "final answer" || resp2.StopReason != llmclient.StopEndTurn {
		t.Errorf("unexpected final turn: %+v", resp2)
	}
}

func TestMockClient_NormalizesScriptedResponses(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryOK("plain text"),
		llmclient.MockEntryWithResponse(llmclient.Response{
			Content: []llmclient.ContentBlock{llmclient.TextBlock("from "), llmclient.TextBlock("blocks")},
		}),
	)

	r1, err := mc.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r1.Content) != 1 || r1.Content[0].Type != llmclient.BlockText || r1.Content[0].Text != "plain text" {
		t.Errorf("Content not derived from Text: %+v", r1.Content)
	}
	if r1.StopReason != llmclient.StopEndTurn {
		t.Errorf("StopReason = %q, want end_turn", r1.StopReason)
	}

	r2, err := mc.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r2.Text != "from blocks" {
		t.Errorf("Text not derived from Content: %q", r2.Text)
	}
}

func TestMockClient_StreamEmitsDeltasAndTerminal(t *testing.T) {
	mc := llmclient.NewMockClient(llmclient.MockEntryWithResponse(llmclient.Response{
		Text:  "hello streaming world",
		Usage: llmclient.Usage{InputTokens: 7, OutputTokens: 3},
	}))

	ch, err := mc.Stream(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunks := drainStream(t, ch)
	if len(chunks) < 3 {
		t.Fatalf("expected >=2 deltas + terminal, got %d chunks", len(chunks))
	}

	var text string
	for _, c := range chunks[:len(chunks)-1] {
		if c.TextDelta == "" {
			t.Errorf("non-terminal chunk without TextDelta: %+v", c)
		}
		text += c.TextDelta
	}
	if text != "hello streaming world" {
		t.Errorf("concatenated deltas = %q", text)
	}

	last := chunks[len(chunks)-1]
	if last.Err != nil {
		t.Fatalf("terminal chunk has Err: %v", last.Err)
	}
	if last.StopReason != llmclient.StopEndTurn {
		t.Errorf("terminal StopReason = %q, want end_turn", last.StopReason)
	}
	if last.Usage == nil {
		t.Fatal("terminal chunk must carry non-nil Usage")
	}
	if last.Usage.InputTokens != 7 || last.Usage.OutputTokens != 3 {
		t.Errorf("terminal usage = %+v", *last.Usage)
	}
}

func TestMockClient_StreamToolUseBlockComplete(t *testing.T) {
	mc := llmclient.NewMockClient(llmclient.MockEntryToolUse("tu_1", "search", json.RawMessage(`{"q":"x"}`)))

	ch, err := mc.Stream(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunks := drainStream(t, ch)
	if len(chunks) != 2 {
		t.Fatalf("expected block + terminal, got %d chunks: %+v", len(chunks), chunks)
	}
	b := chunks[0].Block
	if b == nil || b.Type != llmclient.BlockToolUse || b.ID != "tu_1" || b.Name != "search" {
		t.Fatalf("unexpected block chunk: %+v", chunks[0])
	}
	if string(b.Input) != `{"q":"x"}` {
		t.Errorf("tool input not delivered whole: %s", b.Input)
	}
	if chunks[1].StopReason != llmclient.StopToolUse || chunks[1].Usage == nil {
		t.Errorf("unexpected terminal chunk: %+v", chunks[1])
	}
}

func TestMockClient_StreamMidStreamErr(t *testing.T) {
	boom := errors.New("connection reset")
	mc := llmclient.NewMockClient(llmclient.MockEntryStreamErr("partial out", boom))

	ch, err := mc.Stream(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("expected mid-stream error, not pre-stream: %v", err)
	}
	chunks := drainStream(t, ch)
	if len(chunks) < 2 {
		t.Fatalf("expected deltas then Err chunk, got %d chunks", len(chunks))
	}
	last := chunks[len(chunks)-1]
	if !errors.Is(last.Err, boom) {
		t.Errorf("terminal Err = %v, want %v", last.Err, boom)
	}
	for _, c := range chunks[:len(chunks)-1] {
		if c.Err != nil {
			t.Errorf("Err chunk must be terminal, found early: %+v", c)
		}
	}
}

func TestMockClient_StreamContextCancellation(t *testing.T) {
	mc := llmclient.NewMockClient(llmclient.MockEntryOK("a fairly long streamed answer"))

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := mc.Stream(ctx, llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read one chunk, then cancel and stop consuming.
	guard := time.After(5 * time.Second)
	select {
	case <-ch:
	case <-guard:
		t.Fatal("no first chunk within guard timeout")
	}
	cancel()

	// The channel must close promptly — the producer must not leak.
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-guard:
			t.Fatal("stream did not close after cancellation")
		}
	}
}

func TestMockClient_CompleteWithStreamErrFails(t *testing.T) {
	boom := errors.New("mid-stream boom")
	mc := llmclient.NewMockClient(llmclient.MockEntryStreamErr("partial", boom))
	_, err := mc.Complete(context.Background(), llmclient.Request{})
	if !errors.Is(err, boom) {
		t.Errorf("Complete err = %v, want %v", err, boom)
	}
}

func TestWithRetry_ToolUseIsSuccessNotRetried(t *testing.T) {
	mc := llmclient.NewMockClient(
		llmclient.MockEntryToolUse("tu_1", "search", json.RawMessage(`{}`)),
		llmclient.MockEntryOK("should not be consumed"),
	)
	client := llmclient.WithRetry(mc, deterministicRetryConfig(5, 0))

	resp, err := client.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("tool_use must be success, got err: %v", err)
	}
	if resp.StopReason != llmclient.StopToolUse {
		t.Errorf("StopReason = %q, want tool_use", resp.StopReason)
	}
	if resp.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1 (no retry)", resp.Attempts)
	}
	if mc.CallCount() != 1 {
		t.Errorf("CallCount = %d, want 1", mc.CallCount())
	}
}

func TestWithRetry_PrefersRetryAfterHint(t *testing.T) {
	// BaseDelay is far beyond the context deadline; the only way this test
	// passes is if WithRetry uses the 1ms RetryAfter hint instead.
	mc := llmclient.NewMockClient(
		llmclient.MockEntryErr(&llmclient.RateLimitError{Message: "hinted", RetryAfter: time.Millisecond}),
		llmclient.MockEntryOK("recovered"),
	)
	client := llmclient.WithRetry(mc, llmclient.RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   time.Hour,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Complete(ctx, llmclient.Request{})
	if err != nil {
		t.Fatalf("expected hint-paced retry to succeed, got %v", err)
	}
	if resp.Text != "recovered" || resp.Attempts != 2 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestRung_Apply(t *testing.T) {
	rung := llmclient.Rung{Model: "haiku"}

	injected := rung.Apply(llmclient.Request{Prompt: "hi"})
	if injected.Model != "haiku" {
		t.Errorf("empty Model not injected: %q", injected.Model)
	}

	kept := rung.Apply(llmclient.Request{Prompt: "hi", Model: "opus"})
	if kept.Model != "opus" {
		t.Errorf("explicit Model overridden: %q", kept.Model)
	}
}

func TestRequest_PromptBackCompatRecorded(t *testing.T) {
	// v1 callers keep using Prompt/System; the mock records the request
	// unchanged so consumers (e.g. DLP-gate tests) can assert on it.
	mc := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	_, err := mc.Complete(context.Background(), llmclient.Request{Prompt: "p", System: "s", MaxTokens: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := mc.Calls()
	if len(calls) != 1 || calls[0].Request.Prompt != "p" || calls[0].Request.System != "s" {
		t.Errorf("recorded request mismatch: %+v", calls)
	}
}

// mkBreakpointRequest builds a request carrying the given mix of cache
// breakpoints, mirroring the shape the anthropic adapter validates.
func mkBreakpointRequest(systemHint bool, toolHints, blockHints int) llmclient.Request {
	req := llmclient.Request{
		System:          "sys",
		SystemCacheHint: systemHint,
		Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("hi")}},
		},
	}
	for i := 0; i < toolHints; i++ {
		req.Tools = append(req.Tools, llmclient.ToolDef{
			Name:        "t",
			InputSchema: json.RawMessage(`{"type":"object"}`),
			CacheHint:   true,
		})
	}
	for i := 0; i < blockHints; i++ {
		req.Messages[0].Content = append(req.Messages[0].Content,
			llmclient.ContentBlock{Type: llmclient.BlockText, Text: "b", CacheHint: true})
	}
	return req
}

// TestMockClient_RejectsContractLimits pins the mock→real seam: MockClient
// rejects exactly the requests the anthropic adapter rejects before any HTTP —
// more than MaxCacheBreakpoints total cache breakpoints (counted the same
// way) and ToolChoiceTool with an empty Name — with a typed
// InvalidRequestError, on both Complete and Stream.
func TestMockClient_RejectsContractLimits(t *testing.T) {
	cases := []struct {
		name    string
		req     llmclient.Request
		wantErr bool
	}{
		{"FiveBreakpointsRejected", mkBreakpointRequest(true, 2, 2), true}, // 1+2+2 = 5
		{"FourBreakpointsAccepted", mkBreakpointRequest(true, 2, 1), false},
		{
			name: "UnknownBlockHintNotCounted",
			req: func() llmclient.Request {
				req := mkBreakpointRequest(true, 2, 1) // 4 counted hints...
				req.Messages[0].Content = append(req.Messages[0].Content, llmclient.ContentBlock{
					Type:      llmclient.BlockUnknown,
					Raw:       json.RawMessage(`{"type":"thinking","thinking":"x"}`),
					CacheHint: true,
				}) // ...plus an ignored one on an unknown block
				return req
			}(),
			wantErr: false,
		},
		{
			name: "EmptySystemHintNotCounted",
			req: func() llmclient.Request {
				req := mkBreakpointRequest(true, 2, 2) // would be 5...
				req.System = ""                        // ...but the system hint no longer counts
				return req
			}(),
			wantErr: false,
		},
		{
			name:    "ToolChoiceToolWithoutNameRejected",
			req:     llmclient.Request{Prompt: "p", ToolChoice: llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool}},
			wantErr: true,
		},
		{
			name:    "ToolChoiceToolWithNameAccepted",
			req:     llmclient.Request{Prompt: "p", ToolChoice: llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool, Name: "search"}},
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Complete", func(t *testing.T) {
				mc := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
				_, err := mc.Complete(context.Background(), tc.req)
				var ire *llmclient.InvalidRequestError
				if tc.wantErr && !errors.As(err, &ire) {
					t.Fatalf("err = %T %v, want *llmclient.InvalidRequestError", err, err)
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("Complete: %v", err)
				}
			})
			t.Run("Stream", func(t *testing.T) {
				mc := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
				ch, err := mc.Stream(context.Background(), tc.req)
				var ire *llmclient.InvalidRequestError
				if tc.wantErr {
					if ch != nil {
						t.Error("rejected request must fail pre-stream, no channel")
					}
					if !errors.As(err, &ire) {
						t.Fatalf("err = %T %v, want *llmclient.InvalidRequestError", err, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("Stream: %v", err)
				}
				for range ch {
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// ProviderParams, Thinking, Effort — new Wave 1 contract fields
// ---------------------------------------------------------------------------

// TestValidateProviderParams exercises the exported helper used by all adapters.
func TestValidateProviderParams(t *testing.T) {
	cases := []struct {
		name    string
		pp      json.RawMessage
		wantErr bool
	}{
		{"nil is no-op", nil, false},
		{"empty is no-op", json.RawMessage(""), false},
		{"valid object", json.RawMessage(`{"x":1}`), false},
		{"empty object", json.RawMessage(`{}`), false},
		{"array rejected", json.RawMessage(`[1,2]`), true},
		{"string rejected", json.RawMessage(`"hello"`), true},
		{"number rejected", json.RawMessage(`42`), true},
		{"null rejected", json.RawMessage(`null`), true},
		{"invalid JSON rejected", json.RawMessage(`{bad`), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := llmclient.ValidateProviderParams(tc.pp, "test")
			var ire *llmclient.InvalidRequestError
			if tc.wantErr && !errors.As(err, &ire) {
				t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestMockClient_ProviderParamsValidation proves MockClient rejects non-object
// ProviderParams before consuming any entry, and accepts object/empty forms.
func TestMockClient_ProviderParamsValidation(t *testing.T) {
	cases := []struct {
		name    string
		pp      json.RawMessage
		wantErr bool
	}{
		{"nil accepted", nil, false},
		{"empty accepted", json.RawMessage(""), false},
		{"valid object accepted", json.RawMessage(`{"extra_field":"v"}`), false},
		{"array rejected", json.RawMessage(`[1,2]`), true},
		{"string rejected", json.RawMessage(`"hello"`), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mc := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
			beforeCount := mc.CallCount()
			_, err := mc.Complete(context.Background(), llmclient.Request{
				Prompt:         "p",
				ProviderParams: tc.pp,
			})
			var ire *llmclient.InvalidRequestError
			if tc.wantErr {
				if !errors.As(err, &ire) {
					t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
				}
				// Rejection must not consume an entry or record a call.
				if mc.CallCount() != beforeCount {
					t.Errorf("CallCount changed after rejection: %d -> %d", beforeCount, mc.CallCount())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// MockClient records provider-neutral effort without simulating its semantics.
func TestMockClient_EffortRecorded(t *testing.T) {
	for _, effort := range []string{"", "low", "medium", "high", "xhigh", "max", "provider-specific-future"} {
		t.Run(effort, func(t *testing.T) {
			mc := llmclient.NewMockClient(llmclient.MockEntryOK("ok"), llmclient.MockEntryOK("stream"))
			req := llmclient.Request{Prompt: "p", Effort: effort}
			if _, err := mc.Complete(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			ch, err := mc.Stream(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			for chunk := range ch {
				if chunk.Err != nil {
					t.Fatal(chunk.Err)
				}
			}
			if mc.CallCount() != 2 {
				t.Fatalf("CallCount = %d", mc.CallCount())
			}
			for _, call := range mc.Calls() {
				if call.Request.Effort != effort {
					t.Fatalf("effort lost: %+v", call.Request)
				}
			}
		})
	}
}

// TestMockClient_ThinkingPassesThrough proves that a Thinking field does NOT
// cause MockClient to reject the request — mock passes it through (it only
// validates ProviderParams shape).
func TestMockClient_ThinkingPassesThrough(t *testing.T) {
	mc := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	resp, err := mc.Complete(context.Background(), llmclient.Request{
		Prompt:   "p",
		Thinking: &llmclient.ThinkingConfig{Type: "enabled", BudgetTokens: 1024},
	})
	if err != nil {
		t.Fatalf("Thinking should pass through mock, got: %v", err)
	}
	if resp.Text != "ok" {
		t.Errorf("resp.Text = %q, want ok", resp.Text)
	}
}

// TestMockClient_RejectionConsumesNothing pins the documented bookkeeping: a
// rejected request consumes no scripted entry and records no call, so the
// next valid call still gets the first entry — exactly like the real client,
// which fails before any HTTP.
func TestMockClient_RejectionConsumesNothing(t *testing.T) {
	mc := llmclient.NewMockClient(llmclient.MockEntryOK("first"))

	if _, err := mc.Complete(context.Background(), mkBreakpointRequest(true, 2, 2)); err == nil {
		t.Fatal("want rejection for 5 breakpoints")
	}
	if got := mc.CallCount(); got != 0 {
		t.Errorf("CallCount after rejection = %d, want 0 (call not recorded)", got)
	}

	resp, err := mc.Complete(context.Background(), llmclient.Request{Prompt: "ok now"})
	if err != nil {
		t.Fatalf("valid Complete after rejection: %v", err)
	}
	if resp.Text != "first" {
		t.Errorf("Text = %q, want %q (rejection must not consume the scripted entry)", resp.Text, "first")
	}
}
