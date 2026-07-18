package bedrock

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// controlPlaneURL derives the Bedrock control-plane origin from the configured
// data-plane (runtime) origin. For the standard AWS hostname pattern the
// transformation is:
//
//	https://bedrock-runtime.{region}.amazonaws.com
//	  → https://bedrock.{region}.amazonaws.com
//
// For a custom or VPC BaseURL the runtime host is used unchanged — the caller
// is responsible for routing to a host that serves the Bedrock control-plane
// ListFoundationModels API. This mirrors the tradeoff made by the Anthropic
// adapter's Ping (which probes /v1/models on the same BaseURL as Complete).
func controlPlaneURL(runtimeBase string) string {
	return strings.Replace(runtimeBase, "bedrock-runtime.", "bedrock.", 1)
}

// foundationModelsPath is the Bedrock control-plane endpoint that lists
// available foundation models. Ping issues a signed GET to this path: the
// response body is intentionally discarded — we need only a 200 status.
//
// This endpoint is on the Bedrock control-plane service (signed for service
// "bedrock"), which is a GET with an empty body. It consumes no inference
// tokens and does not invoke any model.
const foundationModelsPath = "/foundation-models"

// Ping is the zero-token credential-validity and reachability probe. It issues
// a SigV4-signed GET to the Bedrock control-plane ListFoundationModels endpoint
// and maps the outcome through the shared typed-error taxonomy.
//
// What a successful Ping (nil return) proves:
//   - The AWS credentials (access key + secret + optional session token) are
//     valid and not expired at the time of the call.
//   - The Bedrock control-plane endpoint is reachable at the derived
//     control-plane URL.
//   - The IAM principal has at least bedrock:ListFoundationModels permission
//     in the configured region.
//
// What Ping does NOT prove:
//   - That any specific model is accessible — a principal may list foundation
//     models but lack bedrock:InvokeModel on the target model id. Invoke
//     failures surface on the first Complete or Stream call.
//   - That the data-plane endpoint (used by Complete and Stream) is reachable
//     — Ping contacts the control plane, not the runtime. A custom BaseURL
//     (VPC endpoint) routes the control-plane probe through that same host,
//     which is usually correct for split endpoints but may differ from the
//     runtime path.
//   - That the account has sufficient quota for inference. Quota exhaustion
//     surfaces as a RateLimitError on the first inference call.
//
// Error mapping follows the shared taxonomy:
//   - 200 → nil: credentials valid, endpoint reachable.
//   - 401/403 → *llmclient.AuthError: bad or expired credential, or the IAM
//     principal lacks the required permission — do not retry; reconfigure.
//   - 429 → *llmclient.RateLimitError (with any Retry-After hint): retryable.
//   - 5xx → *llmclient.ServerError: transient server fault — retryable.
//   - other non-200 → *llmclient.InvalidRequestError: endpoint or config error.
//   - transport failure → retryable *llmclient.ServerError.
//   - context cancellation/deadline passes through untyped so WithRetry never
//     retries a caller-chosen stop.
//
// The response body is read up to maxErrorBodyBytes and discarded on 200; on
// error it is used for the typed-error message then discarded.
//
// Signing: the probe is signed for service "bedrock" (the control-plane
// service name) with an empty payload (SHA256 of the empty string). The same
// credential triple and region as Complete and Stream are used, so Ping
// validates exactly the credentials the inference calls will use.
func (c *Client) Ping(ctx context.Context) error {
	creds, err := c.credCache.get(ctx)
	if err != nil {
		return c.mapCredentialErr(err)
	}

	cpURL := controlPlaneURL(c.baseURL) + foundationModelsPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cpURL, nil)
	if err != nil {
		return &llmclient.InvalidRequestError{
			Message: c.redact("bedrock: ping: building request: " + err.Error()),
		}
	}
	req.Header.Set("Accept", "application/json")

	// Sign the GET with an empty payload. The control-plane service name is
	// "bedrock"; signRequest accepts nil/empty payload and uses the
	// emptyPayloadHash constant for x-amz-content-sha256.
	signRequest(req, creds, service, c.region, c.now(), nil)

	resp, err := c.completeClient.Do(req)
	if err != nil {
		return c.mapTransportErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.mapHTTPError(resp)
	}
	// Drain a bounded amount of the (ignored) 200 success body so the
	// underlying connection can be reused; past the cap we close and move on.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
	return nil
}
