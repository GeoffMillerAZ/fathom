package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// maxErrorBodyBytes caps how much of an error response body is read for the
// error message.
const maxErrorBodyBytes = 1 << 20

// mapHTTPError maps a non-200 Messages API response to the shared typed
// errors in llmclient. The body's API error envelope is parsed for the
// message; the API key never appears in the result (server-echoed
// occurrences are redacted). The caller still owns closing resp.Body.
func (c *Client) mapHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	msg := parseAPIErrorMessage(body)
	if msg == "" {
		msg = fmt.Sprintf("HTTP %s: %s", resp.Status, snippet(body, 512))
	}
	msg = c.redact(msg)

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return &llmclient.RateLimitError{
			RetryAfter: c.parseRetryAfter(resp.Header.Get("Retry-After")),
			Message:    msg,
		}
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return &llmclient.AuthError{Message: msg}
	case resp.StatusCode >= 500:
		// Includes 529 overloaded_error and every other 5xx: retryable.
		return &llmclient.ServerError{StatusCode: resp.StatusCode, Message: msg}
	default:
		// 400, 404, 413, 422 and any other 4xx — plus any unexpected non-2xx
		// such as a 3xx redirect, which the default clients never follow
		// (CheckRedirect returns ErrUseLastResponse, so the redirect response
		// lands here): the request or endpoint configuration itself is bad.
		return &llmclient.InvalidRequestError{Message: msg}
	}
}

// parseAPIErrorMessage extracts "type: message" from the API error envelope
// {"type":"error","error":{"type":...,"message":...}}, or "" when the body
// is not that shape (e.g. malformed JSON).
func parseAPIErrorMessage(body []byte) string {
	var env struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Error.Message == "" {
		return ""
	}
	if env.Error.Type != "" {
		return env.Error.Type + ": " + env.Error.Message
	}
	return env.Error.Message
}

// streamErrorToTyped maps a mid-stream SSE error event to the shared typed
// errors. Unrecognised error types are treated as retryable server failures.
func streamErrorToTyped(errType, msg string) error {
	full := msg
	if errType != "" {
		full = errType + ": " + msg
	}
	switch errType {
	case "rate_limit_error":
		return &llmclient.RateLimitError{Message: full}
	case "authentication_error", "permission_error":
		return &llmclient.AuthError{Message: full}
	case "invalid_request_error", "not_found_error", "request_too_large":
		return &llmclient.InvalidRequestError{Message: full}
	case "overloaded_error":
		return &llmclient.ServerError{StatusCode: 529, Message: full}
	default:
		// api_error and anything unrecognised.
		return &llmclient.ServerError{StatusCode: 500, Message: full}
	}
}

// parseRetryAfter parses a Retry-After header value: either an integer
// number of seconds or an HTTP-date. The injected clock makes the HTTP-date
// form deterministic in tests. Unparseable or past values yield zero
// ("unknown" per the RateLimitError contract).
func (c *Client) parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(c.now()); d > 0 {
			return d
		}
	}
	return 0
}

// redact removes the resolved API key from s. Every error message that can
// carry server-derived content goes through this — the key must never appear
// in any error path output.
func (c *Client) redact(s string) string {
	if c.key == "" || !strings.Contains(s, c.key) {
		return s
	}
	return strings.ReplaceAll(s, c.key, "[redacted]")
}

// redactErr is a safety net for transport-level errors: in the (normal) case
// where the key does not appear, the error chain is preserved untouched;
// when it does, the chain is flattened to a redacted message.
func (c *Client) redactErr(err error) error {
	if err == nil || c.key == "" || !strings.Contains(err.Error(), c.key) {
		return err
	}
	return errors.New(c.redact(err.Error()))
}

// snippet truncates b to at most n bytes for inclusion in error messages.
func snippet(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
