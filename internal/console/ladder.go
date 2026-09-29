package console

// ladder.go — the realization ladder (phase Y, docs/design/realization-
// grade.md; OC-25 / RG-5): wired → fresh → queried → preventing → gating,
// each rung a precondition for the next. The ladder is the one projection
// behind the realization grade (cmd/fathom's realizationGrade folds it into
// a letter), the project overview's rungs and `fathom fleet realization`,
// so the letter and the rungs cannot disagree.
//
// Each rung carries its evidence. A rung no instrument could judge is
// unknown and says why; a rung that does not apply (a repository with no
// plans has nothing to gate) is not applicable; neither counts against the
// repository. Only the FIRST unmet rung carries a remedy: five remedies for
// a ladder whose bottom rung is broken are noise.
//
// Every traffic rung reads one window (design §2: "in the window, not
// ever"): calls, obligations injections, sessions and the verify ledger's
// gate runs over the same days, so its evidence says which days it counts.

import (
	"fmt"
	"strings"
)

// Rung states.
const (
	RungMet           = "met"
	RungUnmet         = "unmet"
	RungNotApplicable = "not-applicable"
	RungUnknown       = "unknown"
)

// LadderFacts is what the ladder reads for one repository: its wiring and
// freshness notes and its traffic over the window. Pure: no clock, no I/O.
type LadderFacts struct {
	Service string
	// StanzaNote is "present" when the fathom stanza is in AGENTS.md, and
	// "present in <CLAUDE file> …" for the layout ADR-059 retired (still
	// wired); HooksNote is the wiring check's note (fleetWiringNotes).
	StanzaNote string
	HooksNote  string
	// Freshness is "fresh", "stale: …", or "unknown…"/"" when undeterminable.
	Freshness   string
	FreshRemedy string
	// The window's traffic. CallLogNote, when set, is why the call log was
	// not read: queried and preventing are then unknown, never unmet.
	WindowDays       int
	Calls            int
	ObligationServes int
	CallLogNote      string
	// Sessions are the distinct sessions in the window (the session
	// ledger); SessionsNote, when set, is why the ledger was not read — a
	// repository whose sessions are not measured is graded, never "idle".
	Sessions     int
	SessionsNote string
	// VerifyRuns are the gate runs in the window that the verify ledger
	// records (SD-1: the recorded-run signal that closed O30 — a pre-push
	// gate's runs never reach the session ledger); VerifyLedgerNote, when
	// set, is why that ledger was not read (gating is then unknown).
	VerifyRuns       int
	VerifyLedgerNote string
	// HasPlans says whether the gate has anything to check here;
	// PlansNote, when set, is why docs/plans was not read (gating is then
	// unknown — an unreadable directory is never "no plans").
	HasPlans  bool
	PlansNote string
}

// Rung is one step of the ladder.
type Rung struct {
	Name     string `json:"name"`
	Order    int    `json:"order"`
	State    string `json:"state"`
	Evidence string `json:"evidence"`
	// Remedy is set only on the first unmet rung.
	Remedy string `json:"remedy,omitempty"`
	fix    string
}

// Fix is the rung's own remedy whatever its position — what the
// realization grade's per-dimension record carries.
func (r Rung) Fix() string { return r.fix }

// Ladder is one repository's realization ladder.
type Ladder struct {
	Service string `json:"service"`
	Rungs   []Rung `json:"rungs"`
	// Grade is A–F with the first gap named ("B — not gating"), or "idle"
	// for a repository with no sessions in the window: idle is not F.
	Grade string `json:"grade"`
	Idle  bool   `json:"idle"`
	// SessionsNote names why the window's sessions were not measured (the
	// ladder is then graded, never idle, and Sessions is not a count).
	SessionsNote string `json:"sessions_note,omitempty"`
	// FirstUnmet names the first unmet rung and Remedy its fix; both stay
	// set on an idle repository, which is told, not graded.
	FirstUnmet string `json:"first_unmet,omitempty"`
	Remedy     string `json:"remedy,omitempty"`
	Realized   int    `json:"realized"`
	Available  int    `json:"available"`
	Sessions   int    `json:"sessions_in_window"`
}

