package extractor

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

type fcEntity struct {
	Name, Sig string
	Kind      core.SourceKind
	Line, End int
}

// fathomCUEEntities is the dialect's own entities of one file, sorted: the
// composed CUE nodes (kinds CUE*) are the cue dialect's and tested there.
func fathomCUEEntities(t *testing.T, id, rel, fixture string) ([]fcEntity, []core.SymbolNode) {
	t.Helper()
	x, ok := NativeDialectExtractor(id)
	if !ok {
		t.Fatalf("NativeDialectExtractor(%q): not a native dialect", id)
	}
	src, err := os.ReadFile(filepath.Join("dialects", id, "testdata", filepath.FromSlash(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := x.Extract("svc", rel, src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var out []fcEntity
	for _, n := range nodes {
		switch n.Kind {
		case KindFathomConcern, KindFathomLayer, KindFathomRuleSet:
			out = append(out, fcEntity{n.Name, n.Signature, n.Kind, n.Line, n.EndLine})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nodes
}

func TestFathomConcerns_Entities(t *testing.T) {
	got, nodes := fathomCUEEntities(t, "fathom-concerns", ".fathom/concerns.cue", ".fathom/concerns.cue")
	want := []fcEntity{
		{"concern/deploy_safety", "no raw-cp deploy instruction anywhere in agent config", KindFathomConcern, 12, 16},
		{"concern/guide_thin", "docs/guide.md stays within its line budget", KindFathomConcern, 23, 26},
		{"concern/intro_thin", "docs/intro.md stays within its line budget", KindFathomConcern, 23, 26},
		{"concern/notes_thin", "docs/notes.md stays within its line budget", KindFathomConcern, 23, 26},
		{"concern/pointers_resolve_off", "", KindFathomConcern, 21, 21},
		{"concern/router_thin", "the root AGENTS.md stays a router", KindFathomConcern, 17, 20},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("concerns:\n got %+v\nwant %+v", got, want)
	}
	// The composed CUE extraction is unchanged: the hidden table the
	// comprehension reads is a reference of the concerns field.
	var calls []string
	for _, n := range nodes {
		if n.Name == "concerns" {
			calls = append(calls, n.Calls...)
		}
	}
	if !reflect.DeepEqual(calls, []string{"svc::concerns._docs"}) {
		t.Errorf("concerns calls = %v, want [svc::concerns._docs]", calls)
	}
}

func TestFathomLayerPacks_Entities(t *testing.T) {
	got, _ := fathomCUEEntities(t, "fathom-layer-packs", ".fathom/layers/demo/layer.cue", ".fathom/layers/demo/layer.cue")
	want := []fcEntity{{"layer/demo", "adr-rule v1", KindFathomLayer, 8, 8}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("layer:\n got %+v\nwant %+v", got, want)
	}
	got, nodes := fathomCUEEntities(t, "fathom-layer-packs", ".fathom/rules/demo.cue", ".fathom/rules/demo.cue")
	want = []fcEntity{{"ruleset/demo", "3 rules", KindFathomRuleSet, 5, 10}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ruleset:\n got %+v\nwant %+v", got, want)
	}
	if len(nodes) < 2 {
		t.Errorf("composed CUE nodes missing: %+v", nodes)
	}
}

func TestFathomLayerPacks_SurvivesSyntaxError(t *testing.T) {
	got, _ := fathomCUEEntities(t, "fathom-layer-packs", ".fathom/rules/broken.cue", ".fathom/rules/broken.cue")
	for _, e := range got {
		if e.Kind == KindFathomRuleSet {
			return
		}
	}
	t.Errorf("a rule file with a syntax error should still name its rule set: %+v", got)
}

// A file that does not evaluate on its own (an unresolved import) falls back
// to the static reading: the labels and literals the syntax carries.
func TestFathomConcerns_StaticFallback(t *testing.T) {
	x, _ := NativeDialectExtractor("fathom-concerns")
	src := []byte("import \"example.com/x\"\nconcerns: {\n\ta: {want: \"w\", check: x.#C}\n\tb: {want: \"v\"}\n}\n")
	nodes, err := x.Extract("svc", ".fathom/concerns.cue", src)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range nodes {
		if n.Kind == KindFathomConcern {
			names = append(names, n.Name)
		}
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"concern/a", "concern/b"}) {
		t.Errorf("static fallback names = %v", names)
	}
}
