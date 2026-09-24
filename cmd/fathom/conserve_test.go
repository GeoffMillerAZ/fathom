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
