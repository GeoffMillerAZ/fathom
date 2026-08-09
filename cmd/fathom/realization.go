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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	// currently realizing value.
	Calls            int
	Sessions         int
	ObligationServes int
	VerifyRuns       int
	// HasPlans says whether the gate has anything to check here. Without
	// it, every repository with no plans reads as "not gating", which is
	// a failure it cannot fix.
	HasPlans bool
}

func boolPtr(b bool) *bool { return &b }

// realizationGrade folds the five dimensions for one service.
//
// Pure over (row, window): same inputs, same letter. No clock, no I/O.
func realizationGrade(s obsServiceRow, w realizationWindow) realizationReport {
	rep := realizationReport{Service: s.Service, Sessions: w.Sessions}

	// 1 — WIRED: can fathom operate here at all?
	hooksVisible := s.HooksNote != "" &&
		!containsAny(s.HooksNote, "user-scope or plugin wiring not visible", "not checkable")
	wired := s.StanzaNote == "present"
	wiredEvidence := "CLAUDE.md stanza present"
	if s.StanzaNote != "present" {
		wiredEvidence = "no fathom stanza in CLAUDE.md"
	}
	if hooksVisible {
		hooksOK := containsAny(s.HooksNote, "hook reference")
		wired = wired && hooksOK
		if !hooksOK {
			wiredEvidence = "no fathom hook is wired here"
		}
	} else {
		wiredEvidence += " (hook wiring not visible from here)"
	}
	rep.Dims = append(rep.Dims, realizationDim{
		Name: "wired", Met: boolPtr(wired), Evidence: wiredEvidence,
		Remedy: "fathom fleet reconcile --apply",
	})

	// 2 — FRESH: are the answers current?
	fresh := s.Freshness == "fresh"
	freshDim := realizationDim{Name: "fresh", Evidence: "index fresh at HEAD"}
	switch {
	case fresh:
		freshDim.Met = boolPtr(true)
	case s.Freshness == "" || strings.HasPrefix(s.Freshness, "unknown"):
		// Nobody could tell. That is an absence of information, not a
		// defect, and grading it as one would punish every repository
		// whose daemon is merely unreachable.
		freshDim.Evidence = "freshness could not be determined (" + orDash(s.Freshness) + ")"
	default:
		freshDim.Met = boolPtr(false)
		freshDim.Evidence = s.Freshness
	}
	freshDim.Remedy = "cd " + orDash(s.Path) + " && fathom ingest --service " + s.Service
	rep.Dims = append(rep.Dims, freshDim)

	// 3 — QUERIED: is anyone actually asking?
	queried := w.Calls > 0
	rep.Dims = append(rep.Dims, realizationDim{
		Name: "queried", Met: boolPtr(queried),
		Evidence: fmt.Sprintf("%s call(s) in the window", fmtThousands(w.Calls)),
		Remedy:   "open a session here and ask a where/who-calls question — an index nobody queries realizes nothing",
	})

	// 4 — PREVENTING: is it acting BEFORE edits, not just answering?
	preventing := w.ObligationServes > 0
	rep.Dims = append(rep.Dims, realizationDim{
		Name: "preventing", Met: boolPtr(preventing),
		Evidence: fmt.Sprintf("%s rule injection(s) in the window", fmtThousands(w.ObligationServes)),
		Remedy:   "fathom capabilities  (the obligations row names what is missing here)",
	})

	// 5 — GATING: is it catching claims?
	gateDim := realizationDim{Name: "gating"}
	switch {
	case !w.HasPlans:
		// Not applicable: nothing to gate. Leaves the denominator.
		gateDim.Evidence = "no plans here for the gate to check"
	default:
		gateDim.Met = boolPtr(w.VerifyRuns > 0)
		gateDim.Evidence = fmt.Sprintf("gate ran in %s session(s) in the window", fmtThousands(w.VerifyRuns))
		gateDim.Remedy = "add `fathom verify --service " + s.Service + "` to the pre-push hook (lefthook.yml)"
	}
	rep.Dims = append(rep.Dims, gateDim)

	// Fold. Not-applicable dimensions leave the denominator entirely.
	for _, d := range rep.Dims {
		if d.Met == nil {
			continue
		}
		rep.Available++
		if *d.Met {
			rep.Realized++
		}
	}

	// A repository with no sessions in the window is IDLE, not failing. An
	// indexed reference corpus nobody codes in is working as intended, and
	// nine false alarms teach a reader to skim past the real one.
	if w.Sessions == 0 {
		rep.Grade = "idle"
		rep.FirstGap = ""
		rep.Remedy = ""
		return rep
	}

	rep.Grade = gradeLetter(rep.Realized, rep.Available)

	// The first BROKEN rung in ladder order, with its remedy. The letter
	// alone was never enough to act on.
	for _, d := range rep.Dims {
		if d.Met != nil && !*d.Met {
			rep.FirstGap = d.Name
			rep.Remedy = d.Remedy
			break
		}
	}
	if rep.FirstGap != "" {
		rep.Grade += " — not " + rep.FirstGap
	}
	return rep
}

func gradeLetter(realized, available int) string {
	if available == 0 {
		return "idle"
	}
	frac := float64(realized) / float64(available)
	switch {
	case frac >= 1:
		return "A"
	case frac >= 0.75:
		return "B"
	case frac >= 0.5:
		return "C"
	case frac >= 0.25:
		return "D"
	default:
		return "F"
	}
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

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
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
	if root == "" {
		return false
	}
	entries, err := os.ReadDir(filepath.Join(root, "docs", "plans"))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "README.md" {
			return true
		}
	}
	return false
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
