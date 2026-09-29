package main

// console_pageviews.go — OC-15 (vision 35 §9, UX-M4): console page views
// are counted per route so that, after 30 days, a page nobody opened can be
// reviewed under law 8's consumer-population rule. The count is keyed by the
// page's route PATTERN ("/projects/{svc}"), never the concrete URL: which
// project or session a person opened is not the measurement, and it stays
// out of the log. Views are HTML page renders — and a page's content opened
// elsewhere by a person (the plan page's drawer fetching a row's evidence
// as ?format=fragment, consoleOut.RawIsView); ?format=json reads are
// machine traffic and are not views, nor is a palette fragment (a
// keystroke) or a HEAD.
//
// Two sinks: an in-memory counter per route (exposed on /metrics as
// fathom_console_page_views_total{route=…}) and one JSONL row per view in
// ~/.fathom/console-views.jsonl (user-central like the session ledger; law
// 13: local, never shared) so the 30-day window survives restarts.
// Best-effort by construction: a view that cannot be logged still renders.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
)

// consolePageViews counts views.
type consolePageViews struct {
	mu      sync.Mutex
	counts  map[string]int64
	logPath string // "" disables the durable log
}

func newConsolePageViews(logPath string) *consolePageViews {
	return &consolePageViews{counts: map[string]int64{}, logPath: logPath}
}

// consoleViewsLogPath is the durable log: FATHOM_CONSOLE_VIEWS, else
// ~/.fathom/console-views.jsonl.
func consoleViewsLogPath() string {
	return fathomhome.PerFile("FATHOM_CONSOLE_VIEWS", "console-views.jsonl")
}

// consoleViewRow is one durable view row: the route pattern and the time.
type consoleViewRow struct {
	SchemaVersion int    `json:"schema_version"`
	At            string `json:"at"`
	Route         string `json:"route"`
	Page          string `json:"page"`
}

// countPageView records one view of the page registered at route.
func countPageView(v *consolePageViews, page *consolePage, now time.Time) {
	if v == nil || page == nil {
		return
	}
	v.mu.Lock()
	v.counts[page.Route]++
	path := v.logPath
	v.mu.Unlock()
	if path == "" {
		return
	}
	row, err := json.Marshal(consoleViewRow{SchemaVersion: 1, At: now.UTC().Format(time.RFC3339), Route: page.Route, Page: page.ID})
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(row, '\n'))
}

// snapshot returns the per-route counts, sorted by route.
func (v *consolePageViews) snapshot() [][2]any {
	v.mu.Lock()
	defer v.mu.Unlock()
	routes := make([]string, 0, len(v.counts))
	for r := range v.counts {
		routes = append(routes, r)
	}
	sort.Strings(routes)
	out := make([][2]any, 0, len(routes))
	for _, r := range routes {
		out = append(out, [2]any{r, v.counts[r]})
	}
	return out
}

// writeMetrics appends the per-route view counters to a /metrics body.
func (v *consolePageViews) writeMetrics(w io.Writer) {
	if v == nil {
		return
	}
	for _, rc := range v.snapshot() {
		fmt.Fprintf(w, "fathom_console_page_views_total{route=%q} %d\n", rc[0], rc[1])
	}
}
