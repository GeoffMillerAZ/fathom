package coverage

import (
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

// goMutestingSample is a representative go-mutesting log transcript
// (github.com/zimmski/go-mutesting): two mutants, one killed, one that
// survived — plus a truncated third mutant (announced, never resolved)
// that ParseMutationReport must drop rather than guess at.
const goMutestingSample = `2015/09/26 10:30:32 Save original state of "widget.go"
2015/09/26 10:30:32 Executing mutation for internal/widget/widget.go:11.31,12.2
2015/09/26 10:30:32 Executing tests
2015/09/26 10:30:32 Mutation is killed

2015/09/26 10:30:33 Executing mutation for internal/widget/widget.go:21.31,22.2
2015/09/26 10:30:33 Executing tests
2015/09/26 10:30:33 Mutation is NOT killed

2015/09/26 10:30:34 Executing mutation for internal/widget/widget.go:31.31,32.2
2015/09/26 10:30:34 (process aborted, no result)
--- score ----
The mutation score is 0.500000 (1 passed, 1 failed, 0 duplicated, 0 skipped, total is 2)
`

func TestParseMutationReport_GoMutestingText(t *testing.T) {
	rep, err := ParseMutationReport(strings.NewReader(goMutestingSample))
	if err != nil {
		t.Fatalf("ParseMutationReport: %v", err)
	}
	if len(rep.Mutants) != 2 {
		t.Fatalf("len(Mutants) = %d, want 2 (the truncated third mutant must be dropped): %+v", len(rep.Mutants), rep.Mutants)
	}
	if rep.Mutants[0].File != "internal/widget/widget.go" || rep.Mutants[0].Line != 11 || !rep.Mutants[0].Killed {
		t.Errorf("mutant 0 = %+v", rep.Mutants[0])
	}
	if rep.Mutants[1].Line != 21 || rep.Mutants[1].Killed {
		t.Errorf("mutant 1 = %+v", rep.Mutants[1])
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
		{File: "internal/other/other.go", Line: 1, Killed: true}, // unmatched file
	}}

	out := JoinMutationSymbols(report, symbols)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2 (NoMutants and the unmatched file omitted): %+v", len(out), out)
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
