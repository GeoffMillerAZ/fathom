package main

// observatory_story.go — the story layer (vision 18 §3): one session as a
// timeline (IQ-4) and the trailing week composed from the instruments
// (IQ-5). Stories are lenses over mechanism-labeled events, never evidence
// (law 14) — both pages carry the banner.

import (
	"encoding/json"
	"errors"
	"html"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
)

// sessionStory is one session's replay model (IQ-4).
type sessionStory struct {
	SessionID  string              `json:"session_id"`
	Ledger     *sessionLedgerEntry `json:"ledger,omitempty"`
	LedgerNote string              `json:"ledger_note,omitempty"`
	Beats      []heartbeat.Beat    `json:"beats,omitempty"`
	BeatsNote  string              `json:"beats_note,omitempty"`
}

// buildSessionStory folds the user-central ledger and beat log for one
// session id. Absences are named (law 3).
func buildSessionStory(sessionID string) sessionStory {
	st := sessionStory{SessionID: sessionID}

	if data, err := os.ReadFile(sessionLedgerPath()); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			var e sessionLedgerEntry
			if json.Unmarshal([]byte(line), &e) == nil && e.SessionID == sessionID {
				cp := e
				st.Ledger = &cp // last row wins: the final verdict for the session
			}
		}
	}
	if st.Ledger == nil {
		st.LedgerNote = "no ledger row for this session (the stop hook writes one at session end)"
	}

	// The session's lines by offset (RT-114), never the whole beat log.
	lines, _, berr := hooksBeatIndex.lines(beatLogPath(), sessionID)
	for _, line := range lines {
		var b heartbeat.Beat
		if json.Unmarshal(line, &b) == nil && b.Meta["session"] == sessionID {
			st.Beats = append(st.Beats, b)
		}
	}
	switch {
	case berr != nil && !errors.Is(berr, os.ErrNotExist):
		st.BeatsNote = "the hooks log could not be read (" + berr.Error() + ") — beats are not measured, never none"
	case len(st.Beats) == 0:
		st.BeatsNote = "no beats recorded for this session id"
	}
	sort.Slice(st.Beats, func(i, j int) bool { return st.Beats[i].At.Before(st.Beats[j].At) })
	return st
}

// beatRole maps a channel to a token role for the timeline dot.
func beatRole(channel, kind string) string {
	if kind == "error" {
		return "danger"
	}
	switch channel {
	case "edit-hook":
		return "accent"
	case "session-hook":
		return "info"
	case "stop-hook":
		return "caution"
	case "discovery":
		return "positive"
	}
	return "ink-muted"
}

func renderSessionStory(st sessionStory) string {
	var b strings.Builder
	// The page heading names the session; an uppercased section label
	// repeated the id in a case that no longer matched it (F0 proof).
	b.WriteString(`<div class="mb-3 rounded-md border border-dashed border-caution px-3 py-2 text-ink-muted" data-law14>a session replay is a lens over mechanism-labeled events, never evidence (law 14)</div>`)

	if len(st.Beats) > 0 {
		events := make([]viz.Event, 0, len(st.Beats))
		for _, beat := range st.Beats {
			label := beat.Channel
			if beat.Kind != "" {
				label += ":" + beat.Kind
			}
			if svc := beat.Meta["service"]; svc != "" {
				label += " (" + svc + ")"
			}
			events = append(events, viz.Event{At: beat.At, Label: label, Role: beatRole(beat.Channel, beat.Kind), Lane: beat.Channel})
		}
		tl := viz.Timeline(events, viz.TimelineOptions{W: 900, Fluid: true, AriaLabel: "session timeline"})
		// A text-dense chart, drawn once with the kit's Fluid option: from
		// md up it fills the card (capped at its native 900 px, so its
		// labels never scale up); below md it keeps its native width and
		// scrolls inside its own keyboard-reachable region, so its labels
		// stay legible and the page never pans sideways at 390 px (design
		// §2; the kit's own advice for text-dense charts, FATHOM-F0-KIT-04).
		tl = `<div class="overflow-x-auto" role="region" tabindex="0" aria-label="session timeline — scrolls sideways"><div class="min-w-[900px] max-w-[900px] md:min-w-0">` + tl + `</div></div>`
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: plural(len(st.Beats), "instrumented event"), Body: template.HTML(tl)}))
	} else {
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "timeline", Body: template.HTML(`<span class="italic text-ink-muted">` + html.EscapeString(st.BeatsNote) + `</span>`)}))
	}

	var card strings.Builder
	if st.Ledger != nil {
		e := st.Ledger
		card.WriteString(kv("repo", html.EscapeString(e.Repo)))
		card.WriteString(kv("arm", html.EscapeString(e.Arm)))
		card.WriteString(kv("turns", metricCell(e.TurnCount, e.TurnNote, "")))
		card.WriteString(kv("first edit at turn", metricCell(e.FirstEditTurn, e.FirstEditNote, "")))
		card.WriteString(kv("tokens (harness-reported)", metricCell(e.TokensTotal, e.TokensNote, "")))
		card.WriteString(kv("verify catches", metricCell(e.VerifyCatches, "verify did not run", "")))
		card.WriteString(kv("discovery actions", metricCell(e.DiscoveryActionsTotal, e.JoinNote, "")))
	} else {
		card.WriteString(`<span class="italic text-ink-muted">` + html.EscapeString(st.LedgerNote) + `</span>`)
	}
	b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "the session's ledger row", Body: template.HTML(card.String())}))
	return b.String()
}

