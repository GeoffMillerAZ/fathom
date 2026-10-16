package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

func TestEffortAndSchemaWireContract(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`)
	for _, effort := range []string{"", "low", "medium", "high", "xhigh", "max"} {
		for _, withSchema := range []bool{false, true} {
			for _, stream := range []bool{false, true} {
				t.Run(effort+map[bool]string{false: "/plain", true: "/schema"}[withSchema]+map[bool]string{false: "/complete", true: "/stream"}[stream], func(t *testing.T) {
					c, _ := captureBody(t, Config{})
					req := llmclient.Request{Prompt: "synthetic", Effort: effort}
					if withSchema {
						req.OutputSchema = schema
					}
					raw, err := c.buildBody(req, stream)
					if err != nil {
						t.Fatal(err)
					}
					var body struct {
						OutputConfig *struct {
							Effort string `json:"effort"`
							Format *struct {
								Type   string          `json:"type"`
								Schema json.RawMessage `json:"schema"`
							} `json:"format"`
						} `json:"output_config"`
					}
					if err := json.Unmarshal(raw, &body); err != nil {
						t.Fatal(err)
					}
					if effort == "" && !withSchema {
						if body.OutputConfig != nil {
							t.Fatalf("zero value emitted output_config: %s", raw)
						}
						return
					}
					if body.OutputConfig == nil || body.OutputConfig.Effort != effort {
						t.Fatalf("effort missing: %s", raw)
					}
					if withSchema {
						if body.OutputConfig.Format == nil || body.OutputConfig.Format.Type != "json_schema" || string(body.OutputConfig.Format.Schema) != string(schema) {
							t.Fatalf("schema dropped: %s", raw)
						}
					} else if body.OutputConfig.Format != nil {
						t.Fatalf("effort invented schema: %s", raw)
					}
				})
			}
		}
	}
}

func TestEffortRejectsUnsupportedAndRawConflictsBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  llmclient.Request
	}{
		{"invalid", llmclient.Request{Effort: "unbounded"}},
		{"whitespace", llmclient.Request{Effort: " high "}},
		{"case", llmclient.Request{Effort: "HIGH"}},
		{"effort override", llmclient.Request{Effort: "high", ProviderParams: json.RawMessage(`{"output_config":{"effort":"low"}}`)}},
		{"equal override", llmclient.Request{Effort: "high", ProviderParams: json.RawMessage(`{"output_config":{"effort":"high"}}`)}},
		{"schema override", llmclient.Request{OutputSchema: json.RawMessage(`{"type":"object"}`), ProviderParams: json.RawMessage(`{"output_config":null}`)}},
		{"effort and schema", llmclient.Request{Effort: "high", OutputSchema: json.RawMessage(`{"type":"object"}`), ProviderParams: json.RawMessage(`{"output_config":{}}`)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				writeJSON(w, textResponseBody(t, "unexpected"))
			}), Config{})
			tc.req.Prompt = "synthetic"
			_, err := c.Complete(context.Background(), tc.req)
			var invalid *llmclient.InvalidRequestError
			if !errors.As(err, &invalid) {
				t.Fatalf("Complete: %v", err)
			}
			_, err = c.Stream(context.Background(), tc.req)
			if !errors.As(err, &invalid) {
				t.Fatalf("Stream: %v", err)
			}
			if called {
				t.Fatal("invalid request reached HTTP")
			}
		})
	}
}

func TestEffortHTTPAndRawOnlyEscapeHatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  llmclient.Request
	}{
		{"typed", llmclient.Request{Effort: "high", OutputSchema: json.RawMessage(`{"type":"object"}`)}},
		{"raw only", llmclient.Request{ProviderParams: json.RawMessage(`{"output_config":{"effort":"future-value"}}`)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, body := captureBody(t, Config{})
			tc.req.Prompt = "synthetic"
			if _, err := c.Complete(context.Background(), tc.req); err != nil {
				t.Fatal(err)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(*body, &wire); err != nil {
				t.Fatal(err)
			}
			if len(wire["output_config"]) == 0 {
				t.Fatal("output config missing")
			}
		})
	}
}

func TestEffortCapabilityCheckPrecedesHTTP(t *testing.T) {
	failure := errors.New("selected model rejects xhigh")
	c, body := captureBody(t, Config{CapabilityCheck: func(req llmclient.Request) error {
		if req.Effort == "xhigh" {
			return failure
		}
		return nil
	}})
	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "synthetic", Effort: "xhigh"})
	if !errors.Is(err, failure) || len(*body) != 0 {
		t.Fatalf("capability rejection lost: %v body=%s", err, *body)
	}
}
