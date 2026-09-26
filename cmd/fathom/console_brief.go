package main

// console_brief.go — the shareable brief at /brief (vision 35 §3,
// Evidence). ?format=md still downloads the markdown; fathom never sends it
// anywhere (law 13).

import (
	"net/http"

	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "brief", Route: "/brief", Href: "/brief", Group: "Evidence", Order: 20, Nav: true,
		Title:    "Brief",
		Question: "What would I show someone who has never heard of fathom?",
		Shows:    []console.MetricID{console.MetricCalls, console.MetricObservations, console.MetricAgentTools},
		Formats:  []string{"md"},
		Live:     []string{"index", "sessions", "plans"},
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
		// The kit's link-mode ghost button (doctrine rule 1), at 44 px.
		Actions: `<span class="[&>a]:min-h-[44px]">` + form.RenderButton(form.Button{Label: "Download markdown", Href: "/brief?format=md", Variant: form.ButtonGhost}) + `</span>`,
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
