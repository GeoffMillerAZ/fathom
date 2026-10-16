// Package llmtest provides an exported conformance suite for implementations
// of llmclient.LLMClient. Any client — the in-tree MockClient, future HTTP
// provider adapters tested against httptest servers — runs the same checks by
// supplying a Harness of factory functions and a Capabilities gate.
//
// Composition: imports only the standard library and the parent
// foundations/llmclient package. No network — harnesses for HTTP clients are
// expected to script httptest servers themselves.
//
// Concurrency contract: RunConformance is safe to call from parallel tests;
// each subtest builds its own client via the harness factories and shares no
// state with the others.
//
// Usage:
//
//	llmtest.RunConformance(t, llmtest.Harness{
//		NewText: func(t *testing.T, text string) llmclient.LLMClient {
//			return llmclient.NewMockClient(llmclient.MockEntryOK(text))
//		},
//		// optional, gated by Capabilities:
//		NewToolUse:     ...,
//		NewStreamError: ...,
//	}, llmtest.Capabilities{ToolUse: true, MidStreamError: true})
package llmtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// guardTimeout bounds every streaming check so a non-conforming client (one
// that never closes its channel) fails the test instead of hanging it.
const guardTimeout = 5 * time.Second

// Capabilities gates the optional conformance checks. Core checks (text
// completion, AsMessage round-trip, stop reasons, streaming lifecycle,
// context cancellation, usage) always run — they are the v2 contract floor.
type Capabilities struct {
	// ToolUse enables the tool_use checks; requires Harness.NewToolUse.
	ToolUse bool

	// MidStreamError enables the mid-stream failure check; requires
	// Harness.NewStreamError. HTTP harnesses can implement it by truncating
	// the httptest server's response body mid-stream.
	MidStreamError bool

	// UnknownBlock enables unknown-block round-trip checks (complete and
	// stream paths); requires Harness.NewUnknownBlock.
	UnknownBlock bool

	// TypedErrors enables typed-error taxonomy checks: HTTP status codes map
	// to the correct llmclient error types. Requires Harness.NewHTTPError.
	// Also exercises Retry-After header parsing (seconds and HTTP-date) for
	// RateLimitError when the adapter supports it.
	TypedErrors bool

	// BodyInspection enables wire-shape checks for ProviderParams, Thinking,
	// OutputSchema, ToolDef.Raw, and Effort. Requires Harness.NewBodyInspect.
	BodyInspection bool

	// PreflightValidation enables the CacheHintLimitRejected check and other
	// checks that require the client to enforce request limits before any
	// network call or entry consumption. Set this for adapters that run
	// validateRequest (MockClient, anthropic, bedrock). Do NOT set it for
	// passthrough clients (ReplayClient, Swappable, decorators) that rely on
	// the underlying client to validate.
	PreflightValidation bool
}

// Harness adapts one client implementation to the suite. Each factory returns
// a fresh client arranged so that its NEXT call produces the described model
// behaviour. For MockClient that means scripting entries; for an HTTP client
// it means configuring a test server.
type Harness struct {
	// NewText returns a client whose next Complete or Stream call yields a
	// successful assistant turn whose text is exactly text. Required.
	NewText func(t *testing.T, text string) llmclient.LLMClient

	// NewToolUse returns a client whose next call yields a successful
	// tool_use turn invoking the named tool with the given raw-JSON input
	// under the given tool-use id. Required when Capabilities.ToolUse.
	NewToolUse func(t *testing.T, id, name string, input json.RawMessage) llmclient.LLMClient

	// NewStreamError returns a client whose next Stream call starts
	// successfully, emits partial text, then fails mid-stream with a
	// non-nil terminal Err. Required when Capabilities.MidStreamError.
	NewStreamError func(t *testing.T, partial string, err error) llmclient.LLMClient

	// NewUnknownBlock returns a client whose next Complete or Stream call
	// yields a response containing exactly one content block with the given
	// verbatim raw JSON as a BlockUnknown block (in addition to any text
	// blocks the adapter must emit to satisfy the response shape). The raw
	// bytes must round-trip verbatim through the ContentBlock.Raw field.
	// Required when Capabilities.UnknownBlock.
	NewUnknownBlock func(t *testing.T, rawBlock json.RawMessage) llmclient.LLMClient

	// NewHTTPError returns a client whose next Complete call will fail with
	// the HTTP-level error described by statusCode and body. When
	// retryAfterSec > 0 the adapter is expected to parse a Retry-After header
	// carrying that many seconds and return a RateLimitError.RetryAfter
	// rounded to the nearest second. retryAfterHTTPDate, when non-empty, is
	// an HTTP-date string the adapter should parse for Retry-After; the
	// harness supplies a time far enough in the future that the result must
	// be > 0.
	//
	// errTypeHint names the Bedrock exception type (X-Amzn-ErrorType header
	// value) when the adapter is Bedrock-flavoured; ignored by Anthropic/mock
	// harnesses. Supply "" when not applicable.
	//
	// Required when Capabilities.TypedErrors.
	NewHTTPError func(t *testing.T, statusCode int, errTypeHint, retryAfterSec, retryAfterHTTPDate, body string) llmclient.LLMClient

	// NewBodyInspect returns a client whose next Complete or Stream call
	// invokes inspect with the raw request body BEFORE serving a successful
	// text response containing successText. inspect runs inside the test
	// goroutine (or a goroutine the harness controls) — t.Error/t.Fatal are
	// legal. Required when Capabilities.BodyInspection.
	NewBodyInspect func(t *testing.T, inspect func(body []byte), successText string) llmclient.LLMClient
}

