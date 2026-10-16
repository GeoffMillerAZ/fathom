package llmreplay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llmreplay "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/replay"
	prellmdlpgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/pre-llm-dlp-gate"
)

// mustLoad parses the recorded cassette bytes, failing the test on error.
func mustLoad(t *testing.T, buf *bytes.Buffer) []llmreplay.Entry {
	t.Helper()
	entries, err := llmreplay.Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return entries
}

// drainAll collects every chunk until the channel closes.
func drainAll(ch <-chan llmclient.StreamChunk) []llmclient.StreamChunk {
	var chunks []llmclient.StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}
	return chunks
}

// scriptedResponse is a multi-block response (text, tool_use, text) with full
// usage accounting. All content is innocuous so the default scrubber is the
// identity and round-trips can assert byte-level equality.
func scriptedResponse() llmclient.Response {
	return llmclient.Response{
		Content: []llmclient.ContentBlock{
			llmclient.TextBlock("hello "),
			llmclient.ToolUseBlock("tu_1", "search", json.RawMessage(`{"q":"weather"}`)),
			llmclient.TextBlock("world"),
		},
		Usage: llmclient.Usage{
			InputTokens:              12,
			OutputTokens:             34,
			CacheCreationInputTokens: 2,
			CacheReadInputTokens:     5,
		},
	}
}

// TestRoundTrip_Complete proves RecordingClient(MockClient) → JSONL bytes →
// ReplayClient reproduces the live response exactly: Text, Content (including
// tool_use raw JSON), StopReason, and all Usage counters.
func TestRoundTrip_Complete(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryWithResponse(scriptedResponse()))
	var buf bytes.Buffer
	rec := llmreplay.NewRecordingClient(mock, &buf)

	req := llmclient.Request{Model: "m-test", Prompt: "hi", MaxTokens: 64, Temperature: 0.5}
	live, err := rec.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("recording Complete: %v", err)
	}

	rp := llmreplay.NewReplayClient(mustLoad(t, &buf))
	replayed, err := rp.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("replay Complete: %v", err)
	}
	if !reflect.DeepEqual(replayed, live) {
		t.Errorf("replayed response diverges from live response:\ngot  %+v\nwant %+v", replayed, live)
	}
}

// TestRoundTrip_StreamSegmentation proves replay re-emits the recorded chunk
// sequence with identical segmentation: same number of chunks, same delta
// boundaries, same block positions, same terminal StopReason/Usage.
func TestRoundTrip_StreamSegmentation(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryWithResponse(scriptedResponse()))
	var buf bytes.Buffer
	rec := llmreplay.NewRecordingClient(mock, &buf)

	req := llmclient.Request{Prompt: "hi"}
	ch, err := rec.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("recording Stream: %v", err)
	}
	live := drainAll(ch)

	// Sanity: the mock splits each text block into multiple deltas, so
	// segmentation is actually being exercised.
	deltas := 0
	for _, c := range live {
		if c.TextDelta != "" {
			deltas++
		}
	}
	if deltas < 2 {
		t.Fatalf("mock stream produced %d text deltas, want >= 2 (segmentation not exercised)", deltas)
	}

	rp := llmreplay.NewReplayClient(mustLoad(t, &buf))
	rch, err := rp.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("replay Stream: %v", err)
	}
	replayed := drainAll(rch)

	if len(replayed) != len(live) {
		t.Fatalf("replayed %d chunks, recorded stream had %d — segmentation not preserved", len(replayed), len(live))
	}
	for i := range live {
		a, b := live[i], replayed[i]
		if b.TextDelta != a.TextDelta {
			t.Errorf("chunk %d: TextDelta = %q, want %q", i, b.TextDelta, a.TextDelta)
		}
		if !reflect.DeepEqual(b.Block, a.Block) {
			t.Errorf("chunk %d: Block = %+v, want %+v", i, b.Block, a.Block)
		}
		if b.StopReason != a.StopReason {
			t.Errorf("chunk %d: StopReason = %q, want %q", i, b.StopReason, a.StopReason)
		}
		if !reflect.DeepEqual(b.Usage, a.Usage) {
			t.Errorf("chunk %d: Usage = %+v, want %+v", i, b.Usage, a.Usage)
		}
		if (b.Err == nil) != (a.Err == nil) {
			t.Errorf("chunk %d: Err = %v, want %v", i, b.Err, a.Err)
		}
	}
}

// TestRecordAndReplay_CompleteError proves a failed Complete is recorded and
// replayed as an equivalent error: same message, surfaced as *ReplayedError.
func TestRecordAndReplay_CompleteError(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryInvalid("bad prompt"))
	var buf bytes.Buffer
	rec := llmreplay.NewRecordingClient(mock, &buf)

	req := llmclient.Request{Prompt: "hi"}
	_, liveErr := rec.Complete(context.Background(), req)
	if liveErr == nil {
		t.Fatal("recording Complete: want scripted error, got nil")
	}

	rp := llmreplay.NewReplayClient(mustLoad(t, &buf))
	_, replayErr := rp.Complete(context.Background(), req)
	var re *llmreplay.ReplayedError
	if !errors.As(replayErr, &re) {
		t.Fatalf("replay Complete error = %v (%T), want *ReplayedError", replayErr, replayErr)
	}
	if re.Message != liveErr.Error() {
		t.Errorf("replayed error message = %q, want %q", re.Message, liveErr.Error())
	}
}

