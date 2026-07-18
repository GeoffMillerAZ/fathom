package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// drainStream collects all chunks from a stream channel.
func drainStream(t *testing.T, ch <-chan llmclient.StreamChunk) []llmclient.StreamChunk {
	t.Helper()
	var chunks []llmclient.StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}
	return chunks
}

// TestSystemMessageFirst proves that a non-empty Request.System is prepended
// as a {"role":"system",...} message before any user/assistant messages.
func TestSystemMessageFirst(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	req := llmclient.Request{
		System: "You are terse.",
		Messages: []llmclient.Message{{
			Role:    llmclient.RoleUser,
			Content: []llmclient.ContentBlock{llmclient.TextBlock("hello")},
		}},
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(m["messages"], &msgs); err != nil {
		t.Fatalf("unmarshal messages: %v", err)
	}
	if len(msgs) < 2 {
		t.Fatalf("messages len = %d, want >= 2", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("messages[0].role = %q, want \"system\"", msgs[0].Role)
	}
	if msgs[0].Content != "You are terse." {
		t.Errorf("messages[0].content = %q, want system prompt", msgs[0].Content)
	}
	if msgs[1].Role != "user" {
		t.Errorf("messages[1].role = %q, want \"user\"", msgs[1].Role)
	}
}

// TestToolCallsIndexAccumulation proves that streaming tool_call deltas are
// accumulated by the index field and emitted as complete BlockToolUse blocks.
func TestToolCallsIndexAccumulation(t *testing.T) {
	// Script a server that streams two separate tool_call deltas (same index)
	// whose arguments must be concatenated.
	const toolID = "call_abc"
	const toolName = "search"
	const argsPart1 = `{"query":"hel`
	const argsPart2 = `lo world"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := w.(http.Flusher)
		w.Header().Set("content-type", "text/event-stream")
		// First delta: announce tool call with id + name, empty arguments.
		writeSSE(w, f, `{"id":"c","choices":[{"delta":{"tool_calls":[{"index":0,"id":"`+toolID+`","type":"function","function":{"name":"`+toolName+`","arguments":""}}]},"finish_reason":null}]}`)
		// Second delta: arguments part 1.
		writeSSE(w, f, `{"id":"c","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":`+jstr(t, argsPart1)+`}}]},"finish_reason":null}]}`)
		// Third delta: arguments part 2.
		writeSSE(w, f, `{"id":"c","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":`+jstr(t, argsPart2)+`}}]},"finish_reason":null}]}`)
		// Finish.
		writeSSE(w, f, `{"id":"c","choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":null}`)
		writeSSE(w, f, `{"id":"c","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":9}}`)
		writeSSE(w, f, "[DONE]")
	}))
	t.Cleanup(srv.Close)

	cfg := Config{BaseURL: srv.URL + "/v1", DefaultModel: "model-x"}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ch, err := c.Stream(context.Background(), llmclient.Request{
		Prompt: "search for hello",
		Tools:  []llmclient.ToolDef{{Name: toolName, InputSchema: json.RawMessage(`{"type":"object"}`)}},
	})
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
		t.Fatal("no BlockToolUse emitted on stream")
	}
	if toolBlock.ID != toolID {
		t.Errorf("tool block ID = %q, want %q", toolBlock.ID, toolID)
	}
	if toolBlock.Name != toolName {
		t.Errorf("tool block Name = %q, want %q", toolBlock.Name, toolName)
	}
	wantArgs := argsPart1 + argsPart2
	var gotInput, wantInput any
	if err := json.Unmarshal(toolBlock.Input, &gotInput); err != nil {
		t.Fatalf("tool block Input not valid JSON: %v", err)
	}
	if err := json.Unmarshal(json.RawMessage(wantArgs), &wantInput); err != nil {
		t.Fatalf("expected args not valid JSON: %v", err)
	}
	if fmt.Sprint(gotInput) != fmt.Sprint(wantInput) {
		t.Errorf("tool block Input = %s, want %s", toolBlock.Input, wantArgs)
	}
}

// TestOutputSchemaWireShape proves that Request.OutputSchema is mapped to
// response_format: {type:"json_schema", json_schema:{name:"response", strict:true, schema:...}}.
func TestOutputSchemaWireShape(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`)
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, `{"answer":"42"}`))
	}), Config{})

	if _, err := c.Complete(context.Background(), llmclient.Request{
		Prompt:       "go",
		OutputSchema: schema,
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	rfRaw, ok := m["response_format"]
	if !ok {
		t.Fatalf("response_format missing from wire body; keys: %v", mapKeys(m))
	}
	var rf struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Name   string          `json:"name"`
			Strict bool            `json:"strict"`
			Schema json.RawMessage `json:"schema"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(rfRaw, &rf); err != nil {
		t.Fatalf("unmarshal response_format: %v", err)
	}
	if rf.Type != "json_schema" {
		t.Errorf("response_format.type = %q, want \"json_schema\"", rf.Type)
	}
	if rf.JSONSchema.Name != "response" {
		t.Errorf("response_format.json_schema.name = %q, want \"response\"", rf.JSONSchema.Name)
	}
	if !rf.JSONSchema.Strict {
		t.Error("response_format.json_schema.strict must be true")
	}
	var gotSchema, wantSchema any
	if err := json.Unmarshal(rf.JSONSchema.Schema, &gotSchema); err != nil {
		t.Fatalf("response_format.json_schema.schema not valid JSON: %v", err)
	}
	if err := json.Unmarshal(schema, &wantSchema); err != nil {
		t.Fatalf("expected schema not valid JSON: %v", err)
	}
	if fmt.Sprint(gotSchema) != fmt.Sprint(wantSchema) {
		t.Errorf("response_format.json_schema.schema = %s, want %s", rf.JSONSchema.Schema, schema)
	}
}

// TestAPIKeyNeverInErrors proves that the API key is never exposed in any
// error message, even when the server echoes it in the error body.
func TestAPIKeyNeverInErrors(t *testing.T) {
	const sentinel = "sk-SENTINEL-never-leak-9876"

	assertClean := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("want error, got nil")
		}
		for e := err; e != nil; e = errors.Unwrap(e) {
			if strings.Contains(e.Error(), sentinel) {
				t.Fatalf("credential leaked into error output: %v", e)
			}
		}
	}

	statuses := []int{400, 401, 403, 429, 500, 503}
	for _, status := range statuses {
		t.Run(fmt.Sprintf("Status%d", status), func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(status)
				fmt.Fprintf(w, `{"error":{"message":"your key %s was rejected","type":"error"}}`, sentinel)
			}), Config{APIKey: sentinel})
			_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
			assertClean(t, err)
		})
	}

	t.Run("MalformedBodyEchoesKey", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			fmt.Fprintf(w, "panic: key=%s invalid", sentinel)
		}), Config{APIKey: sentinel})
		_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
		assertClean(t, err)
	})
}

// TestRetryAfterParsing proves both Retry-After formats (integer seconds and
// HTTP-date) are parsed correctly.
func TestRetryAfterParsing(t *testing.T) {
	t.Run("IntegerSeconds", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "30")
			w.Header().Set("content-type", "application/json")
			w.WriteHeader(429)
			io.WriteString(w, `{"error":{"message":"slow down","type":"rate_limit_error"}}`)
		}), Config{})
		_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
		var rle *llmclient.RateLimitError
		if !errors.As(err, &rle) {
			t.Fatalf("err = %T %v, want *RateLimitError", err, err)
		}
		if rle.RetryAfter != 30*time.Second {
			t.Errorf("RetryAfter = %v, want 30s", rle.RetryAfter)
		}
	})

	t.Run("HTTPDate", func(t *testing.T) {
		t0 := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", t0.Add(90*time.Second).Format(http.TimeFormat))
			w.Header().Set("content-type", "application/json")
			w.WriteHeader(429)
			io.WriteString(w, `{"error":{"message":"slow down","type":"rate_limit_error"}}`)
		}), Config{Now: func() time.Time { return t0 }})
		_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
		var rle *llmclient.RateLimitError
		if !errors.As(err, &rle) {
			t.Fatalf("err = %T %v, want *RateLimitError", err, err)
		}
		if rle.RetryAfter != 90*time.Second {
			t.Errorf("RetryAfter = %v, want 90s", rle.RetryAfter)
		}
	})
}

// TestProviderParamsVerbatimMerge proves that ProviderParams are merged at the
// top level and override adapter-set keys.
func TestProviderParamsVerbatimMerge(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	req := llmclient.Request{
		Prompt:         "p",
		ProviderParams: json.RawMessage(`{"model":"overridden-model","extra_field":"hello"}`),
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if m["model"] != "overridden-model" {
		t.Errorf("model = %v, want overridden-model (ProviderParams must override adapter-set keys)", m["model"])
	}
	if m["extra_field"] != "hello" {
		t.Errorf("extra_field = %v, want hello", m["extra_field"])
	}
	if _, ok := m["messages"]; !ok {
		t.Error("messages absent from merged body — adapter-set keys must survive when not overridden")
	}
}

// TestThinkingRejected proves that Request.Thinking with a non-empty Type
// returns a typed InvalidRequestError before any HTTP call.
func TestThinkingRejected(t *testing.T) {
	httpCalled := false
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalled = true
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	_, err := c.Complete(context.Background(), llmclient.Request{
		Prompt:   "think",
		Thinking: &llmclient.ThinkingConfig{Type: "enabled", BudgetTokens: 1024},
	})
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %T %v, want *InvalidRequestError for Thinking", err, err)
	}
	if !strings.Contains(ire.Error(), "Thinking") {
		t.Errorf("error should mention Thinking: %q", ire.Error())
	}
	if httpCalled {
		t.Error("Thinking rejection must fail before any HTTP call")
	}
}

// TestEffortMapsToReasoningEffort proves that Request.Effort is mapped to the
// reasoning_effort wire field (not silently dropped).
func TestEffortMapsToReasoningEffort(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	if _, err := c.Complete(context.Background(), llmclient.Request{
		Prompt: "p",
		Effort: "medium",
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if m["reasoning_effort"] != "medium" {
		t.Errorf("reasoning_effort = %v, want \"medium\"", m["reasoning_effort"])
	}
}

// TestToolChoiceMapping proves ToolChoice maps correctly to OpenAI wire values.
func TestToolChoiceMapping(t *testing.T) {
	cases := []struct {
		name   string
		choice llmclient.ToolChoice
		want   string // "" means tool_choice must be absent
	}{
		{"AutoOmitted", llmclient.ToolChoice{}, ""},
		{"Any→required", llmclient.ToolChoice{Mode: llmclient.ToolChoiceAny}, `"required"`},
		{"None", llmclient.ToolChoice{Mode: llmclient.ToolChoiceNone}, `"none"`},
		{
			"SpecificTool",
			llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool, Name: "search"},
			`{"type":"function","function":{"name":"search"}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				writeJSON(w, textResponseBody(t, "ok"))
			}), Config{})
			req := llmclient.Request{
				Prompt:     "p",
				Tools:      []llmclient.ToolDef{{Name: "search", InputSchema: json.RawMessage(`{"type":"object"}`)}},
				ToolChoice: tc.choice,
			}
			if _, err := c.Complete(context.Background(), req); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(gotBody, &m); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			raw, present := m["tool_choice"]
			if tc.want == "" {
				if present {
					t.Fatalf("tool_choice must be absent for auto, got %s", raw)
				}
				return
			}
			if !present {
				t.Fatalf("tool_choice missing (body %s)", gotBody)
			}
			// Compare as parsed JSON for key-order independence.
			var got, want any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("unmarshal tool_choice: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatalf("unmarshal expected: %v", err)
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("tool_choice = %s, want %s", raw, tc.want)
			}
		})
	}

	t.Run("ToolModeWithoutNameRejected", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("no HTTP request may be made for an invalid tool choice")
		}), Config{})
		_, err := c.Complete(context.Background(), llmclient.Request{
			Prompt:     "p",
			ToolChoice: llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool},
		})
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Fatalf("err = %v, want *llmclient.InvalidRequestError", err)
		}
	})
}