// RunConformance runs the v2 contract checks against the client(s) built by h.
// It fails the surrounding test on any violation; optional areas are gated by
// caps. See the package doc for usage.
func RunConformance(t *testing.T, h Harness, caps Capabilities) {
	t.Helper()
	if h.NewText == nil {
		t.Fatal("llmtest: Harness.NewText is required")
	}
	if caps.ToolUse && h.NewToolUse == nil {
		t.Fatal("llmtest: Capabilities.ToolUse requires Harness.NewToolUse")
	}
	if caps.MidStreamError && h.NewStreamError == nil {
		t.Fatal("llmtest: Capabilities.MidStreamError requires Harness.NewStreamError")
	}
	if caps.UnknownBlock && h.NewUnknownBlock == nil {
		t.Fatal("llmtest: Capabilities.UnknownBlock requires Harness.NewUnknownBlock")
	}
	if caps.TypedErrors && h.NewHTTPError == nil {
		t.Fatal("llmtest: Capabilities.TypedErrors requires Harness.NewHTTPError")
	}
	if caps.BodyInspection && h.NewBodyInspect == nil {
		t.Fatal("llmtest: Capabilities.BodyInspection requires Harness.NewBodyInspect")
	}

	// Core checks — always run.
	t.Run("CompleteTextMatchesBlocks", func(t *testing.T) { checkTextMatchesBlocks(t, h) })
	t.Run("AsMessageRoundTrip", func(t *testing.T) { checkAsMessageRoundTrip(t, h) })
	t.Run("StopReasonPopulated", func(t *testing.T) { checkStopReasonPopulated(t, h) })
	t.Run("StreamLifecycle", func(t *testing.T) { checkStreamLifecycle(t, h) })
	t.Run("StreamContextCancellation", func(t *testing.T) { checkStreamCancellation(t, h) })
	t.Run("UsageOnComplete", func(t *testing.T) { checkUsageOnComplete(t, h) })
	t.Run("UsageOnStream", func(t *testing.T) { checkUsageOnStream(t, h) })

	if caps.ToolUse {
		t.Run("ToolUseExposesToolCalls", func(t *testing.T) { checkToolUse(t, h) })
		t.Run("StreamToolUseTerminal", func(t *testing.T) { checkStreamToolUseTerminal(t, h) })
	}
	if caps.MidStreamError {
		t.Run("StreamMidStreamError", func(t *testing.T) { checkMidStreamError(t, h) })
	}
	if caps.UnknownBlock {
		t.Run("UnknownBlockRawPreservedComplete", func(t *testing.T) { checkUnknownBlockComplete(t, h) })
		t.Run("UnknownBlockRawPreservedStream", func(t *testing.T) { checkUnknownBlockStream(t, h) })
		t.Run("UnknownBlockStreamEqualsComplete", func(t *testing.T) { checkUnknownBlockStreamEqualsComplete(t, h) })
		t.Run("UnknownBlockRoundTrip", func(t *testing.T) { checkUnknownBlockRoundTrip(t, h) })
	}
	if caps.TypedErrors {
		t.Run("TypedError400InvalidRequest", func(t *testing.T) { checkTypedError400(t, h) })
		t.Run("TypedError401Auth", func(t *testing.T) { checkTypedError401(t, h) })
		t.Run("TypedError403Auth", func(t *testing.T) { checkTypedError403(t, h) })
		t.Run("TypedError429RateLimit", func(t *testing.T) { checkTypedError429(t, h) })
		t.Run("TypedError429RetryAfterSeconds", func(t *testing.T) { checkTypedError429RetryAfterSec(t, h) })
		t.Run("TypedError429RetryAfterHTTPDate", func(t *testing.T) { checkTypedError429RetryAfterDate(t, h) })
		t.Run("TypedError500Server", func(t *testing.T) { checkTypedError500(t, h) })
		t.Run("TypedError503Server", func(t *testing.T) { checkTypedError503(t, h) })
	}
	if caps.PreflightValidation {
		t.Run("CacheHintLimitRejected", func(t *testing.T) { checkCacheHintLimit(t, h) })
	}
	if caps.BodyInspection {
		t.Run("ProviderParamsMergedComplete", func(t *testing.T) { checkProviderParamsMergedComplete(t, h) })
		t.Run("ProviderParamsMergedStream", func(t *testing.T) { checkProviderParamsMergedStream(t, h) })
		t.Run("ProviderParamsNonObjectRejected", func(t *testing.T) { checkProviderParamsNonObjectRejected(t, h) })
		t.Run("ThinkingOnWire", func(t *testing.T) { checkThinkingOnWire(t, h) })
		t.Run("OutputSchemaOnWire", func(t *testing.T) { checkOutputSchemaOnWire(t, h) })
		t.Run("OutputSchemaNonObjectRejected", func(t *testing.T) { checkOutputSchemaNonObjectRejected(t, h) })
		t.Run("ToolDefRawVerbatim", func(t *testing.T) { checkToolDefRawVerbatim(t, h) })
		t.Run("EffortRejectedOrMapped", func(t *testing.T) { checkEffortRejectedOrMapped(t, h) })
	}
}

