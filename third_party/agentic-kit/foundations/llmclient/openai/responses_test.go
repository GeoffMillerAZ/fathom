package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llmtest "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/llm-test"
)

func responseText(text string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}}})
	return b
}

func responseTool(id, name string, input json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"type": "function_call", "id": "item-" + id, "call_id": id, "name": name, "arguments": string(input), "status": "completed"})
	return b
}

func responseEnvelope(items ...json.RawMessage) string {
	b, _ := json.Marshal(map[string]any{"status": "completed", "output": items, "usage": map[string]any{"input_tokens": 10, "output_tokens": 7, "input_tokens_details": map[string]any{"cached_tokens": 3}}})
	return string(b)
}

func terminalEvent(body string) string {
	return `{"type":"response.completed","response":` + body + `}`
}

func responsesFixture(t *testing.T, body string, events ...string) *Client {
	t.Helper()
	return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("path = %s", r.URL.Path)
		}
		wire, _ := io.ReadAll(r.Body)
		if !isStreamRequest(wire) {
			writeJSON(w, body)
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		f := w.(http.Flusher)
		for _, event := range events {
			writeSSE(w, f, event)
		}
	}), Config{UseResponses: true})
}

func TestResponsesConformance(t *testing.T) {
	h := openaiHarness(t)
	h.NewText = func(t *testing.T, text string) llmclient.LLMClient {
		body := responseEnvelope(responseText(text))
		return responsesFixture(t, body, `{"type":"response.output_text.delta","output_index":0,"delta":`+jstr(t, text)+`}`, terminalEvent(body))
	}
	h.NewToolUse = func(t *testing.T, id, name string, input json.RawMessage) llmclient.LLMClient {
		body := responseEnvelope(responseTool(id, name, input))
		return responsesFixture(t, body, terminalEvent(body))
	}
	h.NewStreamError = func(t *testing.T, partial string, _ error) llmclient.LLMClient {
		return responsesFixture(t, "", `{"type":"response.output_text.delta","delta":`+jstr(t, partial)+`}`, `{"type":"error","code":"server_error","message":"fixture failure"}`)
	}
	// Reuse HTTP classification tests, but ensure they exercise the Responses
	// endpoint as well as its request builder.
	h.NewHTTPError = func(t *testing.T, status int, _ string, seconds, date, body string) llmclient.LLMClient {
		return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/responses" {
				t.Errorf("path %s", r.URL.Path)
			}
			if seconds != "" {
				w.Header().Set("Retry-After", seconds)
			} else if date != "" {
				w.Header().Set("Retry-After", date)
			}
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		}), Config{UseResponses: true})
	}
	llmtest.RunConformance(t, h, llmtest.Capabilities{ToolUse: true, MidStreamError: true, TypedErrors: true})
}

