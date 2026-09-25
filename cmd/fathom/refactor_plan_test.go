package main

// refactor_plan_test.go — CV-33 (ADR-044 §4): "fathom refactor plan
// <symbol>" writes a gate-ready plan end to end: real `fathom ingest`
// against a tiny two-package fixture module, then `fathom refactor plan
// OldFunc`, asserting the generated Markdown carries the Baseline header,
// a ratcheted gone: pattern row scoped to the ONE referencing package, the
// final gone: callers row, and the closing conserve instruction — and that
// every row LINTS CLEAN and is individually contract-sound under
// internal/verify.EvaluateGone, even though no GoneEvidence CLI adapter is
// wired yet (a later row; EvaluateGone runs its own grammar check before
// ever consulting the Evidence, so this is real coverage, not a stub).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// fakeGoneEvidence satisfies verify.GoneEvidence with harmless zero values
// so EvaluateGone can run its FULL contract check (goneContractError,
// gated first inside EvaluateGone) on a generated row without a real index
// — this test only needs to know the contract is sound, never a verdict.
type fakeGoneEvidence struct{}

func (fakeGoneEvidence) IndexAtHead() (bool, string)   { return true, "" }
func (fakeGoneEvidence) FailedTypedPackages() []string { return nil }
func (fakeGoneEvidence) GoFiles() []verify.GoFile      { return nil }
func (fakeGoneEvidence) ModuleFile() ([]byte, bool)    { return nil, false }
func (fakeGoneEvidence) CorpusFiles(paths []string) []verify.CorpusFile {
	return nil
}
func (fakeGoneEvidence) TypedCallers(symbol, pkg string) (nonTest, test []verify.GoneReferent) {
	return nil, nil
}

// newRefactorFixtureRepo builds a throwaway two-package Go module: oldpkg
// declares OldFunc, caller imports it and calls it once — a genuine
// cross-package typed reference, so the generated plan's per-package
// ratchet is scoped to a DIFFERENT directory than the target's own.
func newRefactorFixtureRepo(t *testing.T) *selfSkipGitRepo {
	t.Helper()
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module refactorfixture\n\ngo 1.21\n")
	repo.writePlan("oldpkg/old.go", "package oldpkg\n\nfunc OldFunc() {}\n")
	repo.writePlan("caller/user.go",
		"package caller\n\nimport \"refactorfixture/oldpkg\"\n\nfunc CallsOld() {\n\toldpkg.OldFunc()\n}\n")
	repo.commitAll("initial: oldpkg + one caller")
	return repo
}

