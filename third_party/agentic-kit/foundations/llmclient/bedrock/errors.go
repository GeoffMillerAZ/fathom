package bedrock

import (
	"context"
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

// mapHTTPError maps a non-200 Bedrock runtime response to the shared typed
// errors in llmclient. Bedrock surfaces the exception type either in the
// X-Amzn-ErrorType header or the JSON body's "__type"/"message"; the status
// code is the primary classifier. The caller still owns closing resp.Body.
func (c *Client) mapHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	errType := errorTypeOf(resp, body)
	msg := parseAPIErrorMessage(body)
	if msg == "" {
		msg = fmt.Sprintf("HTTP %s: %s", resp.Status, snippet(body, 512))
	}
	if errType != "" {
		msg = errType + ": " + msg
	}
	msg = c.redact(msg)

	switch {
	case resp.StatusCode == http.StatusTooManyRequests || strings.Contains(errType, "ThrottlingException"):
		return &llmclient.RateLimitError{
			RetryAfter: c.parseRetryAfter(resp.Header.Get("Retry-After")),
			Message:    msg,
		}
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return &llmclient.AuthError{Message: msg}
	case resp.StatusCode >= 500:
		// 500/503 ModelErrorException, ServiceUnavailableException, etc.:
		// retryable.
		return &llmclient.ServerError{StatusCode: resp.StatusCode, Message: msg}
	default:
		// 400 ValidationException, 404 ResourceNotFoundException, 413, and any
		// other 4xx — plus an unexpected 3xx that the default clients never
		// follow: the request or endpoint configuration itself is bad.
		return &llmclient.InvalidRequestError{Message: msg}
	}
}

// errorTypeOf extracts the Bedrock exception type from the X-Amzn-ErrorType
// header (its value may be a bare name or "Name:url" — the name is the prefix
// before ':') or the body's "__type". Empty when neither is present.
func errorTypeOf(resp *http.Response, body []byte) string {
	if h := resp.Header.Get("X-Amzn-ErrorType"); h != "" {
		if i := strings.IndexByte(h, ':'); i >= 0 {
			return h[:i]
		}
		return h
	}
	var env struct {
		Type string `json:"__type"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Type != "" {
		// __type is sometimes "com.amazon...#ValidationException"; keep the
		// name after '#'.
		if i := strings.LastIndexByte(env.Type, '#'); i >= 0 {
			return env.Type[i+1:]
		}
		return env.Type
	}
	return ""
}

// parseAPIErrorMessage extracts the human message from the Bedrock error body
// {"message":"..."} (also accepting "Message"), or "" when the body is not
// that shape.
func parseAPIErrorMessage(body []byte) string {
	var env struct {
		Message  string `json:"message"`
		MessageU string `json:"Message"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return ""
	}
	if env.Message != "" {
		return env.Message
	}
	return env.MessageU
}

// mapTransportErr maps an http.Client.Do failure to the typed contract.
// Transport-level failures (reset, DNS, TLS, severed connection) are as
// transient as a 5xx and surface as a typed retryable ServerError; context
// cancellation/deadline passes through untyped so WithRetry never retries a
// caller-chosen stop. The underlying url.Error carries at most the request URL,
// never the secret (which never travels on the wire); redact is the safety net.
func (c *Client) mapTransportErr(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("bedrock: %w", c.redactErr(err))
	}
	return &llmclient.ServerError{
		Message: c.redact(fmt.Sprintf("bedrock: transport: %v", err)),
	}
}

// mapCredentialErr maps a credentialCache.get failure (an awscred.Resolve
// error — an expired SSO token, a failing credential_process, an exhausted
// chain) to the shared typed-error taxonomy. This is a NEW failure mode
// relative to the pre-awscred adapter: credential resolution can now fail
// on ANY call, not just at New() time, because a long-lived Client
// re-resolves near expiry. It surfaces as a non-retryable *AuthError — a
// credential problem is not a transient server fault, and WithRetry must
// not spin on it — except context cancellation/deadline, which passes
// through untyped so WithRetry never retries a caller-chosen stop (mirrors
// mapTransportErr).
func (c *Client) mapCredentialErr(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("bedrock: %w", c.redactErr(err))
	}
	return &llmclient.AuthError{
		Message: c.redact(fmt.Sprintf("bedrock: resolving credentials: %v", err)),
	}
}

// streamErrorToTyped maps a Bedrock event-stream exception event to the shared
// typed errors. Unrecognised types are treated as retryable server failures.
func streamErrorToTyped(errType, msg string) error {
	full := msg
	if errType != "" {
		full = errType + ": " + msg
	}
	switch {
	case strings.Contains(errType, "ThrottlingException"):
		return &llmclient.RateLimitError{Message: full}
	case strings.Contains(errType, "AccessDenied"), strings.Contains(errType, "UnrecognizedClient"):
		return &llmclient.AuthError{Message: full}
	case strings.Contains(errType, "ValidationException"), strings.Contains(errType, "ResourceNotFound"):
		return &llmclient.InvalidRequestError{Message: full}
	default:
		// ModelStreamErrorException, InternalServerException, and anything
		// unrecognised: retryable.
		return &llmclient.ServerError{StatusCode: 500, Message: full}
	}
}

// parseRetryAfter parses a Retry-After header value: an integer number of
// seconds or an HTTP-date. The injected clock makes the HTTP-date form
// deterministic in tests. Unparseable or past values yield zero.
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

// redact removes the resolved secret access key and session token from s. Every
// error message that can carry server-derived content goes through this. The
// secret never legitimately appears (it never leaves the process), so this is
// defense-in-depth against a pathological BaseURL echo. It redacts the most
// recently cached credential (credCache.snapshot(), a non-blocking read that
// never triggers a re-resolve) — sufficient for a defense-in-depth net rather
// than a primary security boundary, and safe even if a concurrent refresh is
// in flight.
func (c *Client) redact(s string) string {
	creds := c.credCache.snapshot()
	for _, secret := range []string{creds.secretAccessKey, creds.sessionToken} {
		if secret != "" && strings.Contains(s, secret) {
			s = strings.ReplaceAll(s, secret, "[redacted]")
		}
	}
	return s
}

// redactErr is a safety net for transport-level errors: when no secret appears
// the error chain is preserved untouched; when one does, the chain is flattened
// to a redacted message.
func (c *Client) redactErr(err error) error {
	if err == nil {
		return err
	}
	if r := c.redact(err.Error()); r != err.Error() {
		return errors.New(r)
	}
	return err
}

// snippet truncates b to at most n bytes for inclusion in error messages.
func snippet(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
