package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	assignment "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/assignment-kit"
)

func TestNamedOfflineSnapshots(t *testing.T) {
	out, e := snapshots()
	if e != nil {
		t.Fatal(e)
	}
	want := map[string]string{
		"command/complete":                      "complete",
		"command/missing-required-unknown":      "continue",
		"investigation/complete-stretch-failed": "complete",
		"investigation/unresolved-ambiguity":    "blocked",
		"investigation/resolved-complete":       "complete",
		"investigation/no-progress-pivot":       "pivot-required",
		"investigation/ambiguity-escalation":    "needs-review",
		"investigation/exhausted-budget-stop":   "stopped",
	}
	if len(out) != len(want) {
		t.Fatalf("got %d snapshots", len(out))
	}
	for name, status := range want {
		r, ok := out[name]
		if !ok || r.Status != status {
			t.Fatalf("%s: %q, want %q", name, r.Status, status)
		}
		if r.Trust == "" || r.ActionAuthority {
			t.Fatalf("%s claims authority", name)
		}
	}
	missing := out["command/missing-required-unknown"]
	if len(missing.Goals) != 2 || missing.Goals[1].Assessment.RequiredStatus != fitness.RequiredUnknown {
		t.Fatal("missing required input was not projected unknown")
	}
	stretch := out["investigation/complete-stretch-failed"]
	if len(stretch.Goals) != 4 || stretch.Goals[3].Kind != "stretch" || stretch.Goals[3].Assessment.RequiredStatus != fitness.RequiredFail {
		t.Fatal("stretch failure did not remain visible")
	}
	unresolved := out["investigation/unresolved-ambiguity"]
	if unresolved.Concerns[0].State != "open" {
		t.Fatal("open concern lost")
	}
	pivot := out["investigation/no-progress-pivot"]
	if !reflect.DeepEqual(pivot.MatchedRules, []string{"stalled"}) {
		t.Fatal("pivot rule missing")
	}
	escalation := out["investigation/ambiguity-escalation"]
	if !reflect.DeepEqual(escalation.MatchedRules, []string{"ambiguous"}) {
		t.Fatal("escalation rule missing")
	}
	stop := out["investigation/exhausted-budget-stop"]
	if !stop.Budgets[0].Exhausted {
		t.Fatal("exhausted budget lost")
	}
	b, e := json.Marshal(out)
	if e != nil || !json.Valid(b) {
		t.Fatal("reports not plain JSON", e)
	}
}
func TestCatalogChangeAndIndependentScope(t *testing.T) {
	defs, e := definitions(recipes)
	if e != nil {
		t.Fatal(e)
	}
	ca, cs, ci, e := commandQualification(defs)
	if e != nil {
		t.Fatal(e)
	}
	ia, is, ii, e := investigation(defs)
	if e != nil {
		t.Fatal(e)
	}
	revised := append([]catalog.AuthoredNode{}, recipes...)
	revised[0].Synopsis += " Revised boundary."
	newDefs, e := definitions(revised)
	if e != nil || newDefs["command.input-bound"] == defs["command.input-bound"] {
		t.Fatal("catalog rule change did not change complete definition digest", e)
	}
	updated, _, _, e := commandQualification(newDefs)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := assignment.Evaluate(updated, cs, ci, at); e == nil {
		t.Fatal("old scope/input pins accepted changed catalog definition")
	}
	ca.Goals = ca.Goals[:1]
	fp, e := assignment.Fingerprint(ca)
	if e != nil {
		t.Fatal(e)
	}
	cs.AssignmentDigest, ci.AssignmentDigest = fp, fp
	if _, e := assignment.Evaluate(ca, cs, ci, at); e == nil {
		t.Fatal("independent command scope missed omitted privacy requirement")
	}
	ia.Goals = append(ia.Goals[:2], ia.Goals[3:]...)
	fp, e = assignment.Fingerprint(ia)
	if e != nil {
		t.Fatal(e)
	}
	is.AssignmentDigest, ii.AssignmentDigest = fp, fp
	if _, e := assignment.Evaluate(ia, is, ii, at); e == nil {
		t.Fatal("independent investigation scope missed omitted counterevidence requirement")
	}
	moved := append([]catalog.AuthoredNode{}, recipes...)
	moved[0].StableKey = "command.moved"
	movedDefs, e := definitions(moved)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, _, e := commandQualification(movedDefs); e == nil {
		t.Fatal("moved stable key accepted with missing exact binding")
	}
}
