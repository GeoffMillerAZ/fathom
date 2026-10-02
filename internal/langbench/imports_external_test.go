package langbench

import "testing"

// TestScoreImportsExternalCrateIsOutOfScope (LB-F197, O289): a statement that
// binds an in-repo name and an outside crate's pairs one oracle set with both
// bindings; the outside crate's has no file to resolve to and is out of scope,
// not unresolved.
func TestScoreImportsExternalCrateIsOutOfScope(t *testing.T) {
	inScope := map[string]bool{"lib.rs": true, "a.rs": true}
	truth := []OracleImport{{Path: "lib.rs", Line: 1, ToPath: "a.rs", Explicit: true}}
	sites := []ImportSite{
		{Path: "lib.rs", StartLine: 1, EndLine: 1, Local: "X", Targets: []string{"a.rs"}},
		{Path: "lib.rs", StartLine: 1, EndLine: 1, Local: "Y", External: true},
		{Path: "lib.rs", StartLine: 1, EndLine: 1, Local: "Y", External: true}, // its whole-module reading
		{Path: "lib.rs", StartLine: 1, EndLine: 1, Local: "Z"},                  // an in-repo name that did not resolve
	}
	counts := map[string]int{}
	ScoreImports(truth, sites, inScope, counts)
	if counts[CntImports] != 2 || counts[CntImportsInRepo] != 2 || counts[CntImportsAgree] != 1 || counts[CntImportsUnresolved] != 1 || counts[CntImportsExternal] != 1 {
		t.Errorf("counts = %v, want imports 2, in_repo 2, agree 1, unresolved 1, external 1", counts)
	}
}
