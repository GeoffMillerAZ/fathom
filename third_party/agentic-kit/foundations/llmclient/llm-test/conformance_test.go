package llmtest_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llmtest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/llm-test"
)

// --------------------------------------------------------------------------
// mockBodyInspectClient
//
// A test-only LLMClient that serialises the typed request fields into a
// JSON object, calls an inspect function, then returns a canned success
// response.  This lets body-inspection suite cases exercise pre-flight
// validation (ProviderParams, OutputSchema, Effort) encoded in the mock
// without a real HTTP wire layer.
// --------------------------------------------------------------------------

type mockBodyInspectClient struct {
	inspect     func(body []byte)
	successText string
}

var _ llmclient.LLMClient = (*mockBodyInspectClient)(nil)

func (m *mockBodyInspectClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	if err := llmclient.ValidateProviderParams(req.ProviderParams, "mock-inspect"); err != nil {
		return llmclient.Response{}, err
	}
	if err := llmclient.ValidateOutputSchema(req.OutputSchema, "mock-inspect"); err != nil {
		return llmclient.Response{}, err
	}
	if req.Effort != "" {
		return llmclient.Response{}, &llmclient.InvalidRequestError{
			Message: "mock-inspect: Request.Effort set but mock has no effort knob",
		}
	}
	body := marshalInspectBody(req)
	if m.inspect != nil {
		m.inspect(body)
	}
	return llmclient.Response{
		Text:       m.successText,
		Content:    []llmclient.ContentBlock{llmclient.TextBlock(m.successText)},
		StopReason: llmclient.StopEndTurn,
		Usage:      llmclient.Usage{InputTokens: 3, OutputTokens: 7},
	}, nil
}

func (m *mockBodyInspectClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	if err := llmclient.ValidateProviderParams(req.ProviderParams, "mock-inspect"); err != nil {
		return nil, err
	}
	if err := llmclient.ValidateOutputSchema(req.OutputSchema, "mock-inspect"); err != nil {
		return nil, err
	}
	if req.Effort != "" {
		return nil, &llmclient.InvalidRequestError{
			Message: "mock-inspect: Request.Effort set but mock has no effort knob",
		}
	}
	body := marshalInspectBody(req)
	if m.inspect != nil {
		m.inspect(body)
	}
	ch := make(chan llmclient.StreamChunk, 2)
	u := llmclient.Usage{InputTokens: 3, OutputTokens: 7}
	ch <- llmclient.StreamChunk{TextDelta: m.successText}
	ch <- llmclient.StreamChunk{StopReason: llmclient.StopEndTurn, Usage: &u}
	close(ch)
	return ch, nil
}

// marshalInspectBody serialises a Request into a JSON object whose keys
// match what real provider adapters would put on the wire — good enough for
// the suite's key-presence assertions without importing a real adapter.
func marshalInspectBody(req llmclient.Request) []byte {
	type thinkingObj struct {
		Type         string `json:"type,omitempty"`
		BudgetTokens int    `json:"budget_tokens,omitempty"`
	}
	type toolObj struct {
		Name string          `json:"name"`
		Raw  json.RawMessage `json:"raw,omitempty"`
	}

	obj := map[string]interface{}{
		"prompt":   req.Prompt,
		"messages": req.Messages,
	}
	if len(req.ProviderParams) > 0 {
		// Merge provider params at top-level so key-presence checks work.
		var pp map[string]json.RawMessage
		if err := json.Unmarshal(req.ProviderParams, &pp); err == nil {
			for k, v := range pp {
				obj[k] = v
			}
		}
	}
	if len(req.OutputSchema) > 0 {
		obj["output_config"] = map[string]interface{}{
			"format": map[string]interface{}{
				"type":   "json_schema",
				"schema": req.OutputSchema,
			},
		}
	}
	if req.Thinking != nil && req.Thinking.Type != "" {
		obj["thinking"] = thinkingObj{
			Type:         req.Thinking.Type,
			BudgetTokens: req.Thinking.BudgetTokens,
		}
	}
	if len(req.Tools) > 0 {
		tools := make([]json.RawMessage, 0, len(req.Tools))
		for _, td := range req.Tools {
			if len(td.Raw) > 0 {
				tools = append(tools, td.Raw)
			} else {
				b, _ := json.Marshal(map[string]interface{}{
					"name":        td.Name,
					"description": td.Description,
				})
				tools = append(tools, b)
			}
		}
		obj["tools"] = tools
	}
	b, _ := json.Marshal(obj)
	return b
}

// --------------------------------------------------------------------------
// Harness builders
// --------------------------------------------------------------------------

