package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// astroSubject is a page that imports and renders a component, declares a
// function in its frontmatter and styles itself with a custom property.
var astroSubject = map[string]string{
	"src/components/Card.astro": "---\ninterface Props {\n  title: string;\n}\n---\n<h2><slot /></h2>\n",
	"src/pages/index.astro": `---
import Card from '../components/Card.astro';
function label(n: number) {
  return fmt(n);
}
---
<Card>{label(1)}</Card>
<style>
  h2 { color: var(--accent); }
</style>
`,
}

// astroOracle is astro_oracle.mjs's shape for astroSubject.
const astroOracle = `{"tool": "@astrojs/compiler parse + typescript createSourceFile", "version": "@astrojs/compiler 4.0.0, typescript 5.9.3", "tier": "A",
 "definitions": [{"path": "src/components/Card.astro", "name": "Props", "line": 2, "end": 4, "kind": "interface"},
  {"path": "src/pages/index.astro", "name": "label", "line": 3, "end": 5, "kind": "function"}],
 "calls": [{"path": "src/pages/index.astro", "from": "<module>", "to": "Card"},
  {"path": "src/pages/index.astro", "from": "label", "to": "fmt"}],
 "imports": [{"path": "src/pages/index.astro", "line": 2, "to_path": "src/components/Card.astro", "module": "../components/Card.astro"}],
 "disclose": ["astro: the oracle's own counts"]}`

// TestCmdLangOracleAstroUnit (wave lf7, K19, design 1.8's Astro paragraph):
// scored under the astro scoring manifest, the frontmatter's TypeScript
// definitions are Astro's L1, the component tag and the function's call its
// L2, the style's CSS pairs leave fathom's side counted, and the relative
// import of a component is a resolved import and a file edge.
func TestCmdLangOracleAstroUnit(t *testing.T) {
	subject := t.TempDir()
	for rel, text := range astroSubject {
		p := filepath.Join(subject, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oracle := filepath.Join(t.TempDir(), "astro.json")
	if err := os.WriteFile(oracle, []byte(astroOracle), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "astro.json")
	um := filepath.Join(t.TempDir(), "unmatched.json")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle([]string{"-dialect", "astro", "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-manifest", filepath.Join("..", "..", "calibration", "oracles", "astro-compiler", "astro.scoring.json"), "-tool-oracle", oracle,
		"-unmatched", um, "-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	u := readUnmatched(t, um)
	if got := r.Counts[cntAstroInjectedCSSPairs]; got != 1 {
		t.Errorf("%s = %d, want 1 (h2 -> --accent, the style's CSS)\n%s", cntAstroInjectedCSSPairs, got, raw)
	}
	if u.Calls != nil && len(u.Calls.FathomOnly)+len(u.Calls.OracleOnly) > 0 {
		t.Errorf("L2 pairs differ: fathom only %+v, oracle only %+v", u.Calls.FathomOnly, u.Calls.OracleOnly)
	}
	for k, want := range map[string]int{
		langbench.CntCallTruth: 2, langbench.CntCallTruthFound: 2, langbench.CntCallFathom: 2,
		langbench.CntImportsAgree: 1, langbench.CntFileEdgesTruth: 1, langbench.CntFileEdgesAgree: 1,
	} {
		if r.Counts[k] != want {
			t.Errorf("%s = %d, want %d\n%s", k, r.Counts[k], want, raw)
		}
	}
	if !bytes.Contains(raw, []byte("the oracle's own counts")) {
		t.Errorf("the oracle's disclosure is not in the notes:\n%s", raw)
	}
}

// TestAstroOwnCallersStaysAstros: the style rule moves no other dialect's
// pairs (an html manifest keeps htmlOwnCallers' answer).
func TestAstroOwnCallersStaysAstros(t *testing.T) {
	nodes := []core.SymbolNode{
		{Name: core.ModuleSymbolName, Kind: "AstroComponent", FilePath: "a.astro", Calls: []string{"Card"}},
		{Name: "h2", Kind: "CSSSelector", FilePath: "a.astro", Calls: []string{"--accent"}},
	}
	files := map[string]bool{"a.astro": true}
	counts := map[string]int{}
	got := ownCallers(langbench.Manifest{ID: "astro"}, nodes, files, counts)
	if len(got) != 1 || got[0].Kind != "AstroComponent" || counts[cntAstroInjectedCSSPairs] != 1 {
		t.Errorf("astro: callers %+v, counts %v", got, counts)
	}
	counts = map[string]int{}
	if got := ownCallers(langbench.Manifest{ID: "typescript"}, nodes, files, counts); len(got) != 2 || len(counts) != 0 {
		t.Errorf("another dialect's nodes moved: %+v, counts %v", got, counts)
	}
}
