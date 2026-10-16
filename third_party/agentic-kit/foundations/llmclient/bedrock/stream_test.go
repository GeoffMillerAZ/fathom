package bedrock

import (
	"bytes"
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

// drainStream collects chunks until the channel closes, failing the test if it
// does not close within guardTimeout.
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

// writeFrames writes raw event-stream bytes with the eventstream content type
// and flushes each frame so the bytes hit the wire immediately.
func writeFrames(t *testing.T, w http.ResponseWriter, frames ...[]byte) {
	t.Helper()
	w.Header().Set("content-type", "application/vnd.amazon.eventstream")
	f, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("response writer is not a flusher")
	}
	for _, fr := range frames {
		if _, err := w.Write(fr); err != nil {
			return
		}
		f.Flush()
	}
}

// textStreamFrames builds the canonical happy-path streaming sequence for the
// given text and stop reason.
func textStreamFrames(t *testing.T, text, stopReason string) [][]byte {
	t.Helper()
	return [][]byte{
		chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":1}}}`),
		chunkFrame(t, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+jstr(t, text)+`}}`),
		chunkFrame(t, `{"type":"content_block_stop","index":0}`),
		chunkFrame(t, `{"type":"message_delta","delta":{"stop_reason":"`+stopReason+`"},"usage":{"output_tokens":7}}`),
		chunkFrame(t, `{"type":"message_stop"}`),
	}
}

func TestStreamHappyPath(t *testing.T) {
	const want = "streamed bedrock text"
	var gotPath string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		// Streaming must be SigV4-signed too.
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Error("stream request not SigV4-signed")
		}
		writeFrames(t, w, textStreamFrames(t, want, "end_turn")...)
	}), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)

	wantPath := "/model/anthropic.claude-3-5-sonnet-20241022-v2:0/invoke-with-response-stream"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}

	var text string
	for i, ck := range chunks {
		if ck.Err != nil {
			t.Fatalf("unexpected Err chunk at %d: %v", i, ck.Err)
		}
		text += ck.TextDelta
	}
	if text != want {
		t.Errorf("streamed text = %q, want %q", text, want)
	}
	last := chunks[len(chunks)-1]
	if last.StopReason != llmclient.StopEndTurn {
		t.Errorf("terminal StopReason = %q, want end_turn", last.StopReason)
	}
	if last.Usage == nil || last.Usage.OutputTokens != 7 || last.Usage.InputTokens != 5 {
		t.Errorf("terminal Usage = %+v, want input 5 / output 7", last.Usage)
	}
}

func TestStreamToolUse(t *testing.T) {
	const id, name = "tu_1", "search"
	input := `{"query":"hi"}`
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		half := len(input) / 2
		writeFrames(
			t, w,
			chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":4,"output_tokens":1}}}`),
			chunkFrame(t, `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"`+id+`","name":"`+name+`","input":{}}}`),
			chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":`+jstr(t, input[:half])+`}}`),
			chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":`+jstr(t, input[half:])+`}}`),
			chunkFrame(t, `{"type":"content_block_stop","index":0}`),
			chunkFrame(t, `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`),
			chunkFrame(t, `{"type":"message_stop"}`),
		)
	}), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)

	var toolBlock *llmclient.ContentBlock
	for _, ck := range chunks {
		if ck.Block != nil && ck.Block.Type == llmclient.BlockToolUse {
			b := *ck.Block
			toolBlock = &b
		}
	}
	if toolBlock == nil {
		t.Fatal("no tool_use block emitted")
	}
	if toolBlock.ID != id || toolBlock.Name != name {
		t.Errorf("tool block id/name = %q/%q", toolBlock.ID, toolBlock.Name)
	}
	if string(toolBlock.Input) != input {
		t.Errorf("tool input = %q, want %q", toolBlock.Input, input)
	}
	last := chunks[len(chunks)-1]
	if last.StopReason != llmclient.StopToolUse {
		t.Errorf("terminal StopReason = %q, want tool_use", last.StopReason)
	}
}

