package console

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRefactorView_NotRunIsNeverConserved pins OC-26: every ADR-044
// plane gets a chip; a plane the report does not carry — no report for the
// range yet, or no such plane in this build — reads "not run", never
// "conserved"; an unproven plane names its missing inputs; and the reading
// set carries an excerpt per partial item with its unexecuted lines marked.
func TestRefactorView_NotRunIsNeverConserved(t *testing.T) {
	// No report for the range: every plane is "not run".
	p := RefactorView(RefactorInput{Service: "proof", Since: "HEAD~1", Registered: []string{"absence", "integrity"}})
	if p.State != StateAbsent || len(p.Planes) != len(ADR44Planes) {
		t.Fatalf("no report = %+v", p)
	}
	for _, c := range p.Planes {
		if c.Verdict != "not run" || c.Reason == "" {
			t.Errorf("plane %s with no report = %+v, want not run", c.Plane, c)
		}
	}
	if p.Residue.State != StateAbsent || p.Residue.Changed.State != StateAbsent {
		t.Errorf("no report, no reading set: %+v", p.Residue)
	}

	src := "package proof\n\nfunc Settle(amount int) int {\n\tif err := validate(amount); err != nil {\n\t\treturn 0\n\t}\n\tif amount < 0 {\n\t\treturn -amount\n\t}\n\treturn amount\n}\n"
	in := RefactorInput{
		Service: "proof", Since: "HEAD~1", Ran: true, Base: "0123456789abcdef", Head: "worktree",
		Registered: []string{"absence", "integrity", "surface"},
		Planes: []ConservePlane{
			{Plane: "absence", Verdict: "conserved", Missing: []string{}, Findings: []ConserveFinding{{Plane: "absence", Kind: "edited", Key: "TestZ", Declared: "none", Informational: true}}},
			{Plane: "integrity", Verdict: "losses", Missing: []string{}, Findings: []ConserveFinding{{Plane: "integrity", Kind: "deleted", Key: "TestX", Declared: "none"}, {Plane: "integrity", Kind: "edited", Key: "TestY", Declared: "prior"}}},
			{Plane: "surface", Verdict: "unproven", Missing: []string{"typed pass failed"}, Findings: []ConserveFinding{}},
		},
		Residue: &Residue{
			Changed: 3, Attested: 1, Partial: 1, Unreached: 1, ClassedByExecution: 3,
			Items: []ResidueItem{
				{Symbol: "Settle", File: "settle.go", Line: 3, Class: "partial", Evidence: "executed", UnexecutedLines: []string{"7-9"}},
				{Symbol: "Audit", File: "settle.go", Line: 20, Class: "unreached"},
			},
			Unanalyzed: []string{"README.md"}, Missing: []string{},
		},
		Sources: map[string]string{"settle.go": src},
	}
	p = RefactorView(in)
	chips := map[string]PlaneChip{}
	for _, c := range p.Planes {
		chips[c.Plane] = c
	}
	if c := chips["absence"]; c.Verdict != "conserved" || c.Role != "positive" || c.Losses != 0 || !strings.Contains(c.Reason, "shown for visibility") {
		t.Errorf("absence (a conserved plane with an informational finding) = %+v, want no loss counted", c)
	}
	if chips["integrity"].Verdict != "losses" || chips["integrity"].Losses != 1 || chips["integrity"].Findings != 2 {
		t.Errorf("integrity = %+v", chips["integrity"])
	}
	if chips["surface"].Verdict != "unproven" || !strings.Contains(chips["surface"].Reason, "typed pass failed") {
		t.Errorf("surface = %+v", chips["surface"])
	}
	// Registered but not in the report, and ADR-044's generated plane with
	// no implementation in this build: both "not run", with different reasons.
	for _, name := range []string{"certificates", "generated", "governance", "resurrection"} {
		c := chips[name]
		if c.Verdict != "not run" || c.Verdict == "conserved" || !strings.Contains(c.Reason, "registered") {
			t.Errorf("%s = %+v, want not run naming that no plane is registered", name, c)
		}
	}
	if len(p.Findings) != 3 {
		t.Errorf("findings = %+v", p.Findings)
	}
	r := p.Residue
	if r.State != StatePresent || r.Changed.V != 3 || r.Partial.V != 1 || r.Unexecuted.State != StateZero {
		t.Fatalf("residue = %+v", r)
	}
	ex := r.Items[0].Excerpt
	if len(ex) != 9 || ex[0].Number != 3 || ex[len(ex)-1].Number != 11 {
		t.Fatalf("excerpt = %+v", ex)
	}
	for _, l := range ex {
		want := l.Number >= 7 && l.Number <= 9
		if l.Unexecuted != want {
			t.Errorf("line %d unexecuted = %v, want %v", l.Number, l.Unexecuted, want)
		}
	}
	if r.Items[1].Excerpt != nil {
		t.Error("only a partial item carries an excerpt")
	}
	// A reading set with a missing input classes nothing (I44).
	in.Residue.Missing = []string{"the index was ingested at abc, HEAD is def — re-ingest"}
	r = RefactorView(in).Residue
	if r.State != StateAbsent || r.Attested.State != StateAbsent || r.Changed.State != StatePresent || !strings.Contains(r.Reason, "re-ingest") {
		t.Errorf("missing input = %+v", r)
	}
	raw, _ := json.Marshal(r.Attested)
	if strings.Contains(string(raw), `"v"`) {
		t.Errorf("an unclassed count carries no number: %s", raw)
	}
	// A failed report: every plane not run, the page an error.
	p = RefactorView(RefactorInput{Service: "proof", Since: "nope", Err: "unknown revision nope"})
	if p.State != StateError || p.Planes[0].Verdict != "not run" {
		t.Errorf("failed report = %+v", p)
	}
	if lo, hi, ok := parseLineRange("12-14"); !ok || lo != 12 || hi != 14 {
		t.Error("line range")
	}
	if _, _, ok := parseLineRange("9-3"); ok {
		t.Error("an inverted range is refused")
	}
}

