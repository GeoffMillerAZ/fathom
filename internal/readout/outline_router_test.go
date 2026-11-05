package readout

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fathomformat"
	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// TestReadoutDoesNotImportFusion pins LL-6 (ADR-092): the outline builds a
// langroute.Router (newOutlineRouter), not a fusion.Pipeline, so the readout's
// closure holds neither the store nor the LLM client through the pipeline. It
// fails if the package, or anything it imports, reaches internal/fusion.
func TestReadoutDoesNotImportFusion(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go toolchain on PATH")
	}
	out, err := exec.Command(goBin, "list", "-deps", "-f", "{{.ImportPath}}", ".").CombinedOutput()
	if err != nil {
		t.Skipf("go list failed (offline or no module cache): %v\n%s", err, out)
	}
	for _, p := range strings.Fields(string(out)) {
		if strings.HasSuffix(p, "/internal/fusion") {
			t.Fatalf("internal/readout reaches %s; the outline must build a langroute.Router (newOutlineRouter)", p)
		}
	}
}

// TestNewOutlineRouter: the outline's router builds, and reads a Go file.
func TestNewOutlineRouter(t *testing.T) {
	r, err := newOutlineRouter()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, ok := r.Exts()[".go"]; !ok {
		t.Error("the outline router has no .go extractor")
	}
	if _, ok := r.Exts()[".cue"]; !ok {
		t.Error("the outline router lost the CUE extractor it had through the pipeline")
	}
}

// TestNewOutlineRouterKeepsProductFormats pins the registrations the outline
// had through the pipeline: fathom's config and plan formats (WithFormats) and
// Timoni modules (WithTimoni) resolve through the router with their tool kinds.
func TestNewOutlineRouterKeepsProductFormats(t *testing.T) {
	r, err := newOutlineRouter()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	plan, err := os.ReadFile(filepath.Join("..", "..", "docs", "plans", "lang-library.md"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		rel     string
		content []byte
		want    langroute.ToolKind
		notExt  string // an extension whose plain extractor must not be the one resolved
	}{
		{"fathom config", ".fathom/config.toml", []byte("[service]\n"), fathomformat.ToolFathomConfig, ""},
		{"plan", "docs/plans/lang-library.md", plan, fathomformat.ToolFathomPlanRows, ""},
		{"timoni module", "mod/timoni.cue", []byte("package main\n"), langroute.ToolTimoni, ".cue"},
	}
	for _, c := range cases {
		e, kind, ok, _ := r.ResolveExtractor(c.rel, c.content, nil)
		if !ok || e == nil {
			t.Errorf("%s: no extractor claims %s", c.name, c.rel)
			continue
		}
		if kind != c.want {
			t.Errorf("%s: tool kind %q, want %q", c.name, kind, c.want)
		}
		// Without WithTimoni the .cue extractor still reads the file under the
		// Timoni tag, so the kind alone does not prove the registration.
		if c.notExt != "" && e == r.Exts()[c.notExt] {
			t.Errorf("%s: resolved to the plain %s extractor, not the dialect's", c.name, c.notExt)
		}
	}
}
