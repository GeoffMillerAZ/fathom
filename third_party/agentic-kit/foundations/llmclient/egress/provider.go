package llmegress

import "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"

// DescribeProvider preserves the single provider's identity; egress transforms
// content, not model selection or routing.
func (c *egressClient) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	return llmclient.DescribeProvider(c.inner, req)
}

// DescribeProvider preserves the single provider's identity through inbound
// content transformation.
func (c *inboundClient) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	return llmclient.DescribeProvider(c.inner, req)
}
