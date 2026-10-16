package codexcli

import "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"

// DescribeProvider reports explicit model selection. Family remains unknown
// because local provider settings can change routing. Streaming is simulated;
// this completion lane is not a host-policy integration.
func (c *Client) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	model := req.Model
	if model == "" {
		model = c.model
	}
	return llmclient.ProviderInfo{Provider: "codexcli", Model: model, Capabilities: llmclient.ProviderCapabilities{Usage: true, Cancellation: true}}
}