func TestCmdRefactorPlan_GateReadyPlan(t *testing.T) {
	repo := newRefactorFixtureRepo(t)
	headSHA := headSHAOf(t, repo.root)

	storePath := filepath.Join(t.TempDir(), "refactor.sqlite")
	var ing bytes.Buffer
	if code := run([]string{
		"ingest", "--service", "refactorfixture", "--path", repo.root, "--store", storePath,
	}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	outFile := filepath.Join(t.TempDir(), "plan.md")
	code, out, errBuf := runInDir(t, repo.root, []string{
		"refactor", "plan", "OldFunc",
		"--service", "refactorfixture", "--store", storePath, "--out", outFile,
	})
	if code != 0 {
		t.Fatalf("refactor plan failed (%d)\nstdout=%s\nstderr=%s", code, out, errBuf)
	}

	body, rerr := os.ReadFile(outFile)
	if rerr != nil {
		t.Fatalf("read generated plan: %v", rerr)
	}
	doc := string(body)
	t.Logf("generated plan:\n%s", doc)

	if !strings.Contains(doc, "Baseline: "+headSHA) {
		t.Errorf("missing \"Baseline: %s\" header:\n%s", headSHA, doc)
	}
	if !strings.Contains(doc, `paths: "caller/"`) {
		t.Errorf("missing a ratchet row scoped to paths: \"caller/\":\n%s", doc)
	}
	if !strings.Contains(doc, "max: 1") {
		t.Errorf("expected today's count (one reference from caller/) as max: 1:\n%s", doc)
	}
	if !strings.Contains(doc, "gone: callers OldFunc") {
		t.Errorf("missing the final gone: callers row:\n%s", doc)
	}
	if !strings.Contains(doc, "package: oldpkg") {
		t.Errorf("final row should disambiguate via the target's own package:\n%s", doc)
	}
	if !strings.Contains(doc, "fathom conserve --since "+headSHA+" --strict") {
		t.Errorf("missing the closing conserve instruction naming the baseline:\n%s", doc)
	}

	// Lints clean: unique ids, no suspect annotation on any row.
	plan := verify.ParseMarkdownPlan("refactorfixture", body)
	if len(plan.Tasks) == 0 {
		t.Fatalf("generated plan parsed to zero rows:\n%s", doc)
	}
	if warnings := verify.LintPlan(plan); len(warnings) != 0 {
		t.Errorf("LintPlan warnings on the generated plan: %v", warnings)
	}
	seen := map[string]bool{}
	for _, tk := range plan.Tasks {
		if tk.AnnotationSuspect != "" {
			t.Errorf("row %s: suspect annotation %q", tk.ID, tk.AnnotationSuspect)
		}
		if seen[tk.ID] {
			t.Errorf("duplicate row id %q", tk.ID)
		}
		seen[tk.ID] = true
	}

	// Every gone: row is individually contract-sound: EvaluateGone's own
	// grammar check (goneContractError) runs before it ever touches the
	// Evidence, so a fake, empty-answering Evidence proves this without
	// needing a real index — the verdict itself (Unknown, since the CLI's
	// own GoneEvidence adapter is a later row) is not what this asserts.
	sawGone := false
	for _, tk := range plan.Tasks {
		if tk.Gone == "" {
			continue
		}
		sawGone = true
		_, label := verify.EvaluateGone(tk, fakeGoneEvidence{})
		if label == "contract-invalid" {
			t.Errorf("row %s: gone: %q is contract-invalid", tk.ID, tk.Gone)
		}
	}
	if !sawGone {
		t.Errorf("expected at least one gone: row in the generated plan")
	}
}

// TestCmdRefactorPlan_UnreadableSourceFailsLoud pins the fix for the
// defect a review caught: when c.Sources() cannot read the corpus (a
// locked directory under the repo root), buildRefactorPlan must NOT
// silently seed every ratchet's max: at 0 (an all-zero baseline the SAME
// command's own typed-caller-derived package list already contradicts —
// `fathom refactor status`'s first run would then see the true nonzero
// count against a max: 0 baseline the plan never actually measured, and
// fail immediately). It must refuse to write a plan at all, naming what
// could not be read.
func TestCmdRefactorPlan_UnreadableSourceFailsLoud(t *testing.T) {
	repo := newRefactorFixtureRepo(t)

	// Ingest BEFORE the directory goes unreadable, exactly like the
	// verifier's repro: the store's typed call graph (byPkg/CallersDetail)
	// is built from a fully-readable corpus, so it genuinely proves
	// caller/ references OldFunc. Only AFTERWARD does refactor plan's own
	// separate, live c.Sources() re-read of the corpus hit a directory
	// that has since gone unreadable — a transient/partial failure that
	// must never silently disagree with what the typed graph already
	// proved.
	storePath := filepath.Join(t.TempDir(), "refactor3.sqlite")
	var ing bytes.Buffer
	if code := run([]string{
		"ingest", "--service", "refactorfixture", "--path", repo.root, "--store", storePath,
	}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	locked := filepath.Join(repo.root, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	code, stdout, stderr := runInDir(t, repo.root, []string{
		"refactor", "plan", "OldFunc",
		"--service", "refactorfixture", "--store", storePath,
	})
	if code == 0 {
		t.Fatalf("expected a nonzero exit when the corpus could not be fully read, got 0\nstdout=%s", stdout)
	}
	if strings.Contains(stdout, "max: 0") {
		t.Fatalf("must never seed a ratchet's max: at 0 from an unread corpus — a package the same run's own typed callers prove has a live reference:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	if stderr == "" {
		t.Fatalf("expected stderr to name what could not be read")
	}
}

// TestCmdRefactorPlan_RefusesDirtyWorkingTree pins the fix for the defect
// a review caught: `Baseline:` is git rev-parse HEAD (a committed sha),
// but max: was seeded from c.Sources(), which reads the LIVE WORKING
// TREE — so an uncommitted edit at plan-creation time makes max: disagree
// with the count `fathom refactor status` later computes for that exact
// same Baseline sha (git show), from the very first status check. Adding
// a second, uncommitted reference to OldFunc must refuse plan generation
// rather than silently seed a max: that already excludes it.
func TestCmdRefactorPlan_RefusesDirtyWorkingTree(t *testing.T) {
	repo := newRefactorFixtureRepo(t)

	storePath := filepath.Join(t.TempDir(), "refactor4.sqlite")
	var ing bytes.Buffer
	if code := run([]string{
		"ingest", "--service", "refactorfixture", "--path", repo.root, "--store", storePath,
	}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	// An UNCOMMITTED edit: a second call site, never committed, so HEAD
	// (and thus Baseline:) still shows only the original one reference.
	repo.writePlan("caller/user.go",
		"package caller\n\nimport \"refactorfixture/oldpkg\"\n\nfunc CallsOld() {\n\toldpkg.OldFunc()\n\toldpkg.OldFunc()\n}\n")

	code, stdout, stderr := runInDir(t, repo.root, []string{
		"refactor", "plan", "OldFunc",
		"--service", "refactorfixture", "--store", storePath,
	})
	if code == 0 {
		t.Fatalf("expected refactor plan to refuse a dirty working tree, got exit 0\nstdout=%s", stdout)
	}
	if stderr == "" {
		t.Fatalf("expected stderr to name the dirty working tree as the reason")
	}
}

// TestCmdRefactorPlan_ImportTarget pins the import/package shape: the same
// fixture, targeted by its import path directly rather than a symbol name.
func TestCmdRefactorPlan_ImportTarget(t *testing.T) {
	repo := newRefactorFixtureRepo(t)
	headSHA := headSHAOf(t, repo.root)

	storePath := filepath.Join(t.TempDir(), "refactor2.sqlite")
	var ing bytes.Buffer
	if code := run([]string{
		"ingest", "--service", "refactorfixture", "--path", repo.root, "--store", storePath,
	}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	code, stdout, stderr := runInDir(t, repo.root, []string{
		"refactor", "plan", "refactorfixture/oldpkg",
		"--service", "refactorfixture", "--store", storePath,
	})
	if code != 0 {
		t.Fatalf("refactor plan (import) failed (%d)\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}

	if !strings.Contains(stdout, "Baseline: "+headSHA) {
		t.Errorf("missing Baseline header:\n%s", stdout)
	}
	if !strings.Contains(stdout, `gone: pattern "\x22refactorfixture/oldpkg\x22"`) {
		t.Errorf("missing the \\x22-escaped import-path ratchet pattern:\n%s", stdout)
	}
	if !strings.Contains(stdout, `paths: "caller/"`) {
		t.Errorf("missing a ratchet row scoped to caller/:\n%s", stdout)
	}
	if !strings.Contains(stdout, "gone: import refactorfixture/oldpkg") {
		t.Errorf("missing the final gone: import row:\n%s", stdout)
	}

	plan := verify.ParseMarkdownPlan("refactorfixture", []byte(stdout))
	for _, tk := range plan.Tasks {
		if tk.AnnotationSuspect != "" {
			t.Errorf("row %s: suspect annotation %q", tk.ID, tk.AnnotationSuspect)
		}
		if tk.Gone == "" {
			continue
		}
		_, label := verify.EvaluateGone(tk, fakeGoneEvidence{})
		if label == "contract-invalid" {
			t.Errorf("row %s: gone: %q is contract-invalid", tk.ID, tk.Gone)
		}
	}
}
