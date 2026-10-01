package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// copyFixture copies a file-resolver fixture (internal/fusion/testdata/<dir>)
// into a fresh subject directory.
func copyFixture(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join("..", "..", "internal", "fusion", "testdata", dir)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(dst, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// rustFixtureIndex is a rust-analyzer-shaped SCIP index of the rust fixture:
// a file module is defined by its whole file and referenced at `mod x;` and
// in each `use` path; the call inside main is a reference, not an import.
func rustFixtureIndex(t *testing.T) string {
	t.Helper()
	const pre = "rust-analyzer cargo app 0.1.0 "
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "rust-analyzer", Version: "1.98.1"}},
		Documents: []*scippb.Document{
			{RelativePath: "src/main.rs", Occurrences: []*scippb.Occurrence{
				ref(pre+"net/", 0, 4, 7),    // mod net;
				ref(pre+"util/", 1, 4, 8),   // mod util;
				ref(pre+"util/", 3, 11, 15), // use crate::util::helper;
				ref(pre+"util/helper().", 3, 17, 23),
				ref(pre+"net/", 4, 4, 7), // use net::connect;
				ref(pre+"net/connect().", 4, 9, 16),
				def(pre+"main().", []int32{6, 0, 10, 1}, 6, 3, 7),
				ref(pre+"util/helper().", 7, 4, 10),
				ref(pre+"util/other().", 8, 10, 15),
				ref(pre+"net/connect().", 9, 4, 11),
			}},
			{RelativePath: "src/util.rs", Occurrences: []*scippb.Occurrence{
				def(pre+"util/", nil, 0, 0, 2, 17),
				def(pre+"util/helper().", []int32{0, 0, 0, 18}, 0, 7, 13),
				def(pre+"util/other().", []int32{2, 0, 2, 17}, 2, 7, 12),
			}},
			{RelativePath: "src/net/mod.rs", Occurrences: []*scippb.Occurrence{
				def(pre+"net/", nil, 0, 0, 0, 19),
				def(pre+"net/connect().", []int32{0, 0, 0, 19}, 0, 7, 14),
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "rust.scip")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readValidReading(t *testing.T, path string) (langbench.Reading, []byte) {
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
	return r, raw
}

func importNote(r langbench.Reading) string {
	for _, n := range r.Notes {
		if strings.HasPrefix(n, "import_resolution: ") {
			return n
		}
	}
	return ""
}

// TestCmdLangOracleImportResolutionSCIP (LB-F40): a language dialect's
// imports, resolved by its manifest's strategy, are scored against the SCIP
// index's references on the import statements' lines — tier A — and the
// reading carries import_resolution with its counts and disclosure.
func TestCmdLangOracleImportResolutionSCIP(t *testing.T) {
	subject := copyFixture(t, "rustfixture")
	out := filepath.Join(t.TempDir(), "rust.json")
	args := []string{"-dialect", "rust", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "rust", "dialect.json"),
		"-subject", subject, "-subject-name", "fixture/rust", "-scip", rustFixtureIndex(t), "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	if v := r.Metrics[langbench.MImportResolution]; v == nil || *v != 1 || r.MetricTiers[langbench.MImportResolution] != "A" {
		t.Fatalf("import_resolution %v tier %q\n%s", v, r.MetricTiers[langbench.MImportResolution], raw)
	}
	// mod net, mod util, use crate::util::helper, use net::connect: four
	// imports (each `use` read two ways, one binding), all in the subject.
	for k, v := range map[string]int{
		langbench.CntImports: 4, langbench.CntImportsInRepo: 4, langbench.CntImportsAgree: 4,
		langbench.CntImportEdgesTruth: 4, langbench.CntImportEdgesFathom: 4, langbench.CntImportEdgesAgree: 4,
	} {
		if r.Counts[k] != v {
			t.Errorf("%s = %d, want %d", k, r.Counts[k], v)
		}
	}
	if n := importNote(r); !strings.Contains(n, "4 of 4 in-repo imports") || !strings.Contains(n, "excellent >= 0.95, good >= 0.85") {
		t.Errorf("import note = %q", n)
	}
	if strings.Contains(string(raw), "arrive with the L3 strategies") {
		t.Errorf("the pre-LB-F40 not-measured note survived:\n%s", raw)
	}
}

// TestCmdLangOracleImportDump (LB-F40): a neutral oracle file's `imports` —
// a language-native import dump run on the fixture by the tool itself
// (calibration/oracles/ruby-require, calibration/oracles/clang-includes) —
// is scored at the file's tier, and an import the tool resolves outside the
// subject (`require 'json'`, `#include <vector>`) is not in-repo.
func TestCmdLangOracleImportDump(t *testing.T) {
	for _, tc := range []struct {
		dialect, fixture, dump string
		imports, inRepo        int
		tier                   string
	}{
		{"ruby", "rubyfixture", "ruby-require-rubyfixture.json", 3, 2, "B"},
		{"cpp", "cppfixture", "clang-includes-cppfixture.json", 2, 1, "B"},
	} {
		t.Run(tc.dialect, func(t *testing.T) {
			subject := copyFixture(t, tc.fixture)
			out := filepath.Join(t.TempDir(), tc.dialect+".json")
			args := []string{"-dialect", tc.dialect, "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", tc.dialect, "dialect.json"),
				"-subject", subject, "-tool-oracle", filepath.Join("testdata", tc.dump), "-ctags", filepath.Join(t.TempDir(), "none"),
				"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
			var stdout, stderr bytes.Buffer
			if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			r, raw := readValidReading(t, out)
			if v := r.Metrics[langbench.MImportResolution]; v == nil || *v != 1 || r.MetricTiers[langbench.MImportResolution] != tc.tier {
				t.Fatalf("import_resolution %v tier %q\n%s", v, r.MetricTiers[langbench.MImportResolution], raw)
			}
			if r.Counts[langbench.CntImports] != tc.imports || r.Counts[langbench.CntImportsInRepo] != tc.inRepo || r.Counts[langbench.CntImportOracleUnpaired] != 0 {
				t.Errorf("counts %v", r.Counts)
			}
		})
	}
}

// TestCmdLangOracleOneImportsOracle: a subject is measured against one
// imports oracle; a SCIP index and a dump's imports together are refused.
func TestCmdLangOracleOneImportsOracle(t *testing.T) {
	subject := copyFixture(t, "rustfixture")
	dump := filepath.Join(t.TempDir(), "dump.json")
	if err := os.WriteFile(dump, []byte(`{"tool":"dump","tier":"B","imports":[{"path":"src/main.rs","line":2,"to_path":"src/util.rs"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-dialect", "rust", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "rust", "dialect.json"),
		"-subject", subject, "-scip", rustFixtureIndex(t), "-tool-oracle", dump, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", filepath.Join(t.TempDir(), "rust.json"), "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "one per subject") {
		t.Errorf("exit %d, stderr %q: want a refusal", code, stderr.String())
	}
}

// TestCmdLangOracleImportDumpFileEdges (LB-F57): with no SCIP index, a
// neutral file's imports (ruby-require's dump of the fixture, tier B) are
// the truth file_edge_precision and file_edge_recall score fusion's
// file-resolved edges against: each in-scope (path, to_path) pair, at the
// file's tier, with the import-versus-reference difference disclosed.
func TestCmdLangOracleImportDumpFileEdges(t *testing.T) {
	subject := copyFixture(t, "rubyfixture")
	out := filepath.Join(t.TempDir(), "ruby.json")
	args := []string{"-dialect", "ruby", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "ruby", "dialect.json"),
		"-subject", subject, "-tool-oracle", filepath.Join("testdata", "ruby-require-rubyfixture.json"), "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	// lib/app.rb requires helper.rb and models/user.rb: App#run calls helper
	// and Formatting.money (helper.rb) and User.new (user.rb).
	for k, v := range map[string]int{langbench.CntFileEdgesTruth: 2, langbench.CntFileEdgesFathom: 2, langbench.CntFileEdgesAgree: 2} {
		if r.Counts[k] != v {
			t.Errorf("%s = %d, want %d\n%s", k, r.Counts[k], v, raw)
		}
	}
	for _, m := range []string{langbench.MFileEdgePrecision, langbench.MFileEdgeRecall} {
		if v := r.Metrics[m]; v == nil || *v != 1 || r.MetricTiers[m] != "B" {
			t.Errorf("%s = %v tier %q, want 1 at B", m, v, r.MetricTiers[m])
		}
	}
	if !bytes.Contains(raw, []byte(noteFileEdgesFromImports)) {
		t.Errorf("the import-statement truth is not disclosed:\n%s", raw)
	}
}

// TestCmdLangOracleImportDumpFileEdgesOnePerSubject: a SCIP index stays the
// file-edge truth when one is given; a dump's imports never add a second.
func TestCmdLangOracleImportDumpFileEdgesOnePerSubject(t *testing.T) {
	subject := copyFixture(t, "rustfixture")
	out := filepath.Join(t.TempDir(), "rust.json")
	args := []string{"-dialect", "rust", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "rust", "dialect.json"),
		"-subject", subject, "-scip", rustFixtureIndex(t), "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	if r.MetricTiers[langbench.MFileEdgeRecall] != "A" || bytes.Contains(raw, []byte(noteFileEdgesFromImports)) {
		t.Errorf("file edges tier %q, want A from the SCIP index alone\n%s", r.MetricTiers[langbench.MFileEdgeRecall], raw)
	}
}
