package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestActivePlanFiles_UnreadableIsNeverZeroPlans pins plan discovery's
// honesty (I38): *.md directly under docs/plans, README.md and archive/
// excluded; no docs/plans is no plans; but a docs/plans that cannot be read
// is an error — filepath.Glob, used before, dropped the read error, and
// the console's /plans said "0 plans across 0 repositories" while the
// external volume's reads were being refused.
func TestActivePlanFiles_UnreadableIsNeverZeroPlans(t *testing.T) {
	root := t.TempDir()
	if got, err := activePlanFiles(root); err != nil || len(got) != 0 {
		t.Fatalf("no docs/plans = %v, %v; want none and no error", got, err)
	}
	dir := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(filepath.Join(dir, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"b.md", "a.md", "README.md", "notes.txt", filepath.Join("archive", "old.md")} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := activePlanFiles(root)
	if err != nil || len(got) != 2 || got[0] != filepath.Join(dir, "a.md") || got[1] != filepath.Join(dir, "b.md") {
		t.Fatalf("active plans = %v, %v; want a.md, b.md", got, err)
	}

	if os.Getuid() == 0 {
		t.Skip("root reads a mode-000 directory")
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if got, err := activePlanFiles(root); err == nil {
		t.Fatalf("an unreadable docs/plans = %v with no error; want the read error", got)
	}
}
