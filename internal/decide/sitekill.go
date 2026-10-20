package decide

// sitekill.go — KS-12 (docs/plans/decision-port.md section J, ADR-073
// decision 8, DMP-19): a site whose last 100 asks in decisions.jsonl hold
// more than 10% misses turns off for the process's lifetime, with one
// disclosed line per session and site (KS-14's), and never turns back on
// by itself. An ask is a decisions.jsonl row (one per answered question,
// one per failed call); a miss is a row with an error (a timeout, a 429 or
// 529, a refusal, any other provider error: RecordError's rows) and no
// kill_source. A row with a kill_source (the port closed first: a kill, the
// ask cap, a spent ceiling, this verdict) and a daemon's own closure (an
// error that is decision.ErrNoProvider: safe mode, the refuse rung) never
// reached the provider and is not an ask.
//
// The verdict is read from the existing file, never a new one or a table:
// at Open from the rows the Recorder was seeded with (the file's last
// SeedTailBytes, the one read openDecisionRecorder already makes), then on
// each ask from the in-process window each row this Recorder writes moves
// (at most siteKillWindow outcomes per site), never a re-read of the file.
// Rows other processes append after Open are judged by their own
// processes, and by the next Open.

import (
	"fmt"
	"slices"
	"strings"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// KillSourceMisses is the site kill's closing source (Gate.Source,
// decisions.jsonl kill_source).
const KillSourceMisses = "misses"

const (
	// siteKillWindow is how many of a site's last asks are judged.
	siteKillWindow = 100
	// siteKillMissPercent is the share of misses a site may hold: more
	// than this turns it off.
	siteKillMissPercent = 10
)

// missWindow is one site's last siteKillWindow ask outcomes, a ring.
type missWindow struct {
	missed [siteKillWindow]bool
	n      int // outcomes held, at most siteKillWindow
	next   int // the slot the next outcome overwrites
	misses int // misses among the held outcomes
}

func (w *missWindow) push(miss bool) {
	if w.n == siteKillWindow {
		if w.missed[w.next] {
			w.misses--
		}
	} else {
		w.n++
	}
	w.missed[w.next] = miss
	if miss {
		w.misses++
	}
	w.next = (w.next + 1) % siteKillWindow
}

// askOutcome classifies one decisions.jsonl row for the window: counted is
// false for a row the provider never saw, miss is true for an ask that
// failed.
func askOutcome(errText, killSource string) (counted, miss bool) {
	switch {
	case killSource != "":
		return false, false
	case errText == "":
		return true, false
	case strings.Contains(errText, decision.ErrNoProvider.Error()):
		return false, false
	case strings.HasPrefix(errText, notCalibratedText):
		return false, false // the calibration gate refused: the provider never saw it (SR-58)
	}
	return true, true
}

// siteKillVerdict judges site's window (KS-12): closed by KillSourceMisses
// when its last siteKillWindow asks hold more than siteKillMissPercent
// misses; a site with fewer asks is never judged.
func siteKillVerdict(site string, w *missWindow) Gate {
	if w == nil || w.n < siteKillWindow || w.misses*100 <= siteKillMissPercent*w.n {
		return Gate{}
	}
	return Gate{Closed: true, Source: KillSourceMisses, Layer: "log",
		Reason: fmt.Sprintf("site %s missed %d of its last %d asks in decisions.jsonl (more than %d%%), so it is off for this process's lifetime", site, w.misses, w.n, siteKillMissPercent)}
}

// observeLocked moves site's window by one decisions.jsonl row. Callers
// hold r.mu.
func (r *Recorder) observeLocked(site, errText, killSource string) {
	counted, miss := askOutcome(errText, killSource)
	if site == "" || !counted {
		return
	}
	if r.windows == nil {
		r.windows = map[string]*missWindow{}
	}
	w := r.windows[site]
	if w == nil {
		w = &missWindow{}
		r.windows[site] = w
	}
	w.push(miss)
}

// siteKill is site's kill verdict: once closed it stays closed for the
// Recorder's lifetime, whatever later rows say. A nil Recorder and the
// empty site judge nothing.
func (r *Recorder) siteKill(site string) Gate {
	if r == nil || site == "" {
		return Gate{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.killed[site]; ok {
		return g
	}
	g := siteKillVerdict(site, r.windows[site])
	if g.Closed {
		if r.killed == nil {
			r.killed = map[string]Gate{}
		}
		r.killed[site] = g
	}
	return g
}

// judgeSites judges every site the Recorder holds rows for (Open, KS-12).
func (r *Recorder) judgeSites() {
	for _, site := range r.sites() {
		r.siteKill(site)
	}
}

// sites is every site the Recorder holds a window for, sorted.
func (r *Recorder) sites() []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.windows))
	for site := range r.windows {
		out = append(out, site)
	}
	slices.Sort(out)
	return out
}

// KilledSites is every site the Recorder's rows turn off (KS-12), sorted:
// doctor seeds a Recorder from decisions.jsonl's tail and names each.
func (r *Recorder) KilledSites() []string {
	var out []string
	for _, site := range r.sites() {
		if r.siteKill(site).Closed {
			out = append(out, site)
		}
	}
	return out
}
