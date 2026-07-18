package anthropic

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Ping is the zero-token credential-validity probe: it issues
// GET {BaseURL}/v1/models with the same credential headers as Complete and
// Stream and reports whether the credential is valid and the endpoint
// reachable. Listing models consumes no inference tokens, so Ping is safe to
// call on a Settings page after the user enters a key, at startup, or on a
// health check — anywhere a "is this credential good?" answer is needed
// without spending money.
//
// Outcomes map through the same typed-error mapping as every other call:
//
//   - 200 → nil: the credential is valid and the endpoint reachable.
//   - 401/403 → *llmclient.AuthError: the credential is missing, malformed,
//     expired, or lacks permission — prompt for reconfiguration, do not retry.
//   - 429 → *llmclient.RateLimitError (with any Retry-After hint).
//   - 5xx → *llmclient.ServerError (retryable; says nothing about the
//     credential).
//   - other non-200 → *llmclient.InvalidRequestError (endpoint
//     misconfiguration, e.g. a BaseURL without /v1/models).
//   - transport failure → retryable *llmclient.ServerError; context
//     cancellation/deadline passes through untyped.
//
// The success body is never parsed: Ping asserts credential validity and
// reachability, NOT which models exist — model selection stays a
// Request/Config concern. As everywhere in this package, the credential never
// appears in any returned error; server-echoed occurrences are redacted.
//
// When Config.HTTPClient is nil, Ping uses the same default client as
// Complete (10-minute overall timeout); bound it tighter with ctx for
// snappy UI feedback.
func (c *Client) Ping(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+modelsPath, nil)
	if err != nil {
		return fmt.Errorf("anthropic: building request: %w", err)
	}
	httpReq.Header.Set("accept", "application/json")
	c.setAuthHeaders(httpReq.Header)

	resp, err := c.completeClient.Do(httpReq)
	if err != nil {
		return c.mapTransportErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.mapHTTPError(resp)
	}
	// Drain a bounded amount of the (ignored) success body so the underlying
	// connection can be reused; past the cap we just close and move on.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
	return nil
}
