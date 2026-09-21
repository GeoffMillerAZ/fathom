package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// setAuthHeaders applies the credential header. Vanilla mode (Azure
// deployment unset) always sends "Authorization: Bearer <key>" — unchanged
// from before Azure support existed. Azure mode sends "api-key: <key>" by
// default, or falls through to the same Bearer line when Config.AzureAuthScheme
// is AzureAuthSchemeBearer (Entra-ID token passthrough — see doc.go). The
// credential travels only in this one header — never in the URL or body.
// When APIKey is empty (local endpoints like Ollama), no header is set.
func (c *Client) setAuthHeaders(h http.Header) {
	if c.key == "" {
		return
	}
	if c.azureDeployment != "" && c.azureAuthScheme != AzureAuthSchemeBearer {
		h.Set("api-key", c.key)
		return
	}
	h.Set("authorization", "Bearer "+c.key)
}

// buildEndpoint returns the request URL for one REST operation. In vanilla
// mode (Config.AzureDeployment empty) it is baseURL+vanillaPath, byte-identical
// to the pre-Azure behavior. In Azure mode it is baseURL+azurePath with the
// mandatory api-version query parameter appended — Azure requires
// api-version on every request, so both URL-building call sites (do, shared
// by Complete and Stream; and Ping) route through here.
func (c *Client) buildEndpoint(vanillaPath, azurePath string) string {
	if c.azureDeployment == "" {
		return c.baseURL + vanillaPath
	}
	q := url.Values{}
	q.Set("api-version", c.azureAPIVersion)
	return c.baseURL + azurePath + "?" + q.Encode()
}

// azureCompletionsPath returns the Azure deployment-scoped chat completions
// path for the configured deployment, with the deployment name escaped for
// safe inclusion in a URL path segment.
func (c *Client) azureCompletionsPath() string {
	return "/openai/deployments/" + url.PathEscape(c.azureDeployment) + "/chat/completions"
}

// mapTransportErr maps an http.Client.Do failure to the typed contract.
// Transport-level failures surface as a typed retryable ServerError so
// WithRetry retries them. Context cancellation/deadline passes through
// untyped — the caller chose to stop.
func (c *Client) mapTransportErr(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("openai: %w", c.redactErr(err))
	}
	return &llmclient.ServerError{
		Message: c.redact(fmt.Sprintf("openai: transport: %v", err)),
	}
}

// do issues one POST /chat/completions request.
func (c *Client) do(ctx context.Context, hc *http.Client, body []byte, accept string) (*http.Response, error) {
	endpoint := c.buildEndpoint(completionsPath, c.azureCompletionsPath())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: building request: %w", err)
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

// Complete sends a blocking Chat Completions call and maps the full response.
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

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    c.redact(fmt.Sprintf("openai: reading response body: %v", err)),
		}
	}
	if len(raw) > maxBodyBytes {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("openai: response body exceeds the %d-byte cap", maxBodyBytes),
		}
	}

	var w wireResponse
	if err := json.Unmarshal(raw, &w); err != nil {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    c.redact(fmt.Sprintf("openai: malformed response body: %v", err)),
		}
	}
	if len(w.Choices) == 0 {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    "openai: response has no choices",
		}
	}

	blocks, err := parseBlocks(w.Choices[0].Message)
	if err != nil {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    c.redact(fmt.Sprintf("openai: malformed response content: %v", err)),
		}
	}

	return llmclient.Response{
		Text:       textOf(blocks),
		Content:    blocks,
		StopReason: mapFinishReason(w.Choices[0].FinishReason),
		Usage:      w.Usage.toUsage(),
	}, nil
}
