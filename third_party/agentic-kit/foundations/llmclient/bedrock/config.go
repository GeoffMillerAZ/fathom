package bedrock

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/bedrock/awscred"
)

// Defaults applied by New for zero-valued Config fields. The CUE contract in
// schema.cue mirrors these values.
const (
	// service is the SigV4 service name and the host-subdomain for the Bedrock
	// runtime data-plane (InvokeModel / InvokeModelWithResponseStream).
	service = "bedrock"

	// anthropicVersionValue is the anthropic_version that Anthropic-on-Bedrock
	// requires IN THE BODY (the public Anthropic API instead carries it in the
	// anthropic-version HEADER; Bedrock injects model + version via the URL and
	// body). This is the fixed Bedrock token, distinct from the public API's
	// dated version.
	anthropicVersionValue = "bedrock-2023-05-31"

	defaultMaxTokens = 4096

	// defaultCompleteTimeout bounds a non-streaming Complete call when no
	// Config.HTTPClient is injected. Stream gets no overall timeout — ctx
	// governs (see the package doc).
	defaultCompleteTimeout = 10 * time.Minute

	// maxCacheBreakpoints is the per-request cache_control limit, enforced
	// locally before any HTTP — the shared contract limit.
	maxCacheBreakpoints = llmclient.MaxCacheBreakpoints

	// maxBodyBytes caps every read of provider-controlled bytes outside the
	// error path. 32 MiB is far beyond any legitimate payload; exceeding it is
	// treated as a malfunctioning (or proxied/malicious) server and surfaces
	// as a typed retryable ServerError instead of unbounded memory growth.
	maxBodyBytes = 32 << 20

	// Credential environment variables consulted when the corresponding Config
	// field is empty. These are the conventional AWS names so the adapter drops
	// into an instance-role / CI environment without extra wiring.
	envAccessKeyID     = "AWS_ACCESS_KEY_ID"
	envSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	envSessionToken    = "AWS_SESSION_TOKEN"
	envRegion          = "AWS_REGION"
)

// Config configures a Client. A usable config needs, at minimum, a region and
// resolvable credentials (an access-key id + secret, via the fields or their
// AWS_* environment variables) and a model — either Config.DefaultModel or
// Request.Model per call.
type Config struct {
	// Region is the AWS region, e.g. "us-gov-west-1" (GovCloud) or
	// "us-east-1". It selects both the SigV4 credential scope and the
	// bedrock-runtime host when BaseURL is empty. When empty it is read from
	// AWS_REGION. Required (no default): GovCloud and commercial endpoints are
	// distinct and must be chosen explicitly.
	Region string

	// AccessKeyID is the AWS access-key id. When empty it is read from
	// AWS_ACCESS_KEY_ID. Resolved once, in New.
	AccessKeyID string

	// SecretAccessKey is the AWS secret access key — used only to DERIVE the
	// SigV4 signing key; it never travels on the wire and never appears in any
	// error or artifact. When empty it is read from AWS_SECRET_ACCESS_KEY.
	SecretAccessKey string

	// SessionToken is the AWS session token for temporary credentials
	// (STS-vended, instance-role, SSO). Empty for long-lived keys. When sent it
	// travels in the X-Amz-Security-Token header and is part of the signed
	// header set. When empty it is read from AWS_SESSION_TOKEN.
	SessionToken string

	// BaseURL overrides the endpoint origin (scheme + host, no trailing path).
	// Empty derives the standard data-plane host
	// "https://bedrock-runtime.{Region}.amazonaws.com". Set it for a GovCloud
	// FIPS endpoint, a VPC endpoint, or a test server. A trailing slash is
	// trimmed. The SigV4 host signed is whatever host this resolves to.
	BaseURL string

	// DefaultModel is the Bedrock model id used when Request.Model is empty,
	// e.g. "anthropic.claude-3-5-sonnet-20241022-v2:0" or a GovCloud inference
	// profile id. When both are empty a call fails with a typed
	// InvalidRequestError before any HTTP.
	DefaultModel string

	// DefaultMaxTokens is used when Request.MaxTokens is zero. The
	// Anthropic-on-Bedrock body requires max_tokens, so it is always sent.
	// Default 4096.
	DefaultMaxTokens int

	// HTTPClient, when non-nil, is used verbatim for both Complete and Stream.
	// When nil, Complete uses a client with a 10-minute overall timeout and
	// Stream uses a client with no overall timeout (ctx governs a stream's
	// lifetime). The default clients also REFUSE TO FOLLOW REDIRECTS: a signed
	// request must never be replayed to a redirect target.
	HTTPClient *http.Client

	// Now overrides the signing clock; nil uses time.Now. Tests pin it to make
	// the X-Amz-Date and signature deterministic. The same clock also governs
	// credential-expiry judgment (when to re-resolve via the awscred chain —
	// see credentialCache), so pinning it lets tests deterministically drive
	// a Client across an expiry boundary too.
	Now func() time.Time

	// CapabilityCheck, when non-nil, is called in buildBody before any wire
	// encoding. It is the model-catalog integration point: callers wire a
	// function here that validates the named model supports the requested
	// features (thinking, effort, structured outputs, etc.) and returns a
	// typed error when not. A nil CapabilityCheck is a no-op (permissive).
	// See the llmclient package doc, "Model-capability validation seam".
	CapabilityCheck func(req llmclient.Request) error
}

