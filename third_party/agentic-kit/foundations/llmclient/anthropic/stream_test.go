package anthropic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// guardTimeout bounds every streaming assertion so a non-conforming producer
// fails the test instead of hanging it.
const guardTimeout = 5 * time.Second

// drainStream collects chunks until the channel closes, failing the test if
// it does not close within guardTimeout.
func drainStream(t *testing.T, ch <-chan llmclient.StreamChunk) []llmclient.StreamChunk {
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

// sseHandler writes the raw SSE payload in deliberately awkward chunks:
// every chunk boundary is flushed, boundaries land mid-line, and one
// boundary splits a multi-byte UTF-8 rune when splitRune is set. This proves
// the parser is byte-boundary agnostic.
func sseHandler(t *testing.T, payload string, splitRune bool) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Error("response writer is not a flusher")
			return
		}
		w.Header().Set("content-type", "text/event-stream")

		rest := payload
		if splitRune {
			// Split inside the first multi-byte rune: write up to and
			// including its FIRST byte, flush, then continue.
			idx := strings.IndexFunc(payload, func(r rune) bool { return r > 0x7f })
			if idx < 0 {
				t.Error("splitRune set but payload has no multi-byte rune")
				return
			}
			io.WriteString(w, payload[:idx+1]) // half a rune
			f.Flush()
			rest = payload[idx+1:]
		}
		// 7-byte flushed chunks: virtually every line is split mid-line.
		for len(rest) > 0 {
			n := 7
			if n > len(rest) {
				n = len(rest)
			}
			io.WriteString(w, rest[:n])
			f.Flush()
			rest = rest[n:]
		}
	}
}

// streamTextFixture is a realistic raw SSE stream: ping events, a comment
// heartbeat line, and one delta whose JSON is split across two data: lines
// (joined with \n per the SSE spec — still valid JSON).
const streamTextFixture = "event: message_start\n" +
	"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"model-x\",\"usage\":{\"input_tokens\":12,\"cache_creation_input_tokens\":3,\"cache_read_input_tokens\":4,\"output_tokens\":1}}}\n" +
	"\n" +
	"event: ping\n" +
	"data: {\"type\": \"ping\"}\n" +
	"\n" +
	"event: content_block_start\n" +
	"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n" +
	"\n" +
	"event: content_block_delta\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello, \"}}\n" +
	"\n" +
	": heartbeat comment line, must be ignored\n" +
	"event: content_block_delta\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":0,\n" +
	"data:  \"delta\":{\"type\":\"text_delta\",\"text\":\"wörld\"}}\n" +
	"\n" +
	"event: content_block_stop\n" +
	"data: {\"type\":\"content_block_stop\",\"index\":0}\n" +
	"\n" +
	"event: message_delta\n" +
	"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":9}}\n" +
	"\n" +
	"event: message_stop\n" +
	"data: {\"type\":\"message_stop\"}\n" +
	"\n"

func TestStreamTextMultiDelta(t *testing.T) {
	var gotBody []byte
	handler := func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		sseHandler(t, streamTextFixture, true)(w, r)
	}
	c := newTestClient(t, http.HandlerFunc(handler), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)

	if !strings.Contains(string(gotBody), `"stream":true`) {
		t.Errorf("request body must set stream:true, got %s", gotBody)
	}

	var text string
	deltas := 0
	for i, ck := range chunks {
		if ck.Err != nil {
			t.Fatalf("unexpected Err chunk at %d: %v", i, ck.Err)
		}
		if ck.TextDelta != "" {
			deltas++
			text += ck.TextDelta
		}
	}
	if text != "Hello, wörld" {
		t.Errorf("streamed text = %q, want %q", text, "Hello, wörld")
	}
	if deltas < 2 {
		t.Errorf("deltas = %d, want at least 2 (multi-delta)", deltas)
	}

	last := chunks[len(chunks)-1]
	if last.StopReason != llmclient.StopEndTurn {
		t.Errorf("terminal StopReason = %q, want end_turn", last.StopReason)
	}
	if last.Usage == nil {
		t.Fatal("terminal chunk must carry non-nil Usage")
	}
	wantUsage := llmclient.Usage{InputTokens: 12, OutputTokens: 9, CacheCreationInputTokens: 3, CacheReadInputTokens: 4}
	if *last.Usage != wantUsage {
		t.Errorf("Usage = %+v, want %+v", *last.Usage, wantUsage)
	}
}

