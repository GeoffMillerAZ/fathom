package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

func tsFixtureSubject(t *testing.T) string {
	t.Helper()
	subject := t.TempDir()
	for _, f := range []string{"greeter.ts", "main.ts", "math.ts"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "internal", "scip", "testdata", "tsfixture", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subject, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return subject
}

func readReading(t *testing.T, path string) langbench.Reading {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r langbench.Reading
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("reading does not validate: %v", err)
	}
	return r
}

// TestLangOracleInstruments (owner decisions D2 and D5): a SCIP index as
// the L1 oracle (tier A, -scip-defs) in place of ctags; repeatable
// -tool-oracle files, one carrying definitions and another carrying
// language call pairs at their own tiers; and -only restricting a subject
// to the files a grammar supports, disclosed in the reading.
func TestLangOracleInstruments(t *testing.T) {
	subject := tsFixtureSubject(t)
	man, err := os.ReadFile(filepath.Join("..", "..", "internal", "extractor", "dialects", "typescript", "dialect.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(man, &m)
	ok := m["oracle_kinds"].(map[string]any)
	ok["scip"] = map[string]string{"type": "class", "function": "function", "function@scoped": "method", "interface": "interface", "enum": "enum"}
	ok["tool"] = map[string]string{"cls": "class", "fn": "function", "meth": "method", "iface": "interface", "enum": "enum"}
	manPath := filepath.Join(t.TempDir(), "dialect.json")
	b, _ := json.Marshal(m)
	_ = os.WriteFile(manPath, b, 0o644)
	scip := filepath.Join("..", "..", "internal", "scip", "testdata", "tsfixture", "index-no-display.scip")
	queue := filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")

	// D2: SCIP definitions are the L1 oracle, tier A; ctags is not scored.
	out := filepath.Join(t.TempDir(), "typescript.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "typescript", "-manifest", manPath, "-subject", subject, "-subject-name", "fx",
		"-ctags", filepath.Join(t.TempDir(), "absent"), "-scip", scip, "-scip-defs", "-out", out, "-queue", queue}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r := readReading(t, out)
	if r.MetricTiers[langbench.MSymRecall] != "A" || *r.Metrics[langbench.MSymRecall] != 1 || *r.Metrics[langbench.MRangeAgreement] != 1 {
		t.Fatalf("scip-defs L1: metrics %v tiers %v", r.Metrics, r.MetricTiers)
	}
	for _, o := range r.Oracles {
		if o.Tool == "universal-ctags" {
			t.Errorf("ctags listed although SCIP was the L1 oracle: %+v", o)
		}
	}

	// D5: tool-oracle files repeat; definitions (tier B) from one, call
	// pairs (tier C) from another.
	dir := t.TempDir()
	defs := filepath.Join(dir, "defs.json")
	_ = os.WriteFile(defs, []byte(`{"tool":"dumper","version":"1","tier":"B","definitions":[
	 {"path":"greeter.ts","name":"Greeter","line":3,"end":8,"kind":"cls"},
	 {"path":"greeter.ts","name":"greet","line":4,"end":7,"kind":"meth"},
	 {"path":"main.ts","name":"run","line":3,"end":6,"kind":"fn"},
	 {"path":"math.ts","name":"add","line":1,"end":3,"kind":"fn"}]}`), 0o644)
	calls := filepath.Join(dir, "calls.json")
	_ = os.WriteFile(calls, []byte(`{"tool":"tags.scm","version":"x","tier":"C","calls":[
	 {"path":"greeter.ts","from":"greet","to":"add"},
	 {"path":"main.ts","from":"run","to":"greet"},
	 {"path":"main.ts","from":"run","to":"missing"}]}`), 0o644)
	out2 := filepath.Join(t.TempDir(), "typescript.json")
	// A working ctags is present: an oracle file carrying definitions is
	// this subject's L1 oracle instead (D2), never a refused second one.
	ctags := fakeCtags(t, t.TempDir(), `{"_type": "tag", "name": "add", "path": "math.ts", "line": 1, "kind": "function", "end": 3}`+"\n")
	args = []string{"-dialect", "typescript", "-manifest", manPath, "-subject", subject, "-subject-name", "fx",
		"-ctags", ctags, "-tool-oracle", defs, "-tool-oracle", calls, "-out", out2, "-queue", queue}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r = readReading(t, out2)
	if r.MetricTiers[langbench.MSymRecall] != "B" || r.MetricTiers[langbench.MCallRecall] != "C" {
		t.Fatalf("tiers %v", r.MetricTiers)
	}
	for _, o := range r.Oracles {
		if o.Tool == "universal-ctags" {
			t.Errorf("ctags listed although an oracle file carried the definitions: %+v", o)
		}
	}
	if got := *r.Metrics[langbench.MCallRecall]; got < 0.66 || got > 0.67 {
		t.Errorf("call_recall = %v, want 2/3", got)
	}

	// SQL (D2): -only keeps a subject to the files the grammar supports;
	// the reading names the restriction.
	out3 := filepath.Join(t.TempDir(), "typescript.json")
	args = []string{"-dialect", "typescript", "-manifest", manPath, "-subject", subject, "-subject-name", "fx",
		"-ctags", filepath.Join(t.TempDir(), "absent"), "-only", "math.ts,greet*.ts", "-tool-oracle", defs, "-out", out3, "-queue", queue}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r = readReading(t, out3)
	if r.Subjects[0].Files != 2 || !slices.ContainsFunc(r.Notes, func(n string) bool { return strings.Contains(n, "fx: restricted to") }) {
		t.Fatalf("-only: files %d notes %v", r.Subjects[0].Files, r.Notes)
	}
	if got := *r.Metrics[langbench.MSymRecall]; got != 1 {
		t.Errorf("an oracle definition on a file outside -only was scored: sym_recall %v", got)
	}
}

// TestLangOracleAdjudicate (owner decision D3): -adjudicate admits a
// dialect-only definition by the deterministic check, lists it under the
// subject, and lifts it out of sym_precision's denominator.
func TestLangOracleAdjudicate(t *testing.T) {
	subject := tsFixtureSubject(t)
	man, _ := os.ReadFile(filepath.Join("..", "..", "internal", "extractor", "dialects", "typescript", "dialect.json"))
	var m map[string]any
	_ = json.Unmarshal(man, &m)
	m["oracle_kinds"].(map[string]any)["tool"] = map[string]string{"cls": "class", "fn": "function", "meth": "method", "iface": "interface", "enum": "enum"}
	manPath := filepath.Join(t.TempDir(), "dialect.json")
	b, _ := json.Marshal(m)
	_ = os.WriteFile(manPath, b, 0o644)
	dir := t.TempDir()
	defs := filepath.Join(dir, "defs.json") // an oracle that misses run
	_ = os.WriteFile(defs, []byte(`{"tool":"dumper","tier":"B","definitions":[
	 {"path":"greeter.ts","name":"Greeter","line":3,"end":8,"kind":"cls"},
	 {"path":"greeter.ts","name":"greet","line":4,"end":7,"kind":"meth"},
	 {"path":"math.ts","name":"add","line":1,"end":3,"kind":"fn"}]}`), 0o644)
	adj := filepath.Join(dir, "adj.json")
	_ = os.WriteFile(adj, []byte(`{"classes":{"exported-fn":{"why":"the dumper skips exported functions","pattern":"function\\s+{name}\\b","scope":"module"}},
	 "definitions":[{"path":"main.ts","line":3,"name":"run","class":"exported-fn"},{"path":"main.ts","line":3,"name":"nope","class":"exported-fn"}]}`), 0o644)
	out := filepath.Join(t.TempDir(), "typescript.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "typescript", "-manifest", manPath, "-subject", subject, "-subject-name", "fx",
		"-ctags", filepath.Join(t.TempDir(), "absent"), "-tool-oracle", defs, "-adjudicate", adj, "-out", out,
		"-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r := readReading(t, out)
	if got := *r.Metrics[langbench.MSymPrecision]; got != 1 {
		t.Errorf("sym_precision = %v, want 1 (3 matched of 4, one adjudicated)", got)
	}
	if !slices.Equal(r.Subjects[0].Adjudicated, []string{"main.ts:3 run (exported-fn)"}) {
		t.Errorf("adjudicated = %v", r.Subjects[0].Adjudicated)
	}
	if !slices.ContainsFunc(r.Notes, func(n string) bool { return strings.Contains(n, "1 adjudication claims refused") }) {
		t.Errorf("the refused claim is not disclosed: %v", r.Notes)
	}
}
