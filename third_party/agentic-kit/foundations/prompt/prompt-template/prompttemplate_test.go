package prompttemplate

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"testing"
	"time"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// Golden constants for the canonical render case:
//
//	template: "Hello {{.name}}, you have {{.count}} alerts."
//	params:   {"name": "Ada", "count": 3}
//	rendered: "Hello Ada, you have 3 alerts."
//	canonical params JSON: {"count":3,"name":"Ada"}
const (
	goldenText       = "Hello Ada, you have 3 alerts."
	goldenPromptHash = "d88910b9dc06df18cf0eb9e9c7147de35c26627f09e7fcf939506914525df85d"
	goldenParamsHash = "0e588c326de8f1c5774cef5d23a38d9b1d17ad11d5a8623d64c9a8f8ded7f3b2"
	// emptyParamsHash is sha256("{}") — the hash of nil or empty params.
	emptyParamsHash = "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
)

var goldenTemplate = Template{
	Name:    "alert-summary",
	Version: "1.2.3",
	Text:    "Hello {{.name}}, you have {{.count}} alerts.",
}

func TestTemplateValidate(t *testing.T) {
	tests := []struct {
		name    string
		tpl     Template
		wantErr string // substring of the error; "" means valid
	}{
		{
			name: "valid template",
			tpl:  goldenTemplate,
		},
		{
			name: "valid static template without placeholders",
			tpl:  Template{Name: "static", Version: "0.1.0", Text: "no placeholders here"},
		},
		{
			name:    "empty name",
			tpl:     Template{Version: "1.0.0", Text: "x"},
			wantErr: "Name must not be empty",
		},
		{
			name:    "empty version",
			tpl:     Template{Name: "n", Text: "x"},
			wantErr: "Version must not be empty",
		},
		{
			name:    "empty text",
			tpl:     Template{Name: "n", Version: "1.0.0"},
			wantErr: "Text must not be empty",
		},
		{
			name:    "unparseable template text",
			tpl:     Template{Name: "broken", Version: "1.0.0", Text: "Hello {{.unclosed"},
			wantErr: "does not parse",
		},
		{
			name:    "unknown function",
			tpl:     Template{Name: "badfunc", Version: "1.0.0", Text: "{{nope .x}}"},
			wantErr: "does not parse",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tpl.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() = %q, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestRenderGolden(t *testing.T) {
	got, err := Render(goldenTemplate, map[string]any{"name": "Ada", "count": 3})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := Rendered{
		Text:            goldenText,
		PromptHash:      goldenPromptHash,
		TemplateVersion: "1.2.3",
		ParamsHash:      goldenParamsHash,
	}
	if got != want {
		t.Fatalf("Render() = %+v, want %+v", got, want)
	}
}

// TestParamsHashInsertionOrderIndependence is the core determinism guarantee:
// the same logical params map, built in different insertion orders (including
// nested maps), must produce identical ParamsHash values, because
// encoding/json sorts map keys at every nesting depth.
func TestParamsHashInsertionOrderIndependence(t *testing.T) {
	buildForward := func() map[string]any {
		p := map[string]any{}
		p["alpha"] = "a"
		p["beta"] = 2
		nested := map[string]any{}
		nested["yellow"] = true
		nested["zebra"] = []any{"s", 1}
		p["gamma"] = nested
		return p
	}
	buildReverse := func() map[string]any {
		nested := map[string]any{}
		nested["zebra"] = []any{"s", 1}
		nested["yellow"] = true
		p := map[string]any{}
		p["gamma"] = nested
		p["beta"] = 2
		p["alpha"] = "a"
		return p
	}

	tpl := Template{Name: "det", Version: "1.0.0", Text: "{{.alpha}}-{{.beta}}"}
	r1, err := Render(tpl, buildForward())
	if err != nil {
		t.Fatalf("Render(forward) error = %v", err)
	}
	r2, err := Render(tpl, buildReverse())
	if err != nil {
		t.Fatalf("Render(reverse) error = %v", err)
	}
	if r1.ParamsHash != r2.ParamsHash {
		t.Fatalf("ParamsHash differs across insertion orders: %s vs %s", r1.ParamsHash, r2.ParamsHash)
	}
	if r1 != r2 {
		t.Fatalf("Rendered differs across insertion orders: %+v vs %+v", r1, r2)
	}
}

func TestRenderErrors(t *testing.T) {
	tests := []struct {
		name    string
		tpl     Template
		params  map[string]any
		wantErr string
	}{
		{
			name:    "missing param key errors via missingkey=error",
			tpl:     Template{Name: "m", Version: "1.0.0", Text: "{{.present}} {{.absent}}"},
			params:  map[string]any{"present": "x"},
			wantErr: "absent",
		},
		{
			name:    "nil params with placeholder errors",
			tpl:     Template{Name: "m", Version: "1.0.0", Text: "{{.anything}}"},
			params:  nil,
			wantErr: "anything",
		},
		{
			name:    "invalid template rejected before rendering",
			tpl:     Template{Name: "broken", Version: "1.0.0", Text: "{{.unclosed"},
			params:  map[string]any{},
			wantErr: "does not parse",
		},
		{
			name:    "empty version rejected",
			tpl:     Template{Name: "n", Text: "x"},
			params:  map[string]any{},
			wantErr: "Version must not be empty",
		},
		{
			name:    "NaN param cannot be canonicalized",
			tpl:     Template{Name: "n", Version: "1.0.0", Text: "static"},
			params:  map[string]any{"temperature": math.NaN()},
			wantErr: "cannot be canonicalized",
		},
		{
			name:    "Inf param cannot be canonicalized",
			tpl:     Template{Name: "n", Version: "1.0.0", Text: "static"},
			params:  map[string]any{"limit": math.Inf(1)},
			wantErr: "cannot be canonicalized",
		},
		{
			name:    "channel param cannot be canonicalized",
			tpl:     Template{Name: "n", Version: "1.0.0", Text: "static"},
			params:  map[string]any{"ch": make(chan int)},
			wantErr: "cannot be canonicalized",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Render(tc.tpl, tc.params)
			if err == nil {
				t.Fatalf("Render() = nil error, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Render() error = %q, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestRenderNilAndEmptyParamsHashIdentically(t *testing.T) {
	tpl := Template{Name: "static", Version: "0.1.0", Text: "no placeholders"}
	rNil, err := Render(tpl, nil)
	if err != nil {
		t.Fatalf("Render(nil) error = %v", err)
	}
	rEmpty, err := Render(tpl, map[string]any{})
	if err != nil {
		t.Fatalf("Render(empty) error = %v", err)
	}
	if rNil.ParamsHash != emptyParamsHash {
		t.Fatalf("Render(nil).ParamsHash = %s, want sha256(%q) = %s", rNil.ParamsHash, "{}", emptyParamsHash)
	}
	if rNil.ParamsHash != rEmpty.ParamsHash {
		t.Fatalf("nil and empty params hash differently: %s vs %s", rNil.ParamsHash, rEmpty.ParamsHash)
	}
}

// TestStructParamsHashedByJSONForm documents and verifies that struct values
// inside params are hashed by their JSON encoding (declaration-ordered
// fields, honouring json tags), which is deterministic for a given type.
func TestStructParamsHashedByJSONForm(t *testing.T) {
	type sampling struct {
		Temperature float64 `json:"temperature"`
		TopP        float64 `json:"top_p"`
	}
	tpl := Template{Name: "static", Version: "0.1.0", Text: "static text"}
	params := map[string]any{"sampling": sampling{Temperature: 0.2, TopP: 0.9}}

	got, err := Render(tpl, params)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	wantJSON := `{"sampling":{"temperature":0.2,"top_p":0.9}}`
	sum := sha256.Sum256([]byte(wantJSON))
	if want := hex.EncodeToString(sum[:]); got.ParamsHash != want {
		t.Fatalf("ParamsHash = %s, want sha256(%s) = %s", got.ParamsHash, wantJSON, want)
	}

	// Two renders with equal struct values must agree.
	again, err := Render(tpl, map[string]any{"sampling": sampling{Temperature: 0.2, TopP: 0.9}})
	if err != nil {
		t.Fatalf("Render() second call error = %v", err)
	}
	if got.ParamsHash != again.ParamsHash {
		t.Fatalf("ParamsHash unstable across renders: %s vs %s", got.ParamsHash, again.ParamsHash)
	}
}

func TestVersionFlowsThrough(t *testing.T) {
	tpl := goldenTemplate
	tpl.Version = "9.8.7"
	r, err := Render(tpl, map[string]any{"name": "Ada", "count": 1})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if r.TemplateVersion != "9.8.7" {
		t.Fatalf("TemplateVersion = %q, want %q", r.TemplateVersion, "9.8.7")
	}

	var ai aiprovenance.AIAuthor
	r.ApplyTo(&ai)
	if ai.PromptTemplateVersion != "9.8.7" {
		t.Fatalf("AIAuthor.PromptTemplateVersion = %q, want %q", ai.PromptTemplateVersion, "9.8.7")
	}
}

func TestApplyTo(t *testing.T) {
	r, err := Render(goldenTemplate, map[string]any{"name": "Ada", "count": 3})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	at := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	author := aiprovenance.Author{
		Kind: aiprovenance.KindAI,
		AI: &aiprovenance.AIAuthor{
			ModelID:      "claude-opus-4-8",
			ModelVersion: "20260608",
			InputDigest:  "sha256-input000",
			At:           at,
		},
	}
	r.ApplyTo(author.AI)

	if author.AI.PromptHash != goldenPromptHash {
		t.Fatalf("AIAuthor.PromptHash = %q, want %q", author.AI.PromptHash, goldenPromptHash)
	}
	if author.AI.PromptTemplateVersion != "1.2.3" {
		t.Fatalf("AIAuthor.PromptTemplateVersion = %q, want %q", author.AI.PromptTemplateVersion, "1.2.3")
	}
	if author.AI.ParamsHash != goldenParamsHash {
		t.Fatalf("AIAuthor.ParamsHash = %q, want %q", author.AI.ParamsHash, goldenParamsHash)
	}
	// Fields ApplyTo must not touch.
	if author.AI.ModelID != "claude-opus-4-8" || author.AI.ModelVersion != "20260608" ||
		author.AI.InputDigest != "sha256-input000" || !author.AI.At.Equal(at) {
		t.Fatalf("ApplyTo modified non-prompt fields: %+v", author.AI)
	}
	// The completed Author must satisfy ai-provenance's own invariants.
	if err := author.Validate(); err != nil {
		t.Fatalf("Author.Validate() after ApplyTo = %v, want nil", err)
	}
}

func TestApplyToNilIsNoOp(t *testing.T) {
	r := Rendered{PromptHash: "x", TemplateVersion: "1.0.0", ParamsHash: "y"}
	r.ApplyTo(nil) // must not panic
}

// ─── F6: ParamsFromStruct ────────────────────────────────────────────────────

// TestParamsFromStructBasic proves that string fields come back as plain
// strings and numeric fields come back as their JSON representation.
func TestParamsFromStructBasic(t *testing.T) {
	type params struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
		Flag  bool   `json:"flag"`
	}
	got, err := ParamsFromStruct(params{Name: "Alice", Count: 3, Flag: true})
	if err != nil {
		t.Fatalf("ParamsFromStruct: %v", err)
	}
	if got["name"] != "Alice" {
		t.Errorf("name = %q, want %q", got["name"], "Alice")
	}
	if got["count"] != "3" {
		t.Errorf("count = %q, want \"3\"", got["count"])
	}
	if got["flag"] != "true" {
		t.Errorf("flag = %q, want \"true\"", got["flag"])
	}
}

// TestParamsFromStructJSONTagNames proves that the json: tag name — not the
// Go field name — becomes the map key (key-drift regression).
func TestParamsFromStructJSONTagNames(t *testing.T) {
	type params struct {
		DisplayName string `json:"display_name"`
		MaxRetries  int    `json:"max_retries"`
	}
	got, err := ParamsFromStruct(params{DisplayName: "Bob", MaxRetries: 5})
	if err != nil {
		t.Fatalf("ParamsFromStruct: %v", err)
	}
	// Keys must be the json tag names.
	if _, ok := got["display_name"]; !ok {
		t.Errorf("expected key %q (json tag), not found; got keys: %v", "display_name", mapKeys(got))
	}
	if _, ok := got["max_retries"]; !ok {
		t.Errorf("expected key %q (json tag), not found; got keys: %v", "max_retries", mapKeys(got))
	}
	// Go field names must NOT appear.
	if _, ok := got["DisplayName"]; ok {
		t.Error("Go field name DisplayName should not be a key — json tag name should be used")
	}
}

// TestParamsFromStructKeyDriftRegression is the key-drift pin test: renaming
// the Go field but keeping the json: tag must NOT change the key. Changing the
// json: tag name MUST change the key.
func TestParamsFromStructKeyDriftRegression(t *testing.T) {
	// v1: Go field is "OldGoName", json tag is "stable_key".
	type v1 struct {
		OldGoName string `json:"stable_key"`
	}
	// v2: Go field renamed to "NewGoName", same json tag.
	type v2 struct {
		NewGoName string `json:"stable_key"`
	}
	got1, err := ParamsFromStruct(v1{OldGoName: "x"})
	if err != nil {
		t.Fatalf("ParamsFromStruct v1: %v", err)
	}
	got2, err := ParamsFromStruct(v2{NewGoName: "x"})
	if err != nil {
		t.Fatalf("ParamsFromStruct v2: %v", err)
	}
	if got1["stable_key"] != got2["stable_key"] {
		t.Errorf("key changed after Go field rename: v1=%q v2=%q (json tag preserved — should be identical)", got1["stable_key"], got2["stable_key"])
	}

	// Now change the json tag — the key MUST change.
	type v3 struct {
		NewGoName string `json:"new_key"`
	}
	got3, err := ParamsFromStruct(v3{NewGoName: "x"})
	if err != nil {
		t.Fatalf("ParamsFromStruct v3: %v", err)
	}
	if _, ok := got3["stable_key"]; ok {
		t.Error("old key stable_key should not appear after json tag rename")
	}
	if _, ok := got3["new_key"]; !ok {
		t.Error("new key new_key should appear after json tag rename")
	}
}

// TestParamsFromStructNestedStructEncodedAsJSON proves that nested struct
// values are stored as their canonical JSON string (not recursively expanded).
func TestParamsFromStructNestedStructEncodedAsJSON(t *testing.T) {
	type inner struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	type outer struct {
		Point inner `json:"point"`
	}
	got, err := ParamsFromStruct(outer{Point: inner{X: 1, Y: 2}})
	if err != nil {
		t.Fatalf("ParamsFromStruct: %v", err)
	}
	if got["point"] != `{"x":1,"y":2}` {
		t.Errorf("point = %q, want %q", got["point"], `{"x":1,"y":2}`)
	}
}

// TestParamsFromStructPointerReceiver proves that a pointer to struct works.
func TestParamsFromStructPointerReceiver(t *testing.T) {
	type params struct {
		Greeting string `json:"greeting"`
	}
	got, err := ParamsFromStruct(&params{Greeting: "hello"})
	if err != nil {
		t.Fatalf("ParamsFromStruct: %v", err)
	}
	if got["greeting"] != "hello" {
		t.Errorf("greeting = %q, want %q", got["greeting"], "hello")
	}
}

// TestParamsFromStructNilErrors proves that nil input is rejected.
func TestParamsFromStructNilErrors(t *testing.T) {
	if _, err := ParamsFromStruct(nil); err == nil {
		t.Fatal("ParamsFromStruct(nil) should return error")
	}
}

// TestParamsFromStructSharesHashPathWithRender proves that ParamsFromStruct
// output, when passed to Render via map[string]any, produces the same
// ParamsHash as building the map manually with the same values.
//
// This is the canonical-JSON hash-sharing guarantee from friction §6.
func TestParamsFromStructSharesHashPathWithRender(t *testing.T) {
	type caseCtx struct {
		CaseID string `json:"case_id"`
		Stage  string `json:"stage"`
	}
	tpl := Template{Name: "t", Version: "1.0.0", Text: "case {{.case_id}} stage {{.stage}}"}

	// Path A: manual map.
	manualParams := map[string]any{"case_id": "c-001", "stage": "triage"}
	rA, err := Render(tpl, manualParams)
	if err != nil {
		t.Fatalf("Render (manual): %v", err)
	}

	// Path B: ParamsFromStruct → map[string]any.
	structParams, err := ParamsFromStruct(caseCtx{CaseID: "c-001", Stage: "triage"})
	if err != nil {
		t.Fatalf("ParamsFromStruct: %v", err)
	}
	anyParams := make(map[string]any, len(structParams))
	for k, v := range structParams {
		anyParams[k] = v
	}
	rB, err := Render(tpl, anyParams)
	if err != nil {
		t.Fatalf("Render (struct): %v", err)
	}

	if rA.ParamsHash != rB.ParamsHash {
		t.Errorf("ParamsHash mismatch: manual=%s struct=%s", rA.ParamsHash, rB.ParamsHash)
	}
	if rA.Text != rB.Text {
		t.Errorf("rendered text mismatch: manual=%q struct=%q", rA.Text, rB.Text)
	}
}

// TestParamsFromStructNonStructNonMapErrors proves that non-struct, non-map
// values (arrays, primitives) are rejected.
func TestParamsFromStructNonStructNonMapErrors(t *testing.T) {
	cases := []struct {
		name string
		v    any
	}{
		{"slice", []string{"a", "b"}},
		{"string", "hello"},
		{"int", 42},
		{"bool", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParamsFromStruct(tc.v); err == nil {
				t.Errorf("ParamsFromStruct(%T) should return error for non-struct input", tc.v)
			}
		})
	}
}

// mapKeys returns the sorted keys of a map for diagnostic output.
func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
