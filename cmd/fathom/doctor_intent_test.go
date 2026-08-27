package main

import (
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
