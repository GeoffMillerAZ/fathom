package main

// verifyfigure.go — the ONE canonical verify-gate figure (PC-13,
// docs/plans/proof-credibility.md; vision 26 §1.5). Three figures were
// circulating for "what the gate has caught" — the call-log sum, the
// adoption gauge, and a hand-copied sentence in docs/adoption/README.md —
// and they disagreed because they counted different populations over
// different windows (the O6 double-count family). verifyFigure computes
// the figure from the verify ledger alone, and Sentence carries the
// window and every denominator inside the sentence, so a number can never
// travel without the population it was measured on.
//
// A refusal is an EVENT: one ledger row (one plan, one run) whose verdict
// carried at least one failing claimed row — never a sum of failing rows
// across runs, which would count a standing failure once per push.

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// verifyFigureT is the computed figure. Zero Runs means "not instrumented",
// never a measured zero.
type verifyFigureT struct {
	Runs        int    // ledger rows: one per (plan, `fathom verify` run)
	Refusals    int    // rows with Failing > 0 — refusal events
	FailingRows int    // sum of Failing across rows — standing failures re-counted per run; NEVER the headline
	Plans       int    // distinct (repo root, plan path)
	Repos       int    // distinct repo roots
	First, Last string // RFC3339 window bounds ("" when Runs == 0)
	// Ignored is the rows left out because their repo_root is not a
	// registered service root (PE-24); Sentence names them. nil when the
	// rows were not filtered (a pointer keeps the figure comparable).
	Ignored *ledgerroots.Ignored
}

// registeredVerifyFigure is the figure over the rows under roots'
// registered service roots, naming the rows it ignored (PE-24).
func registeredVerifyFigure(rows []verifyLedgerRow, roots ledgerroots.Roots) verifyFigureT {
	kept, ign := verifyLedgerRegisteredRows(rows, roots)
	f := verifyFigure(kept)
	f.Ignored = &ign
	return f
}

// verifyFigure computes the figure over rows. Deterministic: counts and
// min/max only; row order is irrelevant.
func verifyFigure(rows []verifyLedgerRow) verifyFigureT {
	f := verifyFigureT{}
	plans := map[string]bool{}
	repos := map[string]bool{}
	for _, r := range rows {
		f.Runs++
		if r.Failing > 0 {
			f.Refusals++
		}
		f.FailingRows += r.Failing
		plans[r.RepoRoot+"|"+r.Plan] = true
		repos[r.RepoRoot] = true
		if f.First == "" || r.At < f.First {
			f.First = r.At
		}
		if r.At > f.Last {
			f.Last = r.At
		}
	}
	f.Plans = len(plans)
	f.Repos = len(repos)
	return f
}

// ignored is what the figure's rows left out (zero when unfiltered).
func (f verifyFigureT) ignored() ledgerroots.Ignored {
	if f.Ignored == nil {
		return ledgerroots.Ignored{}
	}
	return *f.Ignored
}

// Sentence renders the figure with its window and denominators inline —
// the only shape the number may be quoted in.
func (f verifyFigureT) Sentence() string {
	if f.Runs == 0 {
		return withIgnored("no `fathom verify` run is recorded in ~/.fathom/verify-ledger.jsonl — the gate figure is not instrumented (not a measured zero)", f.ignored(), verifyLedgerNoun)
	}
	day := func(ts string) string {
		if i := strings.Index(ts, "T"); i > 0 {
			return ts[:i]
		}
		return ts
	}
	source := "source ~/.fathom/verify-ledger.jsonl"
	if p := f.ignored().Phrase(verifyLedgerNoun); p != "" {
		source += ", registered roots only: " + p
	}
	return fmt.Sprintf("the verify gate refused %d of %d recorded runs over %d plan(s) in %d repo(s) between %s and %s (a refusal = one run whose verdict carried at least one failing claimed row; %d failing-row readings summed across runs are NOT the figure; %s)",
		f.Refusals, f.Runs, f.Plans, f.Repos, day(f.First), day(f.Last), f.FailingRows, source)
}
