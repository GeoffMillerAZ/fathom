// Package openai — valid test data that must unify with schema.cue.
// Keys in fixtures are placeholders only; NEVER a real key.
package openai

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/openai"

// validMinimal relies entirely on defaults; no API key is set here because
// the key is optional for local endpoints and is resolved at runtime.
validMinimal: openai.#Config & {}

// validOpenAI pins the standard OpenAI endpoint.
validOpenAI: openai.#Config & {
	APIKey:           "test-placeholder-key-not-real"
	BaseURL:          "https://api.openai.com/v1"
	DefaultModel:     "gpt-4o"
	DefaultMaxTokens: 4096
}

// validOllama shows a local Ollama endpoint (no API key required).
validOllama: openai.#Config & {
	BaseURL:          "http://localhost:11434/v1"
	DefaultModel:     "llama3.2"
	DefaultMaxTokens: 2048
}

// validCustom overrides max tokens for a large-context model.
validCustom: openai.#Config & {
	APIKey:           "test-placeholder-key-not-real"
	BaseURL:          "https://api.openai.com/v1"
	DefaultModel:     "o3"
	DefaultMaxTokens: 65536
}

// validAzure pins an Azure OpenAI deployment with the default api-key auth
// scheme.
validAzure: openai.#Config & {
	APIKey:           "test-placeholder-key-not-real"
	BaseURL:          "https://example-resource.openai.azure.com"
	AzureDeployment:  "gpt-4o-deployment"
	AzureAPIVersion:  "2024-10-21"
	DefaultModel:     "gpt-4o"
	DefaultMaxTokens: 4096
}

// validAzureEntra pins an Azure OpenAI deployment with Entra-ID bearer
// passthrough (AzureAuthScheme "bearer"); AzureAPIVersion is left absent to
// exercise the DefaultAzureAPIVersion fallback.
validAzureEntra: openai.#Config & {
	APIKey:          "test-placeholder-entra-token-not-real"
	BaseURL:         "https://example-resource.openai.azure.com"
	AzureDeployment: "gpt-4o-deployment"
	AzureAuthScheme: "bearer"
	DefaultModel:    "gpt-4o"
}
