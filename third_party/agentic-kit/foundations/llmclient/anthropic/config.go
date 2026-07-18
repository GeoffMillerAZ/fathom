package anthropic

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// Defaults applied by New for zero-valued Config fields. The CUE contract in
// schema.cue mirrors these values.
const (
	defaultAPIKeyEnv = "ANTHROPIC_API_KEY"
	defaultBaseURL   = "https://api.anthropic.com"
	defaultMaxTokens = 4096
	defaultVersion   = "2023-06-01"

	// defaultCompleteTimeout bounds a non-streaming Complete call when no
	// Config.HTTPClient is injected. Long completions are slow; this matches
	// the official SDK's overall default. Stream never gets an overall
	// timeout — ctx governs (see the package doc).
	defaultCompleteTimeout = 10 * time.Minute

	// messagesPath is the Messages API endpoint, joined to Config.BaseURL.
	messagesPath = "/v1/messages"

	// modelsPath is the Models API endpoint, joined to Config.BaseURL. Ping
	// GETs it as the zero-token credential-validity probe.
	modelsPath = "/v1/models"

	// maxCacheBreakpoints is the Messages API's per-request cache_control
	// limit, enforced locally before any HTTP. It is the shared contract
	// limit, so MockClient rejects exactly what this client rejects.
	maxCacheBreakpoints = llmclient.MaxCacheBreakpoints

	// maxBodyBytes caps every read of provider-controlled bytes outside the
	// error path (which has its own tighter cap, maxErrorBodyBytes): a single
	// SSE line, the accumulated data of one SSE event, and a non-streaming
	// success body. 32 MiB is far beyond any legitimate Messages API payload;
	// exceeding it is treated as a malfunctioning (or malicious — BaseURL may
	// point at a proxy) server and surfaces as a typed retryable ServerError
	// instead of unbounded memory growth.
	maxBodyBytes = 32 << 20
)

// The auth schemes accepted by Config.AuthScheme. The CUE contract in
// schema.cue mirrors this closed set.
const (
	// AuthSchemeAPIKey sends the credential in the x-api-key request header.
	// This is the default and the scheme for ordinary Anthropic API keys.
	AuthSchemeAPIKey = "x-api-key"

	// AuthSchemeBearer sends the credential as "Authorization: Bearer
	// <token>" — the scheme for OAuth access tokens (e.g. Claude Code
	// credentials). Pair it with Config.BetaHeaders when the API requires an
	// opt-in beta for the token type (see the BetaHeaders doc).
	AuthSchemeBearer = "bearer"
)

// Config configures a Client. The zero value is usable as long as the
// ANTHROPIC_API_KEY environment variable is set; every other field has a
// default (see the constants above and schema.cue).
type Config struct {
	// APIKey is the literal credential — an API key or, with AuthScheme
	// AuthSchemeBearer, an OAuth access token. When empty, it is read from
	// the environment variable named by APIKeyEnv. Resolution happens once,
	// in New; the resolved credential never appears in errors or artifacts,
	// regardless of scheme.
	//
	// Credential RESOLUTION is the caller's concern: obtaining a token from
	// an OAuth flow, a secrets manager, or Claude Code's local credential
	// store (the pattern: read the store in an environment-specific adapter,
	// then pass the resolved token here) happens outside this package.
	APIKey string

	// APIKeyEnv names the environment variable consulted when APIKey is
	// empty. Default "ANTHROPIC_API_KEY".
	APIKeyEnv string

	// AuthScheme selects the header that carries the credential:
	// AuthSchemeAPIKey ("x-api-key", the default) or AuthSchemeBearer
	// ("bearer", which sends "Authorization: Bearer <credential>"). Any
	// other value is rejected by New. The no-leak guarantee is identical for
	// both schemes. Note Go's http.Client strips the Authorization header on
	// cross-host redirects but NOT x-api-key — moot for the default clients,
	// which refuse to follow redirects entirely (see HTTPClient below), so
	// neither scheme's credential can ever travel to a redirect target.
	AuthScheme string

	// BetaHeaders are values for the anthropic-beta request header, sent as
	// one header line per entry on every request (including Ping). Empty by
	// default — the proven bearer-token path needs none — but the API
	// requires "oauth-2025-04-20" when authenticating with Claude Code OAuth
	// credentials; set that here when using such tokens. Entries must be
	// non-empty.
	BetaHeaders []string

	// BaseURL is the API origin, default "https://api.anthropic.com".
	// A trailing slash is trimmed.
	BaseURL string

	// DefaultModel is used when Request.Model is empty. When both are empty
	// a call fails with a typed InvalidRequestError before any HTTP.
	DefaultModel string

	// DefaultMaxTokens is used when Request.MaxTokens is zero. The Messages
	// API requires max_tokens (there is no server default), so it is always
	// sent. Default 4096.
	DefaultMaxTokens int

	// Version is the anthropic-version header value, default "2023-06-01".
	Version string

	// HTTPClient, when non-nil, is used verbatim for both Complete and
	// Stream. When nil, Complete uses a client with a 10-minute overall
	// timeout and Stream uses a client with no overall timeout (ctx governs
	// a stream's lifetime — see the package doc).
	//
	// The default clients also REFUSE TO FOLLOW REDIRECTS (CheckRedirect
	// returns http.ErrUseLastResponse): the API never redirects, and
	// following one would re-send the credential header to the redirect
	// target — Go strips only Authorization/Cookie on cross-host redirects,
	// not custom headers like x-api-key, and same-host redirects keep even
	// Authorization. Refusing all redirects protects both auth schemes
	// uniformly instead of relying on Go's partial stripping. An injected
	// client is used verbatim, so set an equivalent CheckRedirect on any
	// HTTPClient you supply, especially when BaseURL points at a proxy you
	// do not fully control.
	HTTPClient *http.Client

	// CapabilityCheck, when non-nil, is called in buildBody before any wire
	// encoding. It is the model-catalog integration point: callers wire a
	// function here that validates the named model supports the requested
	// features (thinking, effort, structured outputs, etc.) and returns a
	// typed error when not. A nil CapabilityCheck is a no-op (permissive).
	// The kit ships no catalog; callers supply their own check. See the
	// llmclient package doc, "Model-capability validation seam".
	CapabilityCheck func(req llmclient.Request) error
}