func TestResponsesRequestSemantics(t *testing.T) {
	req := llmclient.Request{Prompt: "ignored", System: "instructions", Model: "requested-model", MaxTokens: 123, Temperature: 0.4, Effort: "high", OutputSchema: json.RawMessage(`{"type":"object"}`), Tools: []llmclient.ToolDef{{Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}}, ToolChoice: llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool, Name: "lookup"}, Messages: []llmclient.Message{
		{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("question")}},
		{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{llmclient.TextBlock("checking"), llmclient.ToolUseBlock("call", "lookup", json.RawMessage(`{"q":1}`))}},
		{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.ToolResultBlock("call", "failed", true), llmclient.ImageBlock("image/png", "eA==")}},
	}}
	client, _ := New(Config{UseResponses: true})
	for _, stream := range []bool{false, true} {
		raw, err := client.buildResponsesBody(req, stream)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if body["model"] != "requested-model" || body["instructions"] != "instructions" || body["max_output_tokens"] != float64(123) || body["stream"] != stream || body["store"] != false || body["temperature"] != 0.4 {
			t.Fatalf("body %s", raw)
		}
		input := body["input"].([]any)
		if len(input) != 5 || input[0].(map[string]any)["content"] != "question" || input[2].(map[string]any)["call_id"] != "call" || input[3].(map[string]any)["output"] != "Error: failed" {
			t.Fatalf("input %v", input)
		}
		tool := body["tools"].([]any)[0].(map[string]any)
		if tool["strict"] != false || tool["type"] != "function" || tool["name"] != "lookup" || tool["function"] != nil {
			t.Fatalf("tool %v", tool)
		}
		if body["text"].(map[string]any)["format"].(map[string]any)["type"] != "json_schema" || body["reasoning"].(map[string]any)["effort"] != "high" {
			t.Fatalf("mapping %s", raw)
		}
	}
	// Prompt fallback, explicit Raw, and a trusted optional provider extension.
	req = llmclient.Request{Prompt: "question", ProviderParams: json.RawMessage(`{"metadata":{"purpose":"test"}}`), Tools: []llmclient.ToolDef{{Name: "native", Raw: json.RawMessage(`{"type":"web_search"}`)}}, Messages: []llmclient.Message{{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"reasoning","id":"r","summary":[]}`)}}}}}
	client, _ = New(Config{UseResponses: true, DefaultModel: "default"})
	raw, err := client.buildResponsesBody(req, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"model":"default"`) || !strings.Contains(string(raw), `"purpose":"test"`) || !strings.Contains(string(raw), `"type":"reasoning"`) || !strings.Contains(string(raw), `"type":"web_search"`) {
		t.Fatal(string(raw))
	}
	raw, err = client.buildResponsesBody(llmclient.Request{Prompt: "fallback"}, false)
	if err != nil || !strings.Contains(string(raw), `"content":"fallback"`) {
		t.Fatalf("%s %v", raw, err)
	}
}

func TestResponsesUnsupportedBeforeHTTP(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }), Config{UseResponses: true})
	cases := []llmclient.Request{
		{Thinking: &llmclient.ThinkingConfig{}}, {StopSequences: []string{"stop"}}, {SystemCacheHint: true}, {MaxTokens: -1},
		{ProviderParams: json.RawMessage(`[]`)}, {OutputSchema: json.RawMessage(`null`)},
		{Tools: []llmclient.ToolDef{{Name: "tool", CacheHint: true}}}, {Tools: []llmclient.ToolDef{{Name: "tool", Raw: json.RawMessage(`null`)}}},
		{Messages: []llmclient.Message{{Role: "system"}}}, {Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{{Type: llmclient.BlockText, CacheHint: true}}}}},
		{ToolChoice: llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool}},
	}
	for _, field := range []string{"model", "input", "max_output_tokens", "stream", "store", "instructions", "temperature", "reasoning", "text", "tools", "tool_choice", "previous_response_id", "conversation", "prompt", "truncation", "parallel_tool_calls", "max_tool_calls", "background", "stream_options"} {
		cases = append(cases, llmclient.Request{ProviderParams: json.RawMessage(`{"` + field + `":true}`)})
	}
	for i, req := range cases {
		for _, stream := range []bool{false, true} {
			var err error
			if stream {
				_, err = client.Stream(context.Background(), req)
			} else {
				_, err = client.Complete(context.Background(), req)
			}
			var invalid *llmclient.InvalidRequestError
			if !errors.As(err, &invalid) {
				t.Errorf("case %d stream %v: %v", i, stream, err)
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatal("unsupported request reached HTTP")
	}
	if _, err := New(Config{UseResponses: true, AzureDeployment: "deployment", BaseURL: "https://example.invalid"}); err == nil {
		t.Fatal("Azure routing accepted")
	}
}

func collectResponses(t *testing.T, c *Client) (llmclient.Response, error) {
	t.Helper()
	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "question"})
	if err != nil {
		return llmclient.Response{}, err
	}
	var resp llmclient.Response
	terminal := false
	for chunk := range ch {
		if terminal {
			t.Fatal("chunk after terminal")
		}
		if chunk.Err != nil {
			err = chunk.Err
			terminal = true
		}
		if chunk.TextDelta != "" {
			resp.Text += chunk.TextDelta
		}
		if chunk.Block != nil {
			resp.Content = append(resp.Content, *chunk.Block)
		}
		if chunk.StopReason != "" {
			resp.StopReason = chunk.StopReason
			terminal = true
			if chunk.Usage == nil {
				t.Fatal("missing usage")
			}
			resp.Usage = *chunk.Usage
		}
	}
	if !terminal {
		t.Fatal("stream missing terminal")
	}
	return resp, err
}

