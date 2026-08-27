package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RC-7: "go.mod declares another module" and "go.mod could not be read" are
// different facts, and only one of them is about the repository.
//
// Observed before this fix: the check reported "<root>'s go.mod does not
// declare github.com/geoffmilleraz/fathom" against the fathom checkout
// itself, whose go.mod declares exactly that on line 1 — the read had
// failed — and then SKIPped instead of FAILing.
func TestFathomSourceTreeDistinguishesUnreadableFromForeign(t *testing.T) {
	mk := func(t *testing.T, body string) string {
		t.Helper()
		root := t.TempDir()
		if body != "" {
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}

	if yes, why := fathomSourceTree(mk(t, "module github.com/geoffmilleraz/fathom\n\ngo 1.26\n")); !yes || why != "" {
		t.Fatalf("the fathom checkout must be recognised: yes=%v why=%q", yes, why)
	}

	yes, why := fathomSourceTree(mk(t, "module example.com/other\n"))
	if yes || !strings.Contains(why, "example.com/other") {
		t.Fatalf("a foreign module must be NAMED: yes=%v why=%q", yes, why)
	}

	yes, why = fathomSourceTree(mk(t, ""))
	if yes || !strings.Contains(why, "no go.mod") {
		t.Fatalf("a missing go.mod is its own fact: yes=%v why=%q", yes, why)
	}

	root := mk(t, "module github.com/geoffmilleraz/fathom\n")
	blind := filepath.Join(root, "go.mod")
	if err := os.Chmod(blind, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blind, 0o644) })
	yes, why = fathomSourceTree(root)
	if yes {
		t.Skip("this platform still read the chmod-000 file (running as root?)")
	}
	if !strings.Contains(why, "NOT BE READ") {
		t.Fatalf("an unreadable go.mod must say so, not claim the module is different: %q", why)
	}
	if strings.Contains(why, "does not declare") || strings.Contains(why, "declares module") {
		t.Fatalf("blindness must never be reported as a fact about the module line: %q", why)
	}

	// And the check WARNs rather than skipping quietly, because it could not
	// determine whether it applies.
	row := checkBinaryFresh(root)
	if row.Status != doctorWarn {
		t.Fatalf("an undetermined check is a WARN, got %v (%s)", row.Status, row.Reason)
	}
}

// RC-8: "matches no INDEXED file" is not "matches no file". A scope covering
// real files fathom does not index must never be reported as dead, because
// the dead-guard remedy tells the reader to DELETE it.
func TestDeadScopeDistinguishesUnindexedFromAbsent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"app.css", "input.css"} {
		if err := os.WriteFile(filepath.Join(root, "ui", n), []byte("body{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got := filesOnDiskUnder(root, []string{"ui/**"}); got != 2 {
		t.Fatalf("a scope over real files must count them, got %d", got)
	}
	if got := filesOnDiskUnder(root, []string{"does/not/exist/**"}); got != 0 {
		t.Fatalf("a scope over nothing counts zero, got %d", got)
	}
	// Nested vendor-ish dirs are skipped for speed, not silently counted.
	if got := filesOnDiskUnder(root, []string{"**/*.css"}); got != 2 {
		t.Fatalf("recursive scope must find both, got %d", got)
	}
}

// RC-6: the doctor row and the verb must not report different numbers for
// the same tree. They did — 12 against 88 — because the row built a partial
// audit and printed the result as a total.
func TestDoctorUnprovenReconcilesWithVerb(t *testing.T) {
	root := repoRootForDocs(t)
	t.Setenv("HOME", t.TempDir()) // isolate the ledger

	row := checkUnprovenClaims(root, "http://127.0.0.1:1", "")
	if row.Status == doctorPass && !strings.Contains(row.Reason, "no unproven") {
		t.Skip("this tree reports no unproven claims; nothing to reconcile")
	}
	// The row must carry the kinds the verb computes WITHOUT an index —
	// U12 and U13 were simply never wired in.
	for _, kind := range []string{"U12", "U13"} {
		if !strings.Contains(row.Reason, kind) {
			t.Errorf("%s needs no index and must be counted in the doctor row: %s", kind, row.Reason)
		}
	}
	// And the non-zero path must carry what it could not compute, which
	// only the zero path used to do.
	if !strings.Contains(row.Reason, "NOT COMPUTED") {
		t.Errorf("a partial count must name what is missing from it: %s", row.Reason)
	}
	if row.Status != doctorWarn {
		t.Errorf("unproven claims are a WARN, never a FAIL: %v", row.Status)
	}
}

// RC-4: an unreadable PLAN must not silently shrink U12's population either.
func TestPlanContractsNameUnreadablePlans(t *testing.T) {
	root := t.TempDir()
	plans := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(plans, "good.md")
	if err := os.WriteFile(good, []byte("- [x] a {id: G-1, symbol: Foo, package: internal/store}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(plans, "bad.md")
	if err := os.WriteFile(bad, []byte("- [x] b {id: B-1, symbol: Bar, package: internal/store}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, _, cov := planRowContracts(root)
	if len(rows) != 2 || !cov.Complete() {
		t.Fatalf("baseline: rows=%d complete=%v", len(rows), cov.Complete())
	}

	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })

	rows, _, cov = planRowContracts(root)
	if len(rows) == 2 {
		t.Skip("this platform still read the chmod-000 file (running as root?)")
	}
	if cov.Complete() {
		t.Fatal("an unreadable plan must make coverage incomplete, not a quietly smaller population")
	}
	if !strings.Contains(cov.Note(), "bad.md") {
		t.Fatalf("the unreadable plan must be named: %q", cov.Note())
	}
}