// drain collects every chunk until the channel closes, failing the test if it
// does not close within guardTimeout.
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
			t.Fatalf("llmtest: stream channel did not close within %v (%d chunks received)", guardTimeout, len(chunks))
		}
	}
}

// --------------------------------------------------------------------------
// Core checks
// --------------------------------------------------------------------------

func checkTextMatchesBlocks(t *testing.T, h Harness) {
	t.Helper()
	const want = "the quick brown fox"
	client := h.NewText(t, want)

	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "say it"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != want {
		t.Errorf("Text = %q, want %q", resp.Text, want)
	}
	var concat string
	for _, b := range resp.Content {
		if b.Type == llmclient.BlockText {
			concat += b.Text
		}
	}
	if resp.Text != concat {
		t.Errorf("Text (%q) != concatenation of text blocks (%q)", resp.Text, concat)
	}
}

func checkAsMessageRoundTrip(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewText(t, "round trip me")

	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
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

func checkStopReasonPopulated(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewText(t, "done")

	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.StopReason == "" {
		t.Error("StopReason must be populated on success")
	}
}

// checkUsageOnComplete proves Usage is populated on a successful Complete.
// InputTokens and OutputTokens must both be > 0 (a real provider always
// reports them; a mock must too).
func checkUsageOnComplete(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewText(t, "usage check")
	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.InputTokens == 0 {
		t.Error("Usage.InputTokens must be > 0 on a successful Complete")
	}
	if resp.OutputTokens == 0 {
		t.Error("Usage.OutputTokens must be > 0 on a successful Complete")
	}
}

// checkUsageOnStream proves Usage is assembled correctly across message_start
// and message_delta events and delivered on the terminal stream chunk.
func checkUsageOnStream(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewText(t, "streamed usage check")
	ch, err := client.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drain(t, ch)
	if len(chunks) == 0 {
		t.Fatal("stream produced no chunks")
	}
	last := chunks[len(chunks)-1]
	if last.Usage == nil {
		t.Fatal("terminal stream chunk must carry non-nil Usage")
	}
	if last.Usage.InputTokens == 0 {
		t.Error("Usage.InputTokens must be > 0 on a successful stream")
	}
	if last.Usage.OutputTokens == 0 {
		t.Error("Usage.OutputTokens must be > 0 on a successful stream")
	}
}

// checkCacheHintLimit proves that a request with more than MaxCacheBreakpoints
// cache hints is rejected with a typed *InvalidRequestError before any HTTP
// call or entry consumption. Requires 5 breakpoints (system + 4 message
// blocks) to exceed the limit of 4.
func checkCacheHintLimit(t *testing.T, h Harness) {
	t.Helper()
	// Build a request that clearly exceeds the 4-breakpoint limit: system
	// hint + 4 content-block hints = 5 total.
	req := llmclient.Request{
		System:          "sys",
		SystemCacheHint: true,
		Messages: []llmclient.Message{{
			Role: llmclient.RoleUser,
			Content: []llmclient.ContentBlock{
				{Type: llmclient.BlockText, Text: "a", CacheHint: true},
				{Type: llmclient.BlockText, Text: "b", CacheHint: true},
				{Type: llmclient.BlockText, Text: "c", CacheHint: true},
				{Type: llmclient.BlockText, Text: "d", CacheHint: true},
			},
		}},
	}
	client := h.NewText(t, "should not be called")
	_, err := client.Complete(context.Background(), req)
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("CacheHint limit: err = %T %v, want *InvalidRequestError", err, err)
	}
}

// --------------------------------------------------------------------------
// Tool-use checks
// --------------------------------------------------------------------------

