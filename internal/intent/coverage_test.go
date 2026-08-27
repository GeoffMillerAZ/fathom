package intent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCorpus builds a minimal but real corpus: a router with a sensitive
// surface, an invariant table, and a condition.
func writeCorpus(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "intent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", `# Project intent — router

## Docs map

| Question | Read |
|---|---|
| Decisions | `+"`../../ADR.md`"+` |

## Sensitive surfaces and the evidence "done" needs

| Surface | Why it is sensitive | Done needs |
|---|---|---|
| `+"`internal/store/**`"+` | store purity (I2) | `+"`TestStorePurity`"+` |
`)
	write("invariants.md", `# Invariants

| Id | Statement | Enforced by | Source |
|---|---|---|---|
| I1 | The store is pure Go. | test `+"`TestStorePurity`"+` | ADR-020 |
| I2 | Answers are deterministic. | gate `+"`task validate`"+` | ADR-001 |
| I3 | Nothing is silent. | VIOLATED | law 3 |
`)
	write("scope.md", "# Scope\n\n**G1. A goal.** Done when: the gate exits 0.\n")
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte("# Decisions\n\n## ADR-001 — a thing\n\n**Status.** Accepted.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// RC-2/RC-5 — the regression test the defect never had.
//
// MEASURED on this repository before the fix: chmod 000 on
// docs/intent/invariants.md made `fathom intent audit` report 0 invariants
// instead of 37 and 85 unproven claims instead of 169, with NOTHING naming
// the file. The repository's most broken state was its healthiest-looking
// one. This asserts that can never happen silently again.
func TestUnreadableInputNeverLooksHealthier(t *testing.T) {
	root := t.TempDir()
	writeCorpus(t, root)

	before, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Invariants) == 0 {
		t.Fatal("fixture must produce invariants for this test to mean anything")
	}
	if !before.Coverage.Complete() {
		t.Fatalf("a readable corpus is complete coverage, got %q", before.Coverage.Note())
	}

	inv := filepath.Join(root, "docs", "intent", "invariants.md")
	if err := os.Chmod(inv, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inv, 0o644) })

	after, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}

	// The count DID drop — that part is unavoidable, the file is unreadable.
	// What must never happen is that it drops SILENTLY.
	if len(after.Invariants) >= len(before.Invariants) {
		t.Skip("this platform still read the chmod-000 file (running as root?); the property under test cannot be exercised")
	}
	if after.Coverage.Complete() {
		t.Fatal("an unreadable corpus file must make coverage INCOMPLETE")
	}
	note := after.Coverage.Note()
	if !strings.Contains(note, "invariants.md") {
		t.Fatalf("the coverage note must name the file it could not read, got %q", note)
	}
	if !strings.Contains(strings.ToLower(note), "denied") && !strings.Contains(strings.ToLower(note), "permitted") {
		t.Fatalf("the note must carry the OS errno, so a TCC wedge is distinguishable from a missing file: %q", note)
	}

	// It is also named in Unparsed, which is what the human-facing verb and
	// the doctor row read.
	found := false
	for _, u := range after.Unparsed {
		if strings.Contains(u.File, "invariants.md") && strings.Contains(u.Reason, "read") {
			found = true
		}
	}
	if !found {
		t.Fatalf("an unreadable corpus file must appear in Unparsed; got %+v", after.Unparsed)
	}

	// And no count derived from it may be rendered as a bare total.
	rendered := after.Coverage.RenderCount(len(after.Invariants), "invariant(s)")
	if !strings.HasPrefix(rendered, "≥") {
		t.Fatalf("a count over incomplete coverage must render as a lower bound, got %q", rendered)
	}

	// The audit itself leads its NOT-COMPUTED list with the blindness.
	a := RunAudit(AuditInput{Corpus: after})
	if len(a.Uncomputable) == 0 || !strings.HasPrefix(a.Uncomputable[0], "COVERAGE INCOMPLETE") {
		t.Fatalf("blindness must LEAD the uncomputable list, got %v", a.Uncomputable)
	}
	if a.Coverage.Complete() {
		t.Fatal("the audit must carry the corpus's coverage, not its own optimism")
	}
}

// A genuinely missing living doc is a fact, not blindness: coverage stays
// complete. Conflating the two is the other half of the same defect.
func TestAbsentFileIsNotBlindness(t *testing.T) {
	root := t.TempDir()
	writeCorpus(t, root)
	if err := os.Remove(filepath.Join(root, "docs", "intent", "scope.md")); err != nil {
		t.Fatal(err)
	}
	c, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Coverage.Complete() {
		t.Fatalf("a file that is simply not there is accounted for, not unreadable: %q", c.Coverage.Note())
	}
}
