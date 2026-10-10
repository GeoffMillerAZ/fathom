package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// htmlPairSubject is a page whose label names an id, whose fragment link
// names an id two elements carry, and whose <script> makes a JavaScript call.
const htmlPairSubject = `<!DOCTYPE html>
<html><body>
<label for="name">Name</label><input id="name">
<p id="dup">a</p><p id="dup">b</p>
<a href="#dup">x</a>
<script>function greet() { return helper(); } function helper() {}</script>
</body></html>
`

// htmlPairOracle is parse5_oracle.mjs's shape for htmlPairSubject.
const htmlPairOracle = `{"tool": "parse5", "version": "8.0.1", "tier": "A",
 "definitions": [{"path": "page.html", "name": "name", "line": 3, "end": 3, "kind": "id"},
  {"path": "page.html", "name": "dup", "line": 4, "end": 4, "kind": "id"},
  {"path": "page.html", "name": "dup", "line": 4, "end": 4, "kind": "id"}],
 "calls": [{"path": "page.html", "from": "<module>", "to": "name"}],
 "call_exclusions": [{"path": "page.html", "to": "dup", "reason": "duplicate-id"}],
 "disclose": ["parse5 8.0.1: the oracle's own counts"]}`

func runHTMLPair(t *testing.T) (langbench.Reading, []byte, unmatchedFile) {
	t.Helper()
	subject := t.TempDir()
	if err := os.WriteFile(filepath.Join(subject, "page.html"), []byte(htmlPairSubject), 0o644); err != nil {
		t.Fatal(err)
	}
	oracle := filepath.Join(t.TempDir(), "parse5.json")
	if err := os.WriteFile(oracle, []byte(htmlPairOracle), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "html.json")
	um := filepath.Join(t.TempDir(), "unmatched.json")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle([]string{"-dialect", "html", "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "html", "dialect.json"), "-tool-oracle", oracle, "-unmatched", um, "-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	return r, raw, readUnmatched(t, um)
}

// TestCmdLangOracleHTMLPairUnit (wave lf5, K19, design §1.8): HTML's L2 pairs
// are the html dialect's own kinds' — the <script>'s JavaScript call is
// JavaScript's L2, left out and counted — an oracle's call exclusions leave
// both sides, and the oracle's disclosures reach the reading's notes; the
// parse5 file is the subject's one L1 oracle, its kind "id" a label.
func TestCmdLangOracleHTMLPairUnit(t *testing.T) {
	r, raw, u := runHTMLPair(t)
	if got := r.Counts[cntHTMLInjectedCallPairs]; got != 1 {
		t.Errorf("%s = %d, want 1 (greet -> helper, the injected JavaScript's)\n%s", cntHTMLInjectedCallPairs, got, raw)
	}
	if u.Calls != nil {
		for _, p := range u.Calls.FathomOnly {
			if p.Caller == "greet" {
				t.Errorf("an injected language's pair scored as HTML's: %+v", p)
			}
		}
	}
	if !bytes.Contains(raw, []byte("the oracle's own counts")) {
		t.Errorf("the oracle's disclosure is not in the notes:\n%s", raw)
	}
	// The html dialect gives every reference to the document (calls.owner
	// "module"): the label's pair is found, the fragment's to the duplicate
	// id leaves both sides.
	if r.Counts[langbench.CntCallTruthFound] != 1 || r.Counts[langbench.CntCallFathom] != 1 || r.Counts[langbench.CntCallFathomInTrue] != 1 {
		t.Errorf("L2 pairs: truth found %d, fathom %d, fathom in truth %d; want 1, 1, 1\n%s", r.Counts[langbench.CntCallTruthFound], r.Counts[langbench.CntCallFathom], r.Counts[langbench.CntCallFathomInTrue], raw)
	}
	if got := r.Counts[cntCallPairsExcludedMine]; got != 1 {
		t.Errorf("%s = %d, want 1 (<module> -> dup)", cntCallPairsExcludedMine, got)
	}
	if r.Counts[langbench.CntOracleDefs] != 3 || r.Counts[langbench.CntMatched] < 2 {
		t.Errorf("parse5's definitions are not the L1 oracle's: oracle_defs %d, matched %d\n%s", r.Counts[langbench.CntOracleDefs], r.Counts[langbench.CntMatched], raw)
	}
}

// TestHTMLPairRulesStayHTMLs (wave lf5): htmlOwnCallers changes no other
// dialect's nodes, and applyCallExclusions drops a file's excluded callee
// from both sides, counted per side.
func TestHTMLPairRulesStayHTMLs(t *testing.T) {
	m := langbench.Manifest{ID: "css", NameMatch: langbench.NameMatchIdentity, Kinds: map[string]langbench.KindSpec{"selector": {SourceKind: "CSSSelector", Canonical: "selector"}}}
	nodes := []core.SymbolNode{{Name: "f", Kind: "JSFunc", FilePath: "a.css", Calls: []string{"g"}}}
	counts := map[string]int{}
	if got := htmlOwnCallers(m, nodes, nil, counts); len(got) != 1 || counts[cntHTMLInjectedCallPairs] != 0 {
		t.Fatalf("another dialect's nodes changed: %v %v", got, counts)
	}
	truth := map[langbench.CallPair]bool{{Path: "a.html", Caller: "<module>", Callee: "dup"}: true, {Path: "a.html", Caller: "<module>", Callee: "x"}: true}
	found := map[langbench.CallPair]bool{{Path: "a.html", Caller: "<module>", Callee: "dup"}: true, {Path: "b.html", Caller: "<module>", Callee: "dup"}: true}
	n := applyCallExclusions(m, []callExclusion{{Path: "a.html", To: "dup", Reason: "duplicate-id"}, {Path: "c.html", To: "dup"}}, map[string]bool{"a.html": true, "b.html": true}, truth, found, counts)
	if n != 1 || len(truth) != 1 || len(found) != 1 || counts[cntCallPairsExcludedTruth] != 1 || counts[cntCallPairsExcludedMine] != 1 {
		t.Fatalf("exclusions = %d, truth %v, found %v, counts %v", n, truth, found, counts)
	}
}
