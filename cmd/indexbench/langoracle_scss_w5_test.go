package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// scssW5Subject has every W5 owner: a module-level variable whose value calls
// a function and reads a variable, a selector that interpolates a variable, a
// mixin reading a variable and a namespaced include, a placeholder extend, and
// two reads of parameters (no definition).
const scssW5Subject = `@use "lib" as l;
$gap: 4px;
$wide: grow($gap);
@function grow($x) { @return $x * 2; }
%panel { margin: 0; }
@mixin card($pad) { padding: $pad + $gap; @include l.shadow; }
.box-#{$gap} { @extend %panel; @include card(1px); }
`

// scssW5Oracle is sass_oracle.cjs's shape for the subject: W5's resolved
// pairs, and the two names no read of the file resolves (the parameters).
const scssW5Oracle = `{"tool": "dart-sass", "version": "sass 1.105.1, sass-parser 0.4.57", "tier": "A",
 "definitions": [
  {"path": "main.scss", "name": "gap", "line": 2, "end": 0, "kind": "variable"},
  {"path": "main.scss", "name": "wide", "line": 3, "end": 0, "kind": "variable"},
  {"path": "main.scss", "name": "grow", "line": 4, "end": 0, "kind": "function"},
  {"path": "main.scss", "name": "panel", "line": 5, "end": 0, "kind": "placeholder"},
  {"path": "main.scss", "name": "card", "line": 6, "end": 0, "kind": "mixin"},
  {"path": "main.scss", "name": "box-#{$gap}", "line": 7, "end": 0, "kind": "class"},
  {"path": "_lib.scss", "name": "shadow", "line": 1, "end": 0, "kind": "mixin"}],
 "calls": [
  {"path": "main.scss", "from": "<module>", "to": "gap"},
  {"path": "main.scss", "from": "<module>", "to": "grow"},
  {"path": "main.scss", "from": "<module>", "to": "panel"},
  {"path": "main.scss", "from": "<module>", "to": "card"},
  {"path": "main.scss", "from": "card", "to": "gap"},
  {"path": "main.scss", "from": "card", "to": "shadow"}],
 "call_exclusions": [
  {"path": "main.scss", "to": "pad", "reason": "w5-unresolved"},
  {"path": "main.scss", "to": "x", "reason": "w5-unresolved"}],
 "imports": [{"path": "main.scss", "line": 1, "to_path": "_lib.scss", "module": "lib"}],
 "disclose": ["W5 (design 1.8): the oracle's own counts"]}`

// TestCmdLangOracleSCSSW5Pairs (LB-Q-scss-rest, design 1.8 W5): SCSS's L2 is
// the name references of its module graph. The scss dialect gives a reference
// to the enclosing @mixin or @function, else the module symbol, also inside a
// variable's value and a selector's interpolation (calls.owner "scope"), so
// the six resolved pairs are found and nothing else is scored: the two
// parameter reads leave both sides, counted.
func TestCmdLangOracleSCSSW5Pairs(t *testing.T) {
	subject := t.TempDir()
	for name, body := range map[string]string{"main.scss": scssW5Subject, "_lib.scss": "@mixin shadow { a: b; }\n"} {
		if err := os.WriteFile(filepath.Join(subject, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oracle := filepath.Join(t.TempDir(), "dart-sass.json")
	if err := os.WriteFile(oracle, []byte(scssW5Oracle), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "scss.json")
	um := filepath.Join(t.TempDir(), "unmatched.json")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle([]string{"-dialect", "scss", "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "scss", "dialect.json"), "-tool-oracle", oracle, "-unmatched", um, "-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	u := readUnmatched(t, um)
	c := r.Counts
	if c[langbench.CntCallTruth] != 6 || c[langbench.CntCallTruthFound] != 6 {
		t.Errorf("call truth %d found %d, want 6 and 6\n%s", c[langbench.CntCallTruth], c[langbench.CntCallTruthFound], raw)
	}
	if c[langbench.CntCallFathom] != 6 || c[langbench.CntCallFathomInTrue] != 6 {
		t.Errorf("fathom pairs %d in truth %d, want 6 and 6", c[langbench.CntCallFathom], c[langbench.CntCallFathomInTrue])
	}
	if c[cntCallPairsExcludedTruth] != 0 || c[cntCallPairsExcludedMine] != 2 {
		t.Errorf("excluded truth %d fathom %d, want 0 and 2 (pad, x)", c[cntCallPairsExcludedTruth], c[cntCallPairsExcludedMine])
	}
	if u.Calls != nil && (len(u.Calls.FathomOnly) > 0 || len(u.Calls.OracleOnly) > 0) {
		t.Errorf("unmatched pairs: fathom only %v, oracle only %v", u.Calls.FathomOnly, u.Calls.OracleOnly)
	}
	if !bytes.Contains(raw, []byte("W5 (design 1.8): the oracle's own counts")) {
		t.Errorf("the oracle's disclosure is not in the notes:\n%s", raw)
	}
}
