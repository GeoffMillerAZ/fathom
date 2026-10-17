package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// IV-3c (ADR-074 decision 6): an invariant no sensitive-surface row names
// is repo-wide, so no generated guard entry carries it and no edit ever
// sees it. It is listed ONCE, at session start, inside the whole-primer
// cap; an invariant a surface names stays out of that list (the edit
// carries it).
func TestComposePrimer_NamesRepoWideInvariantsOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_THREADS", filepath.Join(t.TempDir(), "threads.jsonl"))
	root := t.TempDir()
	writeCorpusFixture(t, root) // I1 named by the internal/store/** row; I2 named by none
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

	primer, _, _ := composePrimer("identity", root, now, ledgerroots.Roots{})
	line := ""
	for _, l := range strings.Split(primer, "\n") {
		if strings.Contains(l, "repo-wide invariants") {
			if line != "" {
				t.Fatalf("repo-wide invariants listed twice:\n%s", primer)
			}
			line = l
		}
	}
	if line == "" {
		t.Fatalf("the primer must list the repo-wide invariants:\n%s", primer)
	}
	if !strings.Contains(line, "I2") {
		t.Fatalf("I2 is named by no surface row, so it is repo-wide: %q", line)
	}
	if strings.Contains(line, "I1") {
		t.Fatalf("I1 is named by a surface row, so the edit carries it, not the primer: %q", line)
	}

	// Never at an edit: no generated guard entry carries a repo-wide id.
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range intentGuardEntries(c) {
		if strings.Contains(e.Note, "I2 ") {
			t.Fatalf("generated entry %v carries repo-wide I2: %q", e.Scopes, e.Note)
		}
	}

	// A repo with no intent corpus gets no line at all.
	bare, _, _ := composePrimer("identity", t.TempDir(), now, ledgerroots.Roots{})
	if strings.Contains(bare, "repo-wide invariants") {
		t.Fatalf("no corpus, no line:\n%s", bare)
	}
}

// The whole-primer cap (I12) holds with the line present: a brief far over
// budget is cut by whole lines, and the repo-wide line is never cut.
func TestComposePrimer_RepoWideLineFitsTheWholePrimerCap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeCorpusFixture(t, root)
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	threads := filepath.Join(t.TempDir(), "threads.jsonl")
	t.Setenv("FATHOM_THREADS", threads)
	f, err := os.Create(threads)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for i := 0; i < 40; i++ {
		_ = enc.Encode(map[string]any{
			"schema_version": 1, "id": fmt.Sprintf("TH-%02d", i), "thread": "handoff", "kind": "note",
			"text": strings.Repeat("a long handoff note that eats budget ", 4), "filed_at": now.Format(time.RFC3339),
			"pointers": map[string]string{"repo_root": root},
		})
	}
	f.Close()
	identity := "identity " + strings.Repeat("x", 900)
	primer, _, _ := composePrimer(identity, root, now, ledgerroots.Roots{})
	if len(primer) > sessionPrimerBudget {
		t.Fatalf("primer is %d B, over the %d B cap", len(primer), sessionPrimerBudget)
	}
	if !strings.Contains(primer, "repo-wide invariants") {
		t.Fatalf("the brief absorbs the cut, never the repo-wide line:\n%s", primer)
	}
}

// The line is bounded: a corpus with hundreds of unrouted invariants names
// the first ones and counts the rest, never silently.
func TestRepoWideInvariantLine_BoundedWithCount(t *testing.T) {
	var ids []string
	for i := 1; i <= 400; i++ {
		ids = append(ids, fmt.Sprintf("I%d", i))
	}
	line := renderRepoWideLine(ids)
	if len(line) > repoWideLineMaxBytes {
		t.Fatalf("line is %d B, over its %d B bound", len(line), repoWideLineMaxBytes)
	}
	if !strings.Contains(line, "more") || !strings.Contains(line, "I1, ") {
		t.Fatalf("a bounded line names its own cut: %q", line)
	}
}
