package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestConsoleSearchFindsFuturesInFamily (AG-37, K8): futures.md split
// behind a router still answers a search for a future — each future once,
// at the member line that defines it, never at the router's listing row —
// and a member directory that cannot be read is named, never read as no
// futures (I38).
func TestConsoleSearchFindsFuturesInFamily(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "docs", "intent", "futures.md"),
		"# Future directions\n\n| Id | Future | Member |\n|---|---|---|\n"+
			"| F1 | CGO-free binary | `futures/f01-f02.md` |\n| F2 | Public release | `futures/f01-f02.md` |\n")
	mustWrite(t, filepath.Join(root, "docs", "intent", "futures", "f01-f02.md"),
		"# Future directions: F1 and F2\n\n## F1. CGO-free binary\n\nbody\n\n## F2. Public release\n\nbody\n")
	mustWrite(t, filepath.Join(root, "docs", "intent", "invariants.md"),
		"# Invariants\n\n| I1 | The store is pure Go. | `stated` |\n")
	docs, err := projectSearchDocs("svc", root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	count := map[string]int{}
	for _, d := range docs {
		if d.Kind == "future" || d.Kind == "invariant" {
			got[d.ID] = d.Ref + " " + d.Title
			count[d.ID]++
		}
	}
	want := map[string]string{
		"F1": "docs/intent/futures/f01-f02.md:3 F1 CGO-free binary",
		"F2": "docs/intent/futures/f01-f02.md:7 F2 Public release",
		"I1": "docs/intent/invariants.md:3 I1 The store is pure Go.",
	}
	for id, w := range want {
		if got[id] != w || count[id] != 1 {
			t.Errorf("%s = %q (%d result(s)), want %q once", id, got[id], count[id], w)
		}
	}

	members := filepath.Join(root, "docs", "intent", "futures")
	if err := os.Chmod(members, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(members, 0o755) })
	if _, err := os.ReadDir(members); err == nil {
		t.Skip("running with privileges that read a mode-000 directory")
	}
	if _, err := projectSearchDocs("svc", root); err == nil || !strings.Contains(err.Error(), "docs/intent/futures/") {
		t.Errorf("err = %v, want the unreadable member directory named", err)
	}
}

// TestGoalsForKeyFollowsScopeFamily (AG-37): the progress page caches a
// root's goals while scope.md and HEAD are unchanged — and with scope.md a
// router, a goal edited in a member must change the key, or the page keeps
// serving the goals of before the edit.
func TestGoalsForKeyFollowsScopeFamily(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "docs", "intent", "README.md"), "# Project intent — router\n")
	mustWrite(t, filepath.Join(root, "docs", "intent", "scope.md"),
		"# Scope\n\n## Current goals\n\n**G2. Two.** `scope/goals.md`\n")
	member := filepath.Join(root, "docs", "intent", "scope", "goals.md")
	mustWrite(t, member, "# Goals\n\n**G2. Two.** Done when: it lands.\n")
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	ids := func() string {
		var out []string
		for _, b := range goalsFor(root, now).bets {
			out = append(out, b.Goal.ID)
		}
		return strings.Join(out, ",")
	}
	if got := ids(); got != "G2" {
		t.Fatalf("goals = %q, want G2 from the member (the router's listing is not a goal)", got)
	}
	mustWrite(t, member, "# Goals\n\n**G2. Two.** Done when: it lands.\n\n**G3. Three.** Done when: it lands too.\n")
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(member, later, later); err != nil {
		t.Fatal(err)
	}
	if got := ids(); got != "G2,G3" {
		t.Errorf("after a member edit goals = %q, want G2,G3: the cache key must cover every scope family file", got)
	}
}
