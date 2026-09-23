package scenariobench

import (
	"reflect"
	"strings"
	"testing"
)

func variantTestCorpus() Corpus {
	return Corpus{
		Version:     1,
		Description: "arm-variant test corpus",
		Arms: map[string]Arm{
			"fathom": {
				Wired:           true,
				MCP:             map[string]any{"fathom": map[string]any{"url": "http://x"}},
				AllowedTools:    []string{"Read", "Grep", "Bash"},
				DisallowedTools: []string{"WebFetch"},
				Instructions:    "stanza",
				Setup:           []string{"echo base-setup"},
				Surface:         "fathom",
				Env:             map[string]string{"FOO": "bar"},
				Prediction:      "fathom wins",
			},
			"control": {
				Wired:        false,
				AllowedTools: []string{"Read", "Grep", "Bash"},
				Instructions: "plain",
				Prediction:   "control loses",
			},
		},
		Tasks: []Task{
			{
				ID: "T1",
				Predictions: map[string]string{
					"fathom":  "fathom wins T1",
					"control": "control loses T1",
				},
			},
			{
				ID: "T2",
				Predictions: map[string]string{
					"fathom":  "fathom wins T2",
					"control": "control loses T2",
				},
			},
		},
		Criterion: Criterion{ID: "C1", Treatment: "fathom", Control: "control", MinTrials: 1},
	}
}

func TestCloneArmKeepsEveryFieldOfTheBase(t *testing.T) {
	c := variantTestCorpus()
	base := c.Arms["fathom"]
	if err := CloneArm(&c, "fathom-jev", "fathom"); err != nil {
		t.Fatalf("CloneArm: %v", err)
	}
	clone, ok := c.Arms["fathom-jev"]
	if !ok {
		t.Fatalf("CloneArm: new arm %q not present", "fathom-jev")
	}
	if !reflect.DeepEqual(clone, base) {
		t.Fatalf("CloneArm: clone = %+v, want every field equal to base %+v", clone, base)
	}
	// the base arm itself must be untouched
	if !reflect.DeepEqual(c.Arms["fathom"], base) {
		t.Fatalf("CloneArm: base arm mutated, got %+v want %+v", c.Arms["fathom"], base)
	}
	// every task's own registered prediction for the base arm is carried
	// over to the clone (SB-10: a cell must never run unregistered).
	for _, task := range c.Tasks {
		if task.Predictions["fathom-jev"] != task.Predictions["fathom"] {
			t.Fatalf("task %s: Predictions[fathom-jev] = %q, want %q (copied from base)", task.ID, task.Predictions["fathom-jev"], task.Predictions["fathom"])
		}
	}
}

func TestCloneArmErrors(t *testing.T) {
	c := variantTestCorpus()
	if err := CloneArm(&c, "new", "nonexistent"); err == nil {
		t.Fatalf("CloneArm from a nonexistent base: want an error, got nil")
	}
	if err := CloneArm(&c, "control", "fathom"); err == nil {
		t.Fatalf("CloneArm onto an already-existing arm name: want an error, got nil")
	}
}

func TestSetupArmAppendsAndNeverMutatesTheBase(t *testing.T) {
	c := variantTestCorpus()
	if err := CloneArm(&c, "fathom-jev", "fathom"); err != nil {
		t.Fatalf("CloneArm: %v", err)
	}
	line := `printf '\n[decide]\nprovider = "typesafe"\n' >> .fathom/config.toml`
	if err := SetupArm(&c, "fathom-jev", line); err != nil {
		t.Fatalf("SetupArm: %v", err)
	}
	got := c.Arms["fathom-jev"].Setup
	want := []string{"echo base-setup", line}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Setup after SetupArm = %v, want %v", got, want)
	}
	// the base arm's own Setup must be untouched by setting up its clone.
	if got := c.Arms["fathom"].Setup; !reflect.DeepEqual(got, []string{"echo base-setup"}) {
		t.Fatalf("base arm Setup mutated by SetupArm on its clone: got %v", got)
	}
}

func TestSetupArmErrorsOnUnknownArm(t *testing.T) {
	c := variantTestCorpus()
	if err := SetupArm(&c, "nonexistent", "echo hi"); err == nil {
		t.Fatalf("SetupArm on an unknown arm: want an error, got nil")
	}
}

