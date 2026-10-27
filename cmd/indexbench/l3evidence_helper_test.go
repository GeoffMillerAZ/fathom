package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// l3OnAmpleEvidence is r's L3 grade with every L3 denominator padded to
// langbench.MinL3Evidence by one synthetic subject (owner ruling D19). A fixture
// this small is thin by design, so it asserts what the rule reads (L3 unmet,
// the evidence stated) and grades its ratios and tiers on the padded copy.
func l3OnAmpleEvidence(t *testing.T, r langbench.Reading) string {
	t.Helper()
	if r.Grade.L3 != langbench.GradeUnmet || !strings.HasPrefix(r.Grade.L3Reason, "evidence: ") {
		t.Errorf("thin fixture: L3 %q (%q), want unmet on thin evidence", r.Grade.L3, r.Grade.L3Reason)
	}
	pad := map[string]int{}
	for _, k := range []string{langbench.CntXrefTruth, langbench.CntXrefFathom, langbench.CntFileEdgesTruth, langbench.CntFileEdgesFathom, langbench.CntImportsInRepo} {
		pad[k] = langbench.MinL3Evidence
	}
	r.Subjects = append(append([]langbench.Subject(nil), r.Subjects...), langbench.Subject{Name: "pad", Counts: pad})
	_, g := langbench.LevelMet(r)
	return g.L3
}
