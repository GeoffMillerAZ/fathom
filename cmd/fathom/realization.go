package main

// realization.go — phase Y (docs/design/realization-grade.md): the grade
// stops measuring CONFIGURATION and starts measuring whether fathom is
// doing anything here.
//
// The incumbent `wiringGrade` (observatory.go) folds four signals, three of
// which are "a file says the right thing" and one of which is "at least one
// call happened, ever". That is the wrong ceiling. A repository can score A
// and deliver nothing: wired perfectly, one call in July, no obligations
// firing, no gate running. Grading configuration rewards the act of
// configuring, which is precisely the behaviour a tool should not optimize
// for.
//
// The dimensions are ordered as a LADDER — each is a precondition for the
// next — which is what makes one letter meaningful rather than an average
// of unrelated things, and what lets the grade name the first broken rung
// instead of leaving a reader to guess.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// realizationDim is one rung.
type realizationDim struct {
	Name string `json:"name"`
	// Met is nil when the dimension does not APPLY here (a repository with
	// no plans has nothing for the gate to check). A not-applicable
	// dimension leaves the denominator rather than counting against the
	// repository — the same discipline wiringGrade already uses for
	// invisible hooks, and the reason an unopinionated repo is not
	// punished for being unopinionated.
	Met *bool `json:"met,omitempty"`
	// Evidence is the number behind the verdict, or the reason there is
	// none. Always present (law 3).
	Evidence string `json:"evidence"`
	// Remedy is attached only to the FIRST unmet rung — the one worth
	// acting on. Listing five remedies for a ladder whose bottom rung is
	// broken is noise.
	Remedy string `json:"remedy,omitempty"`
}

// realizationReport is one repository's ladder plus the fold.
type realizationReport struct {
	Service string           `json:"service"`
	Dims    []realizationDim `json:"dimensions"`
	// Grade is A–F, or "idle" for a repository with no sessions in the
	// window. Idle is NOT F: an indexed reference corpus nobody codes in is
	// working exactly as intended, and grading it as a failure would make
	// the fleet page cry wolf on every such repo.
	Grade string `json:"grade"`
	// FirstGap names the first unmet rung in ladder order — the letter
	// alone was never enough to act on.
	FirstGap string `json:"first_gap,omitempty"`
	Remedy   string `json:"remedy,omitempty"`
	// Realized / Available drive the fleet ranking: how much of what is
	// POSSIBLE here is actually happening.
	Realized  int `json:"realized"`
	Available int `json:"available"`
	// Sessions in the window — the weight in the unrealized-value ranking.
	Sessions int `json:"sessions_in_window"`
}

// realizationWindow is the per-service traffic the grade reads. Passing it
// in (rather than reading a log here) keeps the fold pure and testable.
type realizationWindow struct {
	// Calls in the window, NOT calls ever. The incumbent grade's
	// CallsTotal > 0 test can never go down, which makes it useless as a
	// health signal — a repository busy in July and silent since is not
	// currently realizing value. Sessions, ObligationServes and VerifyRuns
	// are the same window's (design §2): distinct sessions, obligations
	// calls, and the verify ledger's gate runs.
	Calls            int
	Sessions         int
	ObligationServes int
	VerifyRuns       int
	// HasPlans says whether the gate has anything to check here. Without
	// it, every repository with no plans reads as "not gating", which is
	// a failure it cannot fix.
	HasPlans bool
	// WindowDays names the window the call counts cover (0: "the window").
	WindowDays int
	// CallLogNote, when set, is why the call log was not read: queried and
	// preventing are then unknown, never unmet (law 3). VerifyLedgerNote is
	// the same for the verify ledger and the gating rung; SessionsNote for
	// the session ledger and the idle rule.
	CallLogNote      string
	VerifyLedgerNote string
	SessionsNote     string
	// PlansNote, when set, is why docs/plans was not read: gating is then
	// unknown, never not applicable.
	PlansNote string
}

func boolPtr(b bool) *bool { return &b }

// realizationLadderFacts is the ladder projection's input for one row.
func realizationLadderFacts(s obsServiceRow, w realizationWindow) console.LadderFacts {
	return console.LadderFacts{
		Service: s.Service, StanzaNote: s.StanzaNote, HooksNote: s.HooksNote,
		Freshness: s.Freshness, FreshRemedy: daemonRefreshRemedy(s.Path, s.Service),
		WindowDays: w.WindowDays, Calls: w.Calls, ObligationServes: w.ObligationServes, CallLogNote: w.CallLogNote,
		Sessions: w.Sessions, SessionsNote: w.SessionsNote, VerifyRuns: w.VerifyRuns, VerifyLedgerNote: w.VerifyLedgerNote,
		HasPlans: w.HasPlans, PlansNote: w.PlansNote,
	}
}

// realizationGrade folds the five dimensions for one service: the ladder
// projection (console.RealizationLadder, the one the project overview
// draws) read as the grade's record. A rung that is not applicable or
// could not be judged leaves the denominator.
//
// Pure over (row, window): same inputs, same letter. No clock, no I/O.
func realizationGrade(s obsServiceRow, w realizationWindow) realizationReport {
	return realizationReportFrom(console.RealizationLadder(realizationLadderFacts(s, w)))
}

