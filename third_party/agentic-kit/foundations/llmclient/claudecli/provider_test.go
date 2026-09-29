package claudecli

import (
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"testing"
)

func TestProviderMetadata(t *testing.T) {
	c := &Client{model: "default"}
	for _, model := range []string{"", "override"} {
		info := c.DescribeProvider(llmclient.Request{Model: model})
		want := model
		if want == "" {
			want = "default"
		}
		if info.Model != want || info.Provider != "claudecli" || info.Family != "" || info.Capabilities.Tools || info.Capabilities.Streaming || info.Capabilities.StructuredOutput || !info.Capabilities.Cancellation {
			t.Fatalf("%+v", info)
		}
	}
}
