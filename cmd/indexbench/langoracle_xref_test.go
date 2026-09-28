package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangOracleXrefs (LB-F30): a doc-engine dialect's tool oracle with
// cross_references is scored against fusion.ResolveDocXrefs' output and
// the reading gains xref_precision/xref_recall at the oracle's tier.
func TestCmdLangOracleXrefs(t *testing.T) {
	subject := t.TempDir()
	for p, c := range map[string]string{
		"package.json":            `{"name":"root","workspaces":["packages/*"]}`,
		"packages/a/package.json": `{"name":"@x/a","dependencies":{"@x/b":"1"}}`,
		"packages/b/package.json": `{"name":"@x/b"}`,
	} {
		full := filepath.Join(subject, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oracle := filepath.Join(t.TempDir(), "oracle.json")
	_ = os.WriteFile(oracle, []byte(`{"tool":"test","tier":"C","definitions":[
 {"path":"package.json","name":"root","line":1,"kind":"package"},
 {"path":"packages/a/package.json","name":"@x/a","line":1,"kind":"package"},
 {"path":"packages/b/package.json","name":"@x/b","line":1,"kind":"package"}],
 "cross_references":[
 {"path":"package.json","from":"root","to_path":"packages/a/package.json","to":"@x/a"},
 {"path":"package.json","from":"root","to_path":"packages/b/package.json","to":"@x/b"},
 {"path":"packages/a/package.json","from":"@x/a","to_path":"packages/b/package.json","to":"@x/b"},
 {"path":"packages/a/package.json","from":"@x/a","to_path":"elsewhere/package.json","to":"@y/z"}]}`), 0o644)
	out := filepath.Join(t.TempDir(), "package-json.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "package-json", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "package-json", "dialect.json"), "-subject", subject, "-tool-oracle", oracle, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, _ := os.ReadFile(out)
	var r langbench.Reading
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	p, rc := r.Metrics[langbench.MXrefPrecision], r.Metrics[langbench.MXrefRecall]
	if p == nil || rc == nil || *p != 1 || *rc != 1 || r.MetricTiers[langbench.MXrefRecall] != "C" {
		t.Fatalf("xref metrics %v tiers %v\n%s", r.Metrics, r.MetricTiers, raw)
	}
	if !bytes.Contains(raw, []byte("1 oracle cross-references on files fathom does not read")) {
		t.Errorf("out-of-scope cross-reference not disclosed:\n%s", raw)
	}
}