// Client is an Anthropic Messages API implementation of
// llmclient.LLMClient. Construct it with New. Safe for concurrent use:
// all fields are immutable after construction.
type Client struct {
	key         string
	authScheme  string
	betaHeaders []string
	baseURL     string
	model       string
	maxTokens   int
	version     string

	completeClient *http.Client
	streamClient   *http.Client

	// now is the injected clock (used for Retry-After HTTP-date parsing).
	// Defaults to time.Now; overridden in tests.
	now func() time.Time

	// capabilityCheck is the optional model-catalog integration point.
	// See Config.CapabilityCheck.
	capabilityCheck func(req llmclient.Request) error
}

var _ llmclient.LLMClient = (*Client)(nil)

// New validates cfg, resolves the credential exactly once, and returns a
// ready Client. It returns an error when no credential can be resolved or
// when a field is out of range.
func New(cfg Config) (*Client, error) {
	key := cfg.APIKey
	if key == "" {
		env := cfg.APIKeyEnv
		if env == "" {
			env = defaultAPIKeyEnv
		}
		key = os.Getenv(env)
		if key == "" {
			return nil, fmt.Errorf("anthropic: no API key: Config.APIKey is empty and environment variable %q is unset", env)
		}
	}

	authScheme := cfg.AuthScheme
	switch authScheme {
	case "":
		authScheme = AuthSchemeAPIKey
	case AuthSchemeAPIKey, AuthSchemeBearer:
		// valid as given
	default:
		return nil, fmt.Errorf("anthropic: Config.AuthScheme must be %q or %q, got %q", AuthSchemeAPIKey, AuthSchemeBearer, cfg.AuthScheme)
	}

	for i, b := range cfg.BetaHeaders {
		if b == "" {
			return nil, fmt.Errorf("anthropic: Config.BetaHeaders[%d] is empty; every anthropic-beta value must be non-empty", i)
		}
	}
	// Copy so a caller mutating its slice cannot break Client's immutability
	// (the concurrency contract depends on it). Empty stays nil.
	var betaHeaders []string
	if len(cfg.BetaHeaders) > 0 {
		betaHeaders = append(betaHeaders, cfg.BetaHeaders...)
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	maxTokens := cfg.DefaultMaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}
	if maxTokens < 0 {
		return nil, fmt.Errorf("anthropic: Config.DefaultMaxTokens must be positive, got %d", cfg.DefaultMaxTokens)
	}

	version := cfg.Version
	if version == "" {
		version = defaultVersion
	}

	completeClient := cfg.HTTPClient
	streamClient := cfg.HTTPClient
	if cfg.HTTPClient == nil {
		// Never follow a redirect: the API does not redirect, and following
		// one would re-send the credential header to wherever a
		// (mis)configured proxy points — Go strips only Authorization/Cookie
		// on CROSS-HOST redirects (and nothing on same-host ones), and never
		// strips custom headers like x-api-key, so refusing all redirects is
		// what protects both auth schemes uniformly. With ErrUseLastResponse
		// the redirect response itself is returned and maps through the
		// normal non-200 status handling. See the Config.HTTPClient doc for
		// injected clients.
		noRedirect := func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		completeClient = &http.Client{Timeout: defaultCompleteTimeout, CheckRedirect: noRedirect}
		// No overall timeout for streams: ctx governs their lifetime.
		streamClient = &http.Client{CheckRedirect: noRedirect}
	}

	return &Client{
		key:             key,
		authScheme:      authScheme,
		betaHeaders:     betaHeaders,
		baseURL:         baseURL,
		model:           cfg.DefaultModel,
		maxTokens:       maxTokens,
		version:         version,
		completeClient:  completeClient,
		streamClient:    streamClient,
		now:             time.Now,
		capabilityCheck: cfg.CapabilityCheck,
	}, nil
}
