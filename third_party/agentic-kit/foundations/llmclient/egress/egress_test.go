package llmegress_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llmegress "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/egress"
)

// marker wraps every transformed string in guillemets so tests can tell
// transformed text from raw text at a glance.
var marker = llmegress.TransformerFunc(func(_ context.Context, s string) (string, error) {
	return "«" + s + "»", nil
})

// errBoom is the sentinel a boom transformer fails with.
var errBoom = errors.New("boom")

// boom fails on any text containing "BOOM" and passes everything else
// through unchanged.
var boom = llmegress.TransformerFunc(func(_ context.Context, s string) (string, error) {
	if strings.Contains(s, "BOOM") {
		return "", errBoom
	}
	return s, nil
})

// buildReq constructs the all-fields request used by the golden and aliasing
// tests, with every field WithEgress must transform passed through w. Fields
// the gate must NOT touch (tool definitions, tool_use Input, BlockUnknown
// Raw, identifiers) are never wrapped.
func buildReq(w func(string) string) llmclient.Request {
	return llmclient.Request{
		Model:           "model-x",
		Prompt:          w("raw prompt"),
		System:          w("raw system"),
		SystemCacheHint: true,
		MaxTokens:       7,
		Temperature:     0.5,
		StopSequences:   []string{"stop1", "stop2"},
		ToolChoice:      llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool, Name: "lookup"},
		Tools: []llmclient.ToolDef{{
			Name:        "lookup",
			Description: "raw tool description",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
				{Type: llmclient.BlockText, Text: w("raw user text")},
				{Type: llmclient.BlockText, Text: w("second raw text"), CacheHint: true},
			}},
			{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{
				{Type: llmclient.BlockToolUse, ID: "tu_1", Name: "lookup", Input: json.RawMessage(`{"q":"raw query"}`)},
				{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking"}`)},
			}},
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
				{Type: llmclient.BlockToolResult, ToolUseID: "tu_1", Content: w("raw tool output"), IsError: true},
			}},
		},
	}
}

func identity(s string) string { return s }
func wrapped(s string) string  { return "«" + s + "»" }

// TestEveryFieldTransformed is the golden test: every outbound text-bearing
// field of the recorded request is transformed, and everything else —
// tool_use Input, BlockUnknown Raw, tool definitions, identifiers, knobs —
// is byte-for-byte preserved.
func TestEveryFieldTransformed(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, marker)

	if _, err := client.Complete(context.Background(), buildReq(identity)); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("inner calls = %d, want 1", len(calls))
	}
	got := calls[0].Request
	want := buildReq(wrapped)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("recorded request mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestTransformToolJSONOptIn proves the documented opt-in: with the
// TransformToolJSON option, tool_use Input and BlockUnknown Raw are
// transformed like any other field; tool definitions remain untouched.
func TestTransformToolJSONOptIn(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, marker, llmegress.TransformToolJSON())

	if _, err := client.Complete(context.Background(), buildReq(identity)); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got := mock.Calls()[0].Request
	if want := `«{"q":"raw query"}»`; string(got.Messages[1].Content[0].Input) != want {
		t.Errorf("tool_use Input = %q, want %q", got.Messages[1].Content[0].Input, want)
	}
	if want := `«{"type":"thinking"}»`; string(got.Messages[1].Content[1].Raw) != want {
		t.Errorf("BlockUnknown Raw = %q, want %q", got.Messages[1].Content[1].Raw, want)
	}
	if want := `{"type":"object"}`; string(got.Tools[0].InputSchema) != want {
		t.Errorf("ToolDef InputSchema = %q, want untouched %q", got.Tools[0].InputSchema, want)
	}
	if got.Tools[0].Description != "raw tool description" {
		t.Errorf("ToolDef Description = %q, want untouched", got.Tools[0].Description)
	}
}