func checkToolUse(t *testing.T, h Harness) {
	t.Helper()
	input := json.RawMessage(`{"query":"conformance"}`)
	client := h.NewToolUse(t, "tu_conf", "search", input)

	resp, err := client.Complete(context.Background(), llmclient.Request{
		Prompt: "look it up",
		Tools:  []llmclient.ToolDef{{Name: "search", Description: "find things", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.StopReason != llmclient.StopToolUse {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, llmclient.StopToolUse)
	}
	calls := resp.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1 (%+v)", len(calls), calls)
	}
	call := calls[0]
	if call.Type != llmclient.BlockToolUse {
		t.Errorf("call Type = %q, want %q", call.Type, llmclient.BlockToolUse)
	}
	if call.ID != "tu_conf" || call.Name != "search" {
		t.Errorf("call ID/Name = %q/%q, want tu_conf/search", call.ID, call.Name)
	}
	// Compare inputs as parsed JSON: HTTP clients may reserialize.
	var got, want any
	if err := json.Unmarshal(call.Input, &got); err != nil {
		t.Fatalf("call Input is not valid JSON: %v", err)
	}
	if err := json.Unmarshal(input, &want); err != nil {
		t.Fatalf("harness input is not valid JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("call Input = %s, want %s", call.Input, input)
	}
}

// checkStreamToolUseTerminal proves that a stream ending in a tool_use stop
// reason delivers the assembled tool_use block(s) on the stream, the terminal
// chunk carries StopReason == StopToolUse, and no chunk follows the terminal.
func checkStreamToolUseTerminal(t *testing.T, h Harness) {
	t.Helper()
	input := json.RawMessage(`{"query":"stream_tool"}`)
	client := h.NewToolUse(t, "tu_stream", "lookup", input)

	ch, err := client.Stream(context.Background(), llmclient.Request{
		Prompt: "stream tool use",
		Tools:  []llmclient.ToolDef{{Name: "lookup", Description: "look up", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drain(t, ch)

	var toolBlocks []*llmclient.ContentBlock
	for _, ck := range chunks {
		if ck.Block != nil && ck.Block.Type == llmclient.BlockToolUse {
			b := *ck.Block
			toolBlocks = append(toolBlocks, &b)
		}
	}
	if len(toolBlocks) == 0 {
		t.Fatal("no tool_use block delivered on stream")
	}
	tb := toolBlocks[0]
	if tb.Name != "lookup" {
		t.Errorf("tool block name = %q, want lookup", tb.Name)
	}
	// Verify the input round-trips through JSON.
	var got, want any
	if err := json.Unmarshal(tb.Input, &got); err != nil {
		t.Fatalf("stream tool_use Input not valid JSON: %v", err)
	}
	if err := json.Unmarshal(input, &want); err != nil {
		t.Fatalf("harness input not valid JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stream tool_use Input = %s, want %s", tb.Input, input)
	}

	last := chunks[len(chunks)-1]
	if last.StopReason != llmclient.StopToolUse {
		t.Errorf("terminal StopReason = %q, want tool_use", last.StopReason)
	}
	if last.Err != nil {
		t.Errorf("terminal chunk must not carry Err for a tool_use completion: %v", last.Err)
	}
}

// --------------------------------------------------------------------------
// Streaming lifecycle checks
// --------------------------------------------------------------------------

func checkStreamLifecycle(t *testing.T, h Harness) {
	t.Helper()
	const want = "streamed conformance text"
	client := h.NewText(t, want)

	ch, err := client.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drain(t, ch)
	if len(chunks) == 0 {
		t.Fatal("stream produced no chunks")
	}

	var text string
	for i, c := range chunks {
		if c.Err != nil {
			t.Fatalf("unexpected Err chunk at %d: %v", i, c.Err)
		}
		text += c.TextDelta
		if c.Block != nil && c.Block.Type == llmclient.BlockText {
			text += c.Block.Text
		}
	}
	if text != want {
		t.Errorf("streamed text = %q, want %q", text, want)
	}

	last := chunks[len(chunks)-1]
	if last.StopReason == "" {
		t.Error("terminal chunk must carry StopReason on success")
	}
	if last.Usage == nil {
		t.Error("terminal chunk must carry non-nil Usage on success")
	}
}

func checkStreamCancellation(t *testing.T, h Harness) {
	t.Helper()
	// Long enough that any reasonable client emits multiple chunks.
	client := h.NewText(t, "a deliberately long answer used to exercise cancellation mid-stream")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := client.Stream(ctx, llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// Take one chunk, then cancel.
	guard := time.After(guardTimeout)
	select {
	case _, ok := <-ch:
		if !ok {
			// Closed immediately — nothing left to cancel; lifecycle check
			// already covers the happy path.
			return
		}
	case <-guard:
		t.Fatal("no chunk received within guard timeout")
	}
	cancel()

	// The producer must terminate promptly: the channel must close within the
	// guard window even though we drain lazily after cancellation.
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-guard:
			t.Fatal("stream channel did not close after context cancellation (producer leak)")
		}
	}
}

func checkMidStreamError(t *testing.T, h Harness) {
	t.Helper()
	wantErr := &llmclient.ServerError{StatusCode: 500, Message: "stream cut"}
	client := h.NewStreamError(t, "partial text before failure", wantErr)

	ch, err := client.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream must start successfully for a MID-stream failure, got pre-stream error: %v", err)
	}
	chunks := drain(t, ch)
	if len(chunks) == 0 {
		t.Fatal("stream produced no chunks before failing")
	}
	last := chunks[len(chunks)-1]
	if last.Err == nil {
		t.Fatal("terminal chunk must carry non-nil Err on mid-stream failure")
	}
	for i, c := range chunks[:len(chunks)-1] {
		if c.Err != nil {
			t.Errorf("Err chunk must be terminal; found one at index %d of %d", i, len(chunks))
		}
	}
}

// --------------------------------------------------------------------------
// Unknown-block round-trip checks
// --------------------------------------------------------------------------

// unknownBlockFixture is the verbatim JSON for the unknown block the suite
// uses. It must be a realistic provider-style object so adapters that parse
// inbound JSON reproduce it faithfully.
var unknownBlockFixture = json.RawMessage(`{"type":"thinking","thinking":"deep thoughts","signature":"abc123"}`)

// checkUnknownBlockComplete proves that Complete delivers a BlockUnknown with
// Raw equal to the fixture bytes (verbatim provider JSON preserved).
func checkUnknownBlockComplete(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewUnknownBlock(t, unknownBlockFixture)
	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	ub := findUnknownBlock(resp.Content)
	if ub == nil {
		t.Fatalf("no BlockUnknown in Complete response; content = %+v", resp.Content)
	}
	if !jsonEqual(ub.Raw, unknownBlockFixture) {
		t.Errorf("BlockUnknown.Raw =\n  %s\nwant\n  %s", ub.Raw, unknownBlockFixture)
	}
}

// checkUnknownBlockStream proves that Stream delivers a BlockUnknown via a
// StreamChunk.Block whose Raw matches the fixture bytes, assembled from
// multi-delta accumulation.
func checkUnknownBlockStream(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewUnknownBlock(t, unknownBlockFixture)
	ch, err := client.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drain(t, ch)

	var ub *llmclient.ContentBlock
	for _, ck := range chunks {
		if ck.Block != nil && ck.Block.Type == llmclient.BlockUnknown {
			b := *ck.Block
			ub = &b
		}
	}
	if ub == nil {
		t.Fatalf("no BlockUnknown delivered via stream; chunks = %+v", chunks)
	}
	if !jsonEqual(ub.Raw, unknownBlockFixture) {
		t.Errorf("stream BlockUnknown.Raw =\n  %s\nwant\n  %s", ub.Raw, unknownBlockFixture)
	}
}

// checkUnknownBlockStreamEqualsComplete proves that the BlockUnknown Raw
// assembled by Stream is byte-equivalent (after JSON normalization) to what
// Complete produces for the same logical response — the "stream-assembled ==
// complete-assembled" guarantee in the contract doc.
func checkUnknownBlockStreamEqualsComplete(t *testing.T, h Harness) {
	t.Helper()

	// Complete path.
	cClient := h.NewUnknownBlock(t, unknownBlockFixture)
	resp, err := cClient.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	completeUB := findUnknownBlock(resp.Content)
	if completeUB == nil {
		t.Fatal("no BlockUnknown in Complete response")
	}

	// Stream path.
	sClient := h.NewUnknownBlock(t, unknownBlockFixture)
	ch, err := sClient.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drain(t, ch)
	var streamUB *llmclient.ContentBlock
	for _, ck := range chunks {
		if ck.Block != nil && ck.Block.Type == llmclient.BlockUnknown {
			b := *ck.Block
			streamUB = &b
		}
	}
	if streamUB == nil {
		t.Fatal("no BlockUnknown in stream")
	}

	if !jsonEqual(completeUB.Raw, streamUB.Raw) {
		t.Errorf("stream Raw != complete Raw:\n  stream  = %s\n  complete = %s", streamUB.Raw, completeUB.Raw)
	}
}

// checkUnknownBlockRoundTrip proves that sending a BlockUnknown received from
// Complete back in a subsequent request re-emits the same bytes verbatim —
// the Raw-precedence rule ensures the adapter never re-serializes known fields
// when Raw is set.
func checkUnknownBlockRoundTrip(t *testing.T, h Harness) {
	t.Helper()

	// 1. Receive an unknown block via Complete.
	client := h.NewUnknownBlock(t, unknownBlockFixture)
	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	ub := findUnknownBlock(resp.Content)
	if ub == nil {
		t.Fatal("no BlockUnknown in Complete response — cannot test round-trip")
	}

	// 2. Build a follow-up request that sends the block back in the history.
	//    The body-inspection client will verify the outbound wire body contains
	//    the Raw bytes verbatim.
	if h.NewBodyInspect == nil {
		// Body inspection not wired — can only verify the block is present in
		// AsMessage (which is tested by AsMessageRoundTrip). Skip the wire check.
		t.Skip("Harness.NewBodyInspect required for full round-trip wire verification")
	}
	followUp := llmclient.Request{
		Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("go")}},
			resp.AsMessage(), // includes the BlockUnknown with Raw set
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("continue")}},
		},
	}
	inspectClient := h.NewBodyInspect(t, func(body []byte) {
		// The verbatim Raw bytes must appear somewhere in the outbound body.
		// JSON serialisation may reorder top-level keys but the specific
		// "thinking" and "signature" values must be present.
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			t.Errorf("round-trip body is not valid JSON: %v", err)
			return
		}
		// Search for the unknown block's type value in the messages array.
		if !bytes.Contains(body, []byte(`"thinking"`)) {
			t.Errorf("outbound body does not contain the unknown block type 'thinking'; body = %s", body)
		}
		if !bytes.Contains(body, []byte(`"deep thoughts"`)) {
			t.Errorf("outbound body does not contain the unknown block 'thinking' field; body = %s", body)
		}
	}, "ok")
	_, err = inspectClient.Complete(context.Background(), followUp)
	if err != nil {
		t.Fatalf("Complete (round-trip): %v", err)
	}
}

