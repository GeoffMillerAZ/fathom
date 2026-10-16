package openai

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// Defaults applied by New for zero-valued Config fields. The CUE contract in
// schema.cue mirrors these values.
const (
	// defaultBaseURL is the OpenAI API base URL including the /v1 path prefix.
	// For Ollama: http://localhost:11434/v1
	// For vLLM: http://localhost:8000/v1
	defaultBaseURL = "https://api.openai.com/v1"

	// defaultMaxTokens is used when Request.MaxTokens and Config.DefaultMaxTokens
	// are both zero. 4096 is a safe default across all current OpenAI models.
	defaultMaxTokens = 4096

	// completionsPath is the Chat Completions endpoint, joined to Config.BaseURL.
	completionsPath = "/chat/completions"

	// modelsPath is the Models endpoint, joined to Config.BaseURL. Ping GETs it
	// as the zero-token credential-validity probe.
	modelsPath = "/models"

	// defaultCompleteTimeout bounds a non-streaming Complete call when no
	// Config.HTTPClient is injected. Stream never gets an overall timeout —
	// ctx governs (see the package doc).
	defaultCompleteTimeout = 10 * time.Minute

	// maxBodyBytes caps every read of provider-controlled bytes outside the
	// error path. 32 MiB is far beyond any legitimate Chat Completions payload.
	maxBodyBytes = 32 << 20
)

// DefaultAzureAPIVersion is the api-version query parameter applied when
// Config.AzureDeployment is set and Config.AzureAPIVersion is empty.
//
// Pinning doctrine: this constant tracks a known-GA (never preview)
// api-version at the time this package was last updated. Azure ships new
// api-versions on its own cadence, faster than this package releases, so
// the default is expected to lag Azure's latest — that is by design, not
// neglect: a stale-but-stable GA pin never silently opts a caller into
// unannounced preview behavior. Callers who need a newer GA version, or an
// explicit preview version for a feature this constant predates, must set
// Config.AzureAPIVersion themselves; the override always wins.
const DefaultAzureAPIVersion = "2024-10-21"

// The header schemes accepted by Config.AzureAuthScheme. They select how
// the credential travels when Config.AzureDeployment is set (Azure mode);
// both are ignored in vanilla mode, which always sends
// "Authorization: Bearer <key>" regardless of this setting.
const (
	// AzureAuthSchemeAPIKey sends the credential in the "api-key" request
	// header — the default Azure OpenAI credential type (a resource API key
	// from the Azure portal or Key Vault). The zero value of
	// Config.AzureAuthScheme behaves as this scheme.
	AzureAuthSchemeAPIKey = "api-key"

	// AzureAuthSchemeBearer sends the credential as
	// "Authorization: Bearer <credential>" instead of "api-key" — Entra-ID
	// token passthrough. Config.APIKey then holds a caller-minted Entra-ID
	// access token (managed identity or client-credential flow); this
	// package never mints, refreshes, or caches such a token (see doc.go).
	AzureAuthSchemeBearer = "bearer"
)

// Config configures a Client. The zero value is usable as long as APIKey is
// set or the endpoint does not require authentication.
type Config struct {
	// UseResponses opts into POST /responses; false preserves Chat Completions.
	// Azure deployment routing is not supported in this lane.
	UseResponses bool

	// ProviderFamily asserts the model's family for a compatible endpoint.
	// Empty is unknown except at the official OpenAI endpoint. This is caller
	// supplied evidence, not a model-name heuristic or proof of independence.
	ProviderFamily string

	// BaseURL is the API base URL including path prefix, default
	// "https://api.openai.com/v1". A trailing slash is trimmed.
	// Set to "http://localhost:11434/v1" for Ollama or any other
	// OpenAI-compatible endpoint.
	BaseURL string

	// APIKey is the OpenAI API key, sent as "Authorization: Bearer <key>".
	// Optional for local endpoints (Ollama, vLLM) that do not require auth.
	// The credential is resolved at construction and never appears in errors.
	APIKey string

	// DefaultModel is used when Request.Model is empty. When both are empty
	// a call fails with a typed InvalidRequestError before any HTTP.
	DefaultModel string

	// DefaultMaxTokens is used when Request.MaxTokens is zero. Default 4096.
	DefaultMaxTokens int

	// Now is the injected clock for Retry-After HTTP-date parsing.
	// Defaults to time.Now. Override in tests for deterministic behavior.
	Now func() time.Time

	// CapabilityCheck, when non-nil, is called in buildBody before any wire
	// encoding. It is the model-catalog integration point. A nil CapabilityCheck
	// is a no-op (permissive). See the llmclient package doc.
	CapabilityCheck func(req llmclient.Request) error

	// HTTPClient, when non-nil, is used verbatim for both Complete and Stream.
	// When nil, Complete uses a client with a 10-minute overall timeout and
	// Stream uses a client with no overall timeout (ctx governs).
	//
	// The default clients refuse to follow redirects (CheckRedirect returns
	// http.ErrUseLastResponse): the API does not redirect, and following one
	// would re-send the Authorization header. Inject your own client with an
	// equivalent CheckRedirect when using a proxy.
	HTTPClient *http.Client

	// AzureDeployment, when non-empty, switches the client into Azure
	// OpenAI mode. Requests route to
	// {BaseURL}/openai/deployments/{AzureDeployment}/chat/completions
	// (Complete and Stream both route through it) with an api-version query
	// parameter on every request — including the Models Ping, which routes
	// to {BaseURL}/openai/models — and the credential travels in the
	// api-key header by default (see AzureAuthScheme). BaseURL becomes the
	// Azure resource endpoint, e.g. "https://<resource>.openai.azure.com" —
	// it must be set explicitly in Azure mode; it is not defaulted, since
	// defaulting to the OpenAI public API base would silently misroute.
	//
	// Routing reality: Azure routes purely by deployment name, not by
	// model. Request.Model / Config.DefaultModel are still sent in the wire
	// body when set — harmless, and it keeps Rung.Apply/model-catalog
	// behavior uniform across providers — but they have no effect on which
	// deployment actually serves the request; only AzureDeployment does.
	AzureDeployment string

	// AzureAPIVersion is the api-version query parameter sent on every
	// Azure request. Only meaningful when AzureDeployment is non-empty.
	// Empty defaults to DefaultAzureAPIVersion (see its doc comment for the
	// pinning doctrine); set explicitly to use a newer GA version or a
	// preview version this package predates.
	AzureAPIVersion string

	// AzureAuthScheme selects the credential header used in Azure mode:
	// AzureAuthSchemeAPIKey ("api-key", the default when empty) or
	// AzureAuthSchemeBearer ("Authorization: Bearer", for Entra-ID token
	// passthrough). Ignored outside Azure mode (AzureDeployment empty) —
	// the vanilla path always uses Bearer regardless of this field.
	AzureAuthScheme string
}

