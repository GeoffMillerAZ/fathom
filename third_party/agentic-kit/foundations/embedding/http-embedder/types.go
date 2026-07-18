package httpembedder

import (
	"net/http"
	"time"
)

// DefaultTimeout is the per-request HTTP timeout applied when Config.Timeout
// is zero and no custom HTTPClient is injected.
const DefaultTimeout = 30 * time.Second

// DefaultMaxBatch is the maximum number of inputs sent per HTTP request when
// Config.MaxBatch is zero. EmbedBatch transparently chunks larger slices.
const DefaultMaxBatch = 128

// Config configures a Client. The data-shaped fields mirror #Config in
// schema.cue; HTTPClient is behavioral injection and has no CUE counterpart.
type Config struct {
	// BaseURL is the root of an OpenAI-compatible API. The client POSTs to
	// {BaseURL}/embeddings. Required; must be an http or https URL.
	//
	// Examples:
	//	https://api.openai.com/v1      (OpenAI)
	//	http://localhost:11434/v1      (Ollama)
	//	http://localhost:8000/v1       (vLLM)
	//	https://openrouter.ai/api/v1   (OpenRouter)
	BaseURL string

	// Model is the embedding model identifier sent with every request.
	// Required (e.g. "text-embedding-3-small", "nomic-embed-text").
	Model string

	// APIKey is an explicit bearer token. When set it takes precedence over
	// APIKeyEnv. Prefer APIKeyEnv so the secret never appears in config data.
	APIKey string

	// APIKeyEnv names an environment variable holding the bearer token; it is
	// read once at construction time. Both APIKey and APIKeyEnv may be empty
	// for keyless backends such as a local Ollama, in which case no
	// Authorization header is sent.
	APIKeyEnv string

	// Timeout bounds each HTTP request when HTTPClient is nil.
	// Zero means DefaultTimeout. Must not be negative.
	Timeout time.Duration

	// MaxBatch is the maximum number of inputs per HTTP request; EmbedBatch
	// splits larger slices into sequential chunked requests.
	// Zero means DefaultMaxBatch. Must not be negative.
	MaxBatch int

	// HTTPClient, when non-nil, replaces the default HTTP client (for tests,
	// custom transports, proxies). Timeout is then NOT applied — configure
	// timeouts on the injected client or via request contexts.
	HTTPClient *http.Client
}
