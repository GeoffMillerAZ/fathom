package claudecli_test

import (
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/claudecli"
	llmtest "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/llm-test"
)

// conformanceEnvelope is the `"type":"result"` envelope the fake shim prints
// for a successful completion whose result text is exactly text — the same
// shape successEnvelope pins, with the text marshaled through encoding/json
// so the suite's arbitrary strings survive verbatim. Usage is non-zero
// because a real envelope always carries token counts.
func conformanceEnvelope(t *testing.T, text string) string {
	t.Helper()
	env := map[string]any{
		"type":             "result",
		"subtype":          "success",
		"is_error":         false,
		"api_error_status": nil,
		"result":           text,
		"stop_reason":      "end_turn",
		"session_id":       "conformance",
		"total_cost_usd":   0.0,
		"usage": map[string]int{
			"input_tokens":                7,
			"output_tokens":               3,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens":     0,
		},
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal conformance envelope: %v", err)
	}
	return string(raw)
}

// TestClaudeCLIConformance runs llmtest.RunConformance's seven core checks
// (text completion, AsMessage round-trip, stop reason, stream lifecycle,
// stream cancellation, usage on Complete and on Stream) against a Client
// backed by the hermetic fake shim — no network, no real CLI, no credentials
// touched. Every optional capability is off; the t.Log line names each one
// and the adapter fact that keeps it off, so what the suite does and does
// not prove here is stated rather than implied.
func TestClaudeCLIConformance(t *testing.T) {
	t.Log("optional capabilities off — " +
		"ToolUse: every call runs --tools \"\" and Request.Tools is typed-rejected; " +
		"MidStreamError: Stream delegates to one blocking Complete, so no mid-stream failure mode exists; " +
		"UnknownBlock: the envelope carries one text result, never a provider-native block; " +
		"TypedErrors: api_error_status maps to the typed errors, but the suite also demands Retry-After parsing, which the envelope has no field for; " +
		"BodyInspection: there is no outbound wire body — a CLI shell-out has argv and stdin only; " +
		"PreflightValidation: cache hints are silently ignored, not counted against a limit")

	llmtest.RunConformance(t, llmtest.Harness{
		NewText: func(t *testing.T, text string) llmclient.LLMClient {
			c, _, _ := newHappyPathConfig(t, claudecli.Config{}, conformanceEnvelope(t, text))
			return c
		},
	}, llmtest.Capabilities{
		ToolUse:             false,
		MidStreamError:      false,
		UnknownBlock:        false,
		TypedErrors:         false,
		BodyInspection:      false,
		PreflightValidation: false,
	})
}
