package gitro

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// indexState is what a write to the index changes: its bytes and its
// modification time.
func indexState(t *testing.T, dir string) (string, time.Time) {
	t.Helper()
	p := filepath.Join(dir, ".git", "index")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), st.ModTime()
}

// stale makes f stat-dirty (its mtime moves, its content does not): the
// shape in which `git status` and `git diff` refresh the index and write it
// back.
func stale(t *testing.T, dir, f string) {
	t.Helper()
	future := time.Now().Add(time.Duration(time.Now().UnixNano()%1000+60) * time.Second)
	if err := os.Chtimes(filepath.Join(dir, f), future, future); err != nil {
		t.Fatal(err)
	}
}

// TestOutput_NeverWritesTheIndex pins the package's one promise: status and
// a diff against the working tree answer exactly as git does, while the
// repository's own .git/index is never rewritten — the write a console page
// view made in the live checkout (wave C review, defect 11). The control
// proves the fixture would catch a write: plain `git status` rewrites it.
func TestOutput_NeverWritesTheIndex(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "one")
	ctx := context.Background()

	stale(t, dir, "f.go")
	before, beforeAt := indexState(t, dir)
	for _, args := range [][]string{
		{"status", "--porcelain"},
		{"diff", "--no-renames", "--relative", "--name-only", "HEAD"},
		{"diff", "--no-ext-diff", "--no-color", "--no-renames", "-U0", "HEAD", "--", "f.go"},
		{"diff", "--name-only", "HEAD", "--", "f.go"},
	} {
		out, err := Output(ctx, dir, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if strings.TrimSpace(string(out)) != "" {
			t.Errorf("%v on an unchanged tree = %q, want nothing", args, out)
		}
	}
	after, afterAt := indexState(t, dir)
	if after != before || !afterAt.Equal(beforeAt) {
		t.Fatal("a read-only git command rewrote .git/index")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "index.lock")); !os.IsNotExist(err) {
		t.Errorf("index.lock left behind: %v", err)
	}

	// A real change is still seen.
	if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n\nvar x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := Output(ctx, dir, "diff", "--no-ext-diff", "--no-color", "-U0", "HEAD", "--", "f.go")
	if err != nil || !bytes.Contains(out, []byte("@@ -1,0 +2,2 @@")) {
		t.Errorf("a real change = %q (%v)", out, err)
	}
	if out, _ := Output(ctx, dir, "status", "--porcelain"); !strings.Contains(string(out), "f.go") {
		t.Errorf("status with a change = %q", out)
	}
	if after2, _ := indexState(t, dir); after2 != before {
		t.Fatal("reading a real change rewrote .git/index")
	}

	// The control: plain git refreshes and writes the index.
	if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale(t, dir, "f.go")
	git(t, dir, "status", "--porcelain")
	if control, _ := indexState(t, dir); control == before {
		t.Skip("this git did not rewrite the index on a stat-dirty status; the control cannot prove the fixture")
	}
}

// A repository with no index yet (nothing ever added) reads as git does.
func TestOutput_NoIndexYet(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := Output(context.Background(), dir, "status", "--porcelain")
	if err != nil || !strings.Contains(string(out), "?? new.go") {
		t.Errorf("status with no index = %q (%v)", out, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "index")); !os.IsNotExist(err) {
		t.Errorf("a read created .git/index: %v", err)
	}
}

// TestOutput_RacilyCleanEntryIsStillSeen pins the copy's timestamp: git
// trusts an index entry's stat only when the entry is older than the index
// file itself ("racy git"); an edit that keeps the size and lands in the
// index's own second is caught only by re-reading the content. A copy
// stamped "now" made every such entry look settled, and a same-size edit
// right after a commit read as no change (TestReview_ConservationDimension
// WiredEndToEnd failed under load). The copy keeps the index's mtime.
func TestOutput_RacilyCleanEntryIsStillSeen(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "core.trustctime", "false") // the edit's ctime cannot be set back; mtime and size decide
	f := filepath.Join(dir, "w.go")
	then := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.WriteFile(f, []byte("func Old() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(f, then, then); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "base")
	// The index was written in the entry's own second, and the file then
	// changed without changing size or mtime.
	if err := os.Chtimes(filepath.Join(dir, ".git", "index"), then, then); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("func New() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(f, then, then); err != nil {
		t.Fatal(err)
	}
	out, err := Output(context.Background(), dir, "diff", "--name-only", "HEAD")
	if err != nil || strings.TrimSpace(string(out)) != "w.go" {
		t.Errorf("a same-size edit in the index's own second = %q (%v), want w.go", out, err)
	}
	if out, _ := Output(context.Background(), dir, "status", "--porcelain"); !strings.Contains(string(out), "w.go") {
		t.Errorf("status = %q, want w.go modified", out)
	}
}
