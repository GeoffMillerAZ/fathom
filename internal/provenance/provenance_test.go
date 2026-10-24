package provenance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func testGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func testRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.name", "Test Operator")
	testGit(t, root, "config", "user.email", "operator@example.invalid")
	testGit(t, root, "config", "commit.gpgsign", "false")
	testGit(t, root, "config", "core.hooksPath", "/dev/null")
	testGit(t, root, "commit", "--allow-empty", "-qm", "fixture")
	return root
}

func TestProvenanceNoteWritesNote(t *testing.T) {
	root := testRepo(t)
	head := testGit(t, root, "rev-parse", "HEAD")
	commit := testGit(t, root, "cat-file", "commit", head)
	want := Fields{Harness: "test-harness", Model: "test-model", HostLabel: "operator-label", Session: "test-session", Lane: "test-lane"}
	for _, ref := range []string{head, "HEAD", ""} {
		sha, err := ProvenanceNote(context.Background(), root, ref, want)
		if err != nil {
			t.Fatal(err)
		}
		if sha != head {
			t.Fatalf("resolved commit = %q, want %q", sha, head)
		}
		raw := testGit(t, root, "notes", "--ref=fathom/provenance", "show", head)
		var got Fields
		if err := json.Unmarshal([]byte(raw), &got); err != nil || got != want {
			t.Fatalf("note = %s, decode error %v, want %+v", raw, err, want)
		}
		want.Model = "replacement-model"
	}
	if got := testGit(t, root, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD changed: %s", got)
	}
	if got := testGit(t, root, "cat-file", "commit", head); got != commit {
		t.Fatal("target commit changed")
	}
	if got := testGit(t, root, "rev-list", "--count", "HEAD"); got != "1" {
		t.Fatalf("target history grew: %s", got)
	}
	if got := testGit(t, root, "for-each-ref", "--format=%(refname)", "refs/notes"); got != "refs/notes/fathom/provenance" {
		t.Fatalf("unexpected note refs: %s", got)
	}
}

func TestProvenanceNoteUnknownHost(t *testing.T) {
	root := testRepo(t)
	t.Setenv("HOSTNAME", "must-not-be-recorded")
	t.Setenv("HOST", "must-not-be-recorded")
	if _, err := ProvenanceNote(context.Background(), root, "HEAD", Fields{}); err != nil {
		t.Fatal(err)
	}
	raw := testGit(t, root, "notes", "--ref=fathom/provenance", "show", "HEAD")
	want := `{"harness":"unknown","model":"unknown","host_label":"unknown","session":"unknown","lane":"unknown"}`
	if raw != want {
		t.Fatalf("note = %s, want %s", raw, want)
	}
}

func TestProvenanceNoteRejectsInvalidCommit(t *testing.T) {
	for _, ref := range []string{"missing", "--all", "HEAD^{tree}"} {
		t.Run(ref, func(t *testing.T) {
			root := testRepo(t)
			if _, err := ProvenanceNote(context.Background(), root, ref, Fields{}); err == nil {
				t.Fatal("invalid commit accepted")
			}
			if got := testGit(t, root, "for-each-ref", "refs/notes"); got != "" {
				t.Fatalf("invalid commit wrote note: %s", got)
			}
		})
	}
}

func TestProvenanceNoteOutsideRepository(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	_, err := ProvenanceNote(context.Background(), t.TempDir(), "HEAD", Fields{})
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("error = %v, want git diagnostic", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error does not wrap git exit error: %v", err)
	}
}
