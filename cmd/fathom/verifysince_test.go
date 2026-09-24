package main

// verifysince_test.go — CV-14 (ADR-044 §1.4, I45): `fathom verify --since
// <ref>` reads the plan at an EXPLICIT ref (unlike selfskip_test.go's
// implicit HEAD/HEAD~1 comparison) and reports self-declared rows and rows
// deleted since that ref. Reuses selfskip_test.go's throwaway-git-repo
// harness (selfSkipGitRepo, runInDir) and plans_test.go's fixture store
// (seedPlansFixtureStore) — the same payment-service corpus every other
// verify test in this package uses.

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func gitRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("git -C %s rev-parse %s: %v", dir, ref, err)
	}
	return strings.TrimSpace(string(out))
}

// TestCmdVerify_SinceMarksSelfDeclared: a skip added anywhere inside
// base..head (not merely the last commit) fails the gate as self-declared,
// and a row present at base and missing at head is reported deleted —
// across an intermediate commit that touches neither declaration, proving
// --since reads the whole range, not just HEAD~1.
func TestCmdVerify_SinceMarksSelfDeclared(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)

	pf := repo.writePlan("docs/plans/sd.md",
		"- [ ] Charge handler {id: SD-1, symbol: Charge}\n"+
			"- [ ] Retired soon {id: SD-2, symbol: Charge}\n")
	repo.commitAll("base: both rows open, no declarations")
	baseSHA := gitRevParse(t, repo.root, "HEAD")

	repo.writePlan("docs/plans/sd.md",
		"- [ ] Charge handler, retitled {id: SD-1, symbol: Charge}\n"+
			"- [ ] Retired soon {id: SD-2, symbol: Charge}\n")
	repo.commitAll("an intermediate commit touching neither declaration")

	repo.writePlan("docs/plans/sd.md",
		"- [x] Charge handler, retitled {id: SD-1, symbol: Charge, skip_test: true, skip: \"declared mid-range\"}\n")
	repo.commitAll("claim SD-1 with a skip added inside the range; SD-2 removed")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath,
		"--since", baseSHA,
	})
	if code != 1 {
		t.Fatalf("want exit 1 (self-declared skip excuses nothing, I45), got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
	if !strings.Contains(out, "self-declared: skip") {
		t.Errorf("want \"self-declared: skip\" in output, got:\n%s", out)
	}
	wantDeleted := fmt.Sprintf("deleted since %s (%s): SD-2", baseSHA, pf)
	if !strings.Contains(out, wantDeleted) {
		t.Errorf("want %q in output, got:\n%s", wantDeleted, out)
	}
}

// TestCmdVerify_SinceSkipPredatesRange_Passes: the same skip, but present
// already AT the --since ref — it predates the range and stands, exactly
// as a skip predating a single commit stands under the narrower O56 check.
func TestCmdVerify_SinceSkipPredatesRange_Passes(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)

	pf := repo.writePlan("docs/plans/sd2.md",
		"- [ ] Charge handler {id: SD-3, symbol: Charge, skip_test: true, skip: \"predates the range\"}\n")
	repo.commitAll("base: skip already present, not yet claimed")
	baseSHA := gitRevParse(t, repo.root, "HEAD")

	repo.writePlan("docs/plans/sd2.md",
		"- [x] Charge handler {id: SD-3, symbol: Charge, skip_test: true, skip: \"predates the range\"}\n")
	repo.commitAll("claim the row — the skip itself is unchanged")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath,
		"--since", baseSHA, "--quiet-pass",
	})
	if code != 0 {
		t.Fatalf("want exit 0 (skip predates --since's ref), got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
}

// TestCmdVerify_SinceRaisedMaxIsSelfDeclaredButNeverFails: a raised max: is
// reported self-declared (I45) but, unlike skip, never fails the gate on
// its own — it is information for a reviewer, not a refusal.
func TestCmdVerify_SinceRaisedMaxIsSelfDeclaredButNeverFails(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)

	pf := repo.writePlan("docs/plans/sd3.md",
		"- [ ] Ratchet {id: SD-4, oracle: lexical, path: docs/plans/sd3.md, pattern: \"Ratchet\", max: 2}\n")
	repo.commitAll("base: ratchet at max 2")
	baseSHA := gitRevParse(t, repo.root, "HEAD")

	repo.writePlan("docs/plans/sd3.md",
		"- [x] Ratchet {id: SD-4, oracle: lexical, path: docs/plans/sd3.md, pattern: \"Ratchet\", max: 5}\n")
	repo.commitAll("raise the ratchet's max inside the range")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath,
		"--since", baseSHA,
	})
	if code != 0 {
		t.Fatalf("want exit 0 (a self-declared max never fails on its own), got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
	if !strings.Contains(out, "self-declared: max") {
		t.Errorf("want \"self-declared: max\" in output, got:\n%s", out)
	}
}
