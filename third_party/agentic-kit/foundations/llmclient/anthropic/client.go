package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// setAuthHeaders applies the credential and protocol headers shared by every
// request (Complete, Stream, Ping): the credential per the configured auth
// scheme — x-api-key (AuthSchemeAPIKey) or "Authorization: Bearer <token>"
// (AuthSchemeBearer) — the anthropic-version header, and one anthropic-beta
// line per configured Config.BetaHeaders entry. The credential travels ONLY
// in its header, never in the URL or body. Redirect interaction: Go strips
// Authorization on cross-host redirects (and x-api-key never), but the
// default clients refuse to follow redirects at all, so neither scheme's
// header can ever be re-sent to a redirect target.
func (c *Client) setAuthHeaders(h http.Header) {
	if c.authScheme == AuthSchemeBearer {
		h.Set("authorization", "Bearer "+c.key)
	} else {
		h.Set("x-api-key", c.key)
	}
	h.Set("anthropic-version", c.version)
	for _, b := range c.betaHeaders {
		h.Add("anthropic-beta", b)
	}
}

// mapTransportErr maps an http.Client.Do failure to the typed contract.
// Transport-level failures (connection reset, DNS, TLS, severed connection)
// are as transient as a 5xx and surface as a typed retryable ServerError so
// WithRetry retries them; context cancellation/deadline passes through
// untyped — the caller chose to stop, so it must never be retried. The
// underlying url.Error carries at most the request URL, never the credential
// (which travels only in a header; redact is the safety net for pathological
// BaseURLs).
func (c *Client) mapTransportErr(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("anthropic: %w", c.redactErr(err))
	}
	return &llmclient.ServerError{
		Message: c.redact(fmt.Sprintf("anthropic: transport: %v", err)),
	}
}

// do issues one POST /v1/messages request with the standard headers (see
// setAuthHeaders for how the credential travels and mapTransportErr for how
// transport failures are typed).
func (c *Client) do(ctx context.Context, hc *http.Client, body []byte, accept string) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+messagesPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: building request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", accept)
	c.setAuthHeaders(httpReq.Header)

	resp, err := hc.Do(httpReq)
	if err != nil {
		return nil, c.mapTransportErr(err)
	}
	return resp, nil
}

// Complete sends a blocking Messages API call and maps the full response.
func (c *Client) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	body, err := c.buildBody(req, false)
	if err != nil {
		return llmclient.Response{}, err
	}
	resp, err := c.do(ctx, c.completeClient, body, "application/json")
	if err != nil {
		return llmclient.Response{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return llmclient.Response{}, c.mapHTTPError(resp)
	}

	// Read at most maxBodyBytes (+1 to detect overflow): an unbounded body
	// from a malfunctioning or proxied server must not grow memory without
	// bound. Beyond the cap is treated like any other garbled-body failure.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    c.redact(fmt.Sprintf("anthropic: reading response body: %v", err)),
		}
	}
	if len(raw) > maxBodyBytes {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("anthropic: response body exceeds the %d-byte cap", maxBodyBytes),
		}
	}
	var w wireResponse
	if err := json.Unmarshal(raw, &w); err != nil {
		// A garbled body from the provider is a transient server-side
		// problem: surface it as a retryable ServerError.
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    c.redact(fmt.Sprintf("anthropic: malformed response body: %v", err)),
		}
	}
	blocks, err := parseBlocks(w.Content)
	if err != nil {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    c.redact(fmt.Sprintf("anthropic: malformed response content: %v", err)),
		}
	}

	return llmclient.Response{
		Text:         textOf(blocks),
		Content:      blocks,
		StopReason:   llmclient.StopReason(w.StopReason),
		StopSequence: w.StopSequence,
		Usage:        w.Usage.toUsage(),
	}, nil
}
