package custodian

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// RT-134 (design §11.3): the custodian's config walk is capped in depth
// (6), entries (20,000) and time (a deadline 2 s under the hook timeout); a
// capped walk is never served as the corpus; a hook's audit refuses a root
// that is not a git work tree, naming it; FileChanged audits are debounced
// across sessions by a lease keyed on (root, file, mtime).
func TestWalkAgentConfigFilesCappedAndDeadlined(t *testing.T) {
	lim := walkLimits{MaxDepth: walkMaxDepth, MaxEntries: walkMaxEntries}

	// Within the caps: every CLAUDE.md/AGENTS.md, sorted.
	root := t.TempDir()
	for _, p := range []string{"CLAUDE.md", "a/AGENTS.md", "a/b/c/d/e/CLAUDE.md"} {
		writeFile(t, filepath.Join(root, p))
	}
	got, err := walkAgentConfigFiles(root, lim)
	if err != nil || strings.Join(got, ",") != "CLAUDE.md,a/AGENTS.md,a/b/c/d/e/CLAUDE.md" {
		t.Fatalf("walk: %v %v", got, err)
	}

	// Depth: a directory past 6 levels is not walked, and the walk says so.
	deep := t.TempDir()
	writeFile(t, filepath.Join(deep, "1/2/3/4/5/6/7/CLAUDE.md"))
	_, err = walkAgentConfigFiles(deep, lim)
	var capped *WalkCappedError
	if !errors.As(err, &capped) || !strings.Contains(capped.Reason, "deeper than 6") {
		t.Fatalf("depth cap: %v", err)
	}

	// Entries: past the cap the walk stops and names it.
	many := t.TempDir()
	for i := 0; i < 40; i++ {
		writeFile(t, filepath.Join(many, fmt.Sprintf("f%02d.txt", i)))
	}
	_, err = walkAgentConfigFiles(many, walkLimits{MaxDepth: 6, MaxEntries: 25})
	if !errors.As(err, &capped) || !strings.Contains(capped.Reason, "more than 25 entries") {
		t.Fatalf("entry cap: %v", err)
	}

	// Deadline: a walk whose deadline has passed stops.
	big := t.TempDir()
	for i := 0; i < 600; i++ {
		writeFile(t, filepath.Join(big, fmt.Sprintf("d%d/f.txt", i)))
	}
	past := time.Now().Add(-time.Second)
	_, err = walkAgentConfigFiles(big, walkLimits{MaxDepth: 6, MaxEntries: walkMaxEntries, Deadline: past})
	if !errors.As(err, &capped) || !strings.Contains(capped.Reason, "deadline") {
		t.Fatalf("deadline: %v", err)
	}
	// A capped corpus is unable-to-check, never a clean corpus.
	if !strings.Contains(err.Error(), "not checked") {
		t.Fatalf("the capped walk's error must say the corpus is not checked: %v", err)
	}

	// A hook's policy: a non-git root is refused and named.
	SetWalkPolicy(&WalkPolicy{Deadline: time.Now().Add(8 * time.Second), RefuseNonGit: true})
	t.Cleanup(func() { SetWalkPolicy(nil) })
	_, err = AgentConfigFiles(root)
	var ng *NotGitRootError
	if !errors.As(err, &ng) || !strings.Contains(err.Error(), root) {
		t.Fatalf("a hook refuses a non-git root by name: %v", err)
	}
	SetWalkPolicy(nil)
	if files, err := AgentConfigFiles(root); err != nil || len(files) != 3 {
		t.Fatalf("the CLI walks a non-git root, capped: %v %v", files, err)
	}

	// FileChanged debounce: one audit per (root, file, mtime) across sessions.
	leases := t.TempDir()
	now := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)
	mtime := now.Add(-time.Minute)
	if !DebounceFileChanged(leases, root, "CLAUDE.md", mtime, now) {
		t.Fatal("the first FileChanged audit runs")
	}
	if DebounceFileChanged(leases, root, "CLAUDE.md", mtime, now.Add(time.Second)) {
		t.Fatal("a second session's event for the same file and mtime is debounced")
	}
	if !DebounceFileChanged(leases, root, "CLAUDE.md", mtime.Add(time.Second), now.Add(2*time.Second)) {
		t.Fatal("a new mtime is a new change")
	}
	if !DebounceFileChanged(leases, root, "AGENTS.md", mtime, now) {
		t.Fatal("another file is its own lease")
	}
	ents, _ := os.ReadDir(leases)
	for _, e := range ents {
		old := time.Now().Add(-2 * FileChangedDebounceWindow)
		_ = os.Chtimes(filepath.Join(leases, e.Name()), old, old)
	}
	if !DebounceFileChanged(leases, root, "CLAUDE.md", mtime, time.Now()) {
		t.Fatal("an expired lease is taken again")
	}
}

func writeFile(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
