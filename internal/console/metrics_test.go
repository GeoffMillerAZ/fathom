package console_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

func intp(n int) *int { return &n }

// An observations file that exists and cannot be read (chmod 000, the
// external-volume EPERM wedge) is an ERROR naming the repo and the failure
// — never "not reporting", and never folded into a smaller lower bound that
// reads as fewer defects (ADR-045's truth states, invariant I38).
func TestOpenObservations_UnreadableFileIsError(t *testing.T) {
	f := console.FleetFacts{Services: []console.ServiceFact{
		{Name: "alpha", OpenObservations: intp(3)},
		{Name: "beta", ObservationsErr: "open /r/beta/.fathom/observations.md: permission denied"},
		{Name: "gamma"},
	}}
	v := console.OpenObservations(f)
	if v.State != console.StateError {
		t.Fatalf("state = %s (%+v), want error", v.State, v)
	}
	for _, want := range []string{"beta", "permission denied", "1 of 3"} {
		if !strings.Contains(v.Reason, want) {
			t.Errorf("reason %q must name %q", v.Reason, want)
		}
	}
	if err := v.Validate(); err != nil {
		t.Error(err)
	}
	// Without the read failure the same fleet is the named lower bound.
	f.Services[1].ObservationsErr = ""
	if v := console.OpenObservations(f); v.State != console.StatePartial || v.V != 3 {
		t.Errorf("no read failure = %+v, want partial 3", v)
	}
}

// Per-repo sessions share the one dedupe rule: distinct session ids per
// lowered repo label, never ledger rows.
func TestDistinctSessionsByRepo(t *testing.T) {
	data := []byte(`{"session_id":"s1","repo":"alpha"}
{"session_id":"s1","repo":"alpha"}
{"session_id":"s2","repo":"Alpha"}
{"session_id":"s3","repo":"beta"}
{"repo":"beta","at":"2026-09-25T10:00:00Z"}
not json
{"session_id":"s4"}
`)
	got := console.DistinctSessionsByRepo(data)
	if got["alpha"] != 2 || got["beta"] != 2 || len(got) != 2 {
		t.Errorf("by repo = %v, want alpha 2, beta 2", got)
	}
	if sessions, rows := console.DistinctSessions(data); sessions != 5 || rows != 6 {
		t.Errorf("DistinctSessions = %d sessions from %d rows, want 5 from 6", sessions, rows)
	}
}
