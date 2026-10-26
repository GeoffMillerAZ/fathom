package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestClangSiteNotesByDialect (round 3 A4, F4): a note's tier is the oracle
// file's, and its examples are the dialect's: CUDA's reading carries no
// rust-analyzer operators, no fmt, nlohmann or gtest, and no tier its oracles
// were not run at; C++ at tier A keeps its text byte for byte.
func TestClangSiteNotesByDialect(t *testing.T) {
	counts := map[string]int{langbench.CntClangCallsDependent: 2, langbench.CntClangBlindDefs: 1, langbench.CntScipDefsMacroNamed: 1}
	cuda := strings.Join(clangSiteNotes("s", "cuda", "B", counts), "\n")
	for _, bad := range []string{"tier A", "fmt", "nlohmann", "gtest", "rust-analyzer"} {
		if strings.Contains(cuda, bad) {
			t.Errorf("cuda notes carry %q:\n%s", bad, cuda)
		}
	}
	// round 4 C4-R3: the linkage note (owner ruling M3) names its companion's tier
	dir := t.TempDir()
	pre := filepath.Join(dir, "preproc.json")
	for p, body := range map[string]string{
		pre:                     `{"tool":"clang-preprocessor","tier":"A","imports":[]}`,
		rangeCompanionFile(pre): `{"tool":"clang-ast","tier":"B","ranges":[],"internal_linkage":[{"path":"a.cu","name":"cb","line":1}]}`,
	} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	l := langbench.SCIPOptionsFor("cuda", nil).ClangLinkage
	if err := l.Read([]string{pre}, rangeCompanionFile); err != nil {
		t.Fatal(err)
	}
	cuda += "\n" + strings.Join(l.Finish("s", map[[2]string]bool{}, map[string]int{}), "\n")
	if !strings.Contains(cuda, "clang-ast-defs, tier B") {
		t.Errorf("the linkage note does not name its companion's tier B:\n%s", cuda)
	}
	for _, bad := range []string{"tier A"} {
		if strings.Contains(cuda, bad) {
			t.Errorf("cuda notes carry %q:\n%s", bad, cuda)
		}
	}
	if !strings.Contains(cuda, "calibration/oracles/clang-calls, tier B") {
		t.Errorf("cuda notes do not name the call-site oracle's tier B:\n%s", cuda)
	}
	cpp := clangSiteNotes("s", "cpp", "A", counts)
	for _, want := range []string{"s: 2 " + noteClangDependent, "s: 1 " + noteClangBlindDefs, "s: 1 " + noteClangMacroNamed} {
		found := false
		for _, n := range cpp {
			found = found || n == want
		}
		if !found {
			t.Errorf("cpp at tier A lost the note %q:\n%v", want, cpp)
		}
	}
	if why := unwrittenWhy("cuda"); strings.Contains(why, "rust-analyzer") || strings.Contains(why, "?, ==") {
		t.Errorf("unwrittenWhy(cuda) = %q, want C++'s classes, not rust-analyzer's", why)
	}
	if why, cppWhy := unwrittenWhy("cuda"), unwrittenWhy("cpp"); why == unwrittenWhy("rust") || cppWhy == unwrittenWhy("rust") {
		t.Errorf("unwrittenWhy: cuda %q, cpp %q fall to the default", why, cppWhy)
	}
	if got := scipClangNoImportsNote("B"); strings.Contains(got, "cross-file references (tier A)") || got == noteScipClangNoImports {
		t.Errorf("scipClangNoImportsNote(B) = %q, want the index's tier B", got)
	}
	if got := scipClangNoImportsNote("A"); got != noteScipClangNoImports {
		t.Errorf("scipClangNoImportsNote(A) = %q, want the committed text", got)
	}
}

// TestCmdLangOracleNote (round 3 A4): a driver's -note lands in the reading's
// notes under the subject's name, so a measurement record is never a hand edit.
func TestCmdLangOracleNote(t *testing.T) {
	args := metricTiersSubject(t, "B", "-subject-name", "s", "-note", "throughput read 3 times")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, args[len(args)-7])
	if !slices.Contains(r.Notes, "s: throughput read 3 times") {
		t.Errorf("-note not in the reading's notes:\n%s", raw)
	}
}