// fullMockHarness returns a Harness covering all capabilities.
// TypedErrors routes through typed mock entries; BodyInspection uses
// mockBodyInspectClient above.
func fullMockHarness(t *testing.T) llmtest.Harness {
	t.Helper()
	return llmtest.Harness{
		NewText: func(t *testing.T, text string) llmclient.LLMClient {
			return llmclient.NewMockClient(llmclient.MockEntryOK(text))
		},
		NewToolUse: func(t *testing.T, id, name string, input json.RawMessage) llmclient.LLMClient {
			return llmclient.NewMockClient(llmclient.MockEntryToolUse(id, name, input))
		},
		NewStreamError: func(t *testing.T, partial string, err error) llmclient.LLMClient {
			return llmclient.NewMockClient(llmclient.MockEntryStreamErr(partial, err))
		},
		NewUnknownBlock: func(t *testing.T, rawBlock json.RawMessage) llmclient.LLMClient {
			return llmclient.NewMockClient(llmclient.MockEntryWithResponse(llmclient.Response{
				Content: []llmclient.ContentBlock{
					{Type: llmclient.BlockUnknown, Raw: rawBlock},
				},
				Usage: llmclient.Usage{InputTokens: 1, OutputTokens: 1},
			}))
		},
		// NewHTTPError: the mock cannot simulate an HTTP wire layer, so we
		// construct the appropriate typed error entry directly.
		NewHTTPError: func(t *testing.T, statusCode int, _ /*errTypeHint*/, retryAfterSec, retryAfterHTTPDate, _ /*body*/ string) llmclient.LLMClient {
			t.Helper()
			switch {
			case statusCode == 429:
				d := time.Duration(0)
				if retryAfterSec == "30" {
					d = 30 * time.Second
				} else if retryAfterHTTPDate != "" {
					// Harness supplies a date far in the future; use 1 h as a
					// stand-in so RetryAfter > 0.
					d = time.Hour
				}
				return llmclient.NewMockClient(llmclient.MockEntryErr(
					&llmclient.RateLimitError{Message: "rate limited", RetryAfter: d},
				))
			case statusCode == 401 || statusCode == 403:
				return llmclient.NewMockClient(llmclient.MockEntryErr(
					&llmclient.AuthError{Message: "auth error"},
				))
			case statusCode >= 500:
				return llmclient.NewMockClient(llmclient.MockEntryErr(
					&llmclient.ServerError{StatusCode: statusCode, Message: "server error"},
				))
			default: // 400 and other 4xx
				return llmclient.NewMockClient(llmclient.MockEntryErr(
					&llmclient.InvalidRequestError{Message: "invalid request"},
				))
			}
		},
		NewBodyInspect: func(t *testing.T, inspect func(body []byte), successText string) llmclient.LLMClient {
			return &mockBodyInspectClient{inspect: inspect, successText: successText}
		},
	}
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

// TestMockClientConformance proves the in-tree MockClient satisfies the full
// v2 contract, including the optional tool-use and mid-stream-error checks.
func TestMockClientConformance(t *testing.T) {
	llmtest.RunConformance(t, fullMockHarness(t), llmtest.Capabilities{
		ToolUse:             true,
		MidStreamError:      true,
		UnknownBlock:        true,
		TypedErrors:         true,
		BodyInspection:      true,
		PreflightValidation: true,
	})
}

// TestMockClientConformance_RetryWrapped proves the WithRetry decorator
// preserves conformance — wrapping must not change the contract.
func TestMockClientConformance_RetryWrapped(t *testing.T) {
	wrap := func(c llmclient.LLMClient) llmclient.LLMClient {
		return llmclient.WithRetry(c, llmclient.RetryConfig{MaxAttempts: 3})
	}
	llmtest.RunConformance(t, llmtest.Harness{
		NewText: func(t *testing.T, text string) llmclient.LLMClient {
			return wrap(llmclient.NewMockClient(llmclient.MockEntryOK(text)))
		},
		NewToolUse: func(t *testing.T, id, name string, input json.RawMessage) llmclient.LLMClient {
			return wrap(llmclient.NewMockClient(llmclient.MockEntryToolUse(id, name, input)))
		},
		NewStreamError: func(t *testing.T, partial string, err error) llmclient.LLMClient {
			return wrap(llmclient.NewMockClient(llmclient.MockEntryStreamErr(partial, err)))
		},
	}, llmtest.Capabilities{ToolUse: true, MidStreamError: true})
}

// TestMockClientConformance_SwappableWrapped proves the late-bound Swappable
// wrapper preserves conformance once a client is swapped in — the late-bound
// seam must be contract-invisible to everything downstream of it.
func TestMockClientConformance_SwappableWrapped(t *testing.T) {
	wrap := func(c llmclient.LLMClient) llmclient.LLMClient {
		s := llmclient.NewSwappable(nil) // starts unconfigured, like a fresh install...
		s.Swap(c)                        // ...then the credential arrives
		return s
	}
	llmtest.RunConformance(t, llmtest.Harness{
		NewText: func(t *testing.T, text string) llmclient.LLMClient {
			return wrap(llmclient.NewMockClient(llmclient.MockEntryOK(text)))
		},
		NewToolUse: func(t *testing.T, id, name string, input json.RawMessage) llmclient.LLMClient {
			return wrap(llmclient.NewMockClient(llmclient.MockEntryToolUse(id, name, input)))
		},
		NewStreamError: func(t *testing.T, partial string, err error) llmclient.LLMClient {
			return wrap(llmclient.NewMockClient(llmclient.MockEntryStreamErr(partial, err)))
		},
	}, llmtest.Capabilities{ToolUse: true, MidStreamError: true})
}
