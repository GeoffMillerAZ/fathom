package codexcli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// maxEventLineBytes bounds a single JSONL line's scan buffer. A real
// agent_message item is small; this is generous headroom against a
// pathological line, mirroring the kit's other output-size guards.
const maxEventLineBytes = 8 << 20

// reducedTurn is the accumulated result of scanning one call's JSONL event
// stream. Exactly one of (a successful reduction: text+usage) or failure is
// meaningful — see reduceEvents.
type reducedTurn struct {
	// text is the in-order concatenation of every item.completed event whose
	// item.type is "agent_message" — the multi-item case the design brief
	// calls out (a turn may emit more than one agent_message item; this
	// adapter concatenates them all, matching claudecli's Result-is-final-text
	// contract at the reduced level). Concatenation is VERBATIM with no
	// separator inserted between items — the CLI owns any whitespace. When
	// the turn fails, any partial text accumulated here is discarded by
	// Complete (failure wins; no partial Response is ever returned).
	text string

	// usage is populated from turn.completed's usage object. Zero value when
	// the turn failed before reaching turn.completed.
	usage llmclient.Usage

	// failure is non-nil when the stream reported turn.failed (or, absent
	// that, a standalone top-level "error" event with no subsequent
	// turn.completed) — an already-typed llmclient error, mapped by
	// mapEventError.
	failure error
}

// resultEventUsage is the `turn.completed.usage` JSON object the installed
// CLI (0.144.5) emits for `codex exec --json`.
type resultEventUsage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
}

// toUsage maps resultEventUsage onto the shared llmclient.Usage shape.
//
// cached_input_tokens -> CacheReadInputTokens: codex's cache semantics are
// read-only from this adapter's vantage point (a cache HIT discount on
// input, like Anthropic's cache_read_input_tokens) — there is no
// "cache_creation" counter in the event schema, so CacheCreationInputTokens
// is always left at its zero value ("not reported"), never synthesized.
//
// reasoning_output_tokens counted into OutputTokens: verified live that
// reasoning tokens are a real, separately-billed component of a turn (GPT-5
// reasoning models spend hidden "thinking" tokens that are metered as
// output, just like Anthropic extended-thinking tokens are). Every other
// adapter's Usage.OutputTokens represents the full billed-output token
// count for a call, so this adapter folds reasoning_output_tokens into
// OutputTokens rather than dropping it or exposing a third counter that
// does not exist on the shared llmclient.Usage type — a caller applying a
// per-output-token rate table (llmclient/instrument.WithCost) needs the
// reasoning tokens included to get an honest total, and this is the only
// field they can land on.
func (u resultEventUsage) toUsage() llmclient.Usage {
	return llmclient.Usage{
		InputTokens:          u.InputTokens,
		OutputTokens:         u.OutputTokens + u.ReasoningOutputTokens,
		CacheReadInputTokens: u.CachedInputTokens,
	}
}

