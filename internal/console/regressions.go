package console

// regressions.go — WI-20/WI-21/WI-22 (docs/plans/work-intelligence.md Wave
// 2; docs/design/work-intelligence.md §3 "U12. Proof regressions,
// generalized"): U12 generalizes PE-8's ProofRegressions (progressviews.go)
// — "anything once proven that no longer holds, with the reading and
// commit where it changed" — to certificates, conservation planes and
// invariant enforcement, then merges every source into one newest-first
// list (AllProofRegressions).
//
// No new capture (design doc §4, "Tenancy and cost": "No new capture...
// already exist"). certificateRegressions and conservationRegressions'
// invariant half replay store.TestCaseRun history — the SAME attested-run
// evidence internal/certify.CompileCertifications already reads — through
// certify.CertificateState, the ladder's own one code path (state.go: "a
// claim's state is derived by exactly one code path"), rather than a
// second, parallel state computation that could drift from the compiled
// certificate's own state. This replay holds Reached fixed at its current
// value across the whole history: a certificate's reach changing underfoot
// (CertificateStateAt's stricter rule 5) is not modeled here, only its
// run-status transitions — a deliberately narrower first cut the design
// doc's own WI-M5 measurement will validate over 30 real days.
//
// conservationRegressions' conservation-plane half is a projection over
// ConservationReading, a shape no ledger populates today: `fathom
// conserve` prints its report at every push (lefthook's refactor-conserve
// step) and persists nothing, so the design's own "no new capture" rule
// leaves this generalization's conservation half fed only by whatever a
// caller already has on hand. An empty slice is the correct, common
// answer today, not an error (ADR-045's own discipline: an absent input
// says why, never a guessed total).

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// CertHistory is one certificate's (kind, target, test) identity plus its
// attested-run history (store.TestCaseRunsForSymbol's own ordering, newest
// first) and current reach — exactly the evidence
// internal/certify.CompileCertifications already reads per (target, test),
// replayed here instead of recompiled. Kind is a certify Target.Kind
// ("spec", "rule", "port", "behaviour", "incident", "invariant"); an
// "invariant"-kind entry is certificateRegressions' business only via
// conservationRegressions (WI-21), because an invariant's enforcement IS a
// certificate (internal/certify/resolve.go's resolveInvariant).
type CertHistory struct {
	Service  string
	Kind     string
	TargetID string
	Test     string
	Reached  bool
	Runs     []store.TestCaseRun // newest first
}

// ConservationReading is one `fathom conserve` plane's verdict at one
// commit — see the file doc comment for why this is often empty today.
type ConservationReading struct {
	Service string
	Plane   string
	At      string
	Commit  string
	Verdict string // conserve.Conserved | conserve.Losses | conserve.Unproven
}

// replayCertHistory is certificateRegressions' and conservationRegressions'
// shared ladder replay: walk one certificate's runs oldest to newest,
// recomputing certify.CertificateState at each point exactly as
// evaluateCertificate would have at that commit, and report a certificate
// whose most recent live reading was followed by a non-live one — the
// same "last proven, first reading that changed" shape PE-8's
// ProofRegressions uses (progressviews.go), off store evidence instead of
// the verify ledger.
func replayCertHistory(certs []CertHistory) []Regression {
	var out []Regression
	for _, c := range certs {
		n := len(c.Runs)
		if n == 0 {
			continue
		}
		// states[j] is the ladder state computed from c.Runs[j:] — the
		// runs that existed as of run j, since c.Runs is newest-first and
		// a smaller index means a more recent run.
		states := make([]certify.State, n)
		for j := n - 1; j >= 0; j-- {
			st, _, _ := certify.CertificateState(c.Reached, c.Runs[j:])
			states[j] = st
		}
		if states[0] == certify.StateLive {
			continue // currently live: nothing regressed
		}
		liveAt := -1
		for j := 0; j < n; j++ {
			if states[j] == certify.StateLive {
				liveAt = j
				break // smallest j scanned first: the most recent live reading
			}
		}
		if liveAt <= 0 {
			continue // never live, or live only at the newest reading (already excluded above)
		}
		out = append(out, Regression{
			Service:       c.Service,
			ID:            c.TargetID,
			Test:          c.Test,
			ProvenAt:      c.Runs[liveAt].StartedAt,
			ProvenCommit:  c.Runs[liveAt].Commit,
			ChangedAt:     c.Runs[liveAt-1].StartedAt,
			ChangedCommit: c.Runs[liveAt-1].Commit,
			State:         string(states[0]),
		})
	}
	return out
}

