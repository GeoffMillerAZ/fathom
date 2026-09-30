package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// PE-17w: fathom-on versus off is shown only when G2's admissibility test
// passes for the compared units — `fathom rate`'s criterion, the
// observatory's pilot note and its readers (the brief, the impact view)
// otherwise name why the comparison is withheld.
func TestOnOffVerdictWithheldUnlessAdmissible(t *testing.T) {
	n := func(v int) *int { return &v }
	arms := func(on, off int) obsImpact {
		return obsImpact{TokensOnSessions: on, TokensOffSessions: off, TokensOnMedian: n(900), TokensOffMedian: n(1500),
			DerivedOnUnits: on, DerivedOffUnits: off, DerivedOnMed: n(2), DerivedOffMed: n(1)}
	}
	short, full := arms(25, 3), arms(25, 22)
	finishArmComparison(&short, 3, 25, 3)
	finishArmComparison(&full, 22, 25, 22)

	if v := short.CriterionNote; !strings.HasPrefix(v, "NOT YET EVALUABLE") || !strings.Contains(v, "G2 not yet admissible") || !strings.Contains(v, "3 OFF-arm unit") || strings.Contains(v, "cheaper") {
		t.Errorf("3 OFF-arm units: the verdict is withheld by G2's test, naming the short arm: %q", v)
	}
	if v := full.CriterionNote; !strings.HasPrefix(v, "MET: fathom-on is cheaper") {
		t.Errorf("25 and 22 units clear G2: the verdict is read: %q", v)
	}
	if !strings.Contains(short.PilotDetail, "withheld") || !strings.Contains(short.PilotDetail, "G2 not yet admissible") {
		t.Errorf("the observatory's note names why the comparison is withheld: %q", short.PilotDetail)
	}
	if !strings.Contains(full.PilotDetail, "G2 admits") {
		t.Errorf("an admissible comparison says so: %q", full.PilotDetail)
	}

	brief := renderBriefMD(observatoryModel{Impact: short})
	if strings.Contains(brief, "median 900") || strings.Contains(brief, "compare the two medians") || !strings.Contains(brief, "G2 not yet admissible") {
		t.Errorf("the brief prints no on-versus-off medians below G2 and says why:\n%s", brief)
	}
	if b := renderBriefMD(observatoryModel{Impact: full}); !strings.Contains(b, "median 900") || !strings.Contains(b, "compare the two medians") {
		t.Errorf("the brief compares the arms once G2 admits them:\n%s", b)
	}

	view := console.ImpactView(impactInputsFrom(impactSources{imp: short}))
	if len(view.Questions) == 0 || !strings.Contains(view.Questions[0].Answer, "G2 not yet admissible") || !strings.Contains(view.ArmNote, "withheld") {
		t.Errorf("the impact view names the withheld comparison: %+v / %q", view.Questions, view.ArmNote)
	}
}