// TestStreamExceptionFrame proves a Bedrock exception frame mid-stream ends the
// stream with a typed error in a terminal Err chunk.
func TestStreamExceptionFrame(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeFrames(
			t, w,
			chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`),
			chunkFrame(t, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"partial"}}`),
			exceptionFrame(t, "ModelStreamErrorException", "downstream blew up"),
		)
	}), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream must start successfully for a MID-stream failure: %v", err)
	}
	chunks := drainStream(t, ch)
	if len(chunks) == 0 {
		t.Fatal("no chunks before failure")
	}
	last := chunks[len(chunks)-1]
	var se *llmclient.ServerError
	if !errors.As(last.Err, &se) {
		t.Fatalf("terminal Err = %T %v, want ServerError", last.Err, last.Err)
	}
	for i, ck := range chunks[:len(chunks)-1] {
		if ck.Err != nil {
			t.Errorf("Err chunk must be terminal; found one at index %d", i)
		}
	}
}

// TestStreamThrottlingExceptionTyped proves a throttling exception frame maps
// to a retryable RateLimitError.
func TestStreamThrottlingExceptionTyped(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeFrames(
			t, w,
			chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`),
			exceptionFrame(t, "ThrottlingException", "rate exceeded"),
		)
	}), Config{})
	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)
	last := chunks[len(chunks)-1]
	var rle *llmclient.RateLimitError
	if !errors.As(last.Err, &rle) {
		t.Fatalf("terminal Err = %T %v, want RateLimitError", last.Err, last.Err)
	}
}

// TestStreamPreStreamErrorStatus proves a non-200 status fails pre-stream with
// no channel.
func TestStreamPreStreamErrorStatus(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Amzn-ErrorType", "ValidationException")
		w.WriteHeader(400)
		io.WriteString(w, `{"message":"bad"}`)
	}), Config{})
	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if ch != nil {
		t.Error("non-200 must fail pre-stream, no channel")
	}
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %T %v, want InvalidRequestError", err, err)
	}
}

// TestStreamTruncatedBeforeStop proves a stream cut before message_stop ends
// with a typed retryable ServerError.
func TestStreamTruncatedBeforeStop(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeFrames(
			t, w,
			chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`),
			chunkFrame(t, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"partial"}}`),
		)
		// No message_stop; handler returns and the body closes cleanly.
	}), Config{})
	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)
	last := chunks[len(chunks)-1]
	var se *llmclient.ServerError
	if !errors.As(last.Err, &se) {
		t.Fatalf("terminal Err = %T %v, want ServerError", last.Err, last.Err)
	}
}

// TestStreamCancellationNoLeak proves cancelling the context promptly closes
// the channel (the producer selects on ctx.Done()).
func TestStreamCancellationNoLeak(t *testing.T) {
	// A server that keeps the body open so the producer is blocked on the next
	// frame / send when we cancel.
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeFrames(t, w, textStreamFrames(t, "long enough answer to stream a few chunks", "end_turn")...)
		// Hold the connection open after frames so the reader can block.
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}), Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := c.Stream(ctx, llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	// Take one chunk, then cancel and require prompt closure.
	guard := time.After(guardTimeout)
	select {
	case _, ok := <-ch:
		if !ok {
			return
		}
	case <-guard:
		t.Fatal("no chunk within guard")
	}
	cancel()
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-guard:
			t.Fatal("channel did not close after cancellation (producer leak)")
		}
	}
}

// TestStreamFramesArriveSplit proves the decoder is byte-boundary agnostic:
// the same frames delivered in awkward sub-frame chunks still decode.
func TestStreamFramesArriveSplit(t *testing.T) {
	const want = "split across writes"
	all := bytes.Join(textStreamFrames(t, want, "end_turn"), nil)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/vnd.amazon.eventstream")
		f := w.(http.Flusher)
		// Write 7 bytes at a time, flushing between — boundaries land mid-frame.
		for i := 0; i < len(all); i += 7 {
			end := i + 7
			if end > len(all) {
				end = len(all)
			}
			if _, err := w.Write(all[i:end]); err != nil {
				return
			}
			f.Flush()
		}
	}), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)
	var text string
	for _, ck := range chunks {
		if ck.Err != nil {
			t.Fatalf("unexpected Err: %v", ck.Err)
		}
		text += ck.TextDelta
	}
	if text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
}
