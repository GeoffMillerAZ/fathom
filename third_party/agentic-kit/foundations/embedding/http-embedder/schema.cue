// Package httpembedder defines the CUE contract for the OpenAI-compatible
// HTTP embedding client configuration. These constraints mirror the Go Config
// type (types.go) and are evaluated with `cue vet ./...`. The Go-only
// HTTPClient injection point is behavioral and intentionally has no CUE
// counterpart.
package httpembedder

// #Config holds the connection parameters for an OpenAI-compatible
// embeddings endpoint (POST {BaseURL}/embeddings).
#Config: {
	// BaseURL is the root of the API, e.g. "https://api.openai.com/v1",
	// "http://localhost:11434/v1" (Ollama), "http://localhost:8000/v1"
	// (vLLM), "https://openrouter.ai/api/v1" (OpenRouter).
	// Required; must be an http(s) URL.
	BaseURL: string & =~"^https?://"

	// Model is the embedding model identifier sent with every request
	// (e.g. "text-embedding-3-small", "nomic-embed-text"). Required.
	Model: string & !=""

	// APIKey is an explicit bearer token. Optional; prefer APIKeyEnv so the
	// secret never lives in config data. Takes precedence over APIKeyEnv.
	APIKey?: string

	// APIKeyEnv names the environment variable holding the bearer token,
	// read once at construction. Optional; omit both APIKey and APIKeyEnv
	// for keyless backends such as a local Ollama.
	APIKeyEnv?: string

	// Timeout bounds each HTTP request, in nanoseconds (Go time.Duration is
	// int64 nanoseconds). 0 or omitted means the Go DefaultTimeout (30s).
	Timeout?: int & >=0

	// MaxBatch is the maximum number of inputs per HTTP request; larger
	// batches are chunked. 0 or omitted means the Go DefaultMaxBatch (128).
	MaxBatch?: int & >=0
}
