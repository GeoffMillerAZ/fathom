package main

// console_statusz.go — OC-6: /statusz on the console shell. The survey found
// it off the shell (hand-written HTML with its own inline CSS, no nav, no
// landmarks) and silent about the case its own code comment calls unknown:
// typed edges showed for 3 of 18 services and the other 15 rendered nothing,
// which reads as healthy. Now every service row states its typed-edge pass —
// ran in this process, replayed from the store (a pass from before this
// daemon started), failed with a reason, does not apply (no Go module at the
// root: the pass never runs, so it never records), or UNKNOWN because no
// pass is recorded anywhere — and the JSON at the same path is the v3
// report, byte-for-byte the shape it always was.

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "statusz", Route: "/statusz", Href: "/statusz", Group: "System", Order: 0, Nav: true,
		Title:    "Daemon status",
		NavLabel: "Statusz",
		Question: "Is this daemon healthy, which binary is it, and is every service's index fresh?",
		Build:    buildStatuszPage,
	})
}

func buildStatuszPage(cr *consoleRequest) (consoleOut, error) {
	if cr.deps.c == nil {
		return consoleOut{}, &consoleError{Status: http.StatusServiceUnavailable, Title: "Store unreadable",
			Reason: "this daemon has no store open", Remedy: "fathom serve-http --store <path>"}
	}
	report := buildStatuszReport(cr.r.Context(), cr.deps.c, cr.deps.startTime, cr.now)
	return consoleOut{
		Meta: []string{"generated " + report.GeneratedAt, "binary " + statuszBinaryLabel(report.StatuszBuildInfo)},
		// /statusz?format=json is the v3 contract (TestStatuszSchemaVersionPinned):
		// it answers with the report itself, never the console envelope.
		Raw: func(w http.ResponseWriter) bool {
			if !wantsJSON(cr.r) {
				return false
			}
			w.Header().Set("Content-Type", "application/json")
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			_ = enc.Encode(report)
			return true
		},
		Body: func() string { return renderStatusz(report, cr.deps.selfURL) },
	}, nil
}

// statuszServiceCount is the shared services metric over statusz's own
// source (the store's service list) — the consistency test holds it equal
// to the registry count every other page shows.
func statuszServiceCount(report StatuszReport) console.Value[int] {
	f := console.FleetFacts{}
	for _, s := range report.Services {
		f.Services = append(f.Services, console.ServiceFact{Name: s.Service})
	}
	return console.ServiceCount(f)
}

