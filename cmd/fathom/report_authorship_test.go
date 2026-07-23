package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// gitFixtureAuthorship builds a real, go-git-free git repo fixture at root
// with two commits: one carrying a "Co-Authored-By:" trailer (the AI-pairing
// convention this fleet's own CLAUDE.md documents), one without. Two Go
// source files each contribute one func with a branch (cyclomatic
// complexity 2), so the authorship report's complexity join has something
// non-trivial to aggregate. Real `git` is shelled out to (E2-1/E2-2's
// production path, internal/fusion.realGitLog, is not test-injectable at
// this CLI layer — see cmdIngest), matching the task's "go-git-free shell
// fixture" option.
func gitFixtureAuthorship(t *testing.T, root string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(
			os.Environ(),
			"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=alice@example.com",
			"GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=alice@example.com",
		)
		var out, errBuf bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v\nstdout: %s\nstderr: %s", args, err, out.String(), errBuf.String())
		}
	}
	run("init", "-q")
	run("config", "commit.gpgsign", "false")

	src1 := "package fixture\n\nfunc Foo(x int) int {\n\tif x > 0 {\n\t\treturn x\n\t}\n\treturn -x\n}\n"
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(src1), 0o644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	run("add", "a.go")
	run("commit", "-q", "-m", "Add foo\n\nLonger explanation.\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>")

	src2 := "package fixture\n\nfunc Bar(x int) int {\n\tif x > 0 {\n\t\treturn x\n\t}\n\treturn -x\n}\n"
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte(src2), 0o644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	run("add", "b.go")
	run("commit", "-q", "-m", "Add bar")
}

func TestReport_Authorship_WritesJoinDocument(t *testing.T) {
	root := t.TempDir()
	gitFixtureAuthorship(t, root)

	storePath := filepath.Join(t.TempDir(), "s.duckdb")
	outDir := filepath.Join(t.TempDir(), "findings")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "auth-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	var out, errBuf bytes.Buffer
	code := run([]string{
		"report", "--store", storePath, "--service", "auth-svc",
		"--analyzer", "authorship", "--out", outDir,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("report exit = %d, stderr: %s", code, errBuf.String())
	}

	outPath := filepath.Join(outDir, "auth-svc-authorship.md")
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s: %v", outPath, err)
	}
	doc := string(body)

	if !strings.Contains(doc, "authorship report — auth-svc") {
		t.Errorf("doc missing header: %s", doc)
	}
	if !strings.Contains(doc, "| commits (total) | 2 |") {
		t.Errorf("doc missing commits-total row (want 2): %s", doc)
	}
	if !strings.Contains(doc, "| commits co-authored (AI trailer) | 1 |") {
		t.Errorf("doc missing co-authored-commits row (want 1): %s", doc)
	}
	if !strings.Contains(doc, "| co-authored share | 50.0% |") {
		t.Errorf("doc missing 50%% share row: %s", doc)
	}
	if !strings.Contains(doc, "| authorship claims emitted | 1 |") {
		t.Errorf("doc missing claims-emitted row (want 1): %s", doc)
	}
	if !strings.Contains(doc, "avg cyclomatic complexity") {
		t.Errorf("doc missing complexity join: %s", doc)
	}

	// Determinism: re-running against an unchanged index (no re-ingest)
	// reproduces the identical bytes.
	var out2, errBuf2 bytes.Buffer
	code2 := run([]string{
		"report", "--store", storePath, "--service", "auth-svc",
		"--analyzer", "authorship", "--out", outDir,
	}, &out2, &errBuf2)
	if code2 != 0 {
		t.Fatalf("second report exit = %d, stderr: %s", code2, errBuf2.String())
	}
	body2, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s (2nd run): %v", outPath, err)
	}
	if !bytes.Equal(body, body2) {
		t.Errorf("report output not byte-identical across runs:\nfirst:\n%s\nsecond:\n%s", body, body2)
	}
}

// TestReport_Authorship_PersistsLedger asserts the claims ledger side
// effect: after running the report, the store's semantic_claims table holds
// a verified-tier "authorship" claim whose Rule is the exact trailer line —
// "evidence = the trailer line" per docs/plans/new-use-cases.md.
func TestReport_Authorship_PersistsLedger(t *testing.T) {
	root := t.TempDir()
	gitFixtureAuthorship(t, root)

	storePath := filepath.Join(t.TempDir(), "s.duckdb")
	outDir := filepath.Join(t.TempDir(), "findings")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "auth-svc2", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}
	var out, errBuf bytes.Buffer
	if code := run([]string{
		"report", "--store", storePath, "--service", "auth-svc2",
		"--analyzer", "authorship", "--out", outDir,
	}, &out, &errBuf); code != 0 {
		t.Fatalf("report exit = %d, stderr: %s", code, errBuf.String())
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	claims, err := st.ClaimsForService(context.Background(), "auth-svc2", "authorship")
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1 persisted authorship claim, got %d: %+v", len(claims), claims)
	}
	c := claims[0]
	if c.Tier != core.TierVerified {
		t.Errorf("persisted claim tier = %q, want %q", c.Tier, core.TierVerified)
	}
	if c.Rule != "Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>" {
		t.Errorf("persisted claim rule = %q, want the verbatim trailer line", c.Rule)
	}
	if c.Kind != "authorship" {
		t.Errorf("persisted claim kind = %q, want %q", c.Kind, "authorship")
	}
}
