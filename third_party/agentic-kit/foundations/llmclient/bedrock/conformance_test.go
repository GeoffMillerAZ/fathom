package bedrock

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llmtest "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/llm-test"
)

// isStreamRequest reports whether the captured request targeted the streaming
// endpoint (Bedrock selects streaming by URL path, not a body flag).
func isStreamRequest(r *http.Request) bool {
	return r.URL.Path[len(r.URL.Path)-len("/invoke-with-response-stream"):] == "/invoke-with-response-stream"
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

// exceptionTypeFor maps a desired typed error back to the Bedrock exception
// type that reproduces it.
func exceptionTypeFor(err error) (errType, msg string) {
	switch e := err.(type) {
	case *llmclient.ServerError:
		return "InternalServerException", e.Message
	case *llmclient.RateLimitError:
		return "ThrottlingException", e.Message
	case *llmclient.InvalidRequestError:
		return "ValidationException", e.Message
	case *llmclient.AuthError:
		return "AccessDeniedException", e.Message
	default:
		return "InternalServerException", err.Error()
	}
}

// bedrockHarness builds the full llmtest.Harness for the Bedrock adapter.
func bedrockHarness(t *testing.T) llmtest.Harness {
	t.Helper()
	return llmtest.Harness{
		NewText: func(t *testing.T, text string) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !isStreamRequest(r) {
					io.ReadAll(r.Body)
					writeJSON(w, textResponseBody(t, text))
					return
				}
				var frames [][]byte
				frames = append(
					frames,
					chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":1}}}`),
					chunkFrame(t, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
				)
				for _, piece := range runeChunks(text, 8) {
					frames = append(frames,
						chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+jstr(t, piece)+`}}`))
				}
				frames = append(
					frames,
					chunkFrame(t, `{"type":"content_block_stop","index":0}`),
					chunkFrame(t, `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`),
					chunkFrame(t, `{"type":"message_stop"}`),
				)
				writeFrames(t, w, frames...)
			}), Config{})
		},

		NewToolUse: func(t *testing.T, id, name string, input json.RawMessage) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !isStreamRequest(r) {
					io.ReadAll(r.Body)
					writeJSON(w, `{"content":[{"type":"tool_use","id":`+jstr(t, id)+`,"name":`+jstr(t, name)+`,"input":`+string(input)+`}],"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":4,"output_tokens":9}}`)
					return
				}
				in := string(input)
				half := len(in) / 2
				writeFrames(
					t, w,
					chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":4,"output_tokens":1}}}`),
					chunkFrame(t, `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":`+jstr(t, id)+`,"name":`+jstr(t, name)+`,"input":{}}}`),
					chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":`+jstr(t, in[:half])+`}}`),
					chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":`+jstr(t, in[half:])+`}}`),
					chunkFrame(t, `{"type":"content_block_stop","index":0}`),
					chunkFrame(t, `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`),
					chunkFrame(t, `{"type":"message_stop"}`),
				)
			}), Config{})
		},

		NewStreamError: func(t *testing.T, partial string, err error) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				frames := [][]byte{
					chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":1}}}`),
					chunkFrame(t, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
				}
				for _, piece := range runeChunks(partial, 8) {
					frames = append(frames,
						chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+jstr(t, piece)+`}}`))
				}
				errType, msg := exceptionTypeFor(err)
				frames = append(frames, exceptionFrame(t, errType, msg))
				writeFrames(t, w, frames...)
			}), Config{})
		},

		// NewUnknownBlock: serve a response containing the unknown block verbatim
		// on the Complete path, and using start+deltas+stop on the stream path to
		// exercise multi-delta accumulation equivalence.
		NewUnknownBlock: func(t *testing.T, rawBlock json.RawMessage) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !isStreamRequest(r) {
					io.ReadAll(r.Body)
					writeJSON(w, `{"content":[`+string(rawBlock)+`],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":3,"output_tokens":5}}`)
					return
				}
				// Stream path: emit the unknown (thinking) block via
				// start + thinking_delta + signature_delta + stop.
				writeFrames(
					t, w,
					chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":3,"output_tokens":1}}}`),
					chunkFrame(t, `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`),
					chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"deep thoughts"}}`),
					chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"abc123"}}`),
					chunkFrame(t, `{"type":"content_block_stop","index":0}`),
					chunkFrame(t, `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`),
					chunkFrame(t, `{"type":"message_stop"}`),
				)
			}), Config{})
		},

		// NewHTTPError: serve an HTTP error response with the given status,
		// Bedrock exception type header, optional Retry-After, and body.
		NewHTTPError: func(t *testing.T, statusCode int, errTypeHint, retryAfterSec, retryAfterHTTPDate, body string) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.ReadAll(r.Body)
				if errTypeHint != "" {
					w.Header().Set("X-Amzn-ErrorType", errTypeHint)
				}
				if retryAfterSec != "" {
					w.Header().Set("Retry-After", retryAfterSec)
				} else if retryAfterHTTPDate != "" {
					w.Header().Set("Retry-After", retryAfterHTTPDate)
				}
				w.WriteHeader(statusCode)
				io.WriteString(w, body)
			}), Config{})
		},

		// NewBodyInspect: capture the request body, call inspect, then serve the
		// appropriate success response for Complete or Stream.
		NewBodyInspect: func(t *testing.T, inspect func(body []byte), successText string) llmclient.LLMClient {
			return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				inspect(body)
				if isStreamRequest(r) {
					var frames [][]byte
					frames = append(
						frames,
						chunkFrame(t, `{"type":"message_start","message":{"usage":{"input_tokens":3,"output_tokens":1}}}`),
						chunkFrame(t, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
					)
					for _, piece := range runeChunks(successText, 8) {
						frames = append(frames,
							chunkFrame(t, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+jstr(t, piece)+`}}`))
					}
					frames = append(
						frames,
						chunkFrame(t, `{"type":"content_block_stop","index":0}`),
						chunkFrame(t, `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`),
						chunkFrame(t, `{"type":"message_stop"}`),
					)
					writeFrames(t, w, frames...)
					return
				}
				writeJSON(w, textResponseBody(t, successText))
			}), Config{})
		},
	}
}

// TestConformance runs the shared llmtest suite against this client backed by
// scripted httptest servers — the same checks every LLMClient passes.
func TestConformance(t *testing.T) {
	llmtest.RunConformance(t, bedrockHarness(t), llmtest.Capabilities{
		ToolUse:             true,
		MidStreamError:      true,
		UnknownBlock:        true,
		TypedErrors:         true,
		BodyInspection:      true,
		PreflightValidation: true,
	})
}
