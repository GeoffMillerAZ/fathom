// Package httpembedder — valid test data that must unify with schema.cue.
package httpembedder

import "github.com/geoffmilleraz/agentic-kit/foundations/embedding/http-embedder:httpembedder"

// openAI is a well-formed config for the OpenAI embeddings API, with the key
// supplied via environment variable.
openAI: httpembedder.#Config & {
	BaseURL:   "https://api.openai.com/v1"
	Model:     "text-embedding-3-small"
	APIKeyEnv: "OPENAI_API_KEY"
}

// ollamaLocal is a keyless config for a local Ollama server.
ollamaLocal: httpembedder.#Config & {
	BaseURL: "http://localhost:11434/v1"
	Model:   "nomic-embed-text"
}

// vllmTuned is a vLLM config with explicit timeout (10s in nanoseconds) and
// batch size.
vllmTuned: httpembedder.#Config & {
	BaseURL:  "http://localhost:8000/v1"
	Model:    "BAAI/bge-base-en-v1.5"
	Timeout:  10000000000
	MaxBatch: 64
}

// openRouter routes through OpenRouter with an explicit key (discouraged but
// allowed by the contract).
openRouter: httpembedder.#Config & {
	BaseURL: "https://openrouter.ai/api/v1"
	Model:   "openai/text-embedding-3-large"
	APIKey:  "example-not-a-real-key"
}