// TestRefactorView_NoExecutionEvidenceIsNotMeasured pins "absent never 0"
// on the reading set: partial and unexecuted exist only where execution
// evidence classed a function (design §3.5), so with none they are not
// measured — never a colored zero — and with some of the changed functions
// classed by static reach instead, they are a named lower bound.
func TestRefactorView_NoExecutionEvidenceIsNotMeasured(t *testing.T) {
	in := RefactorInput{
		Service: "fathom", Since: "HEAD~5", Ran: true, Base: "437761a6", Head: "worktree",
		Residue: &Residue{
			Changed: 9, Unattested: 9, ClassedByExecution: 0, Missing: []string{},
			Items: []ResidueItem{{Symbol: "F", File: "f.go", Line: 3, Class: "unattested", Evidence: "static-reach"}},
		},
	}
	r := RefactorView(in).Residue
	if r.State != StatePresent || r.Unattested.V != 9 || r.Unattested.State != StatePresent {
		t.Fatalf("static classes = %+v", r)
	}
	for name, v := range map[string]Value[int]{"partial": r.Partial, "unexecuted": r.Unexecuted} {
		if v.State != StateAbsent || !strings.Contains(v.Reason, "no execution evidence") {
			t.Errorf("%s with no execution evidence = %+v, want absent with the reason", name, v)
		}
		raw, _ := json.Marshal(v)
		if strings.Contains(string(raw), `"v"`) {
			t.Errorf("%s: an unmeasured class carries no number: %s", name, raw)
		}
	}
	// A coverage profile that could not be used names why.
	in.Residue.CoverageUnusable = "the profile was taken on a dirty tree"
	if r := RefactorView(in).Residue; !strings.Contains(r.Partial.Reason, "dirty tree") {
		t.Errorf("unusable coverage = %+v", r.Partial)
	}
	// No changed function at all: nothing could be partial — a measured 0.
	empty := RefactorInput{Service: "fathom", Since: "HEAD~1", Ran: true, Residue: &Residue{Missing: []string{}}}
	if r := RefactorView(empty).Residue; r.Partial.State != StateZero || r.Unexecuted.State != StateZero {
		t.Errorf("no changed functions = %+v / %+v, want measured zeros", r.Partial, r.Unexecuted)
	}
	// Some functions classed by execution, the rest by static reach: a
	// lower bound naming the split.
	in.Residue.CoverageUnusable = ""
	in.Residue.ClassedByExecution, in.Residue.Partial = 4, 1
	r = RefactorView(in).Residue
	if r.Partial.State != StatePartial || r.Partial.V != 1 || !strings.Contains(r.Partial.Reason, "4 of 9") {
		t.Errorf("partly classed by execution = %+v", r.Partial)
	}
}