func TestResponsesInterleavedFunctionsAndFinalResult(t *testing.T) {
	a := responseTool("a", "alpha", json.RawMessage(`{"a":1}`))
	b := responseTool("b", "beta", json.RawMessage(`{"b":2}`))
	tail := responseText("tail")
	body := responseEnvelope(a, b, tail)
	c := responsesFixture(t, body,
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"item-a","call_id":"a","name":"alpha","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","output_index":1,"item_id":"item-b","delta":"{\"b\":"}`,
		`{"type":"response.function_call_arguments.delta","output_index":0,"item_id":"item-a","delta":"{\"a\":1}"}`,
		`{"type":"response.output_item.done","output_index":1,"item":`+string(b)+`}`,
		`{"type":"response.output_text.delta","output_index":2,"delta":"tail"}`,
		`{"type":"response.output_item.done","output_index":0,"item":`+string(a)+`}`,
		terminalEvent(body))
	complete, err := c.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := collectResponses(t, c)
	if err != nil {
		t.Fatal(err)
	}
	if stream.Text != complete.Text || !reflect.DeepEqual(stream.ToolCalls(), complete.ToolCalls()) || stream.StopReason != complete.StopReason || stream.Usage != complete.Usage {
		t.Fatalf("stream %+v complete %+v", stream, complete)
	}
	if complete.InputTokens != 7 || complete.CacheReadInputTokens != 3 || complete.OutputTokens != 7 {
		t.Fatalf("usage %+v", complete.Usage)
	}
}

func TestResponsesRefusalTruncationAndMalformed(t *testing.T) {
	refusal := `{"type":"message","content":[{"type":"refusal","refusal":"cannot comply"}]}`
	base := responseEnvelope(responseText("partial"))
	cases := []struct {
		name, body string
		stop       llmclient.StopReason
		bad        bool
	}{
		{"refusal", responseEnvelope(json.RawMessage(refusal)), llmclient.StopRefusal, false},
		{"truncated", strings.Replace(base, `"status":"completed"`, `"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}`, 1), llmclient.StopMaxTokens, false},
		{"filter", strings.Replace(base, `"status":"completed"`, `"status":"incomplete","incomplete_details":{"reason":"content_filter"}`, 1), llmclient.StopRefusal, false},
		{"malformed", `{`, "", true}, {"nonterminal", strings.Replace(base, "completed", "in_progress", 1), "", true},
		{"no usage", `{"status":"completed","output":[]}`, "", true},
		{"failed", `{"status":"failed","error":{"code":"server_error","message":"failed"}}`, "", true},
		{"bad arguments", responseEnvelope(responseTool("a", "tool", json.RawMessage(`{`))), "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			typeName := "response.completed"
			if strings.Contains(tc.body, `"status":"incomplete"`) {
				typeName = "response.incomplete"
			}
			c := responsesFixture(t, tc.body, `{"type":"`+typeName+`","response":`+tc.body+`}`)
			for _, stream := range []bool{false, true} {
				var resp llmclient.Response
				var err error
				if stream {
					resp, err = collectResponses(t, c)
				} else {
					resp, err = c.Complete(context.Background(), llmclient.Request{})
				}
				if tc.bad {
					var server *llmclient.ServerError
					if !errors.As(err, &server) {
						t.Fatalf("stream %v error %v", stream, err)
					}
				} else if err != nil || resp.StopReason != tc.stop {
					t.Fatalf("stream %v: %+v %v", stream, resp, err)
				}
			}
		})
	}
}

