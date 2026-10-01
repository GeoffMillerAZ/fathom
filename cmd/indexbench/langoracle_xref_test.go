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
	// LB-F59: a doc engine parses no tree-sitter grammar, so it reports no
	// files_with_error at all (absent, not zero).
	if bytes.Contains(raw, []byte("files_with_error")) {
		t.Errorf("a doc-engine reading carries files_with_error:\n%s", raw)
	}
}

// TestCmdLangOracleGoModXrefs (LB-Q-go-mod): a native dialect with a
// stand-alone extractor is measured through it, so the go-mod dialect's
// requires, replaces and go.work uses reach the cross-file scoring — L3 for
// an engine that is neither tags nor doc.
func TestCmdLangOracleGoModXrefs(t *testing.T) {
	subject := t.TempDir()
	for p, c := range map[string]string{
		"go.mod":     "module example.com/root\n\ngo 1.22\n\nrequire example.com/sub v0.0.0\n\nreplace example.com/sub => ./sub\n",
		"sub/go.mod": "module example.com/sub\n\ngo 1.22\n",
		"go.work":    "go 1.22\n\nuse (\n\t.\n\t./sub\n)\n",
	} {
		full := filepath.Join(subject, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oracle := filepath.Join(t.TempDir(), "oracle.json")
	_ = os.WriteFile(oracle, []byte(`{"tool":"go","tier":"A","definitions":[
 {"path":"go.mod","name":"example.com/root","line":1,"kind":"module"},
 {"path":"go.mod","name":"example.com/sub","line":5,"kind":"package"},
 {"path":"go.mod","name":"replace/example.com/sub","line":7,"kind":"key"},
 {"path":"sub/go.mod","name":"example.com/sub","line":1,"kind":"module"},
 {"path":"go.work","name":"workspace","line":1,"kind":"namespace"},
 {"path":"go.work","name":"use/.","line":4,"kind":"key"},
 {"path":"go.work","name":"use/./sub","line":5,"kind":"key"}],
 "references":[{"path":"go.mod","from":"example.com/root","to":"example.com/sub"}],
 "cross_references":[
 {"path":"go.mod","from":"example.com/root","to_path":"sub/go.mod","to":"example.com/sub"},
 {"path":"go.mod","from":"replace/example.com/sub","to_path":"sub/go.mod","to":"example.com/sub"},
 {"path":"go.work","from":"use/.","to_path":"go.mod","to":"example.com/root"},
 {"path":"go.work","from":"use/./sub","to_path":"sub/go.mod","to":"example.com/sub"}]}`), 0o644)
	out := filepath.Join(t.TempDir(), "go-mod.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "go-mod", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "go-mod", "dialect.json"), "-subject", subject, "-tool-oracle", oracle, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, _ := os.ReadFile(out)
	var r langbench.Reading
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{langbench.MSymRecall, langbench.MSymPrecision, langbench.MXrefPrecision, langbench.MXrefRecall} {
		if v := r.Metrics[m]; v == nil || *v != 1 {
			t.Errorf("%s = %v, want 1\n%s", m, v, raw)
		}
	}
	// The grade, not level_met: level_met also needs the tenancy floor (1 MB/s of
	// CPU), which three tiny files under the race detector do not reach.
	for lvl, g := range map[string]string{"L1": r.Grade.L1, "L2": r.Grade.L2, "L3": r.Grade.L3} {
		if g != "excellent" {
			t.Errorf("grade %s = %q, want excellent\n%s", lvl, g, raw)
		}
	}
}

// writeTree writes files (subject-relative path -> content) under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// protoFixture is a two-file protobuf subject: a.proto imports b.proto, its
// message A holds a B field and its rpc Get takes a B (two cross-file
// references), and A's Local field is a same-file one.
var protoFixture = map[string]string{
	"a.proto": "syntax = \"proto3\";\npackage acme.v1;\n\nimport \"b.proto\";\n\nmessage A {\n  B b = 1;\n  Local l = 2;\n}\n\nmessage Local {\n  string s = 1;\n}\n\nservice S {\n  rpc Get(B) returns (A);\n}\n",
	"b.proto": "syntax = \"proto3\";\npackage acme.v1;\n\nmessage B {\n  string x = 1;\n}\n",
}

// TestCmdLangOracleTagsToolFormatXrefs (LB-F56, O167): a tags-engine tool
// format (protobuf) is no doc engine, so its L3 is scored from the file
// resolver's edges over its import strategy (fusion.ResolveFileEdges, the
// pass ingest stores file-resolved edges from) against the oracle's
// cross_references at file granularity, at the oracle's tier: grade.L3 is
// no longer not-attempted.
func TestCmdLangOracleTagsToolFormatXrefs(t *testing.T) {
	subject := t.TempDir()
	writeTree(t, subject, protoFixture)
	oracle := filepath.Join(t.TempDir(), "protobuf.json")
	writeTree(t, filepath.Dir(oracle), map[string]string{filepath.Base(oracle): `{"tool":"protoc","version":"34.1","tier":"A",
 "imports":[{"path":"a.proto","line":4,"to_path":"b.proto","module":"b.proto"}],
 "cross_references":[
 {"path":"a.proto","from":"A","to_path":"b.proto","to":"B"},
 {"path":"a.proto","from":"Get","to_path":"b.proto","to":"B"},
 {"path":"a.proto","from":"A","to_path":"google/protobuf/any.proto","to":"Any"}]}`})
	out := filepath.Join(t.TempDir(), "protobuf.json")
	args := []string{"-dialect", "protobuf", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "protobuf", "dialect.json"),
		"-subject", subject, "-subject-name", "fixture/proto", "-tool-oracle", oracle, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	for _, m := range []string{langbench.MXrefPrecision, langbench.MXrefRecall} {
		if v := r.Metrics[m]; v == nil || *v != 1 || r.MetricTiers[m] != "A" {
			t.Errorf("%s = %v tier %q, want 1 at A\n%s", m, v, r.MetricTiers[m], raw)
		}
	}
	if r.Counts[langbench.CntXrefTruth] != 2 || r.Counts[langbench.CntXrefFathom] != 2 {
		t.Errorf("xref counts %v, want 2 truth (A and Get to b.proto) and 2 found", r.Counts)
	}
	if r.Grade.L3 != langbench.GradeExcellent {
		t.Errorf("grade.L3 = %q, want excellent\n%s", r.Grade.L3, raw)
	}
	if !bytes.Contains(raw, []byte("1 oracle cross-references on files fathom does not read")) {
		t.Errorf("out-of-scope cross-reference not disclosed:\n%s", raw)
	}
	if !bytes.Contains(raw, []byte("file-resolved edges")) {
		t.Errorf("the file-resolved basis of xref is not disclosed:\n%s", raw)
	}
}
