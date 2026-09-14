package coverage

import (
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestParseMutationReport_JSONWrapped(t *testing.T) {
	in := `{"mutants":[
		{"file":"internal/widget/widget.go","line":11,"killed":true},
		{"file":"internal/widget/widget.go","line":21,"killed":false}
	]}`
	rep, err := ParseMutationReport(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseMutationReport: %v", err)
	}
	if len(rep.Mutants) != 2 {
		t.Fatalf("len(Mutants) = %d, want 2", len(rep.Mutants))
	}
	if !rep.Mutants[0].Killed || rep.Mutants[1].Killed {
		t.Errorf("mutants = %+v", rep.Mutants)
	}
	if rep.Unattributed != 0 {
		t.Errorf("Unattributed = %d, want 0 (both records carry a line)", rep.Unattributed)
	}
}

func TestParseMutationReport_JSONBareArray(t *testing.T) {
	in := `[{"file":"a.go","line":1,"killed":true}]`
	rep, err := ParseMutationReport(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseMutationReport: %v", err)
	}
	if len(rep.Mutants) != 1 || rep.Mutants[0].File != "a.go" {
		t.Fatalf("mutants = %+v", rep.Mutants)
	}
}

func TestParseMutationReport_JSONMalformed(t *testing.T) {
	if _, err := ParseMutationReport(strings.NewReader(`{"mutants":`)); err == nil {
		t.Fatal("expected error for truncated JSON")
	}
}

func TestParseMutationReport_JSON_LineZeroIsUnattributed(t *testing.T) {
	in := `{"mutants":[{"file":"a.go","line":0,"killed":true}]}`
	rep, err := ParseMutationReport(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseMutationReport: %v", err)
	}
	if rep.Unattributed != 1 {
		t.Errorf("Unattributed = %d, want 1 (a caller-supplied line 0 can't be joined either)", rep.Unattributed)
	}
}

func TestParseMutationReport_EmptyInput(t *testing.T) {
	rep, err := ParseMutationReport(strings.NewReader("   \n  "))
	if err != nil {
		t.Fatalf("ParseMutationReport(empty): %v", err)
	}
	if len(rep.Mutants) != 0 {
		t.Errorf("expected an empty report for blank input, got %+v", rep)
	}
}

// ─── go-mutesting text — real transcripts (testdata/README.md documents how
// they were captured: github.com/zimmski/go-mutesting@latest, built with
// GOTOOLCHAIN=go1.19.13+auto since its own x/tools dependency panics under
// a modern go/types) ───────────────────────────────────────────────────────

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return string(data)
}

// TestParseMutationReport_GoMutestingDebug is the golden test against a
// REAL `go-mutesting --debug ./...` transcript: one killed mutant (a
// statement replacement inside the `if` block, PASS — exec exit 1, tests
// failed) and one survived mutant (`>` -> `>=`, FAIL — exec exit 0, tests
// passed), each with its own diff (both get one under --debug).
func TestParseMutationReport_GoMutestingDebug(t *testing.T) {
	rep, err := ParseMutationReport(strings.NewReader(readTestdata(t, "go-mutesting-debug.txt")))
	if err != nil {
		t.Fatalf("ParseMutationReport: %v", err)
	}
	if len(rep.Mutants) != 2 {
		t.Fatalf("len(Mutants) = %d, want 2: %+v", len(rep.Mutants), rep.Mutants)
	}
	if rep.Unattributed != 0 {
		t.Errorf("Unattributed = %d, want 0 (both mutants got a diff)", rep.Unattributed)
	}
	if rep.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", rep.Skipped)
	}
	// PASS (killed): `return a + b` -> `_, _ = a, b` at widget.go's line 5.
	killed := rep.Mutants[0]
	if killed.File != "widget.go" || killed.Line != 5 || !killed.Killed {
		t.Errorf("killed mutant = %+v, want {widget.go 5 true}", killed)
	}
	// FAIL (survived): `if a > 0` -> `if a >= 0` at widget.go's line 4.
	survived := rep.Mutants[1]
	if survived.File != "widget.go" || survived.Line != 4 || survived.Killed {
		t.Errorf("survived mutant = %+v, want {widget.go 4 false}", survived)
	}
}