const streamToolUseFixture = "event: message_start\n" +
	"data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":21,\"output_tokens\":1}}}\n" +
	"\n" +
	"event: content_block_start\n" +
	"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n" +
	"\n" +
	"event: content_block_delta\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Let me check.\"}}\n" +
	"\n" +
	"event: content_block_stop\n" +
	"data: {\"type\":\"content_block_stop\",\"index\":0}\n" +
	"\n" +
	"event: content_block_start\n" +
	"data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_01\",\"name\":\"get_weather\",\"input\":{}}}\n" +
	"\n" +
	"event: content_block_delta\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"city\\\":\"}}\n" +
	"\n" +
	"event: ping\n" +
	"data: {\"type\": \"ping\"}\n" +
	"\n" +
	"event: content_block_delta\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\\\"Paris\\\"}\"}}\n" +
	"\n" +
	"event: content_block_stop\n" +
	"data: {\"type\":\"content_block_stop\",\"index\":1}\n" +
	"\n" +
	"event: message_delta\n" +
	"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":30}}\n" +
	"\n" +
	"event: message_stop\n" +
	"data: {\"type\":\"message_stop\"}\n" +
	"\n"

func TestStreamToolUseAccumulation(t *testing.T) {
	c := newTestClient(t, sseHandler(t, streamToolUseFixture, false), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "weather?"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)

	var text string
	var blocks []*llmclient.ContentBlock
	for i, ck := range chunks {
		if ck.Err != nil {
			t.Fatalf("unexpected Err chunk at %d: %v", i, ck.Err)
		}
		text += ck.TextDelta
		if ck.Block != nil {
			blocks = append(blocks, ck.Block)
		}
	}
	if text != "Let me check." {
		t.Errorf("text = %q", text)
	}
	if len(blocks) != 1 {
		t.Fatalf("complete blocks = %d, want exactly 1 (emitted on content_block_stop)", len(blocks))
	}
	blk := blocks[0]
	if blk.Type != llmclient.BlockToolUse {
		t.Fatalf("block type = %q, want tool_use", blk.Type)
	}
	if blk.ID != "toolu_01" || blk.Name != "get_weather" {
		t.Errorf("block = %q/%q", blk.ID, blk.Name)
	}
	if string(blk.Input) != `{"city":"Paris"}` {
		t.Errorf("accumulated Input = %s, want {\"city\":\"Paris\"}", blk.Input)
	}

	last := chunks[len(chunks)-1]
	if last.StopReason != llmclient.StopToolUse {
		t.Errorf("terminal StopReason = %q, want tool_use", last.StopReason)
	}
	if last.Usage == nil || last.Usage.OutputTokens != 30 || last.Usage.InputTokens != 21 {
		t.Errorf("terminal Usage = %+v", last.Usage)
	}
}

// TestStreamUnknownBlockEmittedVerbatim proves that an unknown block type
// (e.g. "thinking") is emitted as BlockUnknown with its Raw populated.
// Deltas are accumulated into the start JSON: the thinking_delta's value
// is spliced into the "thinking" field while preserving key order, so
// the result matches what Complete would produce for the same block.
func TestStreamUnknownBlockEmittedVerbatim(t *testing.T) {
	const thinkingStart = `{"type":"thinking","thinking":"","signature":""}`
	// After the thinking_delta carrying "..." is applied, the "thinking"
	// field is replaced in-place (key order preserved).
	const wantRaw = `{"type":"thinking","thinking":"...","signature":""}`
	fixture := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":2}}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":" + thinkingStart + "}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"...\"}}\n\n" +
		"event: content_block_stop\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"
	c := newTestClient(t, sseHandler(t, fixture, false), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)
	var blk *llmclient.ContentBlock
	for _, ck := range chunks {
		if ck.Err != nil {
			t.Fatalf("unexpected Err: %v", ck.Err)
		}
		if ck.Block != nil {
			blk = ck.Block
		}
	}
	if blk == nil {
		t.Fatal("unknown block was not emitted")
	}
	if blk.Type != llmclient.BlockUnknown {
		t.Fatalf("block type = %q, want unknown", blk.Type)
	}
	if string(blk.Raw) != wantRaw {
		t.Errorf("Raw = %s, want %s", blk.Raw, wantRaw)
	}
}

