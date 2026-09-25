package main

// refactor_status_test.go — CV-34 (ADR-044 §4): "fathom refactor status
// --plan FILE" reports each ratchet row's current count against its max
// and against the baseline's own count (git show, never a re-ingest), plus
// drift — references that landed on the branch since the baseline.
//
// Fixture: a plan whose ratchet was seeded when caller/ held ONE reference
// to OldFunc (baseline count 1, max 1) — then a second commit, landed
// AFTER the baseline, adds a second caller in the SAME scoped directory.
// Status must read: baseline 1, current 2, max 1 (violated), drift 1.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCmdRefactorStatus_RatchetAndDrift(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module refactorfixture\n\ngo 1.21\n")
	repo.writePlan("oldpkg/old.go", "package oldpkg\n\nfunc OldFunc() {}\n")
	repo.writePlan("caller/user.go",
		"package caller\n\nimport \"refactorfixture/oldpkg\"\n\nfunc CallsOld() {\n\toldpkg.OldFunc()\n}\n")
	repo.commitAll("baseline: one reference from caller/")
	baseline := headSHAOf(t, repo.root)

	// Drift: a SECOND reference lands in the same scoped directory after
	// the baseline commit.
	repo.writePlan("caller/more.go",
		"package caller\n\nimport \"refactorfixture/oldpkg\"\n\nfunc CallsOldAgain() {\n\toldpkg.OldFunc()\n}\n")
	repo.commitAll("drift: a second caller lands after baseline")

	planBody := "# Refactor: OldFunc\n\nBaseline: " + baseline + "\n\n" +
		"- [ ] caller ratchet {id: RS-1, gone: pattern \"\\bOldFunc\\b\", paths: \"caller/\", max: 1}\n"
	pf := repo.writePlan("docs/plans/refactor-oldfunc.md", planBody)

	code, out, errBuf := runInDir(t, repo.root, []string{
		"refactor", "status", "--plan", pf, "--json",
	})
	if code != 0 {
		t.Fatalf("refactor status failed (%d)\nstdout=%s\nstderr=%s", code, out, errBuf)
	}

	var rep struct {
		Baseline string `json:"baseline"`
		Ratchets []struct {
			ID       string `json:"id"`
			Max      int    `json:"max"`
			Current  int    `json:"current"`
			Baseline int    `json:"baseline"`
			Drift    int    `json:"drift"`
			OverMax  bool   `json:"over_max"`
		} `json:"ratchets"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v\nstdout=%s", err, out)
	}
	if rep.Baseline != baseline {
		t.Errorf("baseline = %q, want %q", rep.Baseline, baseline)
	}
	if len(rep.Ratchets) != 1 {
		t.Fatalf("ratchets = %+v, want exactly 1", rep.Ratchets)
	}
	r := rep.Ratchets[0]
	if r.ID != "RS-1" {
		t.Errorf("id = %q, want RS-1", r.ID)
	}
	if r.Max != 1 {
		t.Errorf("max = %d, want 1", r.Max)
	}
	if r.Baseline != 1 {
		t.Errorf("baseline count = %d, want 1 (the count at the baseline commit)", r.Baseline)
	}
	if r.Current != 2 {
		t.Errorf("current count = %d, want 2 (both callers present now)", r.Current)
	}
	if r.Drift != 1 {
		t.Errorf("drift = %d, want 1 (one new reference landed since baseline)", r.Drift)
	}
	if !r.OverMax {
		t.Errorf("over_max = false, want true (current 2 > max 1)")
	}
}

// TestCmdRefactorStatus_NoDriftWithinBound: a ratchet whose current count
// still equals the baseline (nothing landed) reports zero drift and is not
// over its max.
func TestCmdRefactorStatus_NoDriftWithinBound(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module refactorfixture\n\ngo 1.21\n")
	repo.writePlan("oldpkg/old.go", "package oldpkg\n\nfunc OldFunc() {}\n")
	repo.writePlan("caller/user.go",
		"package caller\n\nimport \"refactorfixture/oldpkg\"\n\nfunc CallsOld() {\n\toldpkg.OldFunc()\n}\n")
	repo.commitAll("baseline: one reference from caller/")
	baseline := headSHAOf(t, repo.root)

	planBody := "Baseline: " + baseline + "\n\n" +
		"- [ ] caller ratchet {id: RS-2, gone: pattern \"\\bOldFunc\\b\", paths: \"caller/\", max: 1}\n"
	pf := repo.writePlan("docs/plans/refactor-oldfunc2.md", planBody)

	code, out, errBuf := runInDir(t, repo.root, []string{
		"refactor", "status", "--plan", pf, "--json",
	})
	if code != 0 {
		t.Fatalf("refactor status failed (%d)\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
	if !strings.Contains(out, `"drift": 0`) {
		t.Errorf("expected zero drift when nothing landed since baseline:\n%s", out)
	}
	if !strings.Contains(out, `"over_max": false`) {
		t.Errorf("expected over_max:false when current == max:\n%s", out)
	}
}

// TestCmdRefactorStatus_RequiresBaselineHeader: a plan with no Baseline:
// header is refused with a named reason, never a silent zero-row report.
func TestCmdRefactorStatus_RequiresBaselineHeader(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/no-baseline.md",
		"- [ ] a row {id: NB-1, gone: pattern \"x\", paths: \"a/\", max: 1}\n")
	repo.commitAll("no baseline header")

	code, _, errBuf := runInDir(t, repo.root, []string{"refactor", "status", "--plan", pf})
	if code == 0 {
		t.Fatalf("want a non-zero exit for a plan with no Baseline: header; stderr=%s", errBuf)
	}
	if !strings.Contains(errBuf, "Baseline") {
		t.Errorf("expected the error to name the missing Baseline header, got: %s", errBuf)
	}
}
