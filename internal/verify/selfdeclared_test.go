package verify

// selfdeclared_test.go — CV-10/CV-35 (ADR-044 §1.4, I45): a declaration
// added inside the change range (skip, disposition, retires:,
// surface_change:, a raised max:) excuses nothing; it is reported
// self-declared. Rows deleted since base are reported too. This
// generalises O56's self-skip check (internal/verify/selfskip.go, kept
// unchanged for callers with no base ref at all).

import (
	"reflect"
	"sort"
	"testing"
)

func TestAnnotateSelfDeclared_RangeWide(t *testing.T) {
	two, four := 2, 4

	atBase := Plan{Service: "svc", Tasks: []Task{
		{ID: "A", Claimed: true}, // gains a skip inside the range
		{ID: "B", Claimed: true, Expect: Expect{SkipTest: true, SkipReason: "already skipped at base"}}, // skip predates the claim
		{ID: "C", Claimed: false, Skipped: false},                                                       // gains a disposition + skip flip at head
		{ID: "D", Retires: []string{"api:foo.Old"}},                                                     // retires predates the claim
		{ID: "E"},            // gains surface_change
		{ID: "F", Max: &two}, // max unchanged
		{ID: "G", Max: &two}, // max raised
		{ID: "H"},            // untouched
		{ID: "DELETED-1"},    // present at base, gone at head
	}}

	current := Plan{Service: "svc", Tasks: []Task{
		{ID: "A", Claimed: true, Expect: Expect{SkipTest: true, SkipReason: "added this edit"}},
		{ID: "B", Claimed: true, Expect: Expect{SkipTest: true, SkipReason: "already skipped at base"}},
		{ID: "C", Claimed: false, Skipped: true, Disposition: "deferred", Reason: "not now"},
		{ID: "D", Retires: []string{"api:foo.Old"}},
		{ID: "E", SurfaceChange: []string{"json:foo.Bar.tag"}},
		{ID: "F", Max: &two},
		{ID: "G", Max: &four},
		{ID: "H"},
	}}

	out := AnnotateSelfDeclared(current, &atBase)

	byID := map[string]Task{}
	for _, tk := range out.Tasks {
		byID[tk.ID] = tk
	}

	if got := byID["A"].SelfDeclared; !reflect.DeepEqual(got, []string{"skip"}) {
		t.Errorf("A SelfDeclared = %v, want [skip]", got)
	}
	if !byID["A"].SelfSkipped {
		t.Errorf("A: claimed row with a range-wide self-declared skip must set SelfSkipped")
	}
	if got := byID["B"].SelfDeclared; len(got) != 0 {
		t.Errorf("B SelfDeclared = %v, want none (skip predates base)", got)
	}
	if byID["B"].SelfSkipped {
		t.Errorf("B: skip predates the claim — must not be flagged")
	}
	if got := byID["C"].SelfDeclared; !reflect.DeepEqual(got, []string{"disposition"}) {
		t.Errorf("C SelfDeclared = %v, want [disposition]", got)
	}
	if got := byID["D"].SelfDeclared; len(got) != 0 {
		t.Errorf("D SelfDeclared = %v, want none (retires predates base)", got)
	}
	if got := byID["E"].SelfDeclared; !reflect.DeepEqual(got, []string{"surface_change"}) {
		t.Errorf("E SelfDeclared = %v, want [surface_change]", got)
	}
	if got := byID["F"].SelfDeclared; len(got) != 0 {
		t.Errorf("F SelfDeclared = %v, want none (max unchanged)", got)
	}
	if got := byID["G"].SelfDeclared; !reflect.DeepEqual(got, []string{"max"}) {
		t.Errorf("G SelfDeclared = %v, want [max] (raised from 2 to 4)", got)
	}
	if got := byID["H"].SelfDeclared; len(got) != 0 {
		t.Errorf("H SelfDeclared = %v, want none", got)
	}

	if want := []string{"DELETED-1"}; !reflect.DeepEqual(out.DeletedRows, want) {
		t.Errorf("DeletedRows = %v, want %v", out.DeletedRows, want)
	}

	// Deterministic: task order preserved, same length.
	if len(out.Tasks) != len(current.Tasks) {
		t.Fatalf("task count changed: %d -> %d", len(current.Tasks), len(out.Tasks))
	}
	for i, tk := range out.Tasks {
		if tk.ID != current.Tasks[i].ID {
			t.Errorf("task order changed at %d: %s != %s", i, tk.ID, current.Tasks[i].ID)
		}
	}
}

func TestAnnotateSelfDeclared_RaisedMaxIsSelfDeclared(t *testing.T) {
	five, ten := 5, 10
	atBase := Plan{Tasks: []Task{{ID: "R-1", Max: &five}}}
	current := Plan{Tasks: []Task{{ID: "R-1", Max: &ten}}}

	out := AnnotateSelfDeclared(current, &atBase)
	if got := out.Tasks[0].SelfDeclared; !reflect.DeepEqual(got, []string{"max"}) {
		t.Fatalf("SelfDeclared = %v, want [max]", got)
	}

	// A row with a brand-new max (absent at base) is also self-declared —
	// no baseline max to compare against means it was never ratcheted
	// before this range.
	atBase2 := Plan{Tasks: []Task{{ID: "R-2"}}}
	current2 := Plan{Tasks: []Task{{ID: "R-2", Max: &five}}}
	out2 := AnnotateSelfDeclared(current2, &atBase2)
	if got := out2.Tasks[0].SelfDeclared; !reflect.DeepEqual(got, []string{"max"}) {
		t.Errorf("brand-new max SelfDeclared = %v, want [max]", got)
	}

	// A row missing entirely at base (a genuinely new row) with a max is
	// self-declared too.
	atBase3 := Plan{}
	current3 := Plan{Tasks: []Task{{ID: "R-3", Max: &five}}}
	out3 := AnnotateSelfDeclared(current3, &atBase3)
	if got := out3.Tasks[0].SelfDeclared; !reflect.DeepEqual(got, []string{"max"}) {
		t.Errorf("new-row max SelfDeclared = %v, want [max]", got)
	}
}