func TestStreamMidStreamErrorEvent(t *testing.T) {
	fixture := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":2}}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"partial \"}}\n\n" +
		"event: error\n" +
		"data: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n"
	c := newTestClient(t, sseHandler(t, fixture, false), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream must start successfully (pre-stream OK): %v", err)
	}
	chunks := drainStream(t, ch)
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want partial text then terminal error", len(chunks))
	}
	for i, ck := range chunks[:len(chunks)-1] {
		if ck.Err != nil {
			t.Errorf("Err chunk must be terminal, found at %d", i)
		}
	}
	last := chunks[len(chunks)-1]
	if last.Err == nil {
		t.Fatal("terminal chunk must carry Err")
	}
	var se *llmclient.ServerError
	if !errors.As(last.Err, &se) {
		t.Fatalf("Err = %T %v, want ServerError", last.Err, last.Err)
	}
	if se.StatusCode != 529 {
		t.Errorf("StatusCode = %d, want 529 for overloaded_error", se.StatusCode)
	}
	if !strings.Contains(se.Message, "Overloaded") {
		t.Errorf("message %q should carry the API message", se.Message)
	}
}

func TestStreamCtxCancelMidStream(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := w.(http.Flusher)
		w.Header().Set("content-type", "text/event-stream")
		io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1}}}\n\n")
		io.WriteString(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"first\"}}\n\n")
		f.Flush()
		// Hold the stream open until the client disconnects: the only way
		// the test finishes is cancellation propagating.
		<-r.Context().Done()
	}), Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := c.Stream(ctx, llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// Take the first delta, then cancel mid-stream.
	guard := time.After(guardTimeout)
	select {
	case ck, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before any chunk")
		}
		if ck.TextDelta != "first" {
			t.Fatalf("first chunk = %+v, want TextDelta first", ck)
		}
	case <-guard:
		t.Fatal("no chunk before guard timeout")
	}
	cancel()

	// The producer must terminate and close the channel promptly — no
	// goroutine leak, channel closed exactly once.
	sawTerminalErr := false
	for {
		select {
		case ck, ok := <-ch:
			if !ok {
				if !sawTerminalErr {
					// Best-effort terminal chunk: closure alone is fine, but
					// when a chunk did arrive it must carry ctx's error.
					return
				}
				return
			}
			if ck.Err != nil {
				sawTerminalErr = true
				if !errors.Is(ck.Err, context.Canceled) {
					t.Errorf("terminal Err = %v, want context.Canceled", ck.Err)
				}
			}
		case <-guard:
			t.Fatal("stream channel did not close after cancellation (producer leak)")
		}
	}
}

func TestStreamUnexpectedEOF(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1}}}\n\n")
		io.WriteString(w, "event: content_block_de") // sever mid-line, no message_stop
	}), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)
	if len(chunks) == 0 {
		t.Fatal("want a terminal Err chunk")
	}
	last := chunks[len(chunks)-1]
	var se *llmclient.ServerError
	if !errors.As(last.Err, &se) {
		t.Fatalf("Err = %T %v, want ServerError (severed stream is retryable)", last.Err, last.Err)
	}
}

func TestStreamPreStreamHTTPError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		writeAPIError(w, 429, "rate_limit_error", "slow down")
	}), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if ch != nil {
		t.Error("pre-stream failure must not return a channel")
	}
	var rle *llmclient.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("err = %T %v, want RateLimitError", err, err)
	}
	if rle.RetryAfter != 3*time.Second {
		t.Errorf("RetryAfter = %v, want 3s", rle.RetryAfter)
	}
}

// TestSSEReaderLargeDelta proves the parser has no 64KB line cap (the
// bufio.Scanner trap): one delta larger than 64KB must survive intact.
func TestSSEReaderLargeDelta(t *testing.T) {
	big := strings.Repeat("x", 80*1024)
	fixture := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1}}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + big + "\"}}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"
	c := newTestClient(t, sseHandler(t, fixture, false), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var text strings.Builder
	for _, ck := range drainStream(t, ch) {
		if ck.Err != nil {
			t.Fatalf("Err: %v", ck.Err)
		}
		text.WriteString(ck.TextDelta)
	}
	if text.String() != big {
		t.Errorf("large delta truncated: got %d bytes, want %d", text.Len(), len(big))
	}
}
