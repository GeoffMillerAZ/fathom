package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Ping is the zero-token credential-validity probe: it issues GET
// {BaseURL}/models and reports whether the credential is valid and the
// endpoint reachable. Listing models consumes no inference tokens, so Ping is
// safe to call on a Settings page after the user enters a key, at startup, or
// in health checks.
//
// In Azure mode (Config.AzureDeployment set) Ping instead issues GET
// {BaseURL}/openai/models with the mandatory api-version query parameter —
// Azure's resource-level model list, not the deployment-scoped completions
// endpoint.
//
// Outcomes map through the same typed-error mapping as every other call:
//
//   - 200 → nil: the credential is valid and the endpoint reachable.
//   - 401/403 → *llmclient.AuthError: invalid or expired credential.
//   - 429 → *llmclient.RateLimitError (with any Retry-After hint).
//   - 5xx → *llmclient.ServerError (retryable).
//   - other non-200 → *llmclient.InvalidRequestError (endpoint misconfiguration).
//   - transport failure → retryable *llmclient.ServerError.
//
// The credential never appears in any returned error.
func (c *Client) Ping(ctx context.Context) error {
	endpoint := c.buildEndpoint(modelsPath, "/openai/models")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("openai: building request: %w", err)
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
	// connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
	return nil
}
