package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestCmdLangOracleCImportsAndFileEdges (LB-F80): the L3 harness reads C's
// import_resolution and file edges. cfixture's main.c includes util.h and
// calls its static inline helper; clang-includes' dump of it (tier B) is the
// import truth and, with no SCIP index, the file-edge truth, scored against
// fusion.ResolveFileEdges over C's include-path strategy.
func TestCmdLangOracleCImportsAndFileEdges(t *testing.T) {
	if _, ok := fileResolveLang["c"]; !ok {
		t.Fatal("fileResolveLang lacks c")
	}
	subject := copyFixture(t, "cfixture")
	out := filepath.Join(t.TempDir(), "c.json")
	args := []string{"-dialect", "c", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "c", "dialect.json"), "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-tool-oracle", filepath.Join("testdata", "clang-includes-cfixture.json"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	for k, v := range map[string]int{langbench.CntImportsInRepo: 1, langbench.CntImportsAgree: 1,
		langbench.CntFileEdgesTruth: 1, langbench.CntFileEdgesFathom: 1, langbench.CntFileEdgesAgree: 1} {
		if r.Counts[k] != v {
			t.Errorf("%s = %d, want %d\n%s", k, r.Counts[k], v, raw)
		}
	}
	for _, m := range []string{langbench.MImportResolution, langbench.MFileEdgePrecision, langbench.MFileEdgeRecall} {
		if v := r.Metrics[m]; v == nil || *v != 1 || r.MetricTiers[m] != "B" {
			t.Errorf("%s = %v tier %q, want 1 at B", m, v, r.MetricTiers[m])
		}
	}
}

// clangFixtureIndex is a scip-clang-shaped SCIP index of cfixture: helper is
// defined in util.h (no enclosing_range, as scip-clang writes none) and
// referenced from main.c's use(); the index holds no occurrence for the
// `#include` line itself, and no Import role anywhere.
func clangFixtureIndex(t *testing.T) string {
	t.Helper()
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{
			{RelativePath: "main.c", Occurrences: []*scippb.Occurrence{
				{Symbol: "cxx . . $ use(0000000000000001).", Range: []int32{2, 4, 7}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
				{Symbol: "cxx . . $ helper(0000000000000002).", Range: []int32{3, 9, 15}},
			}},
			{RelativePath: "util.h", Occurrences: []*scippb.Occurrence{
				{Symbol: "cxx . . $ helper(0000000000000002).", Range: []int32{3, 18, 24}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "c.scip")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCmdLangOracleScipClangImportsFromDump (LB-F80): scip-clang writes no
// reference on an `#include` line and no Import role, so its index is no
// imports oracle; with a clang-includes dump beside it, import_resolution is
// the dump's (tier B) and the file edges stay the index's (tier A), neither
// refused as a second imports oracle.
func TestCmdLangOracleScipClangImportsFromDump(t *testing.T) {
	subject := copyFixture(t, "cfixture")
	out := filepath.Join(t.TempDir(), "c.json")
	args := []string{"-dialect", "c", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "c", "dialect.json"), "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-scip", clangFixtureIndex(t), "-tool-oracle", filepath.Join("testdata", "clang-includes-cfixture.json"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	if v := r.Metrics[langbench.MImportResolution]; v == nil || *v != 1 || r.MetricTiers[langbench.MImportResolution] != "B" {
		t.Errorf("import_resolution = %v tier %q, want 1 at B (the dump's)\n%s", v, r.MetricTiers[langbench.MImportResolution], raw)
	}
	if r.Counts[langbench.CntFileEdgesTruth] != 1 || r.Counts[langbench.CntFileEdgesAgree] != 1 || r.MetricTiers[langbench.MFileEdgeRecall] != "A" {
		t.Errorf("file edges truth %d agree %d tier %q, want 1, 1 at A (the index's)\n%s", r.Counts[langbench.CntFileEdgesTruth],
			r.Counts[langbench.CntFileEdgesAgree], r.MetricTiers[langbench.MFileEdgeRecall], raw)
	}
	if !bytes.Contains(raw, []byte(noteScipClangNoImports)) {
		t.Errorf("scip-clang's missing import truth is not disclosed:\n%s", raw)
	}
}

// TestCmdLangOracleScipClangDefsScopedToIndexedFiles (LB-F80): scip-clang
// indexes only what the compilation database compiles (libuv on macOS builds
// no src/win/ file), so L1 against its definitions scores fathom's
// definitions in the files the index holds; the rest are disclosed, never
// the dialect's precision miss.
func TestCmdLangOracleScipClangDefsScopedToIndexedFiles(t *testing.T) {
	subject := copyFixture(t, "cfixture")
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "main.c", Occurrences: []*scippb.Occurrence{
			{Symbol: "cxx . . $ use(0000000000000001).", Range: []int32{2, 4, 7}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	scip := filepath.Join(t.TempDir(), "c.scip")
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "c.json")
	args := []string{"-dialect", "c", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "c", "dialect.json"), "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", scip, "-scip-defs",
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, rawOut := readValidReading(t, out)
	if r.Counts[langbench.CntFathomDefs] != 1 || r.Counts[langbench.CntMatched] != 1 {
		t.Errorf("fathom_defs %d matched %d, want 1 and 1 (util.h's helper is in no indexed file)\n%s", r.Counts[langbench.CntFathomDefs], r.Counts[langbench.CntMatched], rawOut)
	}
	if v := r.Metrics[langbench.MSymPrecision]; v == nil || *v != 1 {
		t.Errorf("sym_precision = %v, want 1", v)
	}
	if !bytes.Contains(rawOut, []byte(noteScipClangDefsScoped)) {
		t.Errorf("the scoping is not disclosed:\n%s", rawOut)
	}
}

// TestCmdLangOracleScipClangDefsDropInactiveLines (LB-F80): a definition on a
// line the build's own preprocessor discards (an inactive #if branch of the
// one configuration scip-clang indexes) is no precision miss: the
// clang-preprocessor oracle's active_lines say which lines each file keeps.
func TestCmdLangOracleScipClangDefsDropInactiveLines(t *testing.T) {
	subject := copyFixture(t, "cfixture")
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{
			{RelativePath: "main.c", Occurrences: []*scippb.Occurrence{
				{Symbol: "cxx . . $ use(0000000000000001).", Range: []int32{2, 4, 7}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			}},
			{RelativePath: "util.h", Occurrences: []*scippb.Occurrence{
				{Symbol: "cxx . . $ `util.h:2:9`!", Range: []int32{1, 8, 14}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scip := filepath.Join(dir, "c.scip")
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	// util.h's helper (line 4 of 9) sits in a branch the preprocessor dropped.
	active := filepath.Join(dir, "active.json")
	if err := os.WriteFile(active, []byte(`{"tool": "clang-preprocessor", "version": "1", "tier": "A",
 "active_lines": {"main.c": [[1, 5]], "util.h": [[1, 3], [8, 9]]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(extra ...string) (langbench.Reading, []byte) {
		out := filepath.Join(t.TempDir(), "c.json")
		args := append([]string{"-dialect", "c", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "c", "dialect.json"), "-subject", subject,
			"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", scip, "-scip-defs",
			"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, extra...)
		var stdout, stderr bytes.Buffer
		if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		return readValidReading(t, out)
	}
	r, rawOut := run("-tool-oracle", active)
	// main.c's use and the macro-free util.h: fathom defines use (main.c:3) and
	// helper (util.h:4) and the UTIL_H guard macro (util.h:2); helper is inactive.
	if r.Counts[langbench.CntFathomDefs] != 2 {
		t.Errorf("fathom_defs = %d, want 2 (use and UTIL_H; the inactive helper is left out)\n%s", r.Counts[langbench.CntFathomDefs], rawOut)
	}
	if !bytes.Contains(rawOut, []byte(noteScipClangInactive)) {
		t.Errorf("the inactive-line scoping is not disclosed:\n%s", rawOut)
	}
	r2, _ := run()
	if r2.Counts[langbench.CntFathomDefs] != 3 {
		t.Errorf("without the oracle fathom_defs = %d, want 3", r2.Counts[langbench.CntFathomDefs])
	}
}

// TestCmdLangOracleFileEdgeTruthScopedToMeasuredFiles (LB-F80): under -only the
// index still holds files the run does not measure; an index cross-file edge to
// or from one is not the subject's (it was counted in the truth, so a header-only
// measurement of a library whose tests the index also holds read a recall
// ceiling it could not reach). The edges left out are disclosed.
func TestCmdLangOracleFileEdgeTruthScopedToMeasuredFiles(t *testing.T) {
	subject := copyFixture(t, "cfixture")
	out := filepath.Join(t.TempDir(), "c.json")
	args := []string{"-dialect", "c", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "c", "dialect.json"), "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", clangFixtureIndex(t), "-only", "main.c",
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	if r.Counts[langbench.CntFileEdgesTruth] != 0 {
		t.Errorf("file_edges_truth = %d, want 0 (main.c -> util.h ends in a file this run does not measure)\n%s", r.Counts[langbench.CntFileEdgesTruth], raw)
	}
	if !bytes.Contains(raw, []byte(noteFileEdgeTruthScoped)) {
		t.Errorf("the left-out index edges are not disclosed:\n%s", raw)
	}
}

// TestCmdLangOracleFileEdgesDropInactiveCallers (LB-F194): a file edge whose
// caller sits on lines the build's preprocessor discards is no precision miss,
// as L1 and L2 already scope; the count is disclosed.
func TestCmdLangOracleFileEdgesDropInactiveCallers(t *testing.T) {
	subject := copyFixture(t, "cfixture")
	dir := t.TempDir()
	active := filepath.Join(dir, "active.json")
	// main.c's use (line 3) is on a discarded line.
	if err := os.WriteFile(active, []byte(`{"tool": "clang-preprocessor", "version": "1", "tier": "A",
 "active_lines": {"main.c": [[1, 2]], "util.h": [[1, 9]]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(extra ...string) (langbench.Reading, []byte) {
		out := filepath.Join(t.TempDir(), "c.json")
		args := append([]string{"-dialect", "c", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "c", "dialect.json"), "-subject", subject,
			"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", clangFixtureIndex(t),
			"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, extra...)
		var stdout, stderr bytes.Buffer
		if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		return readValidReading(t, out)
	}
	r, raw := run("-tool-oracle", active)
	if r.Counts[langbench.CntFileEdgesFathom] != 0 {
		t.Errorf("file_edges_fathom = %d, want 0 (the only caller is inactive)\n%s", r.Counts[langbench.CntFileEdgesFathom], raw)
	}
	if !bytes.Contains(raw, []byte(noteFileEdgeCallersInactive)) {
		t.Errorf("the scoping is not disclosed:\n%s", raw)
	}
	r2, _ := run()
	if r2.Counts[langbench.CntFileEdgesFathom] != 1 {
		t.Errorf("without the oracle file_edges_fathom = %d, want 1", r2.Counts[langbench.CntFileEdgesFathom])
	}
}
