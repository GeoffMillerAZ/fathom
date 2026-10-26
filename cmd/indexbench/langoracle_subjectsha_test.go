package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSubjectSHA (round 3 A1, F1): a subject's sha is its own repository's
// HEAD, never that of a repository it sits inside (a scratch copy whose .git
// was removed walks up into the fathom checkout): an enclosing repository's
// HEAD is refused, and the driver's -subject-sha names the clone's commit.
func TestSubjectSHA(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	outer := t.TempDir()
	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	run(outer, "init", "-q")
	if err := os.WriteFile(filepath.Join(outer, "f"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(outer, "add", "f")
	run(outer, "commit", "-q", "-m", "x")
	head := run(outer, "rev-parse", "HEAD")
	inner := filepath.Join(outer, "work", "subject")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := subjectSHA(ctx, outer, ""); err != nil || got != head {
		t.Errorf("subject at its repository's root: sha %q, %v; want %q", got, err, head)
	}
	if got, err := subjectSHA(ctx, inner, ""); err != nil || got != "" {
		t.Errorf("subject inside another repository: sha %q, %v; want none (the enclosing HEAD is not the subject's)", got, err)
	}
	pinned := "cab7c66b4fcf35b09ea8fbc4c3c1a9272e92d997"
	if got, err := subjectSHA(ctx, inner, pinned); err != nil || got != pinned {
		t.Errorf("-subject-sha: sha %q, %v; want %q", got, err, pinned)
	}
	if _, err := subjectSHA(ctx, inner, "v12.9"); err == nil {
		t.Error("-subject-sha v12.9: want an error (a full 40-hex commit is required)")
	}
}

// TestCmdLangOracleRequiresSubjectSHA (round 4 C4-R7): a reading that names no
// commit of its subject is refused unless -subject-sha names one: a scratch
// copy with no .git, or a directory inside another repository, has none of
// its own.
func TestCmdLangOracleRequiresSubjectSHA(t *testing.T) {
	allowUnpinnedSubject = false
	t.Cleanup(func() { allowUnpinnedSubject = true })
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(metricTiersSubject(t, "A"), &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "-subject-sha") {
		t.Errorf("a subject with no commit: exit %d, want a refusal naming -subject-sha: %s", code, stderr.String())
	}
	stderr.Reset()
	if code := cmdLangOracle(metricTiersSubject(t, "A", "-subject-sha", "cab7c66b4fcf35b09ea8fbc4c3c1a9272e92d997"), &stdout, &stderr); code != 0 {
		t.Errorf("-subject-sha given: exit %d: %s", code, stderr.String())
	}
}