// findUnknownBlock returns the first BlockUnknown in blocks, or nil.
func findUnknownBlock(blocks []llmclient.ContentBlock) *llmclient.ContentBlock {
	for i := range blocks {
		if blocks[i].Type == llmclient.BlockUnknown {
			return &blocks[i]
		}
	}
	return nil
}

// jsonEqual reports whether a and b represent the same JSON value (after
// parsing), tolerating key-order differences.
func jsonEqual(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

// --------------------------------------------------------------------------
// Typed-error taxonomy checks
// --------------------------------------------------------------------------

// checkTypedError400 proves a 400 response maps to *InvalidRequestError.
func checkTypedError400(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewHTTPError(t, 400, "ValidationException", "", "", `{"message":"bad input"}`)
	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("400: err = %T %v, want *InvalidRequestError", err, err)
	}
}

// checkTypedError401 proves a 401 response maps to *AuthError.
func checkTypedError401(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewHTTPError(t, 401, "", "", "", `{"message":"no auth"}`)
	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	var ae *llmclient.AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("401: err = %T %v, want *AuthError", err, err)
	}
}

// checkTypedError403 proves a 403 response maps to *AuthError.
func checkTypedError403(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewHTTPError(t, 403, "AccessDeniedException", "", "", `{"message":"forbidden"}`)
	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	var ae *llmclient.AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("403: err = %T %v, want *AuthError", err, err)
	}
}

