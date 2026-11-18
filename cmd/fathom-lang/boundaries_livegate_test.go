//go:build boundarygate

package main

import (
	"path/filepath"
	"testing"
)

// TestFamilyDependencyBoundaryGate is the repository's first use. Native Go
// selects the production scope. The companion evaluates it using deps facts.
func TestFamilyDependencyBoundaryGate(t *testing.T) {
	root := familyPhysicalRoot(t, "../..")
	rules := filepath.Join(root, familyRulesFile)
	packages := familyGoList(t, root, "./pkg/lang", "./pkg/lang/iac", "./pkg/lang/deps", "./pkg/lang/cue", "./cmd/fathom-lang")
	files := familySourceFiles(t, root, packages)
	results, findings, code := runFamilyBoundaries(t, root, rules, files)
	if code != 0 {
		t.Fatalf("family boundary gate exited %d:\n%s", code, findings)
	}
	unverified := 0
	for _, rule := range results {
		if rule.MatchedFiles == 0 {
			t.Errorf("rule %s had no governed production files", rule.ID)
		}
		if (rule.ID == "standalone-ownership" || rule.ID == "separate-cue-door") && rule.MatchedFiles != len(files) {
			t.Errorf("rule %s governs %d of %d production files", rule.ID, rule.MatchedFiles, len(files))
		}
		if rule.Verdict == "unverified" {
			unverified++
		}
	}
	t.Logf("family boundary gate: %d production files, %d rules, %d unverified rules", len(files), len(results), unverified)
}