// reduceEvents scans raw as newline-delimited JSON events (see the package
// doc, "Event shapes") and reduces them to a reducedTurn.
//
// The returned error is non-nil only for STRUCTURAL problems with the
// stream itself: a line that is not valid JSON at all, or a stream that
// ends without ever producing a turn.completed or turn.failed event (both
// are "the CLI's own output is not a well-formed event stream" — Complete
// wraps these with the exit code and a stderr tail, exactly like
// claudecli's malformed-envelope path). A turn-level failure the CLI
// reports on a well-formed stream (turn.failed, or a standalone top-level
// error event) is NOT a Go error here — it is reducedTurn.failure, an
// already-typed llmclient error returned alongside a nil error, so Complete
// can return it directly without re-wrapping exit-code/stderr noise around
// a perfectly legible provider-reported failure.
func reduceEvents(raw []byte) (reducedTurn, error) {
	var out reducedTurn
	var sawTurnCompleted, sawTurnFailed bool
	var fallbackErrMsg string

	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), maxEventLineBytes)

	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}

		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			return reducedTurn{}, fmt.Errorf("codexcli: malformed JSONL event line: %w", err)
		}

		switch envelope.Type {
		case "item.completed":
			var item struct {
				Item struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"item"`
			}
			if err := json.Unmarshal(line, &item); err != nil {
				return reducedTurn{}, fmt.Errorf("codexcli: malformed item.completed event: %w", err)
			}
			if item.Item.Type == "agent_message" {
				out.text += item.Item.Text
			}

		case "turn.completed":
			var tc struct {
				Usage resultEventUsage `json:"usage"`
			}
			if err := json.Unmarshal(line, &tc); err != nil {
				return reducedTurn{}, fmt.Errorf("codexcli: malformed turn.completed event: %w", err)
			}
			out.usage = tc.Usage.toUsage()
			sawTurnCompleted = true

		case "turn.failed":
			var tf struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(line, &tf); err != nil {
				return reducedTurn{}, fmt.Errorf("codexcli: malformed turn.failed event: %w", err)
			}
			out.failure = mapEventError(tf.Error.Message)
			sawTurnFailed = true

		case "error":
			// A standalone top-level error event. Verified live to arrive
			// ALONGSIDE turn.failed carrying the identical message (see
			// testdata/PROBES.md, probe 12) — turn.failed is treated as
			// authoritative when both are present (case above wins by
			// setting sawTurnFailed). This branch's captured message is
			// only used as a fallback if the stream ends without ever
			// reaching turn.completed or turn.failed (a hypothetical
			// truncated-stream shape never observed live).
			var e struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(line, &e); err == nil {
				fallbackErrMsg = e.Message
			}

		default:
			// thread.started, turn.started, and any future event type this
			// adapter does not model — observed and ignored, per the
			// design brief's "JSONL reducer, not envelope parser."
		}
	}
	if err := sc.Err(); err != nil {
		return reducedTurn{}, fmt.Errorf("codexcli: reading event stream: %w", err)
	}

	switch {
	case sawTurnCompleted:
		return out, nil
	case sawTurnFailed:
		return out, nil
	case fallbackErrMsg != "":
		out.failure = mapEventError(fallbackErrMsg)
		return out, nil
	default:
		return reducedTurn{}, errors.New("codexcli: event stream ended without a turn.completed or turn.failed event")
	}
}

// nestedErrorEnvelope is the shape observed inside a turn.failed/error
// event's string `message` field for a genuine API-side rejection (verified
// live, testdata/PROBES.md probe 12 — an unrecognised --model value):
//
//	{"type":"error","status":400,"error":{"type":"invalid_request_error","message":"..."}}
//
// The outer message field is itself a JSON-encoded string, not a nested
// object — this adapter double-decodes it.
type nestedErrorEnvelope struct {
	Status int `json:"status"`
	Error  struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// mapEventError maps a turn.failed/error event's raw message string to the
// shared typed errors, mirroring how the anthropic and claudecli adapters
// classify a provider-reported failure by status code.
//
// Only the 400 (invalid_request_error) shape has been observed live (see
// testdata/PROBES.md, probe 12) — the auth-expired (401/403) and
// usage-limit (429) shapes are, per the design brief, mapped SPECULATIVELY
// by keyword content rather than live-verified status codes, since probing
// those live would require deliberately breaking this machine's login or
// exhausting a real subscription quota. Treat classifyByContent's mapping as
// a documented best-effort, not a pinned contract, until a real 401/403/429
// is observed and this comment is updated.
func mapEventError(raw string) error {
	var nested nestedErrorEnvelope
	if err := json.Unmarshal([]byte(raw), &nested); err == nil && nested.Status != 0 {
		msg := nested.Error.Message
		if msg == "" {
			msg = raw
		}
		switch {
		case nested.Status == http.StatusUnauthorized || nested.Status == http.StatusForbidden:
			return &llmclient.AuthError{Message: msg}
		case nested.Status == http.StatusTooManyRequests:
			return &llmclient.RateLimitError{Message: msg}
		case nested.Status >= 500:
			return &llmclient.ServerError{StatusCode: nested.Status, Message: msg}
		default:
			return &llmclient.InvalidRequestError{Message: msg}
		}
	}
	return classifyByContent(raw)
}

// classifyByContent is the SPECULATIVE (never live-verified — see
// mapEventError's doc) fallback for a turn.failed/error message that is not
// the nested-JSON status shape: a plain CLI-local failure string (e.g. an
// auth or quota problem surfaced before any HTTP round trip). Classification
// is by keyword content, matching the design brief's explicit instruction
// to map these speculatively rather than leave them all as opaque plain
// errors. The auth keywords are deliberately full words ("authentication",
// "unauthorized", "not logged in") rather than the substring "auth", which
// would misfire on incidental hits like "author" or "OAuth callback" —
// better to under-classify (a plain, unwrapped error) than to wrongly tell
// a failover router the credential is dead.
func classifyByContent(raw string) error {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "not logged in"),
		strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "authentication"):
		return &llmclient.AuthError{Message: raw}
	case strings.Contains(lower, "rate limit"), strings.Contains(lower, "usage limit"), strings.Contains(lower, "quota"):
		return &llmclient.RateLimitError{Message: raw}
	default:
		return fmt.Errorf("codexcli: %s", raw)
	}
}