func TestAnnotateSelfDeclared_NilBaseFlagsNothing(t *testing.T) {
	five := 5
	current := Plan{Tasks: []Task{
		{ID: "X", Claimed: true, Expect: Expect{SkipTest: true, SkipReason: "r"}},
		{ID: "Y", Max: &five},
	}}
	out := AnnotateSelfDeclared(current, nil)
	for _, tk := range out.Tasks {
		if len(tk.SelfDeclared) != 0 {
			t.Errorf("%s: SelfDeclared = %v, want none with atBase == nil", tk.ID, tk.SelfDeclared)
		}
	}
	if out.DeletedRows != nil {
		t.Errorf("DeletedRows = %v, want nil with atBase == nil", out.DeletedRows)
	}
	// Defensive copy, not the same backing array.
	if &out.Tasks[0] == &current.Tasks[0] {
		t.Errorf("AnnotateSelfDeclared must return a copy of Tasks")
	}
}

func TestOverlaySelfDeclared_FailsClaimedSelfDeclaredSkipAndRendersOthers(t *testing.T) {
	four := 4
	plan := Plan{Tasks: []Task{
		{ID: "S-1", Claimed: true, SelfDeclared: []string{"skip"}, SelfSkipped: true, Expect: Expect{SkipTest: true, SkipReason: "added mid-range"}},
		{ID: "S-2", Claimed: true, SelfDeclared: []string{"max"}, Max: &four},
		{ID: "S-3", Claimed: true}, // nothing self-declared
	}}
	rep := Report{Service: "svc", Tasks: []TaskVerdict{
		{TaskID: "S-1", Status: Pass, Label: "verified", Claimed: true},
		{TaskID: "S-2", Status: Pass, Label: "verified", Claimed: true},
		{TaskID: "S-3", Status: Pass, Label: "verified", Claimed: true},
	}, Summary: Summary{Total: 3, Pass: 3}}

	OverlaySelfDeclared(&rep, plan)

	if rep.Tasks[0].Status != Fail {
		t.Errorf("S-1: status = %s, want Fail (self-declared skip)", rep.Tasks[0].Status)
	}
	if rep.Tasks[1].Status != Pass {
		t.Errorf("S-2: status = %s, want Pass (self-declared max is informational only)", rep.Tasks[1].Status)
	}
	foundInfo := false
	for _, c := range rep.Tasks[1].Criteria {
		if c.Name == "self_declared" {
			foundInfo = true
		}
	}
	if !foundInfo {
		t.Errorf("S-2: no self_declared criterion rendered: %+v", rep.Tasks[1].Criteria)
	}
	if len(rep.Tasks[2].Criteria) != 0 {
		t.Errorf("S-3: no self-declaration, no extra criteria expected: %+v", rep.Tasks[2].Criteria)
	}
	if rep.Summary != (Summary{Total: 3, Pass: 2, Fail: 1}) {
		t.Errorf("summary not recounted: %+v", rep.Summary)
	}
}

func TestOverlaySelfDeclared_NoTouchLeavesSummaryUntouched(t *testing.T) {
	plan := Plan{Tasks: []Task{{ID: "N-1"}}}
	rep := Report{Tasks: []TaskVerdict{{TaskID: "N-1", Status: Pass}}, Summary: Summary{Total: 1, Pass: 1}}
	OverlaySelfDeclared(&rep, plan)
	if rep.Summary != (Summary{Total: 1, Pass: 1}) {
		t.Errorf("summary changed with nothing to overlay: %+v", rep.Summary)
	}
}

// sortedCopy is a tiny local helper so tests above can assert sortedness
// without importing more than sort itself.
func sortedCopy(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return out
}

func TestAnnotateSelfDeclared_KeysAreSorted(t *testing.T) {
	four := 4
	atBase := Plan{Tasks: []Task{{ID: "Z"}}}
	current := Plan{Tasks: []Task{{
		ID: "Z", Claimed: true, Disposition: "deferred", Reason: "x",
		Retires: []string{"api:x"}, SurfaceChange: []string{"json:y"}, Max: &four,
		Expect: Expect{SkipTest: true, SkipReason: "r"},
	}}}
	out := AnnotateSelfDeclared(current, &atBase)
	got := out.Tasks[0].SelfDeclared
	if !reflect.DeepEqual(got, sortedCopy(got)) {
		t.Errorf("SelfDeclared %v is not sorted", got)
	}
	want := []string{"disposition", "max", "retires", "skip", "surface_change"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SelfDeclared = %v, want %v", got, want)
	}
}