// TestParseMutationReport_GoMutestingNoDebug: the SAME run without
// --debug. The PASS (killed) record has no diff at all — go-mutesting
// only prints one for FAIL by default — so its file must be recovered
// from the mutant path's own tail and its line is genuinely unattributed
// (Line 0, counted, never dropped). The FAIL record still has a diff.
func TestParseMutationReport_GoMutestingNoDebug(t *testing.T) {
	rep, err := ParseMutationReport(strings.NewReader(readTestdata(t, "go-mutesting-nodebug.txt")))
	if err != nil {
		t.Fatalf("ParseMutationReport: %v", err)
	}
	if len(rep.Mutants) != 2 {
		t.Fatalf("len(Mutants) = %d, want 2: %+v", len(rep.Mutants), rep.Mutants)
	}
	killed := rep.Mutants[0]
	if killed.File != "widget.go" || killed.Line != 0 || !killed.Killed {
		t.Errorf("non-debug killed mutant = %+v, want {widget.go 0 true} (file from path tail, no line)", killed)
	}
	survived := rep.Mutants[1]
	if survived.File != "widget.go" || survived.Line != 4 || survived.Killed {
		t.Errorf("survived mutant = %+v, want {widget.go 4 false} (FAIL always gets a diff)", survived)
	}
	if rep.Unattributed != 1 {
		t.Errorf("Unattributed = %d, want 1 (the non-debug PASS record)", rep.Unattributed)
	}
}

// TestParseMutationReport_GoMutestingNoDebugMultipkg proves per-package
// path resolution ("sub/sub.go" is recovered correctly, not just the
// leaf "sub.go" or the top-level "widget.go") and that the summary
// cross-check still holds across two packages' worth of records.
func TestParseMutationReport_GoMutestingNoDebugMultipkg(t *testing.T) {
	rep, err := ParseMutationReport(strings.NewReader(readTestdata(t, "go-mutesting-nodebug-multipkg.txt")))
	if err != nil {
		t.Fatalf("ParseMutationReport: %v", err)
	}
	if len(rep.Mutants) != 4 {
		t.Fatalf("len(Mutants) = %d, want 4: %+v", len(rep.Mutants), rep.Mutants)
	}
	var gotFiles []string
	for _, m := range rep.Mutants {
		if m.File != "" {
			gotFiles = append(gotFiles, m.File)
		}
	}
	wantFiles := []string{"widget.go", "widget.go", "sub/sub.go", "sub/sub.go"}
	if strings.Join(gotFiles, ",") != strings.Join(wantFiles, ",") {
		t.Errorf("files = %v, want %v", gotFiles, wantFiles)
	}
	if rep.Unattributed != 2 {
		t.Errorf("Unattributed = %d, want 2 (one non-debug PASS per package)", rep.Unattributed)
	}
}

// TestParseMutationReport_SummaryMismatch: a doctored transcript whose
// summary line disagrees with the parsed PASS/FAIL counts must error, not
// silently trust one side.
func TestParseMutationReport_SummaryMismatch(t *testing.T) {
	text := readTestdata(t, "go-mutesting-debug.txt")
	doctored := strings.Replace(text,
		"The mutation score is 0.500000 (1 passed, 1 failed, 0 duplicated, 0 skipped, total is 2)",
		"The mutation score is 1.000000 (2 passed, 0 failed, 0 duplicated, 0 skipped, total is 2)",
		1)
	if _, err := ParseMutationReport(strings.NewReader(doctored)); err == nil {
		t.Fatal("expected an error for a summary line that disagrees with the parsed counts")
	}
}

// TestParseMutationReport_NoSummaryLine: records with no trailing summary
// at all (a truncated capture) must error rather than report a partial,
// unverified count.
func TestParseMutationReport_NoSummaryLine(t *testing.T) {
	text := readTestdata(t, "go-mutesting-debug.txt")
	idx := strings.Index(text, "The mutation score is")
	truncated := text[:idx]
	if _, err := ParseMutationReport(strings.NewReader(truncated)); err == nil {
		t.Fatal("expected an error when the summary line is missing")
	}
}

// TestParseMutationReport_NoRecordsRecognised: non-empty input that is
// simply not go-mutesting output (and not JSON) must error, never a
// silent empty report.
func TestParseMutationReport_NoRecordsRecognised(t *testing.T) {
	if _, err := ParseMutationReport(strings.NewReader("hello world\nnothing to see here\n")); err == nil {
		t.Fatal("expected an error for input with no recognisable records")
	}
}

