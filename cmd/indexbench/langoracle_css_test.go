package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// cssW2Subject is a stylesheet set and the oracle file lightningcss_oracle.cjs
// writes for it: a.css imports b.css, which imports c.css; d.css imports
// nothing. a.css reads --x (b.css), --deep (c.css, through b.css), spin
// (b.css) and --nowhere (defined nowhere); d.css reads --x with no @import.
var cssW2Subject = map[string]string{
	"a.css": `@import "b.css";

.a, .alt {
  color: var(--x);
  margin: var(--deep);
  animation: spin 1s infinite;
}

.z {
  color: var(--nowhere);
}
`,
	"b.css": `@import "c.css";

:root {
  --x: red;
}

@keyframes spin {
  to { opacity: 0; }
}
`,
	"c.css": `:root {
  --deep: 1px;
}
`,
	"d.css": `.d {
  color: var(--x);
}
`,
}

const cssW2Oracle = `{
 "tool": "lightningcss", "version": "1.33.0", "tier": "A",
 "definitions": [
  {"path": "a.css", "name": ".a", "line": 3, "end": 0, "kind": "selector"},
  {"path": "a.css", "name": ".alt", "line": 3, "end": 0, "kind": "selector"},
  {"path": "a.css", "name": ".z", "line": 9, "end": 0, "kind": "selector"},
  {"path": "b.css", "name": ":root", "line": 3, "end": 0, "kind": "selector"},
  {"path": "b.css", "name": "--x", "line": 4, "end": 0, "kind": "custom_property"},
  {"path": "b.css", "name": "spin", "line": 7, "end": 0, "kind": "keyframes"},
  {"path": "c.css", "name": ":root", "line": 1, "end": 0, "kind": "selector"},
  {"path": "c.css", "name": "--deep", "line": 2, "end": 0, "kind": "custom_property"},
  {"path": "d.css", "name": ".d", "line": 1, "end": 0, "kind": "selector"}
 ],
 "calls": [
  {"path": "a.css", "from": ".a", "to": "--deep"},
  {"path": "a.css", "from": ".a", "to": "--x"},
  {"path": "a.css", "from": ".a", "to": "spin"},
  {"path": "a.css", "from": ".z", "to": "--nowhere"},
  {"path": "d.css", "from": ".d", "to": "--x"}
 ],
 "imports": [
  {"path": "a.css", "line": 1, "to_path": "b.css", "module": "b.css"},
  {"path": "b.css", "line": 1, "to_path": "c.css", "module": "c.css"}
 ]
}
`

// TestCmdLangOracleCSSNameScope (LB-F460, design 1.8 W2): a CSS name pair is
// graded only when its name is defined in the file's @import closure, on both
// sides, and what is left out is counted and disclosed; lightningcss's L1
// definitions replace ctags, and its imports grade import_resolution and the
// file edges.
func TestCmdLangOracleCSSNameScope(t *testing.T) {
	subject := t.TempDir()
	for name, body := range cssW2Subject {
		if err := os.WriteFile(filepath.Join(subject, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oracle := filepath.Join(t.TempDir(), "lightningcss-css.json")
	if err := os.WriteFile(oracle, []byte(cssW2Oracle), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "css.json")
	args := []string{"-dialect", "css", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "css", "dialect.json"),
		"-subject", subject, "-tool-oracle", oracle, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	if got := r.Counts[langbench.CntCallTruth]; got != 3 {
		t.Errorf("call_truth = %d, want 3 (.a's --x, --deep and spin; .z's --nowhere and d.css's --x are outside their closures)\n%s", got, raw)
	}
	if o, f := r.Counts[cntCSSOracleOutsideClosure], r.Counts[cntCSSFathomOutsideClosure]; o != 2 || f != 2 {
		t.Errorf("outside the closure: oracle %d, fathom %d, want 2 and 2\n%s", o, f, raw)
	}
	for _, m := range []string{langbench.MCallRecall, langbench.MCallPrecision, langbench.MSymRecall, langbench.MSymPrecision, langbench.MImportResolution, langbench.MFileEdgePrecision, langbench.MFileEdgeRecall} {
		if v := r.Metrics[m]; v == nil || *v != 1 || r.MetricTiers[m] != "A" {
			t.Errorf("%s = %v at tier %q, want 1 at A\n%s", m, v, r.MetricTiers[m], raw)
		}
	}
	if r.Metrics[langbench.MRangeAgreement] != nil {
		t.Errorf("range_agreement measured; lightningcss writes no end line")
	}
	var disclosed bool
	for _, n := range r.Notes {
		if strings.Contains(n, "2 oracle and 2 fathom name pairs name nothing defined in their file's @import closure (W2)") {
			disclosed = true
		}
	}
	if !disclosed {
		t.Errorf("the scope is not disclosed; notes %q", r.Notes)
	}
}