// RealizationLadder builds one repository's ladder (OC-25, RG-5).
func RealizationLadder(f LadderFacts) Ladder {
	l := Ladder{Service: f.Service, Sessions: f.Sessions, SessionsNote: f.SessionsNote}
	add := func(name, state, evidence, fix string) {
		l.Rungs = append(l.Rungs, Rung{Name: name, Order: len(l.Rungs) + 1, State: state, Evidence: evidence, fix: fix})
	}
	window := "in the window"
	if f.WindowDays > 0 {
		window = fmt.Sprintf("in the last %d days", f.WindowDays)
	}

	// 1 — WIRED: can fathom operate here at all?
	hooksVisible := f.HooksNote != "" &&
		!strings.Contains(f.HooksNote, "user-scope or plugin wiring not visible") &&
		!strings.Contains(f.HooksNote, "not checkable")
	wired := strings.HasPrefix(f.StanzaNote, "present")
	wiredEvidence := "AGENTS.md stanza present"
	switch {
	case !wired:
		wiredEvidence = "no fathom stanza in AGENTS.md"
	case f.StanzaNote != "present":
		wiredEvidence = "fathom stanza " + f.StanzaNote
	}
	if hooksVisible {
		hooksOK := strings.Contains(f.HooksNote, "hook reference")
		wired = wired && hooksOK
		if !hooksOK {
			wiredEvidence = "no fathom hook is wired here"
		}
	} else {
		wiredEvidence += " (hook wiring not visible from here)"
	}
	add("wired", metState(wired), wiredEvidence, "fathom fleet reconcile --apply")

	// 2 — FRESH: are the answers current?
	switch {
	case f.Freshness == "fresh":
		add("fresh", RungMet, "index fresh at HEAD", f.FreshRemedy)
	case f.Freshness == "" || strings.HasPrefix(f.Freshness, "unknown"):
		add("fresh", RungUnknown, "freshness could not be determined ("+orText(f.Freshness, "—")+")", f.FreshRemedy)
	default:
		add("fresh", RungUnmet, f.Freshness, f.FreshRemedy)
	}

	// 3 — QUERIED: is anyone actually asking?
	queriedFix := "open a session here and ask a where/who-calls question — an index nobody queries realizes nothing"
	if f.CallLogNote != "" {
		add("queried", RungUnknown, "not measured — "+f.CallLogNote, queriedFix)
	} else {
		add("queried", metState(f.Calls > 0), fmt.Sprintf("%s call(s) %s", thousands(f.Calls), window), queriedFix)
	}

	// 4 — PREVENTING: is it acting BEFORE edits, not just answering?
	preventFix := "fathom capabilities  (the obligations row names what is missing here)"
	if f.CallLogNote != "" {
		add("preventing", RungUnknown, "not measured — "+f.CallLogNote, preventFix)
	} else {
		add("preventing", metState(f.ObligationServes > 0), fmt.Sprintf("%s rule injection(s) %s", thousands(f.ObligationServes), window), preventFix)
	}

	// 5 — GATING: is it catching claims?
	gateFix := "add `fathom verify --service " + f.Service + "` to the pre-push hook (lefthook.yml)"
	switch {
	case f.PlansNote != "":
		add("gating", RungUnknown, "not measured — "+f.PlansNote, gateFix)
	case !f.HasPlans:
		add("gating", RungNotApplicable, "no plans here for the gate to check", "")
	case f.VerifyLedgerNote != "":
		add("gating", RungUnknown, "not measured — "+f.VerifyLedgerNote, gateFix)
	default:
		add("gating", metState(f.VerifyRuns > 0), fmt.Sprintf("%s verify run(s) %s, recorded in the verify ledger", thousands(f.VerifyRuns), window), gateFix)
	}

	for i := range l.Rungs {
		switch l.Rungs[i].State {
		case RungMet:
			l.Available++
			l.Realized++
		case RungUnmet:
			l.Available++
			if l.FirstUnmet == "" {
				l.FirstUnmet = l.Rungs[i].Name
				l.Remedy = l.Rungs[i].fix
				l.Rungs[i].Remedy = l.Rungs[i].fix
			}
		}
	}
	if f.Sessions == 0 && f.SessionsNote == "" {
		l.Grade, l.Idle = "idle", true
		return l
	}
	l.Grade = gradeLetter(l.Realized, l.Available)
	if l.FirstUnmet != "" {
		l.Grade += " — not " + l.FirstUnmet
	}
	return l
}

func metState(ok bool) string {
	if ok {
		return RungMet
	}
	return RungUnmet
}

// gradeLetter folds realized over available into A–F; nothing available
// grades idle.
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