func TestCloneAndSetupChangeTheCorpusHash(t *testing.T) {
	c := variantTestCorpus()
	before, err := CanonicalCorpusSHA256(c)
	if err != nil {
		t.Fatalf("CanonicalCorpusSHA256(before): %v", err)
	}
	if err := CloneArm(&c, "fathom-jev", "fathom"); err != nil {
		t.Fatalf("CloneArm: %v", err)
	}
	if err := SetupArm(&c, "fathom-jev", "echo jev-setup"); err != nil {
		t.Fatalf("SetupArm: %v", err)
	}
	after, err := CanonicalCorpusSHA256(c)
	if err != nil {
		t.Fatalf("CanonicalCorpusSHA256(after): %v", err)
	}
	if before == after {
		t.Fatalf("corpus hash unchanged after -arm-clone/-arm-setup: %s — a variant must never collide with (or borrow cells from) its base corpus", before)
	}

	// a corpus that reused an untouched base's OWN sha would let the
	// variant's cells silently borrow the base corpus's stored "none"
	// cells (recipe rule, docs/bench/scenarios.md) — reloading the
	// original, unmodified corpus must still reproduce `before` exactly,
	// proving the change in hash came from the mutation, not from
	// CanonicalCorpusSHA256 itself being nondeterministic.
	again, err := CanonicalCorpusSHA256(variantTestCorpus())
	if err != nil {
		t.Fatalf("CanonicalCorpusSHA256(again): %v", err)
	}
	if again != before {
		t.Fatalf("CanonicalCorpusSHA256 nondeterministic across equal corpora: %s != %s", again, before)
	}
}

func TestArmSelectionKeepsExactlyTheNamedArms(t *testing.T) {
	cases := []struct {
		name        string
		single      string
		commaList   string
		wantNil     bool
		wantMembers []string
	}{
		{name: "both empty means every arm", single: "", commaList: "", wantNil: true},
		{name: "single only", single: "fathom", commaList: "", wantMembers: []string{"fathom"}},
		{name: "comma list only", single: "", commaList: "fathom,fathom-jev", wantMembers: []string{"fathom", "fathom-jev"}},
		{name: "single composes with comma list", single: "control", commaList: "fathom,fathom-jev", wantMembers: []string{"control", "fathom", "fathom-jev"}},
		{name: "whitespace and blanks are ignored", single: "  ", commaList: " fathom , , fathom-jev ", wantMembers: []string{"fathom", "fathom-jev"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ArmSelection(tc.single, tc.commaList)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("ArmSelection(%q, %q) = %v, want nil", tc.single, tc.commaList, got)
				}
				return
			}
			if len(got) != len(tc.wantMembers) {
				t.Fatalf("ArmSelection(%q, %q) = %v, want exactly %v", tc.single, tc.commaList, got, tc.wantMembers)
			}
			for _, m := range tc.wantMembers {
				if !got[m] {
					t.Fatalf("ArmSelection(%q, %q) = %v, missing %q", tc.single, tc.commaList, got, m)
				}
			}
		})
	}
}

// TestArmSelectionAgainstRealArms exercises ArmSelection with the exact
// arm names a real corpus defines (variantTestCorpus's "fathom"/"control"
// plus a cloned "fathom-jev"), the shape scenariobench run's own -arms
// flag composes against — proving the filter keeps exactly the two named
// arms of a three-arm corpus, dropping the rest.
func TestArmSelectionAgainstRealArms(t *testing.T) {
	c := variantTestCorpus()
	if err := CloneArm(&c, "fathom-jev", "fathom"); err != nil {
		t.Fatalf("CloneArm: %v", err)
	}
	sel := ArmSelection("", "fathom,fathom-jev")
	var kept []string
	for name := range c.Arms {
		if sel[name] {
			kept = append(kept, name)
		}
	}
	if len(kept) != 2 || !sel["fathom"] || !sel["fathom-jev"] || sel["control"] {
		t.Fatalf("ArmSelection kept %v of arms %v, want exactly [fathom fathom-jev]", kept, armNames(c))
	}
}

func armNames(c Corpus) []string {
	var out []string
	for name := range c.Arms {
		out = append(out, name)
	}
	return out
}

func TestCloneArmRejectsBlankNames(t *testing.T) {
	c := variantTestCorpus()
	if err := CloneArm(&c, "", "fathom"); err == nil {
		t.Fatalf("CloneArm with a blank new name: want an error, got nil")
	}
	if err := CloneArm(&c, "x", ""); err == nil {
		t.Fatalf("CloneArm with a blank base name: want an error, got nil")
	}
}

func TestSetupArmLineSurvivesVerbatim(t *testing.T) {
	c := variantTestCorpus()
	line := `printf '\n[decide]\nprovider = "typesafe"\nendpoint = "https://openrouter.ai/api"\nmodel = "typesafe/jev-1.13"\napi_key_cmd = ["cat", "/Users/geoff/secrets/openrouter/token"]\n' >> .fathom/config.toml`
	if err := SetupArm(&c, "fathom", line); err != nil {
		t.Fatalf("SetupArm: %v", err)
	}
	got := c.Arms["fathom"].Setup
	if len(got) != 2 || got[1] != line {
		t.Fatalf("Setup = %v, want the appended line to survive byte-for-byte", got)
	}
	if !strings.Contains(got[1], `provider = "typesafe"`) {
		t.Fatalf("Setup line lost its content: %q", got[1])
	}
}
