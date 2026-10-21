package certify

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestNativeRuntimeObservationCannotBecomeLiveCertificate(t *testing.T) {
	run := store.TestCaseRun{RunDigest: "native", Status: "pass", Commit: "0123456789abcdef0123456789abcdef01234567", NativeJSON: `{"root":"/synthetic"}`}
	state, _, _, reason, _, err := CertificateStateAt(true, []store.TestCaseRun{run}, func(string) (bool, error) {
		t.Fatal("native observation must not invoke clean-commit promotion")
		return true, nil
	})
	if err != nil || state != StateUnproven || reason != "native runtime evidence is informational; not a live certificate" {
		t.Fatalf("state=%s reason=%s err=%v", state, reason, err)
	}
	state, _, _ = CertificateState(true, []store.TestCaseRun{run})
	if state != StateUnproven {
		t.Fatalf("legacy ladder promoted native %s", state)
	}
}