// TestFailClosed pins the central guarantee for Complete and Stream: any
// Transformer error aborts the call with a typed *TransformError naming the
// offending field, and the inner client is NEVER invoked.
func TestFailClosed(t *testing.T) {
	toolUseMsg := func(input string) []llmclient.Message {
		return []llmclient.Message{{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{
			{Type: llmclient.BlockToolUse, ID: "tu", Name: "n", Input: json.RawMessage(input)},
		}}}
	}
	unknownMsg := func(raw string) []llmclient.Message {
		return []llmclient.Message{{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{
			{Type: llmclient.BlockUnknown, Raw: json.RawMessage(raw)},
		}}}
	}

	cases := []struct {
		name      string
		req       llmclient.Request
		opts      []llmegress.Option
		wantField string // "" means the call must succeed
	}{
		{
			name:      "prompt",
			req:       llmclient.Request{Prompt: "BOOM"},
			wantField: "Prompt",
		},
		{
			name:      "system",
			req:       llmclient.Request{Prompt: "ok", System: "BOOM"},
			wantField: "System",
		},
		{
			name: "text block in later message",
			req: llmclient.Request{Messages: []llmclient.Message{
				{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("fine")}},
				{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
					llmclient.TextBlock("fine too"),
					llmclient.TextBlock("BOOM"),
				}},
			}},
			wantField: "Messages[1].Content[1].Text",
		},
		{
			name: "tool_result content",
			req: llmclient.Request{Messages: []llmclient.Message{
				{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
					llmclient.ToolResultBlock("tu", "BOOM", false),
				}},
			}},
			wantField: "Messages[0].Content[0].Content",
		},
		{
			name:      "tool_use input with opt-in",
			req:       llmclient.Request{Messages: toolUseMsg(`"BOOM"`)},
			opts:      []llmegress.Option{llmegress.TransformToolJSON()},
			wantField: "Messages[0].Content[0].Input",
		},
		{
			name:      "unknown raw with opt-in",
			req:       llmclient.Request{Messages: unknownMsg(`"BOOM"`)},
			opts:      []llmegress.Option{llmegress.TransformToolJSON()},
			wantField: "Messages[0].Content[0].Raw",
		},
		{
			name: "tool_use input untouched by default",
			req:  llmclient.Request{Messages: toolUseMsg(`"BOOM"`)},
			// no opt-in: Input is structured JSON, not transformed, so the
			// boom transformer never sees it and the call succeeds.
			wantField: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []string{"complete", "stream"} {
				mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
				client := llmegress.WithEgress(mock, boom, tc.opts...)

				var err error
				switch mode {
				case "complete":
					_, err = client.Complete(context.Background(), tc.req)
				case "stream":
					var ch <-chan llmclient.StreamChunk
					ch, err = client.Stream(context.Background(), tc.req)
					if err == nil {
						for range ch { // drain per the streaming contract
						}
					} else if ch != nil {
						t.Errorf("%s: failed Stream returned a non-nil channel", mode)
					}
				}

				if tc.wantField == "" {
					if err != nil {
						t.Fatalf("%s: unexpected error: %v", mode, err)
					}
					if mock.CallCount() != 1 {
						t.Errorf("%s: inner calls = %d, want 1", mode, mock.CallCount())
					}
					continue
				}

				var te *llmegress.TransformError
				if !errors.As(err, &te) {
					t.Fatalf("%s: error = %v, want *TransformError", mode, err)
				}
				if te.Field != tc.wantField {
					t.Errorf("%s: Field = %q, want %q", mode, te.Field, tc.wantField)
				}
				if !errors.Is(err, errBoom) {
					t.Errorf("%s: errors.Is(err, errBoom) = false, want true", mode)
				}
				if mock.CallCount() != 0 {
					t.Errorf("%s: inner client invoked %d times after transform failure, want 0 (fail closed)",
						mode, mock.CallCount())
				}
			}
		})
	}
}

// TestDeepCopyNoAliasing proves the gated request is a deep copy in both
// directions: the wrapper never mutates the caller's request, mutating the
// recorded (gated) request cannot reach the caller's memory, and mutating
// the caller's request after the call cannot reach the recorded one.
// Snapshots use %#v dumps (not JSON) because the test deliberately corrupts
// json.RawMessage bytes.
func TestDeepCopyNoAliasing(t *testing.T) {
	dump := func(r llmclient.Request) string { return fmt.Sprintf("%#v", r) }

	req := buildReq(identity)
	callerBefore := dump(req)

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, marker)
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// 1. The wrapper must not have mutated the caller's request.
	if got := dump(req); got != callerBefore {
		t.Fatalf("caller's request mutated by WithEgress:\nbefore: %s\nafter:  %s", callerBefore, got)
	}

	// 2. Mutating everything reachable through the recorded request must not
	// reach the caller's request (the recorded slices alias the gated copy,
	// never caller memory).
	rec := mock.Calls()[0].Request
	rec.Messages[0].Content[0].Text = "mutated"
	rec.Messages[1].Content[0].Input[0] = 'X'
	rec.Messages[1].Content[1].Raw[0] = 'X'
	rec.StopSequences[0] = "mutated"
	rec.Tools[0].InputSchema[0] = 'X'
	if got := dump(req); got != callerBefore {
		t.Fatalf("mutating the gated request reached the caller's request:\nbefore: %s\nafter:  %s", callerBefore, got)
	}

	// 3. Mutating the caller's request must not reach the recorded one.
	// (mock.Calls() copies the records, but their slices alias the stored
	// gated request, so dumps observe any leak through shared memory.)
	recBefore := dump(mock.Calls()[0].Request)
	req.Messages[0].Content[0].Text = "caller-mutated"
	req.Messages[1].Content[0].Input[0] = 'Y'
	req.StopSequences[1] = "caller-mutated"
	req.Tools[0].InputSchema[0] = 'Y'
	if got := dump(mock.Calls()[0].Request); got != recBefore {
		t.Fatalf("mutating the caller's request reached the gated request:\nbefore: %s\nafter:  %s", recBefore, got)
	}
}

