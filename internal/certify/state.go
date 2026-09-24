package certify

// state.go — CertificateState (TC-21, design doc §3/§5): the state ladder
// plus the flakiness fact, extracted out of compile.go's
// evaluateCertificate into one named, independently-tested function so
// "a claim's state is derived by exactly one code path" (evaluateCertificate's
// own doc comment, already the discipline CompileCertifications and
// SweepCertifications share) extends to the ladder computation itself.
//
// Flakiness never leaves a certificate silently reported at the top
// tier: evaluateCertificate drops an otherwise-earned TierVerified to
// TierPreScan when flaky is true — the SAME ladder-drop convention
// internal/memory's own dropOne uses for its "suspect" penalty
// (internal/memory/tier.go: "inventing a sixth tier... would be a new
// ladder", refused there and here) rather than inventing a "suspect"
// EvidenceTier value that core's own ladder does not know how to sort.
import "github.com/geoffmilleraz/fathom/internal/store"

// CertificateState computes the certifications-layer State (design doc
// §3) from whether the test reaches the target and its attested-run
// history (runs, newest first — store.TestCaseRunsForSymbol's own
// ordering; only the newest three are ever consulted for flakiness,
// design doc §3's "no flake in the last three runs"). last is runs[0]
// when reached and at least one run exists (the zero value otherwise —
// the same "nothing to report" the caller already gets from an empty
// runs slice). flaky reports whether a flake sits within that
// three-run window; it is always false when the test is unreached or
// has never been run, since neither state gets far enough to ask.
func CertificateState(reached bool, runs []store.TestCaseRun) (state State, last store.TestCaseRun, flaky bool) {
	if len(runs) > 0 {
		last = runs[0]
	}
	switch {
	case !reached:
		return StateUnreachable, last, false
	case len(runs) == 0:
		return StateUnproven, last, false
	}

	limit := len(runs)
	if limit > 3 {
		limit = 3
	}
	for _, r := range runs[:limit] {
		if r.Status == "flake" {
			flaky = true
		}
	}
	// A failing last run is Broken outright (design doc §3: "the last
	// attested run failed") — checked BEFORE flaky, so a test that
	// flaked earlier in the window and is failing NOW is never
	// mislabeled Flaky (a bug this extraction surfaced and fixes:
	// the prior inline version checked flaky first, which let a
	// currently-failing test hide behind an old flake).
	switch {
	case last.Status == "fail":
		state = StateBroken
	case last.Status == "pass" && flaky:
		state = StateFlaky
	case last.Status == "pass":
		state = StateLive
	default:
		state = StateUnproven
	}
	return state, last, flaky
}