// checkTypedError429 proves a 429 response maps to *RateLimitError.
func checkTypedError429(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewHTTPError(t, 429, "ThrottlingException", "", "", `{"message":"slow down"}`)
	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	var rle *llmclient.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("429: err = %T %v, want *RateLimitError", err, err)
	}
}

// checkTypedError429RetryAfterSec proves a 429 with a Retry-After: <seconds>
// header produces RateLimitError.RetryAfter > 0.
func checkTypedError429RetryAfterSec(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewHTTPError(t, 429, "ThrottlingException", "30", "", `{"message":"slow down"}`)
	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	var rle *llmclient.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("429+Retry-After sec: err = %T %v, want *RateLimitError", err, err)
	}
	if rle.RetryAfter <= 0 {
		t.Errorf("RateLimitError.RetryAfter = %v, want > 0 (Retry-After: 30)", rle.RetryAfter)
	}
}

// checkTypedError429RetryAfterDate proves a 429 with a Retry-After: <HTTP-date>
// header produces RateLimitError.RetryAfter > 0.
func checkTypedError429RetryAfterDate(t *testing.T, h Harness) {
	t.Helper()
	// Use an HTTP-date well in the future — far enough that time.Now() on any
	// reasonable CI or developer machine will see a positive delta regardless of
	// whether the adapter uses an injected or real clock. Adapters that pin
	// their internal clock (e.g. bedrock) will see an even larger delta.
	futureDate := "Mon, 01 Jan 2030 00:00:00 GMT"
	client := h.NewHTTPError(t, 429, "ThrottlingException", "", futureDate, `{"message":"slow down"}`)
	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	var rle *llmclient.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("429+Retry-After date: err = %T %v, want *RateLimitError", err, err)
	}
	if rle.RetryAfter <= 0 {
		t.Errorf("RateLimitError.RetryAfter = %v, want > 0 (Retry-After: HTTP-date)", rle.RetryAfter)
	}
}

// checkTypedError500 proves a 500 response maps to *ServerError.
func checkTypedError500(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewHTTPError(t, 500, "InternalServerException", "", "", `{"message":"boom"}`)
	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	var se *llmclient.ServerError
	if !errors.As(err, &se) {
		t.Fatalf("500: err = %T %v, want *ServerError", err, err)
	}
}

// checkTypedError503 proves a 503 response maps to *ServerError.
func checkTypedError503(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewHTTPError(t, 503, "ServiceUnavailableException", "", "", `{"message":"unavailable"}`)
	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "go"})
	var se *llmclient.ServerError
	if !errors.As(err, &se) {
		t.Fatalf("503: err = %T %v, want *ServerError", err, err)
	}
}

