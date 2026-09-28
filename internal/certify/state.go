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
// anchored memory's dropOne used for its "suspect" penalty before
// SR-13 retired internal/memory ("inventing a sixth tier... would be a new
// ladder", refused there and here) rather than inventing a "suspect"
// EvidenceTier value that core's own ladder does not know how to sort.
import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/store"
)

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

// CertificateStateAt (CV-3, design doc §1.1, O114) is CertificateState
// refined so a certificate is live only for a run bound to a commit AFTER
// WHICH NOTHING IT REACHES CHANGED — fixing the half of TC-8 O114 found
// missing: a certificate stayed live after its reached symbol changed
// underneath it, because nothing bound the attested run to a commit.
// reachUnchangedSince(commit) is CV-4's own check (already closed over
// the certificate's reach — its test file plus the files of the symbols
// it reaches — by the caller), reporting whether none of that reach
// changed between commit and HEAD, worktree edits included.
//
// Rules, in this exact order (design doc §1.1):
//
//  1. unreached                                          -> unreachable
//  2. no runs                                             -> unproven, "no attested run"
//  3. newest run failed                                   -> broken
//  4. newest run unbound (no commit, or a dirty tree)      -> unproven, "run not bound to a commit"
//  5. reach changed since the newest passing bound run     -> unproven, stale=true,
//     "reach changed since the attested run at <sha>"
//  6. a flake in the last three                            -> flaky
//  7. otherwise                                             -> live
//
// last is always the newest attested run (runs[0]) once any run exists —
// the run whose digest/attempts a caller reports — even when an earlier
// rule is what decided state. flaky is computed over the same last-three
// window CertificateState uses, and reported regardless of which rule
// decided state (mirroring CertificateState's own "the flake fact is
// still reported, even though it did not determine state").
//
// stale is true ONLY for rule 5: it tells the caller (evaluateCertificate)
// to mark the compiled claim's own Status core.ClaimStale — a fact about
// the CLAIM, distinct from the certificate's own State ladder (I43's "and
// the claim's status stale"). err is non-nil only when reachUnchangedSince
// itself fails (a git read gone wrong); never swallowed into a guessed
// state.
func CertificateStateAt(reached bool, runs []store.TestCaseRun, reachUnchangedSince func(commit string) (bool, error)) (state State, last store.TestCaseRun, flaky bool, reason string, stale bool, err error) {
	if len(runs) > 0 {
		last = runs[0]
	}
	if !reached {
		return StateUnreachable, last, false, "", false, nil
	}
	if len(runs) == 0 {
		return StateUnproven, last, false, "no attested run", false, nil
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

	// Rule 3 before rule 4 (design doc §1.1's own order): a failing
	// newest run is broken outright, whether or not it was ever bound —
	// a test that never passed has nothing for "was the pass trustworthy"
	// to ask about yet.
	if last.Status == "fail" {
		return StateBroken, last, flaky, "", false, nil
	}

	// Rule 4: the NEWEST run specifically — not the newest passing one —
	// must be bound. An unbound newest run (still running, or run against
	// a dirty tree) leaves the certificate unproven regardless of what an
	// older run showed.
	if last.Commit == "" || last.Dirty {
		return StateUnproven, last, flaky, "run not bound to a commit", false, nil
	}

	// Rule 5: reach must be unchanged since the newest PASSING BOUND run
	// — which is `last` itself when last already passed, or an earlier
	// run when the newest run is, say, a bound "flake" case whose own
	// pass/fail split this ladder does not itself resolve (see
	// newestPassingBoundRun's own doc comment).
	if boundRun, ok := newestPassingBoundRun(runs); ok {
		unchanged, rerr := reachUnchangedSince(boundRun.Commit)
		if rerr != nil {
			return StateUnproven, last, flaky, "", false, fmt.Errorf("CertificateStateAt: reachUnchangedSince(%s): %w", boundRun.Commit, rerr)
		}
		if !unchanged {
			return StateUnproven, last, flaky, fmt.Sprintf("reach changed since the attested run at %s", boundRun.Commit), true, nil
		}
	}

	switch {
	case last.Status == "pass" && flaky:
		return StateFlaky, last, flaky, "", false, nil
	case last.Status == "pass":
		return StateLive, last, flaky, "", false, nil
	default:
		// The newest run is bound and not failing, yet not itself
		// "pass" either (e.g. a bound "flake" case whose own pass/fail
		// split is out of scope here) — matches CertificateState's own
		// pre-existing default, unproven rather than a guess.
		return StateUnproven, last, flaky, "", false, nil
	}
}

// newestPassingBoundRun returns the newest run in runs (searched from
// runs[0] forward, so it prefers the MOST RECENT match) whose Status is
// "pass" and which is bound to a commit on a clean tree — the run rule
// 5 checks the reach against. Runs[0] itself qualifies whenever it
// already passed and was bound (the common case, already confirmed
// non-failing and bound by rules 3/4 before this is ever called); a
// caller reaches this function only after establishing the newest run is
// not itself failing, so a "no match" here means every run in history
// was never both passing and bound — reach is then never checked at all,
// and the final switch's default (unproven) applies.
func newestPassingBoundRun(runs []store.TestCaseRun) (store.TestCaseRun, bool) {
	for _, r := range runs {
		if r.Status == "pass" && r.Commit != "" && !r.Dirty {
			return r, true
		}
	}
	return store.TestCaseRun{}, false
}
