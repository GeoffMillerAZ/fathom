package conserve

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewInput_NeverWritesTheIndex pins conserve as a read (console wave C
// review, defect 11): making the report — NewInput's `git status` and its
// working-tree diffs, then the changed-line reads the reading set makes —
// leaves the repository's .git/index exactly as it was, even when a
// tracked file is stat-dirty (the shape in which plain git refreshes the
// index and rewrites it under index.lock, failing a concurrent `git add`).
func TestNewInput_NeverWritesTheIndex(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("a.go", "package a\n")
	repo.write("b.go", "package a\n\nfunc B() {}\n")
	repo.commitAll("base")
	base := repo.headSHA()
	repo.write("a.go", "package a\n\nfunc A() {}\n")
	repo.commitAll("head")
	future := time.Now().Add(90 * time.Second)
	if err := os.Chtimes(filepath.Join(repo.root, "b.go"), future, future); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(repo.root, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	st0, _ := os.Stat(index)

	in, err := NewInput(context.Background(), repo.root, base, nil, "svc", nil)
	if err != nil {
		t.Fatalf("NewInput: %v", err)
	}
	if len(in.ChangedFiles) != 1 || in.ChangedFiles[0] != "a.go" {
		t.Errorf("changed files = %v, want [a.go] (b.go is stat-dirty, not changed)", in.ChangedFiles)
	}
	if ranges, err := gitChangedLines(context.Background(), repo.root, base, "a.go"); err != nil || len(ranges) == 0 {
		t.Errorf("changed lines of a.go = %v (%v)", ranges, err)
	}
	after, _ := os.ReadFile(index)
	st1, _ := os.Stat(index)
	if string(after) != string(before) || !st1.ModTime().Equal(st0.ModTime()) {
		t.Fatal("making a conservation input rewrote .git/index")
	}
}
