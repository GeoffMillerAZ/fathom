package main

// console_sessions.go — the Sessions pages (vision 35 §3, Evidence): the
// week at /sessions and one session's replay at /sessions/{id}. F0 moves
// both onto the shell; wave F rebuilds them as a session table and a
// windowed, filterable replay (OC-43).

import (
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "sessions", Route: "/sessions", Href: "/sessions", Group: "Evidence", Order: 10, Nav: true,
		Title: "Sessions", Aliases: []string{"story"},
		Question: "What did agents do, with fathom and without?",
		Shows:    []console.MetricID{console.MetricSessions, console.MetricObservations},
		Live:     []string{"sessions"},
		Build:    buildSessionsPage,
	})
	declarePage(consolePage{
		ID: "session", Route: "/sessions/{id}", Group: "Evidence", Order: 11, NavParent: "sessions",
		Title:    "Session",
		Question: "What happened in one session, event by event?",
		Live:     []string{"sessions"},
		Build:    buildSessionPage,
	})
}

// sessionsModel is the week page's JSON projection.
type sessionsModel struct {
	Week weekStory `json:"week"`
}

func buildSessionsPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	ws := buildWeekStory(cr.now)
	// The shared sessions metric is the model's one read of the ledger —
	// the read every other page shows (OC-9, ADR-045 decision 3). A fresh
	// read here would disagree with /impact for up to the model's TTL after
	// every Stop-hook row; the value's as_of names when the read was taken.
	metrics := consoleModelMetrics(m)
	sessions := metrics[console.MetricSessions]
	return consoleOut{
		Meta:    []string{"the week " + ws.From + " … " + ws.To, "generated " + m.GeneratedAt},
		Model:   sessionsModel{Week: ws},
		Metrics: pageMetrics(cr.page, metrics),
		Body: func() string {
			band := stat.RenderBand([]stat.Stat{valueStat("distinct sessions (all time)", sessions)}, stat.BandOptions{AriaLabel: "all sessions"})
			return obsContent("story", band+renderWeekStory(ws, m), m)
		},
	}, nil
}

func buildSessionPage(cr *consoleRequest) (consoleOut, error) {
	id := cr.r.PathValue("id")
	st := buildSessionStory(id)
	// The top bar's h1 truncates, so it names the session by its short id;
	// the meta line carries the full id as selectable text. The law-14
	// caveat is the body's callout, stated once (F0 proof, item 7).
	return consoleOut{
		Title: "Session " + shortSessionID(id),
		Meta:  []string{"session id " + id},
		Model: st,
		Body:  func() string { return renderSessionStory(st) },
	}, nil
}

// shortSessionID is a session id's first eight characters (a UUID's first
// group), or the whole id when it is short.
func shortSessionID(id string) string {
	if r := []rune(id); len(r) > 8 {
		return string(r[:8])
	}
	return id
}
