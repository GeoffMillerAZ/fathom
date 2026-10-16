package llmnoop_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llmnoop "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/noop"
)

// guardTimeout bounds every streaming check so a hung producer fails the test
// instead of hanging it. Matches the conformance suite's guard.
const guardTimeout = 5 * time.Second

// drain collects every chunk until the channel closes, failing the test if it
// does not close within guardTimeout (proves the producer terminates and
// closes exactly once).
func drain(t *testing.T, ch <-chan llmclient.StreamChunk) []llmclient.StreamChunk {
	t.Helper()
	var chunks []llmclient.StreamChunk
	guard := time.After(guardTimeout)
	for {
		select {
		case c, ok := <-ch:
			if !ok {
				return chunks
			}
			chunks = append(chunks, c)
		case <-guard:
			t.Fatalf("stream channel did not close within %v (%d chunks)", guardTimeout, len(chunks))
		}
	}
}

// TestComplete table-drives the canned-refusal Complete contract across the two
// request forms (v1 Prompt, v2 Messages) and a request with tools declared:
// none of them change the output — the client never looks at the request.
func TestComplete(t *testing.T) {
	cases := []struct {
		name string
		req  llmclient.Request
	}{
		{"empty request", llmclient.Request{}},
		{"v1 prompt", llmclient.Request{Prompt: "summarise this"}},
		{"v2 messages", llmclient.Request{Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("hi")}},
		}}},
		{"with tools declared", llmclient.Request{
			Prompt:     "use a tool",
			Tools:      []llmclient.ToolDef{{Name: "search", Description: "find"}},
			ToolChoice: llmclient.ToolChoice{Mode: llmclient.ToolChoiceAny},
		}},
		{"with system + max tokens", llmclient.Request{System: "be terse", MaxTokens: 7, Prompt: "go"}},
	}

	c := llmnoop.New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := c.Complete(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("Complete returned error, want nil (a no-op is a success): %v", err)
			}
			if resp.StopReason != llmclient.StopRefusal {
				t.Errorf("StopReason = %q, want %q", resp.StopReason, llmclient.StopRefusal)
			}
			if resp.Text != llmnoop.Message {
				t.Errorf("Text = %q, want %q", resp.Text, llmnoop.Message)
			}
			if len(resp.Content) != 1 {
				t.Fatalf("Content len = %d, want 1 (a single canned text block): %+v", len(resp.Content), resp.Content)
			}
			b := resp.Content[0]
			if b.Type != llmclient.BlockText {
				t.Errorf("Content[0].Type = %q, want %q", b.Type, llmclient.BlockText)
			}
			if b.Text != llmnoop.Message {
				t.Errorf("Content[0].Text = %q, want %q", b.Text, llmnoop.Message)
			}
			// Text must equal the concatenation of text blocks (the v1 invariant).
			var concat string
			for _, cb := range resp.Content {
				if cb.Type == llmclient.BlockText {
					concat += cb.Text
				}
			}
			if resp.Text != concat {
				t.Errorf("Text (%q) != concat of text blocks (%q)", resp.Text, concat)
			}
			// No tool calls are ever fabricated.
			if calls := resp.ToolCalls(); len(calls) != 0 {
				t.Errorf("ToolCalls len = %d, want 0", len(calls))
			}
		})
	}
}

// TestCompleteContextCancelled proves Complete honours an already-cancelled
// context rather than blindly returning the canned response.
func TestCompleteContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := llmnoop.New().Complete(ctx, llmclient.Request{Prompt: "x"})
	if err != context.Canceled {
		t.Fatalf("Complete err = %v, want context.Canceled", err)
	}
}

