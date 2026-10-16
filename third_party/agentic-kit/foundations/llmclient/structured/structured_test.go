package structured_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	llmclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	structured "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/structured"
)

type claim struct {
	Rule  string `json:"rule"`
	Scope string `json:"scope"`
}

var schema = json.RawMessage(`{"type":"object","properties":{"rule":{"type":"string"},"scope":{"type":"string"}}}`)

// scriptClient returns a fixed response — the mock's option surface isn't
// needed for classification tests.
type scriptClient struct{ resp llmclient.Response }

func (s scriptClient) Complete(_ context.Context, req llmclient.Request) (llmclient.Response, error) {
	if len(req.OutputSchema) == 0 {
		return llmclient.Response{}, errors.New("test: OutputSchema was not set on the request")
	}
	return s.resp, nil
}

func (s scriptClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	ch := make(chan llmclient.StreamChunk)
	close(ch)
	return ch, nil
}

func TestComplete_HappyPath(t *testing.T) {
	c := scriptClient{resp: llmclient.Response{
		Text:       `{"rule":"sort outputs","scope":"internal/query/**"}`,
		StopReason: llmclient.StopEndTurn,
	}}
	var out claim
	resp, err := structured.Complete(context.Background(), c, llmclient.Request{}, schema, &out)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out.Rule != "sort outputs" || out.Scope != "internal/query/**" {
		t.Errorf("decoded %+v", out)
	}
	if resp.Text == "" {
		t.Error("Response must be returned for accounting")
	}
}

func TestComplete_TruncationIsTyped(t *testing.T) {
	c := scriptClient{resp: llmclient.Response{
		Text:       `{"rule":"sort outp`, // mid-JSON prefix — the fleet-compile failure
		StopReason: llmclient.StopMaxTokens,
	}}
	var out claim
	resp, err := structured.Complete(context.Background(), c, llmclient.Request{}, schema, &out)
	if !errors.Is(err, structured.ErrTruncated) {
		t.Fatalf("want ErrTruncated, got %v", err)
	}
	if errors.Is(err, structured.ErrMalformed) {
		t.Error("truncation must not classify as malformed")
	}
	if resp.StopReason != llmclient.StopMaxTokens {
		t.Error("Response must be returned with the error")
	}
}

func TestComplete_MalformedAndUnknownFields(t *testing.T) {
	var out claim
	// Garbage text.
	c := scriptClient{resp: llmclient.Response{Text: "sorry, as an AI...", StopReason: llmclient.StopEndTurn}}
	if _, err := structured.Complete(context.Background(), c, llmclient.Request{}, schema, &out); !errors.Is(err, structured.ErrMalformed) {
		t.Fatalf("garbage: want ErrMalformed, got %v", err)
	}
	// Unknown field = schema drift; strict decode must catch it.
	c = scriptClient{resp: llmclient.Response{Text: `{"rule":"x","scope":"y","confidence":0.9}`, StopReason: llmclient.StopEndTurn}}
	if _, err := structured.Complete(context.Background(), c, llmclient.Request{}, schema, &out); !errors.Is(err, structured.ErrMalformed) {
		t.Fatalf("unknown field: want ErrMalformed, got %v", err)
	}
}

func TestComplete_GuardRails(t *testing.T) {
	var out claim
	ctx := context.Background()
	if _, err := structured.Complete(ctx, nil, llmclient.Request{}, schema, &out); err == nil {
		t.Error("nil client must error")
	}
	c := scriptClient{resp: llmclient.Response{Text: "{}", StopReason: llmclient.StopEndTurn}}
	if _, err := structured.Complete(ctx, c, llmclient.Request{}, nil, &out); err == nil {
		t.Error("empty schema must error")
	}
	if _, err := structured.Complete(ctx, c, llmclient.Request{}, schema, nil); err == nil {
		t.Error("nil out must error")
	}
	pre := llmclient.Request{OutputSchema: schema}
	if _, err := structured.Complete(ctx, c, pre, schema, &out); err == nil {
		t.Error("pre-set OutputSchema must error — ambiguous intent")
	}
}
