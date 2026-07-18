package bedrock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// invokePath is the InvokeModel data-plane path; the {modelId} segment is the
// resolved Bedrock model id (URL-path-escaped).
func invokePath(model string) string {
	return "/model/" + escapeModelID(model) + "/invoke"
}

// invokeStreamPath is the InvokeModelWithResponseStream path.
func invokeStreamPath(model string) string {
	return "/model/" + escapeModelID(model) + "/invoke-with-response-stream"
}

// escapeModelID percent-encodes a Bedrock model id for a single URL path
// segment. Model ids and inference-profile ARNs contain ':' and '/' which must
// be encoded so they do not split the path; the unreserved set and the
// id-typical '.' / '-' pass through. The SAME encoded form is what SigV4 signs
// (signRequest reads req.URL.EscapedPath()), so request and signature agree.
func escapeModelID(model string) string {
	return uriEncode(model, true)
}

// newSignedRequest builds and SigV4-signs one POST to path with body. accept
// sets the Accept header (also signed). The signing instant is c.now(). The
// credential triple is fetched from c.credCache, which re-resolves through
// the awscred chain when the cached value is nearing expiry (see
// credentialCache) — a resolution failure (e.g. an expired SSO token) maps
// to a typed error via mapCredentialErr rather than signing with a stale or
// zero-valued credential.
func (c *Client) newSignedRequest(ctx context.Context, path string, body []byte, accept string) (*http.Request, error) {
	creds, err := c.credCache.get(ctx)
	if err != nil {
		return nil, c.mapCredentialErr(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("bedrock: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	signRequest(req, creds, service, c.region, c.now(), body)
	return req, nil
}

// Complete sends a blocking InvokeModel call and maps the full response.
func (c *Client) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	body, model, err := c.buildBody(req)
	if err != nil {
		return llmclient.Response{}, err
	}
	httpReq, err := c.newSignedRequest(ctx, invokePath(model), body, "application/json")
	if err != nil {
		return llmclient.Response{}, err
	}
	resp, err := c.completeClient.Do(httpReq)
	if err != nil {
		return llmclient.Response{}, c.mapTransportErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return llmclient.Response{}, c.mapHTTPError(resp)
	}

	// Read at most maxBodyBytes (+1 to detect overflow): an unbounded body from
	// a malfunctioning or proxied server must not grow memory without bound.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    c.redact(fmt.Sprintf("bedrock: reading response body: %v", err)),
		}
	}
	if len(raw) > maxBodyBytes {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("bedrock: response body exceeds the %d-byte cap", maxBodyBytes),
		}
	}
	var w wireResponse
	if err := json.Unmarshal(raw, &w); err != nil {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    c.redact(fmt.Sprintf("bedrock: malformed response body: %v", err)),
		}
	}
	blocks, err := parseBlocks(w.Content)
	if err != nil {
		return llmclient.Response{}, &llmclient.ServerError{
			StatusCode: resp.StatusCode,
			Message:    c.redact(fmt.Sprintf("bedrock: malformed response content: %v", err)),
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
