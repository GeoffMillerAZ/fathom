package custodian_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/custodian"
)

// TestADRReferencesResolveConcern (EP-14) is INC-14's outcome. A design
// document declared itself "adopted (ADR-026)" while ADR.md had no such
// section, and three intent documents cited the same phantom record. A
// panel caught it; nothing mechanical would have. This is the mechanism.
func TestADRReferencesResolveConcern(t *testing.T) {
	concern := custodian.Concern{
		ID:    "adr_references_resolve",
		Want:  "every ADR citation resolves to a record that exists",
		Check: custodian.Check{Kind: custodian.KindADRReferencesResolve},
	}

	t.Run("a phantom citation is a violation", func(t *testing.T) {
		root := t.TempDir()
		writeADRFixture(t, root, "## ADR-001 — Real\n\n**Decision.** Done.\n")
		mustWrite(t, filepath.Join(root, "docs", "design", "plane.md"),
			"# A design\n\n*Status: adopted (ADR-026).*\n")

		f := custodian.Audit(root, []custodian.Concern{concern})[0]
		if f.Status != custodian.StatusViolated {
			t.Fatalf("status = %q, want violated; detail=%s", f.Status, f.Detail)
		}
		if !strings.Contains(f.Detail, "ADR-026") {
			t.Errorf("the violation must name the phantom record: %s", f.Detail)
		}
		if len(f.Evidence) == 0 || !strings.Contains(f.Evidence[0], "plane.md:3") {
			t.Errorf("the violation must carry file:line evidence: %v", f.Evidence)
		}
	})

	t.Run("real citations hold", func(t *testing.T) {
		root := t.TempDir()
		writeADRFixture(t, root, "## ADR-001 — Real\n\n## ADR-026 — Also real\n")
		mustWrite(t, filepath.Join(root, "docs", "design", "plane.md"),
			"*Status: adopted (ADR-026), superseding ADR-001.*\n")

		f := custodian.Audit(root, []custodian.Concern{concern})[0]
		if f.Status != custodian.StatusHolds {
			t.Errorf("status = %q, want holds; detail=%s", f.Status, f.Detail)
		}
	})

	t.Run("a repo with no records is unable, never violated", func(t *testing.T) {
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, "docs", "design", "x.md"), "cites ADR-007\n")

		f := custodian.Audit(root, []custodian.Concern{concern})[0]
		if f.Status != custodian.StatusUnable {
			t.Errorf("status = %q, want unable — a repo without decision records is not in violation", f.Status)
		}
		if !strings.Contains(f.Detail, "no decision records") {
			t.Errorf("the reason must name the absence: %s", f.Detail)
		}
	})

	t.Run("file-per-record repos resolve too", func(t *testing.T) {
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, "docs", "adr", "0007-a-choice.md"), "# ADR-0007\n")
		mustWrite(t, filepath.Join(root, "docs", "design", "x.md"), "see ADR-0007\n")

		f := custodian.Audit(root, []custodian.Concern{concern})[0]
		if f.Status != custodian.StatusHolds {
			t.Errorf("status = %q for a docs/adr repo; detail=%s", f.Status, f.Detail)
		}
	})
}

func writeADRFixture(t *testing.T, root, body string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, "ADR.md"), "# Decisions\n\n"+body)
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ADR-027: a citing file this check could not open is an instrument failure,
// not a cleaner repository. Skipping it silently shrinks the checked count
// and hides whatever it cites — so the concern reports unable-to-check
// rather than a clean hold it never earned.
func TestADRReferencesNamesUnreadableCitingFiles(t *testing.T) {
	concern := custodian.Concern{
		ID:    "adr_references_resolve",
		Want:  "every ADR citation resolves to a record that exists",
		Check: custodian.Check{Kind: custodian.KindADRReferencesResolve},
	}

	root := t.TempDir()
	writeADRFixture(t, root, "## ADR-001 \u2014 Real\n")
	mustWrite(t, filepath.Join(root, "docs", "design", "good.md"), "see ADR-001\n")

	if f := custodian.Audit(root, []custodian.Concern{concern})[0]; f.Status != custodian.StatusHolds {
		t.Fatalf("baseline must hold, got %q: %s", f.Status, f.Detail)
	}

	blind := filepath.Join(root, "docs", "design", "blind.md")
	mustWrite(t, blind, "see ADR-999, which does not exist\n")
	if err := os.Chmod(blind, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blind, 0o644) })

	f := custodian.Audit(root, []custodian.Concern{concern})[0]
	if f.Status == custodian.StatusHolds {
		t.Skipf("this platform still read the chmod-000 file (running as root?): %s", f.Detail)
	}
	if f.Status != custodian.StatusUnable {
		t.Fatalf("blindness is unable-to-check, never violated and never a hold: %q (%s)", f.Status, f.Detail)
	}
	if !strings.Contains(f.Detail, "blind.md") || !strings.Contains(f.Detail, "NOT BE READ") {
		t.Fatalf("the unreadable file must be named: %s", f.Detail)
	}
}