func observatorySessionPage(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "session id required: /sessions/<session_id> (ids are in recent ledger rows — see /sessions)", http.StatusBadRequest)
			return
		}
		st := buildSessionStory(id)
		if wantsJSON(r) {
			writeObsJSON(w, st)
			return
		}
		m := cachedObservatoryModel(baseURL, time.Now())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom · session "+id, "story", renderSessionStory(st), m)))
	}
}

// ─── IQ-5: the week ──────────────────────────────────────────────────────

// weekStory is the trailing-7-day composition.
type weekStory struct {
	From            string   `json:"from"`
	To              string   `json:"to"`
	Sessions        int      `json:"sessions"`    // distinct sessions (last ledger row per id), never rows
	LedgerRows      int      `json:"ledger_rows"` // in-window ledger rows the sessions were deduped from
	Catches         int      `json:"catches"`
	CatchesRan      int      `json:"catches_ran"`
	EditSessions    int      `json:"edit_sessions"`
	RecentSessions  []string `json:"recent_sessions,omitempty"` // newest first, capped
	NewObservations int      `json:"new_observations"`
	Note            string   `json:"note,omitempty"`
}

func buildWeekStory(now time.Time) weekStory {
	from := now.UTC().AddDate(0, 0, -7)
	ws := weekStory{From: from.Format("2006-01-02"), To: now.UTC().Format("2006-01-02")}
	data, err := os.ReadFile(sessionLedgerPath())
	if err != nil {
		ws.Note = "session ledger not readable — the week cannot be composed, and that is stated rather than faked"
		return ws
	}
	// One session per id, keeping its LAST in-window row — the impact
	// page's rule (observatory.go, "ONE ROW PER SESSION"): Claude Code's
	// Stop fires at every turn, so a long session appends many rows and
	// the last one has seen the most. Counting rows read "1,053 sessions"
	// for far fewer, and the recent list repeated one id eight times.
	type sessionAt struct {
		e sessionLedgerEntry
		t time.Time
	}
	latest := map[string]sessionAt{}
	var order []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var e sessionLedgerEntry
		if json.Unmarshal([]byte(line), &e) != nil || e.At == "" {
			continue
		}
		t, terr := time.Parse(time.RFC3339, e.At)
		if terr != nil || t.Before(from) {
			continue
		}
		ws.LedgerRows++
		key := e.SessionID
		if key == "" {
			// No id to dedupe on: counted once on its own terms, never
			// merged with every other anonymous row (impact's rule).
			key = "\x00anon:" + e.At + ":" + e.Repo
		}
		if _, seen := latest[key]; !seen {
			order = append(order, key)
		}
		latest[key] = sessionAt{e: e, t: t}
	}
	recent := make([]sessionAt, 0, len(order))
	for _, key := range order {
		s := latest[key]
		ws.Sessions++
		if s.e.VerifyCatches != nil {
			ws.CatchesRan++
			ws.Catches += *s.e.VerifyCatches
		}
		if s.e.FirstEditTurn != nil {
			ws.EditSessions++
		}
		if s.e.SessionID != "" {
			recent = append(recent, s)
		}
	}
	// Newest first by each session's own latest row; ties by id, so the
	// list is deterministic.
	sort.Slice(recent, func(i, j int) bool {
		if !recent[i].t.Equal(recent[j].t) {
			return recent[i].t.After(recent[j].t)
		}
		return recent[i].e.SessionID < recent[j].e.SessionID
	})
	for _, s := range recent {
		if len(ws.RecentSessions) == 8 {
			break
		}
		ws.RecentSessions = append(ws.RecentSessions, s.e.SessionID)
	}
	return ws
}