// TestParseMutationReport_SkipRecord: a SKIP (did-not-compile) record
// carries no killed/survived evidence and must be counted separately,
// never folded into Mutants as either outcome.
func TestParseMutationReport_SkipRecord(t *testing.T) {
	text := `Save mutations into "/tmp/go-mutesting-1"
Mutate "widget.go"
Save mutation into "/tmp/go-mutesting-1/widget.go.0" with checksum aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
Execute built-in exec command for mutation
--- widget.go	2026-09-13 23:10:54
+++ /tmp/go-mutesting-1/widget.go.0	2026-09-13 23:14:57
@@ -2,7 +2,8 @@

 func Add(a, b int) int {
 	if a > 0 {
-		return a + b
+		_, _ = a, b
+
 	}
 	return b
 }

PASS "/tmp/go-mutesting-1/widget.go.0" with checksum aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
SKIP "/tmp/go-mutesting-1/widget.go.1" with checksum bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
The mutation score is 1.000000 (1 passed, 0 failed, 0 duplicated, 1 skipped, total is 2)
`
	rep, err := ParseMutationReport(strings.NewReader(text))
	if err != nil {
		t.Fatalf("ParseMutationReport: %v", err)
	}
	if rep.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", rep.Skipped)
	}
	if len(rep.Mutants) != 1 {
		t.Fatalf("len(Mutants) = %d, want 1 (the SKIP record must not appear in Mutants): %+v", len(rep.Mutants), rep.Mutants)
	}
}

func TestJoinMutationSymbols(t *testing.T) {
	symbols := []core.SymbolNode{
		fn("Covered", "internal/widget/widget.go", 10, 14),
		fn("Survived", "internal/widget/widget.go", 20, 24),
		fn("NoMutants", "internal/widget/widget.go", 30, 34),
	}
	report := MutationReport{Mutants: []MutantResult{
		{File: "internal/widget/widget.go", Line: 11, Killed: true},
		{File: "internal/widget/widget.go", Line: 12, Killed: true},
		{File: "internal/widget/widget.go", Line: 21, Killed: false},
		{File: "internal/other/other.go", Line: 1, Killed: true},   // unmatched file
		{File: "internal/widget/widget.go", Line: 0, Killed: true}, // unattributed (no line)
	}}

	out := JoinMutationSymbols(report, symbols)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2 (NoMutants, the unmatched file, and the line-0 entry all omitted): %+v", len(out), out)
	}
	if out[0].Symbol != symbols[0].Key() || out[0].Mutants != 2 || out[0].Killed != 2 || out[0].Survived != 0 || out[0].Score != 1.0 {
		t.Errorf("Covered = %+v", out[0])
	}
	if out[1].Symbol != symbols[1].Key() || out[1].Mutants != 1 || out[1].Killed != 0 || out[1].Survived != 1 || out[1].Score != 0.0 {
		t.Errorf("Survived = %+v", out[1])
	}
}

func TestJoinMutationSymbols_SuffixPathMatch(t *testing.T) {
	symbols := []core.SymbolNode{fn("F", "internal/widget/widget.go", 10, 14)}
	report := MutationReport{Mutants: []MutantResult{
		{File: "/abs/repo/internal/widget/widget.go", Line: 11, Killed: true},
	}}
	out := JoinMutationSymbols(report, symbols)
	if len(out) != 1 || out[0].Mutants != 1 {
		t.Fatalf("expected an absolute path to suffix-match the repo-relative symbol file, got %+v", out)
	}
}

// TestJoinMutationSymbols_SymbolTiebreak: two symbols sharing the exact
// same (File, Line) — e.g. one spanning the other's declaration line via
// EndLine synthesis — sort by Symbol as the final tiebreak, never by map
// iteration order.
func TestJoinMutationSymbols_SymbolTiebreak(t *testing.T) {
	symbols := []core.SymbolNode{
		fn("Bravo", "a.go", 10, 14),
		fn("Alpha", "a.go", 10, 14),
	}
	report := MutationReport{Mutants: []MutantResult{
		{File: "a.go", Line: 10, Killed: true},
	}}
	out := JoinMutationSymbols(report, symbols)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2 (a mutant inside both identical spans)", len(out))
	}
	if out[0].Symbol != symbols[1].Key() || out[1].Symbol != symbols[0].Key() {
		t.Errorf("order = [%s %s], want Alpha before Bravo (Symbol tiebreak)", out[0].Symbol, out[1].Symbol)
	}
}