// TestStreamOutboundGatedInboundUntouched pins the streaming contract: the
// outbound request is transformed before delegation, and inbound chunks pass
// through verbatim — the model's text is never transformed on the way in.
func TestStreamOutboundGatedInboundUntouched(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("hello world"))
	client := llmegress.WithEgress(mock, marker)

	ch, err := client.Stream(context.Background(), llmclient.Request{Prompt: "raw prompt"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var text strings.Builder
	var terminal llmclient.StreamChunk
	for c := range ch {
		text.WriteString(c.TextDelta)
		if c.StopReason != "" || c.Err != nil {
			terminal = c
		}
	}
	if terminal.Err != nil {
		t.Fatalf("terminal chunk error: %v", terminal.Err)
	}
	if terminal.StopReason != llmclient.StopEndTurn {
		t.Errorf("StopReason = %q, want %q", terminal.StopReason, llmclient.StopEndTurn)
	}
	if got := text.String(); got != "hello world" {
		t.Errorf("inbound text = %q, want untouched %q", got, "hello world")
	}
	if got := mock.Calls()[0].Request.Prompt; got != "«raw prompt»" {
		t.Errorf("outbound prompt = %q, want %q", got, "«raw prompt»")
	}
}

// TestTransformerOncePerLogicalCallUnderRetry pins the composition decision:
// WithEgress(WithRetry(inner)) transforms once per LOGICAL call, and every
// retry attempt re-sends the already-gated request.
func TestTransformerOncePerLogicalCallUnderRetry(t *testing.T) {
	var invocations atomic.Int64
	counting := llmegress.TransformerFunc(func(_ context.Context, s string) (string, error) {
		invocations.Add(1)
		return "T(" + s + ")", nil
	})

	mock := llmclient.NewMockClient(
		llmclient.MockEntryRateLimit("slow down"),
		llmclient.MockEntryRateLimit("slow down"),
		llmclient.MockEntryOK("done"),
	)
	client := llmegress.WithEgress(
		llmclient.WithRetry(mock, llmclient.RetryConfig{MaxAttempts: 3}),
		counting,
	)

	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "p", System: "s"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", resp.Attempts)
	}
	// Two non-empty text fields (Prompt, System), each transformed exactly
	// once for the whole retried call — not once per attempt.
	if got := invocations.Load(); got != 2 {
		t.Errorf("transformer invocations = %d, want 2 (once per field per LOGICAL call)", got)
	}
	calls := mock.Calls()
	if len(calls) != 3 {
		t.Fatalf("inner attempts = %d, want 3", len(calls))
	}
	for i, c := range calls {
		if c.Request.Prompt != "T(p)" || c.Request.System != "T(s)" {
			t.Errorf("attempt %d saw Prompt=%q System=%q, want the already-gated request on every attempt",
				i+1, c.Request.Prompt, c.Request.System)
		}
	}
}

// TestEmptyFieldsSkipTransformer pins the documented empty-field policy:
// fields that carry no text never invoke the Transformer.
func TestEmptyFieldsSkipTransformer(t *testing.T) {
	var invocations atomic.Int64
	counting := llmegress.TransformerFunc(func(_ context.Context, s string) (string, error) {
		invocations.Add(1)
		return s, nil
	})

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, counting)

	req := llmclient.Request{ // Prompt and System empty
		Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
			llmclient.TextBlock("x"),                   // the only non-empty text field
			llmclient.ToolResultBlock("tu", "", false), // empty tool output
		}}},
	}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got := invocations.Load(); got != 1 {
		t.Errorf("transformer invocations = %d, want 1 (empty fields are skipped)", got)
	}
}

// TestWithEgressNilArgsPanic pins the construction contract: a nil inner
// client or nil Transformer is a wiring bug and panics immediately.
func TestWithEgressNilArgsPanic(t *testing.T) {
	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: expected panic, got none", name)
			}
		}()
		fn()
	}
	assertPanics("nil inner", func() { llmegress.WithEgress(nil, marker) })
	assertPanics("nil transformer", func() { llmegress.WithEgress(llmclient.NewMockClient(), nil) })
}

// TestConcurrentUse exercises the wrapper from many goroutines so the race
// detector can vouch for the documented concurrency contract.
func TestConcurrentUse(t *testing.T) {
	mock := llmclient.NewMockClient() // exhausted entries return the fallback response
	client := llmegress.WithEgress(mock, marker)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				if _, err := client.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
					t.Errorf("Complete: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if got := mock.CallCount(); got != 8*25 {
		t.Errorf("inner calls = %d, want %d", got, 8*25)
	}
}
