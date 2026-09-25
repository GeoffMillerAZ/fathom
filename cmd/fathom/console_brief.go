package main

// console_brief.go — the shareable brief at /brief (vision 35 §3,
// Evidence). ?format=md still downloads the markdown; fathom never sends it
// anywhere (law 13).

import (
	"net/http"

	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "brief", Route: "/brief", Href: "/brief", Group: "Evidence", Order: 20, Nav: true,
		Title:    "Brief",
		Question: "What would I show someone who has never heard of fathom?",
		Shows:    []console.MetricID{console.MetricObservations},
		Formats:  []string{"md"},
		Build:    buildBriefPage,
	})
}

// briefModel is the brief page's JSON projection: the markdown itself.
type briefModel struct {
	Markdown string `json:"markdown"`
}

func buildBriefPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	md := renderBriefMD(m)
	return consoleOut{
		Meta:    modelMeta(m),
		Model:   briefModel{Markdown: md},
		Metrics: pageMetrics(cr.page, consoleModelMetrics(m)),
		Actions: `<a class="inline-flex min-h-11 items-center rounded-md border border-line bg-surface px-3 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="/brief?format=md">Download markdown</a>`,
		Raw: func(w http.ResponseWriter) bool {
			if cr.r.URL.Query().Get("format") != "md" {
				return false
			}
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="fathom-brief.md"`)
			_, _ = w.Write([]byte(md))
			return true
		},
		Body: func() string {
			return obsContent("brief", `<div class="max-w-3xl rounded-lg border border-line bg-surface-raised p-5">`+mdToHTML(md)+`</div>`, m)
		},
	}, nil
}