// TestResponsesToolsRequireTerminalCompletedStatus proves malformed or partial
// function records never surface as executable callbacks in either lane.
func TestResponsesToolsRequireTerminalCompletedStatus(t *testing.T) {
	for _, status := range []string{"", "in_progress", "failed"} {
		t.Run("status-"+status, func(t *testing.T) {
			item := json.RawMessage(`{"type":"function_call","call_id":"call","name":"tool","status":"` + status + `","arguments":"{}"}`)
			body := responseEnvelope(item)
			for _, streaming := range []bool{false, true} {
				c := responsesFixture(t, body, `{"type":"response.completed","response":`+body+`}`)
				var resp llmclient.Response
				var err error
				if streaming {
					resp, err = collectResponses(t, c)
				} else {
					resp, err = c.Complete(context.Background(), llmclient.Request{})
				}
				var server *llmclient.ServerError
				if !errors.As(err, &server) || len(resp.ToolCalls()) != 0 {
					t.Fatalf("stream %v: callbacks=%v err=%v", streaming, resp.ToolCalls(), err)
				}
			}
		})
	}
}

// TestResponsesStreamWithholdsToolsUntilTerminalMatch makes the dispatch
// boundary observable: a completed SSE item cannot become a BlockToolUse when
// the mandatory terminal response disagrees with it.
func TestResponsesStreamWithholdsToolsUntilTerminalMatch(t *testing.T) {
	early := responseTool("early", "tool", json.RawMessage(`{"q":"early"}`))
	final := responseTool("final", "tool", json.RawMessage(`{"q":"final"}`))
	body := responseEnvelope(final)
	c := responsesFixture(t, body,
		`{"type":"response.output_item.done","output_index":0,"item":`+string(early)+`}`,
		terminalEvent(body),
	)
	resp, err := collectResponses(t, c)
	var server *llmclient.ServerError
	if !errors.As(err, &server) {
		t.Fatalf("stream error = %v, want ServerError", err)
	}
	if len(resp.ToolCalls()) != 0 {
		t.Fatalf("mismatched stream exposed executable callbacks: %+v", resp.ToolCalls())
	}
}

func TestResponsesStreamFailuresAndFraming(t *testing.T) {
	body := responseEnvelope(responseText("hello"))
	cases := [][]string{
		{}, {`{"type":"response.output_text.delta","delta":"partial"}`}, {`{`}, {`{"type":"error","code":"server_error","message":"failure"}`},
		{`{"type":"response.failed","response":{"status":"failed","error":{"message":"failure"}}}`},
		{`{"type":"response.output_text.delta","delta":"wrong"}`, terminalEvent(body)},
		{`{"type":"response.output_item.done","output_index":0,"item":` + string(responseText("wrong")) + `}`, terminalEvent(body)},
	}
	for i, events := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			_, err := collectResponses(t, responsesFixture(t, body, events...))
			var server *llmclient.ServerError
			if !errors.As(err, &server) {
				t.Fatal(err)
			}
		})
	}
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, ": comment\r\nevent: response.completed\r\ndata:{\"type\":\"response.completed\",\r\ndata: \"response\":"+body+"}\r\n\r\n")
	}), Config{UseResponses: true})
	resp, err := collectResponses(t, c)
	if err != nil || resp.Text != "hello" {
		t.Fatalf("%+v %v", resp, err)
	}
}

func TestResponsesBoundsAndCancellation(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if stream {
					_, _ = io.WriteString(w, "data: ")
				}
				_, _ = io.CopyN(w, strings.NewReader(strings.Repeat("x", maxBodyBytes+1)), maxBodyBytes+1)
			}), Config{UseResponses: true})
			var err error
			if stream {
				_, err = collectResponses(t, c)
			} else {
				_, err = c.Complete(context.Background(), llmclient.Request{})
			}
			var server *llmclient.ServerError
			if !errors.As(err, &server) {
				t.Fatal(err)
			}
		})
	}
	for _, stream := range []bool{false, true} {
		t.Run("cancel-"+fmt.Sprint(stream), func(t *testing.T) {
			started := make(chan struct{})
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				_ = r.Body.Close()
				if stream {
					w.Header().Set("content-type", "text/event-stream")
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
			}), Config{UseResponses: true})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if stream {
					ch, err := c.Stream(ctx, llmclient.Request{})
					if err == nil {
						for range ch {
						}
					}
					done <- err
				} else {
					_, err := c.Complete(ctx, llmclient.Request{})
					done <- err
				}
			}()
			<-started
			cancel()
			err := <-done
			if !stream && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestResponsesPingAndProviderMetadata(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Method != "GET" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		writeJSON(w, `{"data":[]}`)
	}), Config{UseResponses: true})
	if err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if info := c.DescribeProvider(llmclient.Request{}); info.Family != "" || !info.Capabilities.Tools || !info.Capabilities.Refusal || info.Model != "model-x" {
		t.Fatalf("%+v", info)
	}
	public, _ := New(Config{DefaultModel: "default", UseResponses: true})
	info := public.DescribeProvider(llmclient.Request{Model: "requested", ProviderParams: json.RawMessage(`{"model":"override"}`)})
	if info.Family != "openai" || info.Model != "requested" {
		t.Fatalf("%+v", info)
	}
	compatible, _ := New(Config{BaseURL: "http://example.invalid", ProviderFamily: "family", DefaultModel: "custom"})
	if info := compatible.DescribeProvider(llmclient.Request{}); info.Family != "family" || info.Capabilities.Refusal {
		t.Fatalf("%+v", info)
	}
}

