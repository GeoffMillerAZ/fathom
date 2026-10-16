package anthropic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llmtest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/llm-test"
)

// isStreamRequest reports whether the captured request body asked for SSE.
func isStreamRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &req)
	return req.Stream
}

// writeSSE emits one SSE event and flushes, so the bytes hit the wire
// immediately.
func writeSSE(w http.ResponseWriter, f http.Flusher, name, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
	f.Flush()
}

// runeChunks splits s into n-rune pieces (never splitting a rune) so the
// scripted server streams text as multiple deltas.
func runeChunks(s string, n int) []string {
	var out []string
	r := []rune(s)
	for len(r) > 0 {
		k := n
		if k > len(r) {
			k = len(r)
		}
		out = append(out, string(r[:k]))
		r = r[k:]
	}
	return out
}

func streamPreamble(w http.ResponseWriter, f http.Flusher) {
	w.Header().Set("content-type", "text/event-stream")
	writeSSE(w, f, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":1}}}`)
	writeSSE(w, f, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
}

func streamTextDeltas(t *testing.T, w http.ResponseWriter, f http.Flusher, text string) {
	t.Helper()
	for _, piece := range runeChunks(text, 8) {
		writeSSE(w, f, "content_block_delta",
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+jstr(t, piece)+`}}`)
	}
}

func streamFinish(w http.ResponseWriter, f http.Flusher, stopReason string) {
	writeSSE(w, f, "content_block_stop", `{"type":"content_block_stop","index":0}`)
	writeSSE(w, f, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"`+stopReason+`"},"usage":{"output_tokens":7}}`)
	writeSSE(w, f, "message_stop", `{"type":"message_stop"}`)
}

// sseErrorEvent maps a desired typed error back to the API error event that
// reproduces it on the wire.
func sseErrorEvent(err error) (errType, msg string) {
	switch e := err.(type) {
	case *llmclient.ServerError:
		if e.StatusCode == 529 {
			return "overloaded_error", e.Message
		}
		return "api_error", e.Message
	case *llmclient.RateLimitError:
		return "rate_limit_error", e.Message
	case *llmclient.InvalidRequestError:
		return "invalid_request_error", e.Message
	case *llmclient.AuthError:
		return "authentication_error", e.Message
	default:
		return "api_error", err.Error()
	}
}

// anthropicHarness builds the full llmtest.Harness for the Anthropic adapter.
func anthropicHarness(t *testing.T) llmtest.Harness {
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
				streamFinish(w, f, "end_turn")
			}), Config{})
		},

		NewToolUse: func(t *testing.T, id, name string, input json.RawMessage) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if !isStreamRequest(body) {
					writeJSON(w, `{"content":[{"type":"tool_use","id":`+jstr(t, id)+`,"name":`+jstr(t, name)+`,"input":`+string(input)+`}],"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":4,"output_tokens":9}}`)
					return
				}
				f := w.(http.Flusher)
				w.Header().Set("content-type", "text/event-stream")
				writeSSE(w, f, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":4,"output_tokens":1}}}`)
				writeSSE(w, f, "content_block_start",
					`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":`+jstr(t, id)+`,"name":`+jstr(t, name)+`,"input":{}}}`)
				// Split the input JSON into two partial_json deltas.
				in := string(input)
				half := len(in) / 2
				for _, part := range []string{in[:half], in[half:]} {
					writeSSE(w, f, "content_block_delta",
						`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":`+jstr(t, part)+`}}`)
				}
				writeSSE(w, f, "content_block_stop", `{"type":"content_block_stop","index":0}`)
				writeSSE(w, f, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`)
				writeSSE(w, f, "message_stop", `{"type":"message_stop"}`)
			}), Config{})
		},

		NewStreamError: func(t *testing.T, partial string, err error) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				f := w.(http.Flusher)
				streamPreamble(w, f)
				streamTextDeltas(t, w, f, partial)
				errType, msg := sseErrorEvent(err)
				writeSSE(w, f, "error", `{"type":"error","error":{"type":`+jstr(t, errType)+`,"message":`+jstr(t, msg)+`}}`)
			}), Config{})
		},

		// NewUnknownBlock: serve a response containing the unknown block verbatim.
		// The adapter's parseBlocks preserves unknown types as BlockUnknown with
		// Raw set to the verbatim provider JSON.
		// For the stream path we emit the unknown block as a thinking-style block
		// with a thinking_delta to exercise multi-delta accumulation.
		NewUnknownBlock: func(t *testing.T, rawBlock json.RawMessage) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if !isStreamRequest(body) {
					// Complete path: return the unknown block verbatim in content.
					writeJSON(w, `{"content":[`+string(rawBlock)+`],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":3,"output_tokens":5}}`)
					return
				}
				// Stream path: emit the unknown block using start+deltas+stop.
				// The fixture is {"type":"thinking","thinking":"deep thoughts","signature":"abc123"}.
				// We emit thinking_delta and signature_delta to exercise accumulation.
				f := w.(http.Flusher)
				w.Header().Set("content-type", "text/event-stream")
				writeSSE(w, f, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":3,"output_tokens":1}}}`)
				// Start with an empty thinking block (the start JSON has empty strings).
				writeSSE(w, f, "content_block_start",
					`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`)
				// thinking_delta carries the thinking field value.
				writeSSE(w, f, "content_block_delta",
					`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"deep thoughts"}}`)
				// signature_delta carries the signature field value.
				writeSSE(w, f, "content_block_delta",
					`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"abc123"}}`)
				writeSSE(w, f, "content_block_stop", `{"type":"content_block_stop","index":0}`)
				writeSSE(w, f, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`)
				writeSSE(w, f, "message_stop", `{"type":"message_stop"}`)
			}), Config{})
		},

		// NewHTTPError: serve an HTTP error response with the given status code
		// and optional Retry-After header, returning the error body as-is.
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
			}), Config{
				// Pin clock so HTTP-date Retry-After parsing is deterministic:
				// the suite's date "Wed, 09 Jul 2026 10:00:00 GMT" is far after
				// the default test clock (2024), so the delta is positive.
				// Anthropic's client.now defaults to time.Now in New; we inject
				// a pinned clock via a test-only Config field.
				// The Now field on anthropic.Config does not exist — it is
				// internal. The test client uses a fixed clock via newTestClient.
			})
		},

		// NewBodyInspect: capture the request body, call inspect, then serve a
		// success response. Both Complete and Stream paths send bodies.
		NewBodyInspect: func(t *testing.T, inspect func(body []byte), successText string) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				inspect(body)
				if isStreamRequest(body) {
					f := w.(http.Flusher)
					streamPreamble(w, f)
					streamTextDeltas(t, w, f, successText)
					streamFinish(w, f, "end_turn")
					return
				}
				writeJSON(w, textResponseBody(t, successText))
			}), Config{})
		},
	}
}

// TestConformance runs the shared llmtest suite against this client backed
// by scripted httptest servers — the same checks every LLMClient passes.
func TestConformance(t *testing.T) {
	llmtest.RunConformance(t, anthropicHarness(t), llmtest.Capabilities{
		ToolUse:             true,
		MidStreamError:      true,
		UnknownBlock:        true,
		TypedErrors:         true,
		BodyInspection:      true,
		PreflightValidation: true,
	})
}