// realizationReportFrom reads a ladder as the grade's record.
func realizationReportFrom(l console.Ladder) realizationReport {
	rep := realizationReport{Service: l.Service, Sessions: l.Sessions, Realized: l.Realized, Available: l.Available}
	for _, r := range l.Rungs {
		d := realizationDim{Name: r.Name, Evidence: r.Evidence, Remedy: r.Fix()}
		switch r.State {
		case console.RungMet:
			d.Met = boolPtr(true)
		case console.RungUnmet:
			d.Met = boolPtr(false)
		}
		rep.Dims = append(rep.Dims, d)
	}
	// A repository with no sessions in the window is IDLE, not failing. An
	// indexed reference corpus nobody codes in is working as intended, and
	// nine false alarms teach a reader to skim past the real one.
	rep.Grade = l.Grade
	if !l.Idle {
		// The first BROKEN rung in ladder order, with its remedy. The
		// letter alone was never enough to act on.
		rep.FirstGap, rep.Remedy = l.FirstUnmet, l.Remedy
	}
	return rep
}

// rankByUnrealized orders repositories by how much value is on the table:
// unmet dimensions weighted by how busy the repository is. A large, busy
// repository failing two rungs outranks a tiny one failing three, because
// the gap is worth more.
//
// The fleet page's incumbent sort is by call count, which answers "who is
// busy" — something nobody needs to be told. This answers "where should I
// spend the next ten minutes".
//
// Deterministic and stable under equal keys (house contract): ties break on
// sessions, then on service name.
func rankByUnrealized(reports []realizationReport) []realizationReport {
	out := append([]realizationReport(nil), reports...)
	score := func(r realizationReport) int {
		return (r.Available - r.Realized) * r.Sessions
	}
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := score(out[i]), score(out[j])
		if si != sj {
			return si > sj
		}
		if out[i].Sessions != out[j].Sessions {
			return out[i].Sessions > out[j].Sessions
		}
		return out[i].Service < out[j].Service
	})
	return out
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// realizationWindowDays is the ladder's window: the fourteen UTC days the
// traffic sparkline draws (row.TrafficDays), so a rung and the chart beside
// it can never disagree.
const realizationWindowDays = 14

// realizationWindowBounds is the ladder's window as [from, to): the start
// of the UTC day thirteen days before now, to the end of now's UTC day.
func realizationWindowBounds(now time.Time) (from, to time.Time) {
	y, mo, d := now.UTC().Date()
	today := time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -(realizationWindowDays - 1)), today.AddDate(0, 0, 1)
}

// inRealizationWindow reports whether a call-log timestamp's day falls in
// [fromDay, toDay) — the day prefix the traffic sparkline buckets by, so an
// injection is in the window exactly when its call is in the chart.
func inRealizationWindow(ts, fromDay, toDay string) bool {
	if len(ts) < 10 {
		return false
	}
	day := ts[:10]
	return day >= fromDay && day < toDay
}

// windowVerifyRuns counts each service's verify runs in [from, to) from the
// verify ledger (SD-1, the recorded-run signal: every `fathom verify` run
// appends to it, a pre-push gate's included). note is set when the ledger
// could not be read or does not exist: the gating rung is then unknown,
// never a measured "no runs" (law 3) — a daemon that reads another HOME's
// ledger would otherwise tell every gating repository to add the gate.
func windowVerifyRuns(from, to time.Time) (map[string]int, string) {
	path := verifyLedgerPath()
	if path == "" {
		return nil, "no verify ledger is configured (no home directory, no FATHOM_VERIFY_LEDGER)"
	}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil, "no verify ledger at " + path + " — no `fathom verify` run is recorded on this machine"
	}
	runs, _, readErr := ledgerRuns()
	if readErr != "" {
		return nil, "the verify ledger could not be read (" + readErr + ")"
	}
	return console.VerifyRunsIn(runs, strings.ToLower, from, to), ""
}

// sumInts totals a per-day traffic slice — the window's call count.
func sumInts(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}

// repoHasPlans decides whether the verify gate has anything to check here.
// A repository with no plans is not FAILING to gate; the dimension simply
// does not apply, and treating absence as failure would hand every
// plan-less repository a to-do it cannot complete.
func repoHasPlans(root string) bool {
	has, _ := repoPlansRead(root)
	return has
}

// repoPlansRead is repoHasPlans with the read's failure named: a
// docs/plans that exists but cannot be read is not "no plans" (I38), so
// the gating rung reads unknown with this note rather than not applicable.
func repoPlansRead(root string) (bool, string) {
	if root == "" {
		return false, ""
	}
	dir := filepath.Join(root, "docs", "plans")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, ""
		}
		return false, "docs/plans was not read (" + err.Error() + ")"
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "README.md" {
			return true, ""
		}
	}
	return false, ""
}

// unrealizedScore is the fleet-ordering key: how much value is on the
// table here. Unmet rungs weighted by how busy the repository is, so a
// large busy repository failing two rungs outranks a tiny one failing
// three — the gap is worth more.
//
// A row with no ladder scores 0 rather than being dropped: an unrankable
// row is still a row, and silently omitting one from the fleet page would
// be worse than ranking it last.
func unrealizedScore(s obsServiceRow) int {
	if s.Realization == nil {
		return 0
	}
	return (s.Realization.Available - s.Realization.Realized) * s.Realization.Sessions
}

// daemonRefreshRemedy is the command that re-indexes a registered service
// through the daemon that owns the store (the post-commit hook's own path,
// HF-2), waiting for the ingest (--wait: the ingest tool on the refresh
// queue) — a bare post-commit-hook only queues one (RT-111): never a shell `fathom ingest`, whose default store is
// <repo>/workspace/db/fathom.sqlite — one the daemon never reads — and which
// against the shared store is the write INC-3 forbids.
func daemonRefreshRemedy(root, service string) string {
	return "fathom post-commit-hook --dir " + orDash(root) + " --service " + service + " --wait --timeout 2m"
}
