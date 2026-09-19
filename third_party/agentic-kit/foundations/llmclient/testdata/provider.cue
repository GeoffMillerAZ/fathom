package llmclient

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"

unknown: llmclient.#ProviderInfo & {}
known: llmclient.#ProviderInfo & {
	Provider: "example"
	Family: "example-family"
	Model: "example-model"
	Capabilities: {Tools: true, Streaming: true, Cancellation: true}
}
