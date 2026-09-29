package bedrock

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"

// DescribeProvider reports the Anthropic-on-Bedrock adapter's configured model.
// Family stays unknown: arbitrary model IDs, inference profiles and gateways
// can route differently, and this adapter does not resolve model provenance.
func (c *Client) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	model := req.Model
	if model == "" {
		model = c.model
	}
	return llmclient.ProviderInfo{Provider: "bedrock", Model: model, Capabilities: llmclient.ProviderCapabilities{Tools: true, StructuredOutput: true, Streaming: true, Usage: true, Refusal: true, Truncation: true, Cancellation: true}}
}
