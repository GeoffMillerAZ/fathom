package certify

// incident_certificates_test.go — TC-19's own test:
// TestIncidentCertificates_OutcomeGrammar.
import (
	"path/filepath"
	"testing"
)

func TestIncidentCertificates_OutcomeGrammar(t *testing.T) {
	t.Run("an Outcome: line naming certificate <Test> is captured", func(t *testing.T) {
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, "docs", "intent", "incidents.md"), "# Incidents\n\n"+
			"## Open\n\n"+
			"### INC-1 A thing broke\n\n"+
			"What: something.\n"+
			"Outcome: fixed; certificate TestRefund_Idempotent\n\n"+
			"### INC-2 Still open\n\n"+
			"What: still happening.\n"+
			"Outcome: pending\n")

		got := IncidentCertificates(root)
		if got["INC-1"] != "TestRefund_Idempotent" {
			t.Errorf("INC-1 = %q, want TestRefund_Idempotent", got["INC-1"])
		}
		if got["INC-2"] != "" {
			t.Errorf("INC-2 = %q, want \"\" (Outcome: pending names no certificate)", got["INC-2"])
		}
	})

	t.Run("the LAST Outcome: line under one incident wins", func(t *testing.T) {
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, "docs", "intent", "incidents.md"), "# Incidents\n\n"+
			"## Open\n\n"+
			"### INC-3\n\n"+
			"Outcome: pending\n"+
			"Outcome: resolved — certificate TestGateHTTP\n")

		got := IncidentCertificates(root)
		if got["INC-3"] != "TestGateHTTP" {
			t.Errorf("INC-3 = %q, want TestGateHTTP (last Outcome: line wins)", got["INC-3"])
		}
	})

	t.Run("a missing incidents.md yields an empty map, never an error or panic", func(t *testing.T) {
		got := IncidentCertificates(t.TempDir())
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
}
