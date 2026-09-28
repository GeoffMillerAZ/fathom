package langbench

import (
	"reflect"
	"testing"
)

// TestScoreXrefs pins LB-F30's scoring: entity-grain pairs when the oracle
// names targets, file grain when it names only files, duplicates
// collapsed, and a tooling reading's xref metrics recomputed from the
// summed counts at the oracle's tier.
func TestScoreXrefs(t *testing.T) {
	truth := []XrefPair{{"a.yml", "build", "x/action.yml", "X"}, {"a.yml", "build", "y/action.yml", "Y"}, {"a.yml", "build", "y/action.yml", "Y"}}
	found := []XrefPair{{"a.yml", "build", "x/action.yml", "X"}, {"a.yml", "test", "z.yml", "Z"}}
	c := map[string]int{}
	ScoreXrefs(truth, found, c)
	want := map[string]int{CntXrefTruth: 2, CntXrefFathom: 2, CntXrefTruthFound: 1, CntXrefFathomInTrue: 1}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("entity grain counts %v, want %v", c, want)
	}

	c = map[string]int{}
	ScoreXrefs([]XrefPair{{"a.yml", "build", "x/action.yml", ""}}, []XrefPair{{"a.yml", "build", "x/action.yml", "Whatever"}}, c)
	if c[CntXrefTruthFound] != 1 || c[CntXrefFathomInTrue] != 1 {
		t.Fatalf("file grain counts %v", c)
	}

	r := Reading{Dialect: "github-actions", Kind: "tooling", TargetLevel: "L3"}
	MergeSubject(&r, Subject{Name: "s", Files: 3, Bytes: 3e6,
		Counts: map[string]int{CntOracleDefs: 10, CntFathomDefs: 10, CntMatched: 10, CntRefTruth: 4, CntRefTruthFound: 4, CntRefFathom: 4, CntRefFathomInTrue: 4,
			CntXrefTruth: 10, CntXrefTruthFound: 9, CntXrefFathom: 9, CntXrefFathomInTrue: 9},
		Families: map[string]string{FamilyL1: "C", FamilyRefs: "C", FamilyXref: "C"},
		Tenancy:  &SubjectTenancy{FaultingFiles: []string{}, ExtractMS: 100}})
	if r.Metrics[MXrefPrecision] == nil || *r.Metrics[MXrefPrecision] != 1 || r.Metrics[MXrefRecall] == nil || *r.Metrics[MXrefRecall] != 0.9 {
		t.Fatalf("xref metrics %v", r.Metrics)
	}
	if r.MetricTiers[MXrefRecall] != "C" || r.LevelMet != "L3" || r.Grade.L3 != GradeGood {
		t.Fatalf("tiers %v level %s grade %+v", r.MetricTiers, r.LevelMet, r.Grade)
	}
}