func TestResponsesUnknownRoundTripAndIncompleteTool(t *testing.T) {
	reasoning := json.RawMessage(`{"type":"reasoning","id":"reason","summary":[],"encrypted_content":"opaque"}`)
	body := responseEnvelope(reasoning, responseText("answer"))
	c := responsesFixture(t, body, terminalEvent(body))
	resp, err := c.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := collectResponses(t, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(stream.Content) != 1 || !reflect.DeepEqual(stream.Content[0], resp.Content[0]) || string(resp.Content[0].Raw) != string(reasoning) {
		t.Fatalf("lost reasoning: %+v %+v", resp, stream)
	}
	raw, err := c.buildResponsesBody(llmclient.Request{Messages: []llmclient.Message{resp.AsMessage()}}, false)
	if err != nil || !strings.Contains(string(raw), string(reasoning)) {
		t.Fatalf("roundtrip %s %v", raw, err)
	}
	partial := json.RawMessage(`{"type":"function_call","call_id":"call","name":"tool","status":"incomplete","arguments":"{"}`)
	body = strings.Replace(responseEnvelope(partial), `"status":"completed"`, `"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}`, 1)
	c = responsesFixture(t, body, `{"type":"response.incomplete","response":`+body+`}`)
	for _, streaming := range []bool{false, true} {
		if streaming {
			resp, err = collectResponses(t, c)
		} else {
			resp, err = c.Complete(context.Background(), llmclient.Request{})
		}
		if err != nil || resp.StopReason != llmclient.StopMaxTokens || len(resp.ToolCalls()) != 0 || len(resp.Content) != 1 || resp.Content[0].Type != llmclient.BlockUnknown {
			t.Fatalf("partial tool escaped: %+v %v", resp, err)
		}
	}
}

func TestResponsesAggregateBoundAndErrorRedaction(t *testing.T) {
	// Every individual event is small. Ignored lifecycle traffic must still
	// consume the aggregate cap, rather than permitting unlimited buffering.
	line := "data: {\"type\":\"response.in_progress\"}\n\n"
	reader := newSSEReader(strings.NewReader(strings.Repeat(line, maxBodyBytes/len(line)+1)))
	total := 0
	for {
		_, err := nextResponsesEvent(reader, &total)
		if err != nil {
			if !errors.Is(err, errSSETooLarge) {
				t.Fatal(err)
			}
			break
		}
	}
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeSSE(w, w.(http.Flusher), `{"type":"error","message":"test-credential"}`)
	}), Config{UseResponses: true, APIKey: "test-credential"})
	_, err := collectResponses(t, c)
	if err == nil || strings.Contains(err.Error(), "test-credential") {
		t.Fatalf("credential leaked: %v", err)
	}
	checked := false
	c, err = New(Config{UseResponses: true, DefaultModel: "default", CapabilityCheck: func(req llmclient.Request) error {
		checked = req.Model == "default"
		return responsesInvalid("model capability")
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Complete(context.Background(), llmclient.Request{})
	if !checked || err == nil {
		t.Fatalf("capability check: %v %v", checked, err)
	}
}
