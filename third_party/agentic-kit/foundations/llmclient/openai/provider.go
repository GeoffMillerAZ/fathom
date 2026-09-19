package openai

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"

// DescribeProvider reports configured provenance and adapter support. Compatible
// endpoints require an explicit ProviderFamily assertion; model names are never
// used to infer provenance. ProviderParams cannot override core lane metadata.
func (c *Client) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	model := req.Model
	if model == "" {
		model = c.model
	}
	family := c.providerFamily
	if family == "" && c.baseURL == defaultBaseURL && c.azureDeployment == "" {
		family = "openai"
	}
	if c.azureDeployment != "" {
		model = c.azureDeployment
	}
	return llmclient.ProviderInfo{Provider: "openai", Family: family, Model: model,
		Capabilities: llmclient.ProviderCapabilities{Tools: true, StructuredOutput: true, Streaming: true, Usage: true, Refusal: c.useResponses, Truncation: true, Cancellation: true}}
}