// Client is an OpenAI Chat Completions API implementation of
// llmclient.LLMClient. Construct it with New. Safe for concurrent use:
// all fields are immutable after construction.
type Client struct {
	useResponses   bool
	providerFamily string
	key            string
	baseURL        string
	model          string
	maxTokens      int

	completeClient *http.Client
	streamClient   *http.Client

	// now is the injected clock (used for Retry-After HTTP-date parsing).
	// Defaults to time.Now; overridden in tests.
	now func() time.Time

	// capabilityCheck is the optional model-catalog integration point.
	capabilityCheck func(req llmclient.Request) error

	// azureDeployment, when non-empty, is the Azure OpenAI deployment name
	// and the switch into Azure mode throughout the client (endpoint
	// construction in buildEndpoint, auth-header selection in
	// setAuthHeaders).
	azureDeployment string

	// azureAPIVersion is the api-version query parameter sent on every
	// Azure request. Only meaningful when azureDeployment is non-empty;
	// always resolved (defaulted) by New, never empty in Azure mode.
	azureAPIVersion string

	// azureAuthScheme is the resolved Azure credential header scheme
	// (AzureAuthSchemeAPIKey or AzureAuthSchemeBearer). Only meaningful
	// when azureDeployment is non-empty; always resolved by New.
	azureAuthScheme string
}

var _ llmclient.LLMClient = (*Client)(nil)

// New validates cfg and returns a ready Client. APIKey may be empty for local
// endpoints that do not require authentication.
func New(cfg Config) (*Client, error) {
	if cfg.UseResponses && cfg.AzureDeployment != "" {
		return nil, &llmclient.InvalidRequestError{Message: "openai: UseResponses does not support AzureDeployment routing"}
	}
	// Azure validation happens before BaseURL defaulting: defaulting an
	// empty BaseURL to the OpenAI public API base would silently misroute
	// an Azure-mode caller who forgot to set the resource endpoint.
	if cfg.AzureDeployment != "" && strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, &llmclient.InvalidRequestError{Message: "openai: Config.AzureDeployment is set but Config.BaseURL is empty; " +
			"Azure mode requires the resource endpoint (https://<resource>.openai.azure.com), which is not defaulted"}
	}
	if cfg.AzureDeployment == "" && cfg.AzureAPIVersion != "" {
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf(
			"openai: Config.AzureAPIVersion %q is set but Config.AzureDeployment is empty; api-version only applies in Azure mode",
			cfg.AzureAPIVersion,
		)}
	}
	azureAPIVersion := cfg.AzureAPIVersion
	if cfg.AzureDeployment != "" && azureAPIVersion == "" {
		azureAPIVersion = DefaultAzureAPIVersion
	}
	azureAuthScheme := cfg.AzureAuthScheme
	switch azureAuthScheme {
	case "":
		azureAuthScheme = AzureAuthSchemeAPIKey
	case AzureAuthSchemeAPIKey, AzureAuthSchemeBearer:
		// valid
	default:
		return nil, fmt.Errorf("openai: Config.AzureAuthScheme must be %q or %q, got %q",
			AzureAuthSchemeAPIKey, AzureAuthSchemeBearer, cfg.AzureAuthScheme)
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
		return nil, fmt.Errorf("openai: Config.DefaultMaxTokens must be positive, got %d", cfg.DefaultMaxTokens)
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

	return &Client{
		useResponses:    cfg.UseResponses,
		providerFamily:  cfg.ProviderFamily,
		key:             cfg.APIKey,
		baseURL:         baseURL,
		model:           cfg.DefaultModel,
		maxTokens:       maxTokens,
		completeClient:  completeClient,
		streamClient:    streamClient,
		now:             now,
		capabilityCheck: cfg.CapabilityCheck,
		azureDeployment: cfg.AzureDeployment,
		azureAPIVersion: azureAPIVersion,
		azureAuthScheme: azureAuthScheme,
	}, nil
}