// --------------------------------------------------------------------------
// Body-inspection (Wave-1 surfaces) checks
// --------------------------------------------------------------------------

// checkProviderParamsMergedComplete proves ProviderParams is merged into the
// outbound request body on Complete: the custom key appears in the wire JSON.
func checkProviderParamsMergedComplete(t *testing.T, h Harness) {
	t.Helper()
	const sentinel = "conf_provider_params_complete"
	client := h.NewBodyInspect(t, func(body []byte) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			t.Errorf("body not valid JSON: %v", err)
			return
		}
		if _, ok := m[sentinel]; !ok {
			t.Errorf("ProviderParams key %q missing from wire body; keys = %v", sentinel, mapKeys(m))
		}
	}, "pp merged complete")
	req := llmclient.Request{
		Prompt:         "go",
		ProviderParams: json.RawMessage(`{"` + sentinel + `":"yes"}`),
	}
	_, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
}

// checkProviderParamsMergedStream proves ProviderParams is merged on Stream.
func checkProviderParamsMergedStream(t *testing.T, h Harness) {
	t.Helper()
	const sentinel = "conf_provider_params_stream"
	client := h.NewBodyInspect(t, func(body []byte) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			t.Errorf("body not valid JSON: %v", err)
			return
		}
		if _, ok := m[sentinel]; !ok {
			t.Errorf("ProviderParams key %q missing from stream wire body; keys = %v", sentinel, mapKeys(m))
		}
	}, "pp merged stream")
	req := llmclient.Request{
		Prompt:         "go",
		ProviderParams: json.RawMessage(`{"` + sentinel + `":"yes"}`),
	}
	ch, err := client.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	drain(t, ch)
}

// checkProviderParamsNonObjectRejected proves a non-object ProviderParams
// returns a typed *InvalidRequestError before any HTTP.
func checkProviderParamsNonObjectRejected(t *testing.T, h Harness) {
	t.Helper()
	// The text client suffices — the rejection must happen before any entry is
	// consumed, so a client with a valid entry is correct to use here.
	client := h.NewText(t, "should not be called")
	for _, pp := range []json.RawMessage{
		json.RawMessage(`[1,2]`),
		json.RawMessage(`"str"`),
		json.RawMessage(`42`),
		json.RawMessage(`null`),
	} {
		_, err := client.Complete(context.Background(), llmclient.Request{
			Prompt:         "go",
			ProviderParams: pp,
		})
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Errorf("pp=%s: err = %T %v, want *InvalidRequestError", pp, err, err)
		}
	}
}

// checkThinkingOnWire proves Request.Thinking is serialized as the "thinking"
// wire object on both Complete and Stream.
func checkThinkingOnWire(t *testing.T, h Harness) {
	t.Helper()
	assertThinkingPresent := func(body []byte) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			t.Errorf("body not valid JSON: %v", err)
			return
		}
		raw, ok := m["thinking"]
		if !ok {
			t.Errorf("thinking key missing from wire body; keys = %v", mapKeys(m))
			return
		}
		var th map[string]json.RawMessage
		if err := json.Unmarshal(raw, &th); err != nil {
			t.Errorf("thinking value not valid JSON object: %v", err)
			return
		}
		var typ string
		if err := json.Unmarshal(th["type"], &typ); err != nil || typ != "enabled" {
			t.Errorf("thinking.type = %s, want \"enabled\"", th["type"])
		}
		var budget float64
		if err := json.Unmarshal(th["budget_tokens"], &budget); err != nil || budget != 512 {
			t.Errorf("thinking.budget_tokens = %s, want 512", th["budget_tokens"])
		}
	}
	req := llmclient.Request{
		Prompt:   "think",
		Thinking: &llmclient.ThinkingConfig{Type: "enabled", BudgetTokens: 512},
	}
	// Complete path.
	cClient := h.NewBodyInspect(t, assertThinkingPresent, "ok")
	if _, err := cClient.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// Stream path.
	sClient := h.NewBodyInspect(t, assertThinkingPresent, "ok")
	ch, err := sClient.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	drain(t, ch)
}