func renderWeekStory(ws weekStory, m observatoryModel) string {
	var b strings.Builder
	b.WriteString(sectionHeading("the week — " + ws.From + " … " + ws.To))
	b.WriteString(`<div class="mb-3 rounded-md border border-dashed border-caution px-3 py-2 text-ink-muted">template-composed from the instruments; narrative is a lens, never evidence (law 14). The LLM-written weekly stays deferred (IQ-14) until the daemon has a model and two weeks of story rows.</div>`)

	observations := fleetOpenObservations(m)
	// The backlog is a current fleet snapshot, not observations filed this week.
	observations.Label = "open observations (fleet snapshot)"
	b.WriteString(stat.RenderBand([]stat.Stat{
		{Label: "sessions", Value: fmtThousands(ws.Sessions), Sub: "distinct sessions, from " + fmtThousands(ws.LedgerRows) + " ledger rows"},
		{Label: "verify catches", Value: fmtThousands(ws.Catches), Sub: "verify ran in " + fmtThousands(ws.CatchesRan) + " of " + fmtThousands(ws.Sessions) + " sessions"},
		{Label: "sessions that edited", Value: fmtThousands(ws.EditSessions), Sub: "carry a first-edit turn"},
	}, stat.BandOptions{AriaLabel: "week totals"}))
	// This snapshot has a different time grain; give its coverage room to wrap
	// without crowding the week's session metrics in the pinned kit's band.
	b.WriteString(stat.RenderBand([]stat.Stat{observations}, stat.BandOptions{AriaLabel: "fleet snapshot"}))

	if len(ws.RecentSessions) > 0 {
		var links strings.Builder
		for _, id := range ws.RecentSessions {
			links.WriteString(`<div><a class="inline-flex min-h-11 items-center text-accent font-mono text-xs [overflow-wrap:anywhere]" href="` + html.EscapeString(sessionHref(id)) + `">` + html.EscapeString(id) + `</a></div>`)
		}
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "recent sessions (newest first) — open one for its replay", Body: template.HTML(links.String())}))
	}
	if ws.Note != "" {
		b.WriteString(`<div class="mt-2 rounded-md border border-caution bg-caution-soft px-3 py-2">` + html.EscapeString(ws.Note) + `</div>`)
	}
	return b.String()
}

// fleetOpenObservations projects the existing per-service evidence without
// losing its coverage: no known sources is absent, partial coverage is a named
// lower bound, and only complete coverage permits an exact total (including 0).
// The number is the shared projection (console.OpenObservations, OC-9); the
// kit renders the declared semantics, not their validity.
func fleetOpenObservations(m observatoryModel) stat.Stat {
	return valueStat("open observations", console.OpenObservations(fleetFactsFromModel(m)))
}

func observatoryStoryPage(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		now := time.Now()
		ws := buildWeekStory(now)
		if wantsJSON(r) {
			writeObsJSON(w, ws)
			return
		}
		m := cachedObservatoryModel(baseURL, now)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom · the week", "story", renderWeekStory(ws, m), m)))
	}
}
