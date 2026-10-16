package claudecli

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// resultEnvelope is the `"type":"result"` JSON object the installed CLI
// (2.1.211) prints to stdout for `claude -p --output-format json`. Only the
// fields this adapter actually uses are modeled; everything else in the real
// envelope (subtype, duration_ms, num_turns, session_id, total_cost_usd,
// terminal_reason, structured_output, ...) is deliberately left unparsed —
// see the package doc, "Response envelope", for why each is or is not
// surfaced on llmclient.Response.
type resultEnvelope struct {
	IsError        bool          `json:"is_error"`
	APIErrorStatus *int          `json:"api_error_status"`
	Result         string        `json:"result"`
	StopReason     string        `json:"stop_reason"`
	Subtype        string        `json:"subtype"`
	Usage          envelopeUsage `json:"usage"`
}

// envelopeUsage is the envelope's `usage` object — the token-accounting
// fields this adapter maps onto llmclient.Usage.
type envelopeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// toUsage maps envelopeUsage onto the shared llmclient.Usage shape.
func (u envelopeUsage) toUsage() llmclient.Usage {
	return llmclient.Usage{
		InputTokens:              u.InputTokens,
		OutputTokens:             u.OutputTokens,
		CacheCreationInputTokens: u.CacheCreationInputTokens,
		CacheReadInputTokens:     u.CacheReadInputTokens,
	}
}

// parseEnvelope decodes raw as a resultEnvelope. A JSON decode failure is the
// "malformed envelope" case documented in the package doc's error-surface
// section.
func parseEnvelope(raw []byte) (resultEnvelope, error) {
	var env resultEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return resultEnvelope{}, fmt.Errorf("claudecli: malformed response envelope: %w", err)
	}
	return env, nil
}

// mapEnvelopeError maps an is_error:true envelope to the shared typed errors,
// the same way the anthropic adapter maps an HTTP status code (see
// mapHTTPError there): 401/403 → AuthError, 429 → RateLimitError, >=500 →
// ServerError, anything else → InvalidRequestError. When api_error_status is
// absent (nil) — a CLI-local failure with no underlying HTTP response to
// classify — a plain (untyped) error carries the envelope's result text.
func mapEnvelopeError(env resultEnvelope) error {
	msg := env.Result
	if msg == "" {
		msg = fmt.Sprintf("claude CLI reported is_error with no result text (subtype=%q)", env.Subtype)
	}

	if env.APIErrorStatus == nil {
		return fmt.Errorf("claudecli: %s", msg)
	}

	switch status := *env.APIErrorStatus; {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &llmclient.AuthError{Message: msg}
	case status == http.StatusTooManyRequests:
		return &llmclient.RateLimitError{Message: msg}
	case status >= 500:
		return &llmclient.ServerError{StatusCode: status, Message: msg}
	default:
		return &llmclient.InvalidRequestError{Message: msg}
	}
}

// mapStopReason maps the envelope's raw stop_reason onto llmclient's open
// StopReason set. usedOutputSchema is true when the request set
// Request.OutputSchema: the installed CLI resolves --json-schema via an
// internal structured-output tool call, which surfaces as raw stop_reason
// "tool_use" even though Complete has already fully resolved it and Result is
// the final answer — see the package doc, "Documented degradations", for why
// that specific case is remapped to StopEndTurn rather than passed through
// verbatim (a caller must never be told a tool result is owed for a call this
// adapter has already completed).
func mapStopReason(raw string, usedOutputSchema bool) llmclient.StopReason {
	if usedOutputSchema && raw == string(llmclient.StopToolUse) {
		return llmclient.StopEndTurn
	}
	return llmclient.StopReason(raw)
}