// TestStreamLifecycle proves the streaming contract: at least one text delta
// carrying the whole canned message, a terminal success chunk with StopRefusal
// and a non-nil Usage, no premature Err chunk, channel closed exactly once.
func TestStreamLifecycle(t *testing.T) {
	ch, err := llmnoop.New().Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream returned pre-stream error, want nil: %v", err)
	}
	if ch == nil {
		t.Fatal("Stream returned nil channel with nil error")
	}
	chunks := drain(t, ch)
	if len(chunks) == 0 {
		t.Fatal("stream produced no chunks")
	}

	var text string
	for i, c := range chunks[:len(chunks)-1] {
		if c.Err != nil {
			t.Fatalf("non-terminal chunk %d carries Err: %v", i, c.Err)
		}
		if c.StopReason != "" {
			t.Errorf("non-terminal chunk %d carries StopReason %q", i, c.StopReason)
		}
		text += c.TextDelta
		if c.Block != nil && c.Block.Type == llmclient.BlockText {
			text += c.Block.Text
		}
	}
	if text != llmnoop.Message {
		t.Errorf("streamed text = %q, want %q", text, llmnoop.Message)
	}

	last := chunks[len(chunks)-1]
	if last.Err != nil {
		t.Fatalf("terminal chunk carries Err on a clean stream: %v", last.Err)
	}
	if last.StopReason != llmclient.StopRefusal {
		t.Errorf("terminal StopReason = %q, want %q", last.StopReason, llmclient.StopRefusal)
	}
	if last.Usage == nil {
		t.Error("terminal chunk must carry non-nil Usage on success")
	}
}

// TestStreamCancellation proves the producer terminates and closes the channel
// promptly after the context is cancelled, with no goroutine leak. Drained
// lazily (one chunk, then cancel) like the conformance suite.
func TestStreamCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := llmnoop.New().Stream(ctx, llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	guard := time.After(guardTimeout)
	select {
	case _, ok := <-ch:
		if !ok {
			return // closed immediately; nothing to cancel
		}
	case <-guard:
		t.Fatal("no chunk received within guard timeout")
	}
	cancel()

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-guard:
			t.Fatal("stream channel did not close after cancellation (producer leak)")
		}
	}
}

// TestStreamCancelledBeforeRead proves a stream started on an already-cancelled
// context still closes (and does not hang), exercising the ctx.Done() branch of
// the very first send.
func TestStreamCancelledBeforeRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch, err := llmnoop.New().Stream(ctx, llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	// Must drain to completion within the guard window; an Err chunk carrying
	// ctx.Err() is acceptable (best-effort), as is an immediate close.
	chunks := drain(t, ch)
	for i, c := range chunks {
		if c.Err != nil && c.Err != context.Canceled {
			t.Errorf("chunk %d Err = %v, want context.Canceled or nil", i, c.Err)
		}
	}
}

