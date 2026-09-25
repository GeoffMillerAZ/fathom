package main

// conserve_test.go — CV-19 (ADR-044 §0/§6): `fathom conserve --since <ref>
// [--json] [--strict]` end to end against a throwaway git repo, the same
// selfSkipGitRepo/runInDir fixture pattern selfskip_test.go already
// establishes for a git-aware CLI command.

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdConserve_JSONSchemaV1 pins CV-19: `fathom conserve --json` emits
// the fathom.conserve/v1 wire document naming service/base/head and every
// registered plane's verdict.
func TestCmdConserve_JSONSchemaV1(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc Old() {}\n")
	repo.commitAll("base commit")
	base := headSHAOf(t, repo.root)

	// Remove the exported func at head — an undeclared surface loss.
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc New() {}\n")

	storePath := filepath.Join(t.TempDir(), "conserve.sqlite")
	code, stdout, stderr := runInDir(t, repo.root, []string{
		"conserve", "--since", base, "--service", "svc", "--store", storePath, "--json",
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (no --strict); stderr=%s", code, stderr)
	}

	var rep map[string]any
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v\nstdout=%s", err, stdout)
	}
	if rep["schema"] != "fathom.conserve/v1" {
		t.Errorf("schema = %v, want fathom.conserve/v1", rep["schema"])
	}
	if rep["service"] != "svc" {
		t.Errorf("service = %v, want svc", rep["service"])
	}
	if rep["base"] != base {
		t.Errorf("base = %v, want %v", rep["base"], base)
	}
	planes, ok := rep["planes"].([]any)
	if !ok || len(planes) == 0 {
		t.Fatalf("planes = %v, want at least one registered plane", rep["planes"])
	}
	var sawSurfaceLosses bool
	for _, p := range planes {
		pm := p.(map[string]any)
		if pm["plane"] == "surface" && pm["verdict"] == "losses" {
			sawSurfaceLosses = true
		}
	}
	if !sawSurfaceLosses {
		t.Errorf("expected the surface plane to report losses for the removed api:widgets.Old; planes=%+v", planes)
	}
}

// TestCmdConserve_StrictExitsOneOnLosses pins the --strict exit-code
// contract.
func TestCmdConserve_StrictExitsOneOnLosses(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc Old() {}\n")
	repo.commitAll("base commit")
	base := headSHAOf(t, repo.root)
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc New() {}\n")

	storePath := filepath.Join(t.TempDir(), "conserve.sqlite")
	code, _, stderr := runInDir(t, repo.root, []string{
		"conserve", "--since", base, "--service", "svc", "--store", storePath, "--strict",
	})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (--strict, losses present); stderr=%s", code, stderr)
	}
}

// TestCmdConserve_NoLossesExitsZero pins that an unremarkable change (no
// removed/changed surface) is a clean pass even under --strict.
func TestCmdConserve_NoLossesExitsZero(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc Stable() {}\n")
	repo.commitAll("base commit")
	base := headSHAOf(t, repo.root)
	repo.writePlan("widgets/added.go", "package widgets\n\nfunc Added() {}\n")

	storePath := filepath.Join(t.TempDir(), "conserve.sqlite")
	code, _, stderr := runInDir(t, repo.root, []string{
		"conserve", "--since", base, "--service", "svc", "--store", storePath, "--strict",
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}
}

// TestCmdConserve_MissingSinceIsOperationalError pins the required-flag
// contract (exit 2, never a panic or a silent empty report).
func TestCmdConserve_MissingSinceIsOperationalError(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("a.txt", "x\n")
	repo.commitAll("c")
	storePath := filepath.Join(t.TempDir(), "conserve.sqlite")
	code, _, stderr := runInDir(t, repo.root, []string{"conserve", "--service", "svc", "--store", storePath})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr=%s", code, stderr)
	}
}

