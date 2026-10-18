package readout

import (
	"os"
	"strings"
	"testing"
)

// stageCorpus are inputs that stress the lossless stages' encodings: ANSI
// and carriage-return progress, repeated lines, lines that already end in
// ` ×N`, grep-shaped keys, indented lines a key's header could absorb, blank
// lines and a missing trailing newline.
var stageCorpus = []string{
	"",
	"\n",
	"a\n",
	"no trailing newline",
	"\x1b[32mok\x1b[0m pkg\n\x1b[1;31mFAIL\x1b[0m x\n",
	"10%\r50%\r100%\ndone\n",
	"x\nx\nx\ny\nx\n",
	"same ×3\nsame ×3\nlonely ×2\n",
	"\n\n\nz\n",
	"a.go:1:foo\na.go:2:bar\nb.go:9:baz\nb.go:10:qux\n",
	"a.go:\n  indented natural line\na.go:1:x\na.go:2:y\n",
	"a.go:1:x\n  a.go:2:y\na.go:3:z\n",
	"k:\nk:\n",
}

// losslessStages are one stage of each lossless kind, compiled.
func losslessStages(t *testing.T) []Stage {
	t.Helper()
	ss := []Stage{
		{Kind: StageANSI},
		{Kind: StageCollapse},
		{Kind: StageGroup, Key: `^[^:\s]+:`},
		{Kind: StageExtract, Pattern: `^(\S+):(\d+)`, Captures: []string{"file", "line"}, Record: "hit"},
		{Kind: StageJoin, Join: "head_stamp"},
	}
	for i := range ss {
		if err := compileStage(&ss[i]); err != nil {
			t.Fatal(err)
		}
	}
	return ss
}

// TestLosslessStagesRoundTripEveryFixture is RO-6's contract: lossiness is
// computed from the closed stage set, and every stage that claims none
// round-trips every fixture (every loaded rule's, and the stage corpus)
// through its inverse.
func TestLosslessStagesRoundTripEveryFixture(t *testing.T) {
	for kind, want := range stageLoss {
		if got := Lossiness([]Stage{{Kind: kind}}); got != want {
			t.Errorf("Lossiness(%s) = %v, want %v", kind, got, want)
		}
	}
	if got := Lossiness(nil); got != LossNone {
		t.Errorf("Lossiness(nil) = %v, want none", got)
	}
	if got := Lossiness([]Stage{{Kind: StageCollapse}, {Kind: StageFields}, {Kind: StageDrop}}); got != LossLines {
		t.Errorf("Lossiness(collapse, fields, drop) = %v, want lines (the strongest)", got)
	}
	if got := Lossiness([]Stage{{Kind: "exec"}}); got != LossWhole {
		t.Errorf("Lossiness(unknown kind) = %v, want whole", got)
	}

	var fixtures []string
	fixtures = append(fixtures, stageCorpus...)
	sets := []*RuleSet{loadTestRules(t)}
	if rs, err := LoadRules(""); err != nil {
		t.Fatal(err)
	} else {
		sets = append(sets, rs)
	}
	if rs, err := loadRules(os.DirFS("testdata/builtins"), ""); err != nil || len(rs.Rules) == 0 {
		t.Fatalf("testdata built-ins: %v, %+v", err, rs)
	} else {
		sets = append(sets, rs)
	}
	stages := losslessStages(t)
	for _, rs := range sets {
		for _, r := range rs.Rules {
			for _, f := range r.Tests {
				fixtures = append(fixtures, f.Input)
			}
			for _, s := range r.Stages {
				if Lossiness([]Stage{s}) == LossNone {
					stages = append(stages, s)
				}
			}
		}
	}
	joins := map[string][]string{"head_stamp": {"HEAD a1b2c3d"}}
	for _, s := range stages {
		if Lossiness([]Stage{s}) != LossNone {
			t.Fatalf("%s: selected as lossless but Lossiness says %v", s.Kind, Lossiness([]Stage{s}))
		}
		for _, in := range fixtures {
			for _, j := range []map[string][]string{nil, joins} {
				ls, _ := splitLines(in)
				before := inputLines(ls)
				after := s.apply(before, &runCtx{joins: j})
				if err := s.roundTrip(before, after); err != nil {
					t.Errorf("%s on %q: %v", s.Kind, in, err)
				}
			}
		}
	}
}

func TestLossyStagesHaveNoInverse(t *testing.T) {
	for _, k := range []string{StageFields, StageKeep, StageDrop, StageCap, StageHunks, StageSummarize, StageOutline} {
		s := Stage{Kind: k}
		if err := s.roundTrip(nil, nil); err == nil {
			t.Errorf("%s: a lossy stage claimed an inverse", k)
		}
	}
}

func TestLoadRulesRefusesALossyRuleWithoutARecovery(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{UserFile: `schema = 1
[[rule]]
id = "no-recovery"
version = 1
about = "drops lines and names no recovery"
match.argv0 = ["make"]
[[rule.stage]]
kind = "drop"
pattern = '^cc '
[[rule.test]]
input = "cc a\n"
trip = true
`})
	rs, err := LoadRules(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs.Rules {
		if r.ID == "no-recovery" {
			t.Fatalf("no-recovery is served from %s", r.Source)
		}
	}
	if len(rs.Refused) != 1 || !strings.Contains(rs.Refused[0].Reason, "names no recovery") {
		t.Fatalf("rules=%d refused=%+v, want the rule refused for its missing recovery", len(rs.Rules), rs.Refused)
	}
}

func TestStagesKeepTheirEncodingsInvertible(t *testing.T) {
	ss := losslessStages(t)
	collapse, group := ss[1], ss[2]
	in, _ := splitLines("same ×3\nsame ×3\nlonely ×2\nplain\n")
	got := texts(collapse.apply(inputLines(in), &runCtx{}))
	if want := []string{"same ×3 ×2", "lonely ×2 ×1", "plain"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("collapse = %q, want %q", got, want)
	}
	in, _ = splitLines("a.go:1:x\na.go:2:y\nb.go:3:z\n")
	got = texts(group.apply(inputLines(in), &runCtx{}))
	if want := []string{"a.go:", "  1:x", "  2:y", "b.go:3:z"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("group = %q, want %q", got, want)
	}
	// An indented natural line after a key-only line would be absorbed by
	// ungroup, so group leaves that input as it is.
	in, _ = splitLines("a.go:\n  natural\n")
	if got := texts(group.apply(inputLines(in), &runCtx{})); strings.Join(got, "|") != "a.go:|  natural" {
		t.Fatalf("ambiguous group input changed: %q", got)
	}
}
