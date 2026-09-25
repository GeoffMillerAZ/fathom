package main

// verify_gone_evidence_test.go — CV-M6 tooling fix (docs/bench/readings/
// cv-m6-dogfood-2026-09-24.md): `fathom verify --store` had no
// verify.GoneEvidence adapter — cliEvidence (cmd/fathom/genstatuscue.go)
// implemented Evidence/DetailedEvidence/FileEvidence/CertificationEvidence
// but not GoneEvidence, so EVERY `gone:` row read Unknown
// ("no gone evidence available on this evidence source") forever, no
// matter how true the claim. refactor_plan_test.go's own fakeGoneEvidence
// comment named this "a later row" — discovered dogfooding CV-M6
// (docs/plans/retire-unused-analyzers.md, ADR-044): a plan built entirely
// of `gone: import` rows against packages with zero importers could never
// be flipped to [x], because nothing in the CLI path could prove any of
// them.

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// newGoneFixtureRepo builds a throwaway module: keptpkg is imported by
// caller/, driftpkg is imported by nobody — the shape a `gone: import` row
// needs to prove true (and to correctly refuse to prove for keptpkg).
func newGoneFixtureRepo(t *testing.T) *selfSkipGitRepo {
	t.Helper()
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module gonefixture\n\ngo 1.21\n")
	repo.writePlan("keptpkg/kept.go", "package keptpkg\n\nfunc Kept() {}\n")
	repo.writePlan("driftpkg/drift.go", "package driftpkg\n\nfunc Drift() {}\n")
	repo.writePlan("caller/user.go",
		"package caller\n\nimport \"gonefixture/keptpkg\"\n\nfunc CallsKept() {\n\tkeptpkg.Kept()\n}\n")
	return repo
}

func ingestGoneFixture(t *testing.T, repo *selfSkipGitRepo) string {
	t.Helper()
	storePath := filepath.Join(t.TempDir(), "gone.sqlite")
	var ing bytes.Buffer
	if code := run([]string{
		"ingest", "--service", "gonefixture", "--path", repo.root, "--store", storePath,
	}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	return storePath
}

// TestCliEvidence_GoneImport_ProvesAbsence: a `gone: import` row for a
// package genuinely imported nowhere passes real `fathom verify --store`
// once the working tree matches the ingested commit exactly (the ordinary,
// hermetic case: commit, then ingest, then verify).
func TestCliEvidence_GoneImport_ProvesAbsence(t *testing.T) {
	repo := newGoneFixtureRepo(t)
	pf := repo.writePlan("docs/plans/gone.md",
		"- [x] driftpkg is imported nowhere at head {id: GN-1, gone: import gonefixture/driftpkg}\n")
	repo.commitAll("initial: keptpkg (used), driftpkg (unused), and the plan")

	storePath := ingestGoneFixture(t, repo)

	// --strict: without it an Unknown claimed row never fails the gate and
	// never prints its criteria, so this assertion would pass whether or
	// not GoneEvidence is actually wired — --strict is what makes
	// "proven" and "silently Unknown" observably different exit codes.
	code, stdout, stderr := runInDir(t, repo.root, []string{
		"verify", "--service", "gonefixture", "--plan", pf, "--store", storePath, "--strict",
	})
	if code != 0 {
		t.Fatalf("want exit 0 (row proven), got %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if strings.Contains(stdout, "no gone evidence available") {
		t.Fatalf("still reading gone-unevaluated — the CLI adapter is not wired:\n%s", stdout)
	}
}

// TestCliEvidence_GoneImport_NeverFalsePassOnRealImporter: the same
// fixture, but the row targets keptpkg (genuinely imported by caller/) —
// the CLI must FAIL it, never silently pass a real reference, and must
// name the real importer in the failure detail.
func TestCliEvidence_GoneImport_NeverFalsePassOnRealImporter(t *testing.T) {
	repo := newGoneFixtureRepo(t)
	pf := repo.writePlan("docs/plans/gone2.md",
		"- [x] keptpkg is imported nowhere at head {id: GN-2, gone: import gonefixture/keptpkg}\n")
	repo.commitAll("initial: keptpkg (used), driftpkg (unused), and the plan")

	storePath := ingestGoneFixture(t, repo)

	code, stdout, stderr := runInDir(t, repo.root, []string{
		"verify", "--service", "gonefixture", "--plan", pf, "--store", storePath,
	})
	if code != 1 {
		t.Fatalf("want exit 1 (row disproven — caller/ still imports it), got %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "still imports") {
		t.Errorf("expected the real importer to be named in the failure detail:\n%s", stdout)
	}
}

// TestCliEvidence_GoneImport_UnknownOnDirtyTree (I44): an uncommitted edit
// since the last ingest must never let a gone: row read Pass — the index
// no longer provably reflects the working tree the plan is being checked
// against, so the row must stay Unknown, never a fabricated proof.
func TestCliEvidence_GoneImport_UnknownOnDirtyTree(t *testing.T) {
	repo := newGoneFixtureRepo(t)
	pf := repo.writePlan("docs/plans/gone3.md",
		"- [x] driftpkg is imported nowhere at head {id: GN-3, gone: import gonefixture/driftpkg}\n")
	repo.commitAll("initial: keptpkg (used), driftpkg (unused), and the plan")

	storePath := ingestGoneFixture(t, repo)

	// An uncommitted, unrelated edit after ingest — the tree no longer
	// matches the commit the store's typed pass was computed against.
	repo.writePlan("caller/scratch.go", "package caller\n\nfunc Scratch() {}\n")

	code, stdout, stderr := runInDir(t, repo.root, []string{
		"verify", "--service", "gonefixture", "--plan", pf, "--store", storePath, "--strict",
	})
	if code == 0 {
		t.Fatalf("want a non-zero exit under --strict on a dirty tree (unknown, never pass), got 0\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "not proven") {
		t.Errorf("expected an honest \"not proven\" reason in the report, got:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
}
