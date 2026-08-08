package main

// observatory_story.go — the story layer (vision 18 §3): one session as a
// timeline (IQ-4) and the trailing week composed from the instruments
// (IQ-5). Stories are lenses over mechanism-labeled events, never evidence
// (law 14) — both pages carry the banner.

import (
	"encoding/json"
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

	for _, b := range readBeatsFor(beatLogPath()) {
		if b.Meta["session"] == sessionID {
			st.Beats = append(st.Beats, b)
		}
	}
	if len(st.Beats) == 0 {
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
	b.WriteString(sectionHeading("session " + st.SessionID))
	b.WriteString(`<div class="mb-3 rounded-md border border-dashed border-caution px-3 py-2 text-ink-muted">a session replay is a lens over mechanism-labeled events, never evidence (law 14)</div>`)

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
		tl := viz.Timeline(events, viz.TimelineOptions{W: 900, AriaLabel: "session timeline"})
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: plural(len(st.Beats), "instrumented event"), Body: template.HTML(tl)}))
	} else {
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "timeline", Body: template.HTML(`<span class="italic text-ink-faint">` + html.EscapeString(st.BeatsNote) + `</span>`)}))
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
		card.WriteString(`<span class="italic text-ink-faint">` + html.EscapeString(st.LedgerNote) + `</span>`)
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
			http.Error(w, "session id required: /observatory/session?id=<session_id> (ids are in recent ledger rows — see /observatory/story)", http.StatusBadRequest)
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
	Sessions        int      `json:"sessions"`
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
		ws.Sessions++
		if e.VerifyCatches != nil {
			ws.CatchesRan++
			ws.Catches += *e.VerifyCatches
		}
		if e.FirstEditTurn != nil {
			ws.EditSessions++
		}
		if e.SessionID != "" && len(ws.RecentSessions) < 8 {
			ws.RecentSessions = append([]string{e.SessionID}, ws.RecentSessions...)
		}
	}
	return ws
}

func renderWeekStory(ws weekStory, m observatoryModel) string {
	var b strings.Builder
	b.WriteString(sectionHeading("the week — " + ws.From + " … " + ws.To))
	b.WriteString(`<div class="mb-3 rounded-md border border-dashed border-caution px-3 py-2 text-ink-muted">template-composed from the instruments; narrative is a lens, never evidence (law 14). The LLM-written weekly stays deferred (IQ-14) until the daemon has a model and two weeks of story rows.</div>`)

	b.WriteString(stat.RenderBand([]stat.Stat{
		{Label: "sessions", Value: fmtThousands(ws.Sessions)},
		{Label: "verify catches", Value: fmtThousands(ws.Catches), Sub: "verify ran in " + fmtThousands(ws.CatchesRan) + " of " + fmtThousands(ws.Sessions) + " sessions"},
		{Label: "sessions that edited", Value: fmtThousands(ws.EditSessions), Sub: "carry a first-edit turn"},
		{Label: "open observations (fleet)", Value: fmtThousands(fleetOpenObservations(m)), Sub: "friction filed, not yet fixed"},
	}, stat.BandOptions{AriaLabel: "week totals"}))

	if len(ws.RecentSessions) > 0 {
		var links strings.Builder
		for _, id := range ws.RecentSessions {
			links.WriteString(`<div class="py-0.5"><a class="text-accent font-mono text-xs" href="/observatory/session?id=` + html.EscapeString(id) + `">` + html.EscapeString(id) + `</a></div>`)
		}
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "recent sessions (newest first) — open one for its replay", Body: template.HTML(links.String())}))
	}
	if ws.Note != "" {
		b.WriteString(`<div class="mt-2 rounded-md border border-caution bg-caution-soft px-3 py-2">` + html.EscapeString(ws.Note) + `</div>`)
	}
	return b.String()
}

// fleetOpenObservations sums known open-observation counts (unknown repos
// contribute nothing — never zero-faked).
func fleetOpenObservations(m observatoryModel) int {
	total := 0
	for _, s := range m.Services {
		if s.OpenObservations != nil {
			total += *s.OpenObservations
		}
	}
	return total
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