// TestDeterminism proves repeated calls (Complete and Stream) produce byte-for-
// byte identical results — there is no randomness, no model, no clock-dependent
// content. A degrade-gracefully fallback must be reproducible.
func TestDeterminism(t *testing.T) {
	c := llmnoop.New()
	ctx := context.Background()

	first, err := c.Complete(ctx, llmclient.Request{Prompt: "a"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	for i := 0; i < 100; i++ {
		got, err := c.Complete(ctx, llmclient.Request{Prompt: "b"})
		if err != nil {
			t.Fatalf("Complete iter %d: %v", i, err)
		}
		if got.Text != first.Text || got.StopReason != first.StopReason || len(got.Content) != len(first.Content) {
			t.Fatalf("Complete iter %d differs: %+v vs %+v", i, got, first)
		}
	}

	// Stream determinism: same delta sequence and same terminal stop reason.
	var prev []string
	for i := 0; i < 25; i++ {
		ch, err := c.Stream(ctx, llmclient.Request{Prompt: "s"})
		if err != nil {
			t.Fatalf("Stream iter %d: %v", i, err)
		}
		var seq []string
		var stop llmclient.StopReason
		for _, ck := range drain(t, ch) {
			if ck.TextDelta != "" {
				seq = append(seq, ck.TextDelta)
			}
			if ck.StopReason != "" {
				stop = ck.StopReason
			}
		}
		if stop != llmclient.StopRefusal {
			t.Fatalf("Stream iter %d terminal stop = %q, want %q", i, stop, llmclient.StopRefusal)
		}
		if prev == nil {
			prev = seq
			continue
		}
		if strings.Join(seq, "\x00") != strings.Join(prev, "\x00") {
			t.Fatalf("Stream iter %d delta sequence differs: %q vs %q", i, seq, prev)
		}
	}
}

// TestResponseIsolation proves each Complete returns a fresh Content slice:
// mutating one response's blocks must not bleed into a later call's result. A
// shared backing array would be a latent corruption bug for any caller that
// appends tool results onto the returned content.
func TestResponseIsolation(t *testing.T) {
	c := llmnoop.New()
	ctx := context.Background()

	r1, _ := c.Complete(ctx, llmclient.Request{})
	// Mutate r1's content in place and append to it.
	r1.Content[0].Text = "MUTATED"
	r1.Content = append(r1.Content, llmclient.TextBlock("extra"))

	r2, _ := c.Complete(ctx, llmclient.Request{})
	if len(r2.Content) != 1 {
		t.Fatalf("r2.Content len = %d, want 1 (r1 mutation leaked): %+v", len(r2.Content), r2.Content)
	}
	if r2.Content[0].Text != llmnoop.Message {
		t.Errorf("r2.Content[0].Text = %q, want %q (r1 mutation leaked)", r2.Content[0].Text, llmnoop.Message)
	}
}

// TestMessageIsPlainText is a safety check: the canned message must be inert
// plain text — no control characters, no markup, no template/escape sequences
// that a downstream renderer might interpret. A fallback string is shown to
// users and logged; it must never carry a surprise payload.
func TestMessageIsPlainText(t *testing.T) {
	if llmnoop.Message == "" {
		t.Fatal("Message must not be empty")
	}
	if strings.TrimSpace(llmnoop.Message) != llmnoop.Message {
		t.Errorf("Message has leading/trailing whitespace: %q", llmnoop.Message)
	}
	for i, r := range llmnoop.Message {
		// Reject any control character (incl. NUL, newline, ESC) and the
		// Unicode replacement char (a sign of a decoding accident).
		if r < 0x20 || r == 0x7f || r == '�' {
			t.Errorf("Message contains a control/replacement rune %#U at byte %d", r, i)
		}
	}
	for _, bad := range []string{"<", ">", "{{", "}}", "${", "\\u", "%s", "`"} {
		if strings.Contains(llmnoop.Message, bad) {
			t.Errorf("Message contains potentially-interpreted sequence %q: %q", bad, llmnoop.Message)
		}
	}
}

// TestAsMessageRoundTrip proves the Response converts to an assistant Message
// whose content is the verbatim canned block — the multi-turn round-trip the
// llmclient contract requires (Response.AsMessage). This is the same invariant
// the exported conformance suite checks; the no-op client cannot run that suite
// directly because the suite asserts "returned text == requested text", which a
// client that ignores its request structurally cannot satisfy for arbitrary
// text — so each floor check is exercised here against the canned output.
func TestAsMessageRoundTrip(t *testing.T) {
	resp, err := llmnoop.New().Complete(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	msg := resp.AsMessage()
	if msg.Role != llmclient.RoleAssistant {
		t.Errorf("AsMessage Role = %q, want %q", msg.Role, llmclient.RoleAssistant)
	}
	if !reflect.DeepEqual(msg.Content, resp.Content) {
		t.Errorf("AsMessage content not verbatim:\ngot  %+v\nwant %+v", msg.Content, resp.Content)
	}
}

// TestConcurrentUse proves Client is safe for concurrent use (the interface
// requires it). Run with -race to catch data races. A stateless client should
// have none.
func TestConcurrentUse(t *testing.T) {
	c := llmnoop.New()
	const goroutines = 32
	done := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			ctx := context.Background()
			if _, err := c.Complete(ctx, llmclient.Request{Prompt: "p"}); err != nil {
				t.Errorf("Complete: %v", err)
				return
			}
			ch, err := c.Stream(ctx, llmclient.Request{Prompt: "p"})
			if err != nil {
				t.Errorf("Stream: %v", err)
				return
			}
			for range ch {
			}
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
}