// renderStatusz is the page body: a daemon panel and a service table, each
// a renderPanel with its question, verb and source. selfURL is this
// daemon's own address, so the verb chip names the endpoint that answered.
func renderStatusz(report StatuszReport, selfURL string) string {
	verb := "curl -s " + selfURL + "/statusz?format=json"
	var b strings.Builder
	daemon := chrome.RenderDescriptionList([]chrome.DescItem{
		{Term: "generated", Detail: report.GeneratedAt},
		{Term: "uptime", Detail: humanUptime(report.UptimeSec)},
		{Term: "binary", Detail: statuszBinaryLabel(report.StatuszBuildInfo)},
		{Term: "channels", Detail: report.ChannelsNote},
	})
	b.WriteString(renderPanel(consolePanel{
		ID: "daemon", Title: "Daemon", SourceKey: "daemon",
		Question:  "Which binary is serving, and for how long?",
		Verb:      verb,
		Source:    "this daemon process (internal/buildinfo, its own environment)",
		Freshness: "live — computed for this request",
		Body:      daemon,
	}))

	services := statuszServiceCount(report)
	unknownTyped := 0
	rows := make([]table.Row, 0, len(report.Services))
	for _, s := range report.Services {
		commit := shortCommit(s.IndexedCommit)
		if commit == "" {
			commit = s.FreshnessNote
		}
		typed, known := statuszTypedEdgesCell(s.TypedEdges, s.TypedEdgesAbsence)
		if !known {
			unknownTyped++
		}
		rows = append(rows, table.Row{
			ID:        "svc-" + s.Service,
			Href:      projectHref(s.Service),
			Label:     "Open " + s.Service,
			Cells:     []string{s.Service, commit, "", ""},
			HTMLCells: []templ.Component{nil, nil, templ.Raw(typed), templ.Raw(statuszCompileCell(s.LastCompilePerLayer))},
		})
	}
	tbl := renderKitTable(table.TableParams{
		TableID: "statusz-services",
		Caption: fmt.Sprintf("%d services in this daemon's store", len(report.Services)),
		Columns: []table.Column{
			{Key: "service", Label: "Service"},
			{Key: "commit", Label: "Indexed commit"},
			{Key: "typed", Label: "Typed edges"},
			{Key: "compile", Label: "Last compile per layer"},
		},
		Rows: rows,
	})

	state, reason := console.StatePresent, ""
	if unknownTyped > 0 {
		state = console.StatePartial
		reason = fmt.Sprintf("the typed-edge pass is unknown for %d of %d services — each row names why; unknown is not a pass", unknownTyped, len(report.Services))
	}
	b.WriteString(`<div class="mt-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "services", Title: "Services", SourceKey: "index:*",
		Question:    "Is every service indexed, and did its typed call-edge pass run?",
		Verb:        verb,
		Source:      "the daemon's store (service_meta, compile_runs, typed_edge_status), this process's typed-edge passes and each service root's go.mod",
		Denominator: valuePlain(services) + " services",
		Freshness:   "live — computed for this request",
		State:       state, Reason: reason,
		// The kit table scrolls inside its own positioned wrapper at 390 px
		// (FATHOM-F0-KIT-02); the page never pans sideways.
		Body: tbl,
	}))
	b.WriteString(`</div>`)
	return b.String()
}

// statuszTypedEdgesCell renders one service's typed-edge pass. Unknown is
// said in words and marked, never left blank (BP-35's defect class: a
// degradation that reads healthy); a pass that cannot apply says so and is
// not counted as unknown. known is false only for an unknown.
func statuszTypedEdgesCell(te *StatuszTypedEdges, absence statuszTypedAbsence) (cell string, known bool) {
	if te == nil {
		if absence.NotApplicable {
			return `<span class="inline-block min-w-48 text-ink-muted" data-truth-state="absent">` + html.EscapeString(absence.Reason) + `</span>`, true
		}
		reason := absence.Reason
		if reason == "" {
			reason = "no typed-edge pass is recorded for this service"
		}
		return `<span class="inline-block min-w-48 italic text-ink-muted" data-truth-state="absent">unknown — ` + html.EscapeString(reason) + `; never read as a pass</span>`, false
	}
	if te.Failed {
		return `<span class="font-medium text-danger" data-truth-state="error">FAILED — ` + html.EscapeString(te.Reason) + `</span>`, true
	}
	cell = html.EscapeString(plural(te.Packages, "package"))
	if te.Persisted {
		cell += ` <span class="text-ink-muted">(recorded ` + html.EscapeString(te.RecordedAt) + ` by an earlier daemon process, replayed from the store — this daemon has not run the pass since it started)</span>`
	}
	return cell, true
}

// statuszCompileCell renders the last compile per layer, one line per layer.
func statuszCompileCell(per map[string]string) string {
	if len(per) == 0 {
		return `<span class="italic text-ink-muted">no compiled layers</span>`
	}
	layers := make([]string, 0, len(per))
	for l := range per {
		layers = append(layers, l)
	}
	sort.Strings(layers)
	var b strings.Builder
	for _, l := range layers {
		b.WriteString(`<div class="min-w-64 font-mono text-xs [overflow-wrap:anywhere]"><span class="font-semibold text-ink">` + html.EscapeString(l) + `</span> <span class="text-ink-muted">` + html.EscapeString(per[l]) + `</span></div>`)
	}
	return b.String()
}

// humanUptime renders seconds as "6h 18m" (the survey: a raw "22700.0s").
func humanUptime(sec float64) string {
	d := time.Duration(sec * float64(time.Second)).Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
