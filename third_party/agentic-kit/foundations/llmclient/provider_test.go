package llmclient_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
	llmegress "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/egress"
	llminstrument "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/instrument"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/openai"
	llmreplay "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/replay"
)

func TestProviderIdentityAndWrapperPreservation(t *testing.T) {
	var nilProvider *openai.Client
	for _, client := range []llmclient.LLMClient{nil, nilProvider} {
		if got := llmclient.DescribeProvider(client, llmclient.Request{}); got != (llmclient.ProviderInfo{}) {
			t.Fatal("nil provider must remain unknown")
		}
	}
	c, err := openai.New(openai.Config{DefaultModel: "default", UseResponses: true})
	if err != nil {
		t.Fatal(err)
	}
	req := llmclient.Request{Model: "requested"}
	want := llmclient.DescribeProvider(c, req)
	if want.Model != "requested" || want.Family != "openai" || !want.Capabilities.Streaming {
		t.Fatalf("%+v", want)
	}
	clients := []llmclient.LLMClient{
		llmreplay.NewRecordingClient(c, io.Discard),
		llmclient.WithRetry(c, llmclient.RetryConfig{MaxAttempts: 1}),
		llmegress.WithEgress(c, llmegress.TransformerFunc(func(_ context.Context, s string) (string, error) { return s, nil })),
		llminstrument.WithCost(c, nil, nil, "test"),
		llminstrument.WithTelemetry(c, nil, "test", nil),
		llminstrument.WithBudget(c, nil, nil, nil, "pricing-only"),
	}
	for _, wrapped := range clients {
		if got := llmclient.DescribeProvider(wrapped, req); got != want {
			t.Fatalf("%T %+v != %+v", wrapped, got, want)
		}
	}
	legacy := llmclient.NewMockClient()
	for _, unknown := range []llmclient.LLMClient{legacy, llmclient.WithRetry(legacy, llmclient.RetryConfig{MaxAttempts: 1}), llmclient.NewSwappable(c), llmclient.NewFailover([]llmclient.LLMClient{c, c})} {
		if got := llmclient.DescribeProvider(unknown, req); got != (llmclient.ProviderInfo{}) {
			t.Fatalf("mutable/legacy %T has identity %+v", unknown, got)
		}
	}
	a, err := anthropic.New(anthropic.Config{APIKey: "test-credential", DefaultModel: "default"})
	if err != nil {
		t.Fatal(err)
	}
	info := a.DescribeProvider(llmclient.Request{ProviderParams: json.RawMessage(`{"model":"override"}`)})
	if info.Model != "override" || info.Family != "anthropic" || !llmclient.IndependentProviders(want, info) {
		t.Fatalf("%+v", info)
	}
	for _, family := range []string{"", "unknown", " UNKNOWN ", " OPENAI "} {
		if llmclient.IndependentProviders(want, llmclient.ProviderInfo{Family: family}) {
			t.Fatalf("independence falsely certified %q", family)
		}
	}
}
