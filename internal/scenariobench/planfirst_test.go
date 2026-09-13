package scenariobench

import (
	"strings"
	"testing"
)

// TestPlanFirstInstructionFor_WiredNamesGrammarAndVerify (SB-53): only the
// wired arm's plan-first instruction names the plan grammar and the verify
// verb; the control keeps the plain checklist.
func TestPlanFirstInstructionFor_WiredNamesGrammarAndVerify(t *testing.T) {
	wired := planFirstInstructionFor(Arm{Wired: true})
	control := planFirstInstructionFor(Arm{Wired: false})
	for _, want := range []string{"fathom verify", "{id:", "symbol:", "package:", "docs/plans/"} {
		if !strings.Contains(wired, want) {
			t.Errorf("wired instruction lacks %q", want)
		}
	}
	if strings.Contains(control, "fathom verify") || strings.Contains(control, "symbol:") {
		t.Errorf("control instruction must stay a plain checklist, got %q", control)
	}
	if !strings.HasPrefix(wired, PlanFirstInstruction) || control != PlanFirstInstruction {
		t.Error("both arms must share the checklist text; the grammar is an addendum")
	}
}
