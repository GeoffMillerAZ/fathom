package llminstrument

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"

// DescribeProvider preserves metadata through cost accounting.
func (c *costClient) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	return llmclient.DescribeProvider(c.inner, req)
}

// DescribeProvider preserves metadata through telemetry.
func (c *TelemetryClient) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	return llmclient.DescribeProvider(c.inner, req)
}

// DescribeProvider preserves metadata through budget checks. The budget's
// defaultModel is a pricing hint and does not alter the dispatched model.
func (c *budgetClient) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	return llmclient.DescribeProvider(c.inner, req)
}
