package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// TestResolveFileEdges_PythonModuleSymbol (LB-F348, K18): with python's
// module symbol, an import of a module (`import a.b`, `from a import b` naming
// a submodule, the module a `from` import reads from) is an edge to that
// module's symbol, made from the definition around the import (the importing
// file's module symbol at top level, a function for a local import); the
// file's top-level calls resolve from its module symbol; and the module symbol
// is no top-level name of its file, so the rules that count a file's names (a
// bare use of a whole-module binding reads its sole definition) are unchanged.
func TestResolveFileEdges_PythonModuleSymbol(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"pkg/__init__.py": "",
		"pkg/util.py":     "def helper():\n    return 1\n",
		"pkg/one.py":      "def only():\n    return 1\n",
		"main.py": strings.Join([]string{
			"import pkg.util",
			"from pkg import util as u2",
			"from pkg.util import helper",
			"",
			"setup(pkg.util.helper())",
			"",
			"",
			"def run():",
			"    import pkg.one as po",
			"    return po",
			"",
		}, "\n"),
	}
	var rels []string
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	srcs, symbols := loadFileResolveFixture(t, dir, "python", rels)
	edges, report := fusion.ResolveFileEdges(context.Background(), "python", dir, srcs, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	mod := func(pkg string) string { return fkey(pkg, core.ModuleSymbolName) }
	got := map[[2]string]bool{}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			t.Errorf("edge %+v: kind %q, want file-resolved", e, e.Kind)
		}
		got[[2]string{e.FromSymbol, e.ToSymbol}] = true
	}
	want := [][2]string{
		{mod("main"), mod("pkg/util")},                 // import pkg.util; from pkg.util import helper; from pkg import util
		{mod("main"), mod("pkg/__init__")},             // from pkg import util: the module it reads from
		{mod("main"), fkey("pkg/util", "helper")},      // the top-level call pkg.util.helper(), the named import's name
		{fkey("main", "run"), mod("pkg/one")},          // a function-local import, made from the function
		{fkey("main", "run"), fkey("pkg/one", "only")}, // a bare use of a whole-module binding: its sole definition, as before
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s -> %s", w[0], w[1])
		}
	}
	for e := range got {
		if strings.HasSuffix(e[1], "."+core.ModuleSymbolName) && !strings.HasSuffix(e[0], "."+core.ModuleSymbolName) && e[0] != fkey("main", "run") {
			t.Errorf("unexpected edge to a module symbol %s -> %s", e[0], e[1])
		}
	}
	if t.Failed() {
		t.Logf("edges: %v", got)
	}
}