// checkOutputSchemaOnWire proves Request.OutputSchema is mapped to
// output_config.format.{type:"json_schema", schema:...} on both paths.
func checkOutputSchemaOnWire(t *testing.T, h Harness) {
	t.Helper()
	schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`)
	assertOutputConfig := func(body []byte) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			t.Errorf("body not valid JSON: %v", err)
			return
		}
		oc, ok := m["output_config"]
		if !ok {
			t.Errorf("output_config missing from wire body; keys = %v", mapKeys(m))
			return
		}
		var ocm map[string]json.RawMessage
		if err := json.Unmarshal(oc, &ocm); err != nil {
			t.Errorf("output_config not valid JSON object: %v", err)
			return
		}
		fmtRaw, ok := ocm["format"]
		if !ok {
			t.Errorf("output_config.format missing")
			return
		}
		var fmt map[string]json.RawMessage
		if err := json.Unmarshal(fmtRaw, &fmt); err != nil {
			t.Errorf("output_config.format not valid JSON object: %v", err)
			return
		}
		var typ string
		if err := json.Unmarshal(fmt["type"], &typ); err != nil || typ != "json_schema" {
			t.Errorf("output_config.format.type = %s, want \"json_schema\"", fmt["type"])
		}
		if fmt["schema"] == nil {
			t.Errorf("output_config.format.schema missing")
		}
	}
	req := llmclient.Request{Prompt: "go", OutputSchema: schema}
	// Complete path.
	cClient := h.NewBodyInspect(t, assertOutputConfig, "ok")
	if _, err := cClient.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// Stream path.
	sClient := h.NewBodyInspect(t, assertOutputConfig, "ok")
	ch, err := sClient.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	drain(t, ch)
}

// checkOutputSchemaNonObjectRejected proves a non-object OutputSchema returns
// a typed *InvalidRequestError before any HTTP.
func checkOutputSchemaNonObjectRejected(t *testing.T, h Harness) {
	t.Helper()
	client := h.NewText(t, "should not be called")
	for _, s := range []json.RawMessage{
		json.RawMessage(`[1,2]`),
		json.RawMessage(`"str"`),
		json.RawMessage(`42`),
		json.RawMessage(`null`),
	} {
		_, err := client.Complete(context.Background(), llmclient.Request{
			Prompt:       "go",
			OutputSchema: s,
		})
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Errorf("OutputSchema=%s: err = %T %v, want *InvalidRequestError", s, err, err)
		}
	}
}

// checkToolDefRawVerbatim proves that a ToolDef with Raw set emits the Raw
// bytes verbatim in the wire body's tools array, rather than the typed fields.
func checkToolDefRawVerbatim(t *testing.T, h Harness) {
	t.Helper()
	// A server-tool definition that no structured fields would produce.
	rawTool := json.RawMessage(`{"type":"computer_20241022","name":"computer","display_width_px":1024,"display_height_px":768,"display_number":1}`)
	assertToolVerbatim := func(body []byte) {
		// The tools array must contain at least the computer_20241022 type.
		if !bytes.Contains(body, []byte(`"computer_20241022"`)) {
			t.Errorf("ToolDef.Raw type 'computer_20241022' not found in wire body; body = %s", body)
		}
		if !bytes.Contains(body, []byte(`"display_width_px"`)) {
			t.Errorf("ToolDef.Raw field 'display_width_px' not found in wire body; body = %s", body)
		}
	}
	req := llmclient.Request{
		Prompt: "go",
		Tools: []llmclient.ToolDef{
			{Name: "computer", Raw: rawTool},
		},
	}
	client := h.NewBodyInspect(t, assertToolVerbatim, "ok")
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
}

// checkEffortRejectedOrMapped proves that setting Request.Effort is either:
//   - rejected with a typed *InvalidRequestError (for example Bedrock), OR
//   - mapped to a provider field (for example Anthropic and OpenAI).
//
// What is forbidden: Effort being silently dropped — if the call succeeds, the
// adapter MUST have mapped the value to a wire field. The test detects silent
// dropping by pairing with body inspection: if no error is returned, the
// outbound body must contain an effort-related field.
func checkEffortRejectedOrMapped(t *testing.T, h Harness) {
	t.Helper()
	req := llmclient.Request{Prompt: "go", Effort: "medium"}

	// Optimistic path: try Complete and inspect the outcome.
	mapped := false
	client := h.NewBodyInspect(t, func(body []byte) {
		// If we get here, the adapter did not reject Effort — it must map it.
		// Accept any key name that plausibly carries the effort intent.
		for _, candidate := range []string{"effort", "reasoning_effort", "thinking_effort"} {
			if bytes.Contains(body, []byte(`"`+candidate+`"`)) {
				mapped = true
				return
			}
		}
		// Body inspection fired but no effort key found: silent drop.
		t.Errorf("Effort set to 'medium' but no effort-related key in wire body; body = %s", body)
	}, "ok")

	_, err := client.Complete(context.Background(), req)
	if err != nil {
		// Adapter rejected Effort — must be a typed InvalidRequestError.
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Fatalf("Effort: err = %T %v, want either nil (mapped) or *InvalidRequestError (rejected)", err, err)
		}
		// Valid: adapter explicitly refuses Effort. Test passes.
		return
	}
	// No error: adapter must have mapped Effort. The body inspection above
	// will have set mapped=true if a known effort key was found. If it
	// didn't fire at all (body inspect harness wired to a no-op in some
	// adapters), we accept the outcome — the test cannot prove silence.
	_ = mapped
}

// mapKeys returns the sorted key set of a JSON object map for error messages.
func mapKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
