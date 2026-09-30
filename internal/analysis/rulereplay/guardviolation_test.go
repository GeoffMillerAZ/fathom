package rulereplay

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// TestIntroducedViolations: a record maps to the guardspec.Violation it was
// observed as, and only New records are introduced ones, oldest commit first,
// each naming its commit.
func TestIntroducedViolations(t *testing.T) {
	rec := ViolationRecord{RuleID: "a-not-b", Kind: "import_direction", Severity: "warn", From: "a", To: "b", Message: "a must not import b"}
	if g := rec.GuardViolation(); g.RuleID != "a-not-b" || g.Kind != guardspec.LayerRuleKind("import_direction") || g.Severity != guardspec.SeverityWarn || g.From != "a" || g.To != "b" || g.Message != "a must not import b" {
		t.Fatalf("GuardViolation() = %+v", g)
	}

	fresh, inherited := rec, rec
	fresh.New = true
	rep := Report{Commits: []CommitResult{
		{SHA: "1111111111111111", Subject: "clean"},
		{SHA: "2222222222222222", Subject: "introduce", Violations: []ViolationRecord{fresh}},
		{SHA: "3333333333333333", Subject: "carry", Violations: []ViolationRecord{inherited}},
	}}
	got := rep.IntroducedViolations()
	if len(got) != 1 {
		t.Fatalf("IntroducedViolations() = %+v, want one (the inherited repeat is not a second finding)", got)
	}
	if !strings.Contains(got[0].Message, "a must not import b") || !strings.Contains(got[0].Message, "222222222222") || !strings.Contains(got[0].Message, "introduce") {
		t.Errorf("message %q must carry the rule's text and the introducing commit", got[0].Message)
	}
	if len((Report{}).IntroducedViolations()) != 0 {
		t.Error("an empty report introduced violations")
	}
}
