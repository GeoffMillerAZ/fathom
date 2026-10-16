package anthropic

import (
	"encoding/json"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// DescribeProvider reports model identity and supported adapter mappings. A
// custom gateway's provenance is unknown; the wire dialect is not a family.
func (c *Client) DescribeProvider(req llmclient.Request) llmclient.ProviderInfo {
	model := req.Model
	if model == "" {
		model = c.model
	}
	var params map[string]json.RawMessage
	if json.Unmarshal(req.ProviderParams, &params) == nil {
		if raw, ok := params["model"]; ok {
			model = ""
			_ = json.Unmarshal(raw, &model)
		}
	}
	family := ""
	if c.baseURL == defaultBaseURL {
		family = "anthropic"
	}
	return llmclient.ProviderInfo{Provider: "anthropic", Family: family, Model: model, Capabilities: llmclient.ProviderCapabilities{Tools: true, StructuredOutput: true, Streaming: true, Usage: true, Refusal: true, Truncation: true, Cancellation: true}}
}