// certificateRegressions (WI-20, design doc §3 "U12" ¶2) lists every
// non-invariant certificate that read live and now reads flaky, broken or
// unproven, with the reading and commit where it changed, newest change
// first.
func certificateRegressions(certs []CertHistory) []Regression {
	subset := make([]CertHistory, 0, len(certs))
	for _, c := range certs {
		if c.Kind == "invariant" {
			continue // conservationRegressions' job (WI-21)
		}
		subset = append(subset, c)
	}
	out := replayCertHistory(subset)
	for i := range out {
		out[i].Kind = "certificate"
	}
	sortRegressions(out)
	return out
}

// conservationRegressions (WI-21, design doc §3 "U12" ¶2) lists two kinds
// of regression, newest change first: an invariant's enforcing test that
// regressed off live (replayCertHistory over invariant-kind certificates
// only), and a conservation plane whose newest reading is no longer
// `conserved` after once being so.
func conservationRegressions(certs []CertHistory, planes []ConservationReading) []Regression {
	var invariants []CertHistory
	for _, c := range certs {
		if c.Kind == "invariant" {
			invariants = append(invariants, c)
		}
	}
	out := replayCertHistory(invariants)
	for i := range out {
		out[i].Kind = "invariant"
	}
	out = append(out, conservationPlaneRegressions(planes)...)
	sortRegressions(out)
	return out
}

// conservationPlaneRegressions finds, per (service, plane), the newest
// `conserved` reading followed by one that is not — mirroring PE-8's own
// "last proven, first reading that changed" shape.
func conservationPlaneRegressions(planes []ConservationReading) []Regression {
	byKey := map[string][]ConservationReading{}
	var keys []string
	for _, r := range planes {
		k := r.Service + "|" + r.Plane
		if _, ok := byKey[k]; !ok {
			keys = append(keys, k)
		}
		byKey[k] = append(byKey[k], r)
	}
	sort.Strings(keys)
	var out []Regression
	for _, k := range keys {
		rs := append([]ConservationReading(nil), byKey[k]...)
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].At < rs[j].At })
		n := len(rs)
		if n == 0 || rs[n-1].Verdict == conserve.Conserved {
			continue // no readings, or still conserved: nothing regressed
		}
		liveAt := -1
		for j := n - 1; j >= 0; j-- {
			if rs[j].Verdict == conserve.Conserved {
				liveAt = j
				break
			}
		}
		if liveAt < 0 || liveAt == n-1 {
			continue // never conserved, or conserved only at the newest reading
		}
		p, c := rs[liveAt], rs[liveAt+1]
		out = append(out, Regression{
			Service: p.Service, ID: p.Plane, Kind: "conservation",
			ProvenAt: p.At, ProvenCommit: p.Commit,
			ChangedAt: c.At, ChangedCommit: c.Commit,
			State: c.Verdict,
		})
	}
	return out
}

// AllProofRegressions (WI-22, needs WI-20, WI-21, progress-and-economics.md
// PE-8) merges PE-8's plan-row regressions with WI-20's certificate
// regressions and WI-21's conservation-and-invariant regressions into one
// list, newest change first — the generalized reading design doc §3
// "U12" describes: "anything once proven that no longer holds ... one
// list, newest first."
func AllProofRegressions(h History, f HistoryFilter, certs []CertHistory, planes []ConservationReading) []Regression {
	rows := ProofRegressions(h, f)
	out := make([]Regression, 0, len(rows))
	for _, r := range rows {
		r.Kind = "plan"
		out = append(out, r)
	}
	out = append(out, certificateRegressions(certs)...)
	out = append(out, conservationRegressions(certs, planes)...)
	sortRegressions(out)
	return out
}

// sortRegressions is every merge point's own newest-change-first order
// (PE-8's ProofRegressions sorts the same way): ChangedAt descending, then
// a stable identity tiebreak so two regressions changed at the same
// instant order deterministically (internal/CLAUDE.md: "sort every output
// slice").
func sortRegressions(regs []Regression) {
	sort.SliceStable(regs, func(i, j int) bool {
		if regs[i].ChangedAt != regs[j].ChangedAt {
			return regs[i].ChangedAt > regs[j].ChangedAt
		}
		ki := RowKey(regs[i].Service, regs[i].Kind+"|"+regs[i].Plan, regs[i].ID+"|"+regs[i].Test)
		kj := RowKey(regs[j].Service, regs[j].Kind+"|"+regs[j].Plan, regs[j].ID+"|"+regs[j].Test)
		return ki < kj
	})
}
