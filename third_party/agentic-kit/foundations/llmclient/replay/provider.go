package llmreplay

import "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"

// DescribeProvider preserves the live recording target's metadata. ReplayClient
// deliberately has no provider identity: a cassette is not a live model route.
func (c *RecordingClient) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	return llmclient.DescribeProvider(c.inner, req)
}
