package verify

import (
	"strings"
	"testing"
)

func sampleMutationReport() Report {
	return Report{
		Service: "svc",
		Tasks: []TaskVerdict{
			{
				TaskID: "T1", Title: "task one", Status: Pass, Label: "verified", Claimed: true,
				Criteria: []Criterion{
					{Name: "symbol_present", Status: Pass, Detail: "pkg.Foo at internal/widget/widget.go:10"},
					{Name: "tier", Status: Pass, Detail: "verified"},
					{Name: "test_reaches", Status: Pass, Detail: "1 test(s) statically reach it, e.g. pkg.TestFoo"},
					{Name: "no_contradiction", Status: Pass, Detail: "no conflicting evidence"},
					{Name: "runtime_pass", Status: Unknown, Detail: "not evaluated"},
				},
			},
		},
		Summary: Summary{Total: 1, Pass: 1},
	}
}

func TestOverlayMutation_NoScores_Untouched(t *testing.T) {
	rep := sampleMutationReport()
	before := rep.Tasks[0].Criteria[2].Detail
	OverlayMutation(&rep, nil, false)
	if rep.Tasks[0].Criteria[2].Detail != before {
		t.Fatalf("detail changed with no scores: %q", rep.Tasks[0].Criteria[2].Detail)
	}
}

func TestOverlayMutation_ScoreZero_Advisory(t *testing.T) {
	rep := sampleMutationReport()
	scores := []MutationScore{{File: "internal/widget/widget.go", Line: 10, Mutants: 4, Killed: 0, Survived: 4, Score: 0}}
	OverlayMutation(&rep, scores, false)

	tv := rep.Tasks[0]
	if tv.Status != Pass {
		t.Fatalf("advisory mutation must not change task status, got %v", tv.Status)
	}
	got := tv.Criteria[2].Detail
	if !strings.Contains(got, "covered-not-asserted") {
		t.Errorf("detail = %q, want covered-not-asserted note", got)
	}
	if rep.Summary.Pass != 1 || rep.Summary.Fail != 0 {
		t.Errorf("summary = %+v, want unchanged pass=1", rep.Summary)
	}
}

func TestOverlayMutation_ScoreZero_Strict(t *testing.T) {
	rep := sampleMutationReport()
	scores := []MutationScore{{File: "internal/widget/widget.go", Line: 10, Mutants: 4, Killed: 0, Survived: 4, Score: 0}}
	OverlayMutation(&rep, scores, true)

	tv := rep.Tasks[0]
	if tv.Status != Fail {
		t.Fatalf("strict mutation must fail the task, got %v", tv.Status)
	}
	if tv.Label != "covered-not-asserted" {
		t.Errorf("label = %q, want covered-not-asserted", tv.Label)
	}
	if tv.Criteria[2].Status != Fail {
		t.Errorf("test_reaches criterion status = %v, want Fail", tv.Criteria[2].Status)
	}
	if rep.Summary.Fail != 1 || rep.Summary.Pass != 0 {
		t.Errorf("summary = %+v, want fail=1", rep.Summary)
	}
}

func TestOverlayMutation_NonzeroScore_AdvisoryNoteOnly(t *testing.T) {
	rep := sampleMutationReport()
	scores := []MutationScore{{File: "internal/widget/widget.go", Line: 10, Mutants: 4, Killed: 3, Survived: 1, Score: 0.75}}
	OverlayMutation(&rep, scores, true) // strict has no effect above score 0

	tv := rep.Tasks[0]
	if tv.Status != Pass {
		t.Fatalf("a nonzero mutation score must never fail the task, got %v", tv.Status)
	}
	if !strings.Contains(tv.Criteria[2].Detail, "score 0.75") {
		t.Errorf("detail = %q, want the score noted", tv.Criteria[2].Detail)
	}
}

func TestOverlayMutation_UnmatchedLocation_Untouched(t *testing.T) {
	rep := sampleMutationReport()
	scores := []MutationScore{{File: "internal/other/other.go", Line: 99, Mutants: 1, Killed: 0, Score: 0}}
	before := rep.Tasks[0].Criteria[2].Detail
	OverlayMutation(&rep, scores, true)
	if rep.Tasks[0].Criteria[2].Detail != before || rep.Tasks[0].Status != Pass {
		t.Fatalf("an unmatched location must never be touched: %+v", rep.Tasks[0])
	}
}