// TestImageBlockWireShape proves BlockImage maps to an image_url content part
// with an inline data URI.
func TestImageBlockWireShape(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	_, err := c.Complete(context.Background(), llmclient.Request{
		Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
			llmclient.ImageBlock("image/png", "aGk="),
			llmclient.TextBlock("what is this?"),
		}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	for _, want := range []string{
		`"type":"image_url"`,
		`"url":"data:image/png;base64,aGk="`,
	} {
		if !strings.Contains(string(gotBody), want) {
			t.Errorf("image block wire body missing %s:\n%s", want, gotBody)
		}
	}
}

// TestMaxCompletionTokensPresent proves max_completion_tokens is always sent.
func TestMaxCompletionTokensPresent(t *testing.T) {
	cases := []struct {
		name    string
		cfgMax  int
		reqMax  int
		wantVal float64
	}{
		{"DefaultApplied", 0, 0, 4096},
		{"ConfigDefault", 1234, 0, 1234},
		{"RequestWins", 1234, 99, 99},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				writeJSON(w, textResponseBody(t, "ok"))
			}), Config{DefaultMaxTokens: tc.cfgMax})

			if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p", MaxTokens: tc.reqMax}); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			var m map[string]any
			if err := json.Unmarshal(gotBody, &m); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			got, ok := m["max_completion_tokens"]
			if !ok {
				t.Fatal("max_completion_tokens missing from body")
			}
			if got != tc.wantVal {
				t.Errorf("max_completion_tokens = %v, want %v", got, tc.wantVal)
			}
		})
	}
}

