package certify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestGaps_UnreadableIntentCorpusIsNamedNeverDropped: a target source that
// exists and cannot be read (the external volume's EPERM wedge, a chmod)
// is named in the report's Unread list — never a quietly smaller
// target universe that a caller then reads as "declares none" (I38).
//
// certifies: invariant I38 (asserts)
func TestGaps_UnreadableIntentCorpusIsNamedNeverDropped(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "intent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	router := write("README.md", "# Project intent — router\n")
	inv := write("invariants.md", "# Invariants\n\n| Id | Statement | Enforced by | Source |\n|---|---|---|---|\n| I1 | The store is pure Go. | test `TestStorePurity` | ADR-020 |\n| I2 | Answers are deterministic. | gate `task validate` | ADR-001 |\n")
	inc := write("incidents.md", "# Incidents\n\n### INC-1 — a thing broke\n\nOutcome: pending.\n")

	full, err := Gaps(ctx, st, "svc", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if full.Summary.Declared != 3 || len(full.Unread) != 0 {
		t.Fatalf("readable corpus: declared %d, unreadable %v; want 3 targets and nothing unread", full.Summary.Declared, full.Unread)
	}

	for _, p := range []string{inv, inc} {
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	}
	if _, err := os.ReadFile(inv); err == nil {
		t.Skip("this platform still reads a chmod-000 file (running as root?)")
	}
	part, err := Gaps(ctx, st, "svc", root, nil)
	if err != nil {
		t.Fatalf("an unreadable corpus file is a named lower bound, not a failed call: %v", err)
	}
	joined := strings.Join(part.Unread, "; ")
	if !strings.Contains(joined, "invariants.md") || !strings.Contains(joined, "incidents.md") || !strings.Contains(strings.ToLower(joined), "permi") {
		t.Errorf("unreadable = %v; want both files named with the OS's reason", part.Unread)
	}
	if strings.Count(joined, "incidents.md") != 1 {
		t.Errorf("each unreadable file is named once: %v", part.Unread)
	}

	// The router itself unreadable: nothing of the corpus is read, and the
	// report says so rather than reporting a repository with no intent.
	if err := os.Chmod(router, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(router, 0o644) })
	blind, err := Gaps(ctx, st, "svc", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if blind.Summary.Declared != 0 || !strings.Contains(strings.Join(blind.Unread, "; "), "README.md") {
		t.Errorf("router unreadable: declared %d, unreadable %v", blind.Summary.Declared, blind.Unread)
	}
}
