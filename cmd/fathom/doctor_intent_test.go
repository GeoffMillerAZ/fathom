package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctorIntentRowsNeverFail (EP-11): neither row may FAIL. A repo that
// has not adopted the practice is not broken, and a repo carrying honest
// debt is not broken — doctor's value is that red means something is
// actually wrong, and a row that is red everywhere teaches the reader to
// skim past the ones that matter (O23's lesson).
func TestDoctorIntentRowsNeverFail(t *testing.T) {
	t.Run("a repo with no corpus passes with a reason", func(t *testing.T) {
		root := t.TempDir()
		got := checkIntent(root)
		if got.Status == doctorFail {
			t.Errorf("not adopting Project Intent is a choice, not a failure: %+v", got)
		}
		if !strings.Contains(got.Reason, "not adopted") {
			t.Errorf("the row must say why it is quiet: %q", got.Reason)
		}
		if claims := checkUnprovenClaims(root, "http://127.0.0.1:1", "svc"); claims.Status == doctorFail {
			t.Errorf("unproven_claims must never fail: %+v", claims)
		}
	})

	t.Run("a repo with debt warns, never fails", func(t *testing.T) {
		root := t.TempDir()
		writeCorpusFixture(t, root)
		t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(root, "l.jsonl"))
		t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(root, "v.jsonl"))

		if got := checkIntent(root); got.Status == doctorFail {
			t.Errorf("intent row failed on a healthy corpus: %+v", got)
		}
		claims := checkUnprovenClaims(root, "http://127.0.0.1:1", "svc")
		if claims.Status == doctorFail {
			t.Errorf("unproven_claims must warn at worst: %+v", claims)
		}
	})
}

// TestDoctorIntentRowsSkipOnUnreadableRouter (O59, ADR-027): a router that
// EXISTS but could not be read (a wedged TCC verdict on an external volume
// is the live case) is not the same fact as a router that genuinely is not
// there. Both checkIntent and checkUnprovenClaims funnel through
// intent.Load and its `c.Tier == intent.TierNone` disjunct, which is true
// in BOTH cases — the read failure must be told apart via c.Coverage, not
// silently reported as "not adopted, which is a choice". Live evidence,
// 2026-09-14: the same tree reported WARN "127 unproven claim(s)" twice
// and PASS "no intent corpus" once in between, purely because the router
// read raced a TCC wedge.
func TestDoctorIntentRowsSkipOnUnreadableRouter(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod 000 does not block reads")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "intent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	router := filepath.Join(dir, "README.md")
	if err := os.WriteFile(router, []byte("# router\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(router, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(router, 0o644) })

	got := checkIntent(root)
	if got.Status != doctorSkipped {
		t.Errorf("intent row status = %v, want skipped for an unreadable router (never a guessed pass): %+v", got.Status, got)
	}
	if !strings.Contains(got.Reason, "README.md") {
		t.Errorf("skip reason must name the unreadable path: %q", got.Reason)
	}

	claims := checkUnprovenClaims(root, "http://127.0.0.1:1", "svc")
	if claims.Status != doctorSkipped {
		t.Errorf("unproven_claims status = %v, want skipped for an unreadable router: %+v", claims.Status, claims)
	}
	if !strings.Contains(claims.Reason, "README.md") {
		t.Errorf("skip reason must name the unreadable path: %q", claims.Reason)
	}
}

// TestDoctorIntentRowsPassOnGenuinelyAbsentCorpus: the control — no
// docs/intent/ at all (a not-exist error on the router, or none) must stay
// an honest PASS, never downgraded to SKIP by the fix above.
func TestDoctorIntentRowsPassOnGenuinelyAbsentCorpus(t *testing.T) {
	root := t.TempDir()
	if got := checkIntent(root); got.Status != doctorPass {
		t.Errorf("intent row status = %v, want pass for a genuinely absent corpus: %+v", got.Status, got)
	}
	claims := checkUnprovenClaims(root, "http://127.0.0.1:1", "svc")
	if claims.Status != doctorPass {
		t.Errorf("unproven_claims status = %v, want pass for a genuinely absent corpus: %+v", claims.Status, claims)
	}
}

// TestCheckUnprovenClaims_ReasonIncludesEvidenceTimestamp pins DG-15:
// intent.Claim.Evidence ("verify ledger " + measured-at) is already
// computed for every U1/U2 claim (internal/intent/audit.go) but was
// discarded when doctor_intent.go rendered the one-line reason — a reader
// deciding whether "first" is still current needs that timestamp.
func TestCheckUnprovenClaims_ReasonIncludesEvidenceTimestamp(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(root, "intent-ledger.jsonl"))
	ledgerPath := filepath.Join(root, "verify-ledger.jsonl")
	t.Setenv("FATHOM_VERIFY_LEDGER", ledgerPath)

	row := verifyLedgerRow{
		SchemaVersion: verifyLedgerSchemaVersion,
		At:            "2026-01-02T03:04:05Z",
		RepoRoot:      root,
		Service:       "svc",
		Plan:          "docs/plans/demo.md",
		Total:         1,
		Claimed:       1,
		Failing:       1,
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	got := checkUnprovenClaims(root, "http://127.0.0.1:1", "svc")
	if got.Status != doctorWarn {
		t.Fatalf("status = %v, want warn (a failing plan row is honest debt, never a doctor failure): %+v", got.Status, got)
	}
	if !strings.Contains(got.Reason, "verify ledger 2026-01-02T03:04:05Z") {
		t.Fatalf("reason must carry the claim's evidence timestamp, not just its ref/detail: %q", got.Reason)
	}
}