// Client is an AWS Bedrock runtime implementation of llmclient.LLMClient for
// Claude (Anthropic-on-Bedrock). Construct it with New. Safe for concurrent
// use: every field except credCache is immutable after construction, and
// credCache is itself internally mutex-guarded (see credentialCache) — no
// external synchronization is required around a shared *Client.
type Client struct {
	// credCache holds the active credential triple and re-resolves it, under
	// its own mutex, as expiry approaches. See credentialCache.
	credCache *credentialCache

	region    string
	baseURL   string
	host      string
	model     string
	maxTokens int

	completeClient *http.Client
	streamClient   *http.Client

	now func() time.Time

	// capabilityCheck is the optional model-catalog integration point.
	// See Config.CapabilityCheck.
	capabilityCheck func(req llmclient.Request) error
}

var _ llmclient.LLMClient = (*Client)(nil)

// New validates cfg, resolves credentials and region exactly once via the
// awscred chain (explicit Config fields → AWS_* env vars → shared
// config/credentials files → IMDSv2), and returns a ready Client. It returns
// an error when the region cannot be resolved, when no leg of the chain
// produces credentials, or when a field is out of range.
//
// Explicit credentials never fall through to ambient state: a complete
// Config.AccessKeyID+SecretAccessKey pair is used verbatim without touching
// ~/.aws, credential_process, SSO, or IMDS, and a PARTIAL pair (one half
// set) fails with a typed *awscred.PartialCredentialsError instead of
// silently signing as whatever identity the shared files name — see
// awscred.Resolve for the full leg semantics.
//
// Credentials that carry an expiry (SSO, credential_process,
// IMDS-vended — anything but explicit/env/shared-static) are not merely
// resolved once and frozen: Complete, Stream, and Ping re-resolve through
// the same chain when the cached credential is within 5 minutes of expiry
// (see credentialCache), so a long-lived Client backed by `aws sso login`
// keeps working across a token refresh without reconstruction.
func New(cfg Config) (*Client, error) {
	acfg := awscred.Config{
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		SessionToken:    cfg.SessionToken,
		Region:          cfg.Region,
		Now:             cfg.Now,
	}
	resolved, err := awscred.Resolve(context.Background(), acfg)
	if err != nil {
		return nil, fmt.Errorf("bedrock: resolving credentials: %w", err)
	}
	if resolved.Region == "" {
		return nil, fmt.Errorf("bedrock: no region: Config.Region is empty and no region was found via %s/%s or a shared-profile region key", envRegion, "AWS_DEFAULT_REGION")
	}
	region := resolved.Region

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://bedrock-runtime." + region + ".amazonaws.com"
	}
	host, err := hostOf(baseURL)
	if err != nil {
		return nil, fmt.Errorf("bedrock: invalid BaseURL %q: %w", baseURL, err)
	}

	maxTokens := cfg.DefaultMaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}
	if maxTokens < 0 {
		return nil, fmt.Errorf("bedrock: Config.DefaultMaxTokens must be positive, got %d", cfg.DefaultMaxTokens)
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	completeClient := cfg.HTTPClient
	streamClient := cfg.HTTPClient
	if cfg.HTTPClient == nil {
		noRedirect := func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		completeClient = &http.Client{Timeout: defaultCompleteTimeout, CheckRedirect: noRedirect}
		streamClient = &http.Client{CheckRedirect: noRedirect}
	}

	credCache := &credentialCache{
		creds: credentials{
			accessKeyID:     resolved.AccessKeyID,
			secretAccessKey: resolved.SecretAccessKey,
			sessionToken:    resolved.SessionToken,
		},
		expiry:  resolved.Expiry,
		now:     now,
		resolve: func(ctx context.Context) (awscred.Credentials, error) { return awscred.Resolve(ctx, acfg) },
	}

	return &Client{
		credCache:       credCache,
		region:          region,
		baseURL:         baseURL,
		host:            host,
		model:           cfg.DefaultModel,
		maxTokens:       maxTokens,
		completeClient:  completeClient,
		streamClient:    streamClient,
		now:             now,
		capabilityCheck: cfg.CapabilityCheck,
	}, nil
}

// hostOf extracts the host (host[:port]) from a "scheme://host[:port]" origin
// without importing net/url's full parser surface — it is enough to split on
// "://" and take up to the first '/'.
func hostOf(baseURL string) (string, error) {
	rest := baseURL
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	} else {
		return "", fmt.Errorf("missing scheme")
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	if rest == "" {
		return "", fmt.Errorf("empty host")
	}
	return rest, nil
}
