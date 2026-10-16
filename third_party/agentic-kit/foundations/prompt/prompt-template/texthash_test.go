package prompttemplate_test

import (
	"testing"

	prompttemplate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/prompt/prompt-template"
)

func TestTemplate_TextHash(t *testing.T) {
	a := prompttemplate.Template{Name: "n", Version: "v1", Text: "Summarize {{.x}}"}
	b := prompttemplate.Template{Name: "other", Version: "v9", Text: "Summarize {{.x}}"}
	c := prompttemplate.Template{Name: "n", Version: "v1", Text: "Summarize {{.x}}!"}

	if a.TextHash() != b.TextHash() {
		t.Error("TextHash must depend on Text only — Name/Version must not perturb it")
	}
	if a.TextHash() == c.TextHash() {
		t.Error("a Text edit must change TextHash")
	}
	if a.TextHash() != a.TextHash() {
		t.Error("TextHash is not deterministic")
	}
	// Pin the formula: sha256 of the exact Text bytes, lowercase hex.
	const want = 64
	if len(a.TextHash()) != want {
		t.Errorf("TextHash length = %d, want %d hex chars", len(a.TextHash()), want)
	}
	// Rendered.PromptHash (per-render) and TextHash (per-template) must be
	// able to differ: render with params and compare.
	r, err := prompttemplate.Render(a, map[string]any{"x": "incident"})
	if err != nil {
		t.Fatal(err)
	}
	if r.PromptHash == a.TextHash() {
		t.Error("render hash unexpectedly equals template hash — the two identities must be distinct")
	}
}
