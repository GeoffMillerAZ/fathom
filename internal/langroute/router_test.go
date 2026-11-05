package langroute

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// stubExtractor stands in for a registered extractor.
type stubExtractor struct{}

func (stubExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	return nil, nil
}

var _ extractor.Extractor = stubExtractor{}

// TestRouterWithoutOptionsSkipsProductFormats: a Router built with no options
// reads the languages and skips CUE, Timoni and fathom's own formats (LL-4).
func TestRouterWithoutOptionsSkipsProductFormats(t *testing.T) {
	r := newTestRouter(t)
	if e, ok := r.Exts()[".go"]; !ok || e == nil {
		t.Fatal("a bare Router has no .go extractor")
	}
	if _, ok := r.Exts()[".cue"]; ok {
		t.Error("a bare Router claims .cue; CUE comes through WithCUE")
	}
	if _, _, ok, _ := r.ResolveExtractor("schema/a.cue", []byte("package a\n"), nil); ok {
		t.Error("a bare Router resolves a .cue file")
	}
	if _, _, ok := r.TimoniDispatch("mods/app/timoni.cue", []byte("package main\n"), nil); ok {
		t.Error("a bare Router claims a Timoni module; Timoni comes through WithTimoni")
	}
	if _, _, ok := r.formatDispatch(".fathom/config.toml", []byte("x = 1\n")); ok {
		t.Error("a bare Router claims a fathom format; formats come through WithFormats")
	}
	// The generic TOML extractor still reads the file, as it does for any project.
	if e, ok := r.Resolve("", ".fathom/config.toml", []byte("x = 1\n")); !ok || e == nil {
		t.Error("a bare Router does not read .fathom/config.toml as plain TOML")
	}
}

// TestRouterOptionsRegister: the registration options reach the dispatch.
func TestRouterOptionsRegister(t *testing.T) {
	cue := stubExtractor{}
	formats := func(relPath string, content []byte) (extractor.Extractor, ToolKind, bool) {
		if relPath == "docs/plans/a.md" {
			return cue, "fathom-plan-rows", true
		}
		return nil, "", false
	}
	r := newTestRouter(t, WithCUE(cue), WithTimoni(cue), WithFormats(formats))
	if e, _, ok, _ := r.ResolveExtractor("a.cue", []byte("package a\n"), nil); !ok || e != cue {
		t.Errorf("WithCUE: ResolveExtractor = %v, %v", e, ok)
	}
	if e, tk, ok, _ := r.ResolveExtractor("docs/plans/a.md", []byte("# plan\n"), nil); !ok || e != cue || tk != "fathom-plan-rows" {
		t.Errorf("WithFormats: ResolveExtractor = %v, %q, %v", e, tk, ok)
	}
	if e, ok := r.Resolve("", "docs/plans/a.md", []byte("# plan\n")); !ok || e != cue {
		t.Errorf("WithFormats: Resolve = %v, %v", e, ok)
	}
	if e, tk, ok := r.TimoniDispatch("x/timoni.cue", []byte("package main\n"), nil); !ok || e != cue || tk != ToolTimoni {
		t.Errorf("WithTimoni: TimoniDispatch = %v, %q, %v", e, tk, ok)
	}
}

// TestRouterExtractFileGates: the gates run before dispatch, with no store.
func TestRouterExtractFileGates(t *testing.T) {
	r := newTestRouter(t)
	built := 0
	idx := func() *SiblingIndex { built++; return nil }
	cases := []struct {
		name, path string
		src        []byte
		skipped    string
		wantBuilt  int
	}{
		{"binary", "a.bin", []byte("\x00\x01\x02"), "binary", 0},
		{"oversize", "a.go", []byte(strings.Repeat("x", MaxExtractSize+1)), "oversize", 0},
		{"go", "a.go", []byte("package a\n\nfunc F() {}\n"), "", 1},
		{"nothing reads it", "a.unknownext", []byte("text\n"), "no-extractor", 1},
	}
	for _, c := range cases {
		built = 0
		ex, err := r.ExtractFile(nil, "svc", idx, c.path, c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if ex.Skipped != c.skipped {
			t.Errorf("%s: Skipped = %q, want %q", c.name, ex.Skipped, c.skipped)
		}
		if built != c.wantBuilt {
			t.Errorf("%s: the sibling index was built %d times, want %d (only past the gates)", c.name, built, c.wantBuilt)
		}
	}
}

// TestLangrouteClosure: the router links core, extractor, langclass and
// tsengine, never the connector, the store or the pipeline (ADR-092).
func TestLangrouteClosure(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go toolchain on PATH")
	}
	out, err := exec.Command(goBin, "list", "-deps", "-f", "{{.ImportPath}}", ".").CombinedOutput()
	if err != nil {
		t.Skipf("go list failed (offline or no module cache): %v\n%s", err, out)
	}
	for _, p := range strings.Fields(string(out)) {
		for _, banned := range []string{"/internal/connector", "/internal/store", "/internal/fusion", "/internal/verify", "/internal/llm", "/internal/extractor/cuelang", "/internal/planrowsdialect"} {
			if strings.HasSuffix(p, banned) {
				t.Errorf("internal/langroute reaches %s", p)
			}
		}
	}
}
