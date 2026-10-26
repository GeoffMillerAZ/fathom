package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// metricTiersSubject writes a one-file C++ subject, its scip-clang index and a
// clang-calls file naming tier (none when tier is ""), and returns lang-oracle's
// arguments over them.
func metricTiersSubject(t *testing.T, tier string, extra ...string) []string {
	t.Helper()
	subject := t.TempDir()
	src := "int g(int);\nint run(int x) {\n  return g(x);\n}\n"
	if err := os.WriteFile(filepath.Join(subject, "m.cc"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "m.cc", Occurrences: []*scippb.Occurrence{
			{Symbol: "cxx . . $ run(0000000000000001).", Range: []int32{1, 4, 7}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			{Symbol: "cxx . . $ g(0000000000000002).", Range: []int32{2, 9, 10}},
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scip := filepath.Join(dir, "cpp.scip")
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	tierField := ""
	if tier != "" {
		tierField = `"tier": "` + tier + `", `
	}
	sites := filepath.Join(dir, "clang-calls.json")
	if err := os.WriteFile(sites, []byte(`{"tool": "clang-ast", "version": "test clang", `+tierField+`"call_sites": [
 {"path": "m.cc", "line": 3, "col": 10, "name": "g", "class": "resolved"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var tierArgs []string
	if !slices.Contains(extra, "-scip-tier") && !slices.Contains(extra, "-no-scip-tier") {
		tierArgs = []string{"-scip-tier", "A"} // round 4 C4-R2: a scip-clang index names its tier
	}
	extra = slices.DeleteFunc(slices.Clone(extra), func(a string) bool { return a == "-no-scip-tier" })
	return append(append(tierArgs, "-dialect", "cpp", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "cpp", "dialect.json"), "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", scip, "-scip-defs", "-tool-oracle", sites,
		"-out", filepath.Join(t.TempDir(), "cpp.json"), "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")), extra...)
}

// TestCmdLangOracleMetricTiers (round 3 A3, F3): end to end, a tier-B
// call-site oracle beside a tier-A index lowers the calls metrics to B and
// leaves the index's own metrics at A; an untiered call-site file is refused.
func TestCmdLangOracleMetricTiers(t *testing.T) {
	args := metricTiersSubject(t, "B")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, rawOut := readValidReading(t, args[len(args)-3])
	for m, want := range map[string]string{"call_precision": "B", "call_recall": "B", "def_mapping": "A"} {
		if got := r.MetricTiers[m]; got != want {
			t.Errorf("metric_tiers[%s] = %q, want %q\n%s", m, got, want, rawOut)
		}
	}
	stderr.Reset()
	if code := cmdLangOracle(metricTiersSubject(t, ""), &stdout, &stderr); code == 0 {
		t.Errorf("an untiered call-site file: exit 0, want a refusal")
	}
	// round 4 C4-R2: an untiered preprocessor file (its imports grade
	// import_resolution) and an untiered ranges companion (range_agreement,
	// the linkage rule) are refused too, never read as tier A.
	for _, c := range []struct{ name, file, body string }{
		{"preproc", "preproc.json", `{"tool": "clang-preprocessor", "version": "test", "imports": [{"path": "m.cc", "line": 1, "to_path": "m.h"}]}`},
		{"ranges companion", "preproc.ranges.json", `{"tool": "clang-ast", "version": "test", "ranges": [{"path": "m.cc", "name": "run", "kind": "function", "line": 2, "end": 4}]}`},
	} {
		dir := t.TempDir()
		pre := filepath.Join(dir, "preproc.json")
		if err := os.WriteFile(pre, []byte(`{"tool": "clang-preprocessor", "version": "test", "tier": "A", "imports": []}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, c.file), []byte(c.body), 0o644); err != nil {
			t.Fatal(err)
		}
		stderr.Reset()
		if code := cmdLangOracle(metricTiersSubject(t, "B", "-tool-oracle", pre), &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), filepath.Join(dir, c.file)+": feeds") {
			t.Errorf("an untiered %s: exit %d, want a refusal naming it: %s", c.name, code, stderr.String())
		}
	}
}

// TestCmdLangOracleSCIPTier (round 3 A2, F2): an index's tier is an input.
// An index read from a translated compile database (-scip-tier B) caps every
// metric it feeds at B, whatever the tier of the files beside it, and names
// itself at B among the reading's oracles; a tier other than A-D is refused.
func TestCmdLangOracleSCIPTier(t *testing.T) {
	args := metricTiersSubject(t, "A", "-scip-tier", "B")
	out := args[len(args)-5]
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, rawOut := readValidReading(t, out)
	for m, want := range map[string]string{"call_precision": "B", "call_recall": "B", "def_mapping": "B"} {
		if got := r.MetricTiers[m]; got != want {
			t.Errorf("metric_tiers[%s] = %q, want %q\n%s", m, got, want, rawOut)
		}
	}
	named := false
	for _, o := range r.Oracles {
		named = named || (o.Tool == "scip-clang" && o.Tier == "B")
	}
	if !named {
		t.Errorf("the index is not named at tier B: %+v", r.Oracles)
	}
	if code := cmdLangOracle(metricTiersSubject(t, "A", "-scip-tier", "AA"), &stdout, &stderr); code != 2 {
		t.Errorf("-scip-tier AA: exit %d, want 2", code)
	}
	// round 4 C4-R2: a scip-clang index's tier is required, never A by default
	if code := cmdLangOracle(metricTiersSubject(t, "A", "-no-scip-tier"), &stdout, &stderr); code != 2 {
		t.Errorf("a scip-clang index without -scip-tier: exit %d, want 2", code)
	}
}
