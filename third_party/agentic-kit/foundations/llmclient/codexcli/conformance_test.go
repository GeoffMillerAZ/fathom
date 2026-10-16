package codexcli_test

import (
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/codexcli"
	llmtest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/llm-test"
)

// conformanceStream is the JSONL event stream the fake shim prints for a
// successful turn whose single agent_message text is exactly text — the same
// four-event shape singleMessageStream pins, with the text marshaled through
// encoding/json so the suite's arbitrary strings survive verbatim. Usage is
// non-zero because a real turn.completed always carries token counts.
func conformanceStream(t *testing.T, text string) string {
	t.Helper()
	item, err := json.Marshal(map[string]any{
		"type": "item.completed",
		"item": map[string]any{"id": "item_0", "type": "agent_message", "text": text},
	})
	if err != nil {
		t.Fatalf("marshal conformance agent_message: %v", err)
	}
	return `{"type":"thread.started","thread_id":"conformance"}` + "\n" +
		`{"type":"turn.started"}` + "\n" +
		string(item) + "\n" +
		`{"type":"turn.completed","usage":{"input_tokens":7,"cached_input_tokens":0,"output_tokens":3,"reasoning_output_tokens":0}}` + "\n"
}

// TestCodexCLIConformance runs llmtest.RunConformance's seven core checks
// (text completion, AsMessage round-trip, stop reason, stream lifecycle,
// stream cancellation, usage on Complete and on Stream) against a Client
// backed by the hermetic fake shim — no network, no real CLI, no credentials
// touched. Every optional capability is off; the t.Log line names each one
// and the adapter fact that keeps it off, so what the suite does and does
// not prove here is stated rather than implied.
func TestCodexCLIConformance(t *testing.T) {
	t.Log("optional capabilities off — " +
		"ToolUse: codex exec's own tool surface cannot carry caller-declared tools and Request.Tools is typed-rejected; " +
		"MidStreamError: Stream reduces the whole event stream to one Response before sending, so no mid-stream failure mode exists; " +
		"UnknownBlock: only agent_message text is surfaced, never a provider-native block; " +
		"TypedErrors: turn.failed's nested status maps to the typed errors, but the suite also demands Retry-After parsing, which the event schema has no field for; " +
		"BodyInspection: there is no outbound wire body — a CLI shell-out has argv, stdin and scratch files only; " +
		"PreflightValidation: cache hints are silently ignored, not counted against a limit")

	llmtest.RunConformance(t, llmtest.Harness{
		NewText: func(t *testing.T, text string) llmclient.LLMClient {
			c, _, _ := newHappyPathConfig(t, codexcli.Config{}, conformanceStream(t, text))
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
