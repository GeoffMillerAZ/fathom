package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llmtest "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/llm-test"
)

// newTestClient spins up an httptest server around h and returns a Client
// pointed at it. Zero-valued cfg fields get test defaults.
func newTestClient(t *testing.T, h http.Handler, cfg Config) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cfg.BaseURL = srv.URL + "/v1"
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = "model-x"
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// jstr JSON-encodes s (for embedding text in fixture JSON).
func jstr(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal %q: %v", s, err)
	}
	return string(b)
}

// isStreamRequest reports whether the captured request body asked for SSE.
func isStreamRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &req)
	return req.Stream
}

// textResponseBody returns a minimal non-streaming success body.
func textResponseBody(t *testing.T, text string) string {
	t.Helper()
	return `{"id":"chatcmpl-test","object":"chat.completion","model":"model-x",` +
		`"choices":[{"index":0,"message":{"role":"assistant","content":` + jstr(t, text) + `},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":3,"completion_tokens":7}}`
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("content-type", "application/json")
	io.WriteString(w, body)
}

// writeSSE emits one SSE data line and flushes.
func writeSSE(w http.ResponseWriter, f http.Flusher, data string) {
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

// streamPreamble starts an SSE response with the usage event header.
func streamPreamble(w http.ResponseWriter, f http.Flusher) {
	w.Header().Set("content-type", "text/event-stream")
	// No preamble chunk needed for OpenAI — the first data line is already
	// a full delta event.
}

// streamTextDelta emits one or more text delta events.
func streamTextDeltas(t *testing.T, w http.ResponseWriter, f http.Flusher, text string) {
	t.Helper()
	// Split into rune chunks of 8 for multi-chunk streaming.
	r := []rune(text)
	for i := 0; i < len(r); i += 8 {
		end := i + 8
		if end > len(r) {
			end = len(r)
		}
		piece := string(r[i:end])
		writeSSE(w, f, `{"id":"c","choices":[{"delta":{"content":`+jstr(t, piece)+`},"finish_reason":null}]}`)
	}
}

// streamFinish emits the finish_reason and usage chunk, then [DONE].
func streamFinish(w http.ResponseWriter, f http.Flusher, stopReason string) {
	writeSSE(w, f, `{"id":"c","choices":[{"delta":{},"finish_reason":"`+stopReason+`"}],"usage":null}`)
	// Usage chunk (include_usage:true).
	writeSSE(w, f, `{"id":"c","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":7}}`)
	writeSSE(w, f, "[DONE]")
}

// openaiHarness builds the full llmtest.Harness for the OpenAI adapter.
func openaiHarness(t *testing.T) llmtest.Harness {
	t.Helper()
	return llmtest.Harness{
		NewText: func(t *testing.T, text string) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if !isStreamRequest(body) {
					writeJSON(w, textResponseBody(t, text))
					return
				}
				f := w.(http.Flusher)
				streamPreamble(w, f)
				streamTextDeltas(t, w, f, text)
				streamFinish(w, f, "stop")
			}), Config{})
		},

		NewToolUse: func(t *testing.T, id, name string, input json.RawMessage) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if !isStreamRequest(body) {
					writeJSON(w, `{"id":"c","object":"chat.completion","model":"model-x",`+
						`"choices":[{"index":0,"message":{"role":"assistant","content":null,`+
						`"tool_calls":[{"id":`+jstr(t, id)+`,"type":"function","function":{"name":`+jstr(t, name)+`,"arguments":`+jstr(t, string(input))+`}}]},`+
						`"finish_reason":"tool_calls"}],`+
						`"usage":{"prompt_tokens":4,"completion_tokens":9}}`)
					return
				}
				// Stream path: emit tool_call deltas.
				f := w.(http.Flusher)
				w.Header().Set("content-type", "text/event-stream")
				// Split input JSON into two halves for delta streaming.
				in := string(input)
				half := len(in) / 2
				writeSSE(w, f, `{"id":"c","choices":[{"delta":{"tool_calls":[{"index":0,"id":`+jstr(t, id)+`,"type":"function","function":{"name":`+jstr(t, name)+`,"arguments":""}}]},"finish_reason":null}]}`)
				writeSSE(w, f, `{"id":"c","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":`+jstr(t, in[:half])+`}}]},"finish_reason":null}]}`)
				writeSSE(w, f, `{"id":"c","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":`+jstr(t, in[half:])+`}}]},"finish_reason":null}]}`)
				writeSSE(w, f, `{"id":"c","choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":null}`)
				writeSSE(w, f, `{"id":"c","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":9}}`)
				writeSSE(w, f, "[DONE]")
			}), Config{})
		},

		NewStreamError: func(t *testing.T, partial string, err error) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				f := w.(http.Flusher)
				w.Header().Set("content-type", "text/event-stream")
				streamTextDeltas(t, w, f, partial)
				// Emit a malformed event to force a mid-stream ServerError: the
				// client's SSE parser will fail to unmarshal it and produce a
				// terminal Err chunk. OpenAI has no native error-event type, so
				// we send invalid JSON to trigger the "malformed SSE event" path.
				writeSSE(w, f, "NOT_VALID_JSON")
			}), Config{})
		},

		// NewUnknownBlock: OpenAI does not have native "thinking" blocks. We
		// return the raw block JSON as a text content string (the best available
		// fallback for an OpenAI-flavoured response). The conformance suite's
		// fixture is {"type":"thinking",...}; we embed it as text content so the
		// adapter parses it as a BlockText block — which is not a BlockUnknown.
		//
		// Since OpenAI has no unknown block protocol, we skip UnknownBlock in caps.
		NewUnknownBlock: nil,

		NewHTTPError: func(t *testing.T, statusCode int, _ /*errTypeHint*/, retryAfterSec, retryAfterHTTPDate, body string) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.ReadAll(r.Body)
				if retryAfterSec != "" {
					w.Header().Set("Retry-After", retryAfterSec)
				} else if retryAfterHTTPDate != "" {
					w.Header().Set("Retry-After", retryAfterHTTPDate)
				}
				w.WriteHeader(statusCode)
				io.WriteString(w, body)
			}), Config{})
		},

		NewBodyInspect: func(t *testing.T, inspect func(body []byte), successText string) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				inspect(body)
				if isStreamRequest(body) {
					f := w.(http.Flusher)
					w.Header().Set("content-type", "text/event-stream")
					streamTextDeltas(t, w, f, successText)
					streamFinish(w, f, "stop")
					return
				}
				writeJSON(w, textResponseBody(t, successText))
			}), Config{})
		},
	}
}

// TestConformance runs the shared llmtest suite against this client backed by
// scripted httptest servers.
func TestConformance(t *testing.T) {
	llmtest.RunConformance(t, openaiHarness(t), llmtest.Capabilities{
		ToolUse:        true,
		MidStreamError: true,
		UnknownBlock:   false, // OpenAI has no native unknown block protocol
		TypedErrors:    true,
		BodyInspection: false, // OpenAI uses different wire keys (reasoning_effort, response_format)
		// than the Anthropic-format checks in the harness (thinking, output_config);
		// OpenAI-specific wire shapes are verified in client_test.go instead.
		PreflightValidation: false, // cache hints not applicable to OpenAI
	})
}