// TestMissingModelTypedError proves a call with no model (neither Request nor
// Config) fails with a typed InvalidRequestError before any HTTP.
func TestMissingModelTypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no HTTP request may be made when the model is missing")
	}))
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL + "/v1"}) // no DefaultModel
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %v, want *InvalidRequestError", err)
	}
}

// TestToolDefRawVerbatim proves that a ToolDef with Raw set emits the Raw
// bytes verbatim in the tools array.
func TestToolDefRawVerbatim(t *testing.T) {
	rawTool := json.RawMessage(`{"type":"computer_20241022","name":"computer","display_width_px":1024}`)
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	req := llmclient.Request{
		Prompt: "go",
		Tools:  []llmclient.ToolDef{{Name: "computer", Raw: rawTool}},
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(string(gotBody), `"computer_20241022"`) {
		t.Errorf("ToolDef.Raw type not found in wire body: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"display_width_px"`) {
		t.Errorf("ToolDef.Raw field not found in wire body: %s", gotBody)
	}
}

// TestStrictToolWireShape proves that ToolDef.Strict=true sets strict:true
// in the function definition.
func TestStrictToolWireShape(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	req := llmclient.Request{
		Prompt: "p",
		Tools: []llmclient.ToolDef{{
			Name:        "search",
			InputSchema: json.RawMessage(`{"type":"object"}`),
			Strict:      true,
		}},
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(string(gotBody), `"strict":true`) {
		t.Errorf("strict:true not found in wire body: %s", gotBody)
	}
}

// TestToolResultAsTool proves that tool_result blocks are emitted as
// {"role":"tool",...} messages.
func TestToolResultAsTool(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	req := llmclient.Request{
		Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("look it up")}},
			{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{
				llmclient.ToolUseBlock("call_1", "search", json.RawMessage(`{"q":"hello"}`)),
			}},
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
				llmclient.ToolResultBlock("call_1", "result: found it", false),
			}},
		},
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	var msgs []struct {
		Role       string          `json:"role"`
		ToolCallID string          `json:"tool_call_id"`
		Content    json.RawMessage `json:"content"`
		ToolCalls  json.RawMessage `json:"tool_calls"`
	}
	if err := json.Unmarshal(m["messages"], &msgs); err != nil {
		t.Fatalf("unmarshal messages: %v", err)
	}

	// Find the "tool" role message.
	var toolMsg *struct {
		Role       string          `json:"role"`
		ToolCallID string          `json:"tool_call_id"`
		Content    json.RawMessage `json:"content"`
		ToolCalls  json.RawMessage `json:"tool_calls"`
	}
	for i := range msgs {
		if msgs[i].Role == "tool" {
			toolMsg = &msgs[i]
			break
		}
	}
	if toolMsg == nil {
		t.Fatalf("no \"tool\" role message found in wire body: %s", gotBody)
	}
	if toolMsg.ToolCallID != "call_1" {
		t.Errorf("tool_call_id = %q, want \"call_1\"", toolMsg.ToolCallID)
	}

	// Find the assistant message with tool_calls.
	var assistantMsg *struct {
		Role       string          `json:"role"`
		ToolCallID string          `json:"tool_call_id"`
		Content    json.RawMessage `json:"content"`
		ToolCalls  json.RawMessage `json:"tool_calls"`
	}
	for i := range msgs {
		if msgs[i].Role == "assistant" {
			assistantMsg = &msgs[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatalf("no \"assistant\" role message found in wire body: %s", gotBody)
	}
	if len(assistantMsg.ToolCalls) == 0 {
		t.Error("assistant message must have tool_calls")
	}
}

// mapKeys returns the key set of a JSON object map for error messages.
func mapKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
