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

// TestCmdVerify_SinceFollowsAMovedPlan: a plan moved inside the range (an
// archive) is compared with its own copy at --since's ref, at the path it
// had there. A skip that predates the range stands; a skip gained in the
// move is still self-declared, so a move launders nothing.
func TestCmdVerify_SinceFollowsAMovedPlan(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	for _, tc := range []struct {
		name, base, moved string
		uncommitted       bool
		want              int
	}{
		{"skip predates the move", "- [x] Charge handler {id: SD-6, symbol: Charge, skip_test: true, skip: \"predates the range\"}\n",
			"- [x] Charge handler {id: SD-6, symbol: Charge, skip_test: true, skip: \"predates the range\"}\n", false, 0},
		{"skip predates an uncommitted move", "- [x] Charge handler {id: SD-6, symbol: Charge, skip_test: true, skip: \"predates the range\"}\n",
			"- [x] Charge handler {id: SD-6, symbol: Charge, skip_test: true, skip: \"predates the range\"}\n", true, 0},
		{"skip gained in the move", "- [ ] Charge handler {id: SD-6, symbol: Charge}\n",
			"- [x] Charge handler {id: SD-6, symbol: Charge, skip_test: true, skip: \"gained in the move\"}\n", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newSelfSkipGitRepo(t)
			repo.writePlan("docs/plans/sd6.md", tc.base)
			repo.commitAll("base")
			baseSHA := gitRevParse(t, repo.root, "HEAD")
			repo.writePlan("README.md", "an unrelated commit, so HEAD~1 has the plan where base had it\n")
			repo.commitAll("unrelated")
			repo.writePlan("docs/plans/archive/.keep", "")
			repo.run("mv", "docs/plans/sd6.md", "docs/plans/archive/sd6.md")
			pf := repo.writePlan("docs/plans/archive/sd6.md", tc.moved)
			if !tc.uncommitted {
				repo.commitAll("archive the plan")
			}

			code, out, errBuf := runInDir(t, repo.root, []string{
				"verify", "--service", "payment-service", "--plan", pf, "--store", storePath,
				"--since", baseSHA,
			})
			if code != tc.want {
				t.Fatalf("exit %d, want %d\nstdout=%s\nstderr=%s", code, tc.want, out, errBuf)
			}
			if tc.want == 1 && !strings.Contains(out, "self-declared: skip") {
				t.Errorf("want \"self-declared: skip\" in output, got:\n%s", out)
			}
		})
	}
}
