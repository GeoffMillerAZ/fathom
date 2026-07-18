package jsonschemagen_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
	jsonschemagen "github.com/geoffmilleraz/agentic-kit/toolchains/jsonschema"
)

// TestOutputSchemaEndToEnd is the structured-outputs proof: the SAME CUE
// contract that validates an artifact constrains the model's generation.
// It derives JSON Schema from testdata/outputschema/outputschema.cue via the
// cue binary, sets the derived #ScoredAnalysis definition as
// Request.OutputSchema, sends the request through the real anthropic adapter,
// and asserts the schema arrives on the wire under output_config.format.
func TestOutputSchemaEndToEnd(t *testing.T) {
	skipIfNoCUE(t)

	derived, err := jsonschemagen.DeriveFromDir("testdata/outputschema", jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}
	var doc struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(derived, &doc); err != nil {
		t.Fatalf("derived output is not valid JSON: %v", err)
	}
	schema, ok := doc.Defs["ScoredAnalysis"]
	if !ok {
		t.Fatalf("derived $defs missing ScoredAnalysis; got keys %v", keys(doc.Defs))
	}

	// The derived definition must satisfy the contract-side validation.
	if err := llmclient.ValidateOutputSchema(schema, "e2e"); err != nil {
		t.Fatalf("derived schema rejected by ValidateOutputSchema: %v", err)
	}

	// Capture the wire body via a fake Messages endpoint.
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{}"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer srv.Close()

	client, err := anthropic.New(anthropic.Config{
		APIKey:       "test-key",
		BaseURL:      srv.URL,
		DefaultModel: "example-model",
	})
	if err != nil {
		t.Fatalf("anthropic.New: %v", err)
	}

	_, err = client.Complete(t.Context(), llmclient.Request{
		Prompt:       "analyze this",
		OutputSchema: schema,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var wire struct {
		OutputConfig struct {
			Format struct {
				Type   string          `json:"type"`
				Schema json.RawMessage `json:"schema"`
			} `json:"format"`
		} `json:"output_config"`
	}
	if err := json.Unmarshal(gotBody, &wire); err != nil {
		t.Fatalf("wire body is not valid JSON: %v", err)
	}
	if wire.OutputConfig.Format.Type != "json_schema" {
		t.Errorf("output_config.format.type = %q, want %q", wire.OutputConfig.Format.Type, "json_schema")
	}

	// The schema must arrive semantically identical to what was derived.
	var want, got any
	if err := json.Unmarshal(schema, &want); err != nil {
		t.Fatalf("unmarshal derived schema: %v", err)
	}
	if err := json.Unmarshal(wire.OutputConfig.Format.Schema, &got); err != nil {
		t.Fatalf("unmarshal wire schema: %v", err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("wire schema differs from derived schema:\nderived: %s\nwire:    %s", wantJSON, gotJSON)
	}

	// And the derived schema constrains what the CUE says it does: the enum
	// and the required fields must be present so the provider-side decoder is
	// actually constrained, not decorative.
	for _, needle := range []string{`"positive"`, `"negative"`, `"neutral"`, `"required"`} {
		if !containsJSON(schema, needle) {
			t.Errorf("derived schema missing %s:\n%s", needle, schema)
		}
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func containsJSON(raw json.RawMessage, needle string) bool {
	return json.Valid(raw) && strings.Contains(string(raw), needle)
}