// TestRecordAndReplay_PreStreamError proves a pre-stream failure (Stream
// returns an error, no channel) is recorded and replayed as a pre-stream
// failure with the same message.
func TestRecordAndReplay_PreStreamError(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryErr(&llmclient.AuthError{Message: "no key"}))
	var buf bytes.Buffer
	rec := llmreplay.NewRecordingClient(mock, &buf)

	req := llmclient.Request{Prompt: "hi"}
	ch, liveErr := rec.Stream(context.Background(), req)
	if liveErr == nil || ch != nil {
		t.Fatalf("recording Stream = (%v, %v), want pre-stream error with nil channel", ch, liveErr)
	}

	rp := llmreplay.NewReplayClient(mustLoad(t, &buf))
	rch, replayErr := rp.Stream(context.Background(), req)
	if rch != nil {
		t.Fatal("replay Stream returned a channel for a recorded pre-stream failure")
	}
	var re *llmreplay.ReplayedError
	if !errors.As(replayErr, &re) {
		t.Fatalf("replay Stream error = %v (%T), want *ReplayedError", replayErr, replayErr)
	}
	if re.Message != liveErr.Error() {
		t.Errorf("replayed error message = %q, want %q", re.Message, liveErr.Error())
	}
}

// TestDefaultScrub_RedactsCassette proves scrubbing is on by default: a
// sentinel secret planted in the request (prompt, system, message text,
// tool_result content) and in the response never reaches the cassette bytes,
// while the LIVE response handed back to the caller is untouched.
func TestDefaultScrub_RedactsCassette(t *testing.T) {
	const sentinel = "api_key=SENTINEL-XYZZY-99"

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("response leaks " + sentinel))
	var buf bytes.Buffer
	rec := llmreplay.NewRecordingClient(mock, &buf)

	req := llmclient.Request{
		Prompt: "prompt has " + sentinel,
		System: "system has " + sentinel,
		Messages: []llmclient.Message{{
			Role: llmclient.RoleUser,
			Content: []llmclient.ContentBlock{
				llmclient.TextBlock("user says " + sentinel),
				llmclient.ToolResultBlock("tu_1", "tool output has "+sentinel, false),
			},
		}},
	}
	live, err := rec.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("recording Complete: %v", err)
	}

	// The live path is never scrubbed — only the cassette is.
	if !strings.Contains(live.Text, sentinel) {
		t.Errorf("live response was altered by recording: %q", live.Text)
	}

	cassette := buf.String()
	if strings.Contains(cassette, "SENTINEL-XYZZY-99") {
		t.Fatalf("cassette leaked the sentinel secret:\n%s", cassette)
	}
	if !strings.Contains(cassette, "[REDACTED:API_KEY]") {
		t.Errorf("cassette does not contain the redaction label:\n%s", cassette)
	}
}

// TestNoScrub_RetainsSecrets locks the documented danger of the explicit
// opt-out: with NoScrub the sentinel survives verbatim in the cassette.
func TestNoScrub_RetainsSecrets(t *testing.T) {
	const sentinel = "api_key=SENTINEL-XYZZY-99"

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	var buf bytes.Buffer
	rec := llmreplay.NewRecordingClient(mock, &buf, llmreplay.NoScrub())

	if _, err := rec.Complete(context.Background(), llmclient.Request{Prompt: sentinel}); err != nil {
		t.Fatalf("recording Complete: %v", err)
	}
	if !strings.Contains(buf.String(), sentinel) {
		t.Error("NoScrub cassette should retain the sentinel verbatim (the opt-out is total)")
	}
}

// TestWithScrubber_ExtendedGate proves a caller-supplied gate scrubber works:
// the extended policy's AWS access-key rule redacts an AKIA-shaped sentinel
// that the default policy would not match.
func TestWithScrubber_ExtendedGate(t *testing.T) {
	const sentinel = "AKIAABCDEFGHIJKLMNOP" // AKIA + 16 uppercase alphanumerics

	gate, err := prellmdlpgate.NewGate(prellmdlpgate.ExtendedPolicy())
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	var buf bytes.Buffer
	rec := llmreplay.NewRecordingClient(mock, &buf,
		llmreplay.WithScrubber(llmreplay.NewGateScrubber(gate)))

	req := llmclient.Request{Prompt: "use key " + sentinel + " now"}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatalf("recording Complete: %v", err)
	}

	cassette := buf.String()
	if strings.Contains(cassette, sentinel) {
		t.Fatalf("cassette leaked the AKIA sentinel:\n%s", cassette)
	}
	if !strings.Contains(cassette, "[REDACTED:AWS_ACCESS_KEY_ID]") {
		t.Errorf("cassette does not contain the AWS redaction label:\n%s", cassette)
	}
}