// TestCmdConserve_MovedFindingTextNeverContradicts pins the fix for the
// defect a review caught: a resurrection "moved" finding's own Detail
// text says "no retirement was declared" while its wire-level Declared
// value is DeclaredPrior ("prior") — the ordinary meaning of which,
// everywhere else in this tool, is "a declaration excusing this loss WAS
// present at Base." Printed on one line by the human-facing text
// renderer, those two clauses directly contradict each other. An ordinary
// undeclared rename must render with a phrase that actually matches what
// happened instead.
func TestCmdConserve_MovedFindingTextNeverContradicts(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	// >=5 statements: clone.FuncBodyHash's own floor for "means anything"
	// (design §3.3) — a 1-line stub body never reaches Resurrected at all.
	body := "\ta := x + 1\n\tb := a + 1\n\tc := b + 1\n\td := c + 1\n\treturn d\n"
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc Old(x int) int {\n"+body+"}\n")
	repo.commitAll("base commit")
	base := headSHAOf(t, repo.root)

	// An ordinary, undeclared rename: identical body under a new name, no
	// retires:/gone: row anywhere — nothing was ever claimed about it.
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc New(x int) int {\n"+body+"}\n")

	storePath := filepath.Join(t.TempDir(), "conserve2.sqlite")
	code, stdout, stderr := runInDir(t, repo.root, []string{
		"conserve", "--since", base, "--service", "svc", "--store", storePath,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "[moved]") {
		t.Fatalf("expected a [moved] resurrection finding in the text report:\n%s", stdout)
	}
	if strings.Contains(stdout, "no retirement was declared") && strings.Contains(stdout, "(declared: prior)") {
		t.Errorf("moved finding's text is self-contradictory — says no declaration was ever made, then labels it \"prior\" (which everywhere else means one WAS made at base):\n%s", stdout)
	}
}

// TestCmdConserve_NoPlanFlagStillReadsRetiresDeclarations pins the fix for
// O117 (.fathom/observations.md): `fathom conserve`'s real invocation shape
// — no --plan, exactly what lefthook.yml's refactor-conserve step runs at
// pre-push (CV-47) — must still discover docs/plans/*.md and read its
// retires:/surface_change: declarations, rather than silently seeing zero
// plans because discoverPlanPaths handed NewInput an absolute path where a
// repo-relative one was expected (gitShowFile's `git show ref:<abs>`
// always reports the path absent; worktreeFile's filepath.Join(root, abs)
// doubles the path). A genuine `retires: api:widgets.Old` row present at
// base, whose identical body then reappears under a new name at head,
// must report resurrection/body_resurrected (Declared: none — a real
// loss: the retirement claim was false), never resurrection/moved
// (Declared: prior — "no retirement was declared," the wrong reading when
// one plainly was declared).
func TestCmdConserve_NoPlanFlagStillReadsRetiresDeclarations(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	// >=5 statements: clone.FuncBodyHash's own floor (design §3.3) — a
	// 1-line stub body never reaches Resurrected at all.
	body := "\ta := x + 1\n\tb := a + 1\n\tc := b + 1\n\td := c + 1\n\treturn d\n"
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc Old(x int) int {\n"+body+"}\n")
	repo.writePlan("docs/plans/resurrect.md",
		"- [ ] Retire Old {id: RC-1, retires: api:widgets.Old}\n")
	repo.commitAll("base commit")
	base := headSHAOf(t, repo.root)

	// The body reappears under a new name at head — an agent's retirement
	// claim (declared above, present at base) proving false.
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc New(x int) int {\n"+body+"}\n")

	storePath := filepath.Join(t.TempDir(), "conserve3.sqlite")
	// Deliberately NO --plan: lefthook.yml's exact invocation shape
	// (`bin/fathom conserve --service fathom --since "$since" --store
	// workspace/db/fleet.sqlite`), the only real pre-push call.
	code, stdout, stderr := runInDir(t, repo.root, []string{
		"conserve", "--since", base, "--service", "svc", "--store", storePath,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "[body_resurrected]") {
		t.Errorf("expected a [body_resurrected] finding (retirement was claimed at base and the body reappeared anyway) with no --plan flag; got:\n%s", stdout)
	}
	if strings.Contains(stdout, "[moved]") {
		t.Errorf("resurrection reported [moved] (\"no retirement was declared\") even though docs/plans/resurrect.md declares retires: api:widgets.Old at base — the declaration was invisible to the default (no --plan) discovery path, exactly O117's bug:\n%s", stdout)
	}
}

func headSHAOf(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return string(bytes.TrimSpace(out.Bytes()))
}
