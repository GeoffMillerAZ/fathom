package claudecli

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"

// DescribeProvider reports the explicit model selection. Family remains unknown
// because the CLI inherits ambient gateway settings. Streaming is simulated;
// caller tools and structured output are unsupported by this completion lane.
func (c *Client) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	model := req.Model
	if model == "" {
		model = c.model
	}
	return llmclient.ProviderInfo{Provider: "claudecli", Model: model, Capabilities: llmclient.ProviderCapabilities{Usage: true, Cancellation: true}}
}
