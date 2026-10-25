package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangOracleScipClangRangesFromCompanion (LB-Q-c-rest step 5):
// scip-clang writes no end line, so L1 against its index cannot score
// range_agreement; a companion file of the compiler's own definition ranges
// (calibration/oracles/clang-ast-defs) beside the tool oracle does. The index
// stays L1's oracle: sym_recall is the index's, and only the range counts come
// from the companion.
func TestCmdLangOracleScipClangRangesFromCompanion(t *testing.T) {
	subject := copyFixture(t, "cfixture")
	dir := t.TempDir()
	dump, err := os.ReadFile(filepath.Join("testdata", "clang-includes-cfixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(dir, "dump.json")
	if err := os.WriteFile(tool, dump, 0o644); err != nil {
		t.Fatal(err)
	}
	// use (main.c:3-5) agrees; helper (util.h:4-6) is given a last line three
	// lines off, and a definition in a file this run does not measure is dropped.
	ranges := `{"tool":"clang-ast","tier":"A","version":"clang 21","units":2,"errored":0,"ranges":[
	 {"path":"main.c","name":"use","kind":"function","line":3,"end":5},
	 {"path":"util.h","name":"helper","kind":"function","line":4,"end":9},
	 {"path":"other.c","name":"gone","kind":"function","line":1,"end":2}]}`
	if err := os.WriteFile(filepath.Join(dir, "dump.ranges.json"), []byte(ranges), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(tool string) (langbench.Reading, []byte) {
		out := filepath.Join(t.TempDir(), "c.json")
		args := []string{"-dialect", "c", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "c", "dialect.json"), "-subject", subject,
			"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", clangFixtureIndex(t), "-scip-tier", "A", "-scip-defs", "-tool-oracle", tool,
			"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
		var stdout, stderr bytes.Buffer
		if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		return readValidReading(t, out)
	}
	r, raw := run(tool)
	if r.Counts[langbench.CntRangeEligible] != 2 || r.Counts[langbench.CntRangeOK] != 1 {
		t.Errorf("range_eligible %d range_ok %d, want 2 and 1\n%s", r.Counts[langbench.CntRangeEligible], r.Counts[langbench.CntRangeOK], raw)
	}
	if v := r.Metrics[langbench.MRangeAgreement]; v == nil || *v != 0.5 {
		t.Errorf("range_agreement = %v, want 0.5", v)
	}
	if len(r.NotMeasured) != 0 {
		t.Errorf("not_measured = %v, want none once a companion measures the range", r.NotMeasured)
	}
	if !bytes.Contains(raw, []byte("range_agreement measured against clang-ast's AST")) {
		t.Errorf("the companion is not disclosed:\n%s", raw)
	}
	if !bytes.Contains(raw, []byte("include-path resolution read 0 compile database(s); 0 quoted include(s)")) {
		t.Errorf("the include-path counts are not disclosed:\n%s", raw)
	}
	// Without the companion the index's own L1 leaves range_agreement unmeasured.
	if err := os.Remove(filepath.Join(dir, "dump.ranges.json")); err != nil {
		t.Fatal(err)
	}
	r2, raw2 := run(tool)
	if r2.Counts[langbench.CntRangeEligible] != 0 || len(r2.NotMeasured) != 1 {
		t.Errorf("no companion: range_eligible %d not_measured %v, want 0 and range_agreement\n%s", r2.Counts[langbench.CntRangeEligible], r2.NotMeasured, raw2)
	}
}
