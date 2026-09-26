package main

// console_packs.go — /packs (vision 35 §4.5 "the dialect matrix", design
// §3 "Packs", plan OC-31): dialect packs as a language × capability matrix
// — extraction engine, typed-edge producer, fleet use, integrity reach,
// inventory keys, and precision and recall only where a published reading
// measured them ("unmeasured" otherwise, never a number) — and layer packs
// with their source and where each is active. The page's JSON section is
// `fathom report packs --json`.

import (
	"fmt"
	"html"
	"strings"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/console"
)

const verbReportPacks = "fathom report packs --json"

func init() {
	declarePage(consolePage{
		ID: "packs", Route: "/packs", Href: "/packs", Group: "Knowledge", Order: 3, Nav: true,
		Title:    "Packs",
		Question: "Which dialect and layer packs exist, what does each contribute, and where is each active?",
		Verbs:    []string{verbReportPacks},
		Shows:    []console.MetricID{console.MetricClaims},
		Build:    buildPacksPage,
	})
}

func buildPacksPage(cr *consoleRequest) (consoleOut, error) {
	src, err := knowledgeSourceFor(cr)
	if err != nil {
		return consoleOut{}, err
	}
	m, err := console.PackMatrix(cr.r.Context(), src)
	if err != nil {
		return consoleOut{}, err
	}
	indexed := 0
	for _, d := range m.Dialects {
		if d.Files.V > 0 {
			indexed++
		}
	}
	return consoleOut{
		Meta:     []string{plural(len(m.Dialects), "registered language"), fmt.Sprintf("%d indexed in this fleet", indexed), plural(len(m.Layers), "layer pack")},
		Actions:  knowledgeActions([][2]string{{"Semantic layers", "/layers"}}),
		Metrics:  pageMetrics(cr.page, console.Metrics{console.MetricClaims: m.Claims}),
		Sections: map[string]any{verbReportPacks: m},
		Body:     func() string { return renderPacksPage(m, indexed) },
	}, nil
}

// measuredText renders a published precision or recall, or "unmeasured".
func measuredText(v console.Value[float64]) string {
	if !v.Known() {
		return `<span class="italic text-ink-muted" data-truth-state="absent" title="` + html.EscapeString(v.Reason) + `">unmeasured</span>`
	}
	return fmt.Sprintf("%.2f", v.V)
}

func renderPacksPage(m console.PacksView, indexed int) string {
	var b strings.Builder
	measured := 0
	for _, d := range m.Dialects {
		if d.Precision.Known() {
			measured++
		}
	}
	b.WriteString(stat.RenderBand([]stat.Stat{
		{Label: "languages registered", Value: fmtThousands(len(m.Dialects)), Sub: "one extractor each"},
		{Label: "indexed in this fleet", Value: fmtThousands(indexed), Sub: "languages with at least one file"},
		{Label: "with a measured reading", Value: fmtThousands(measured), Sub: "precision and recall published"},
		valueStat("current claims", m.Claims),
		{Label: "layer packs", Value: fmtThousands(len(m.Layers)), Sub: "embedded, project and kit"},
	}, stat.BandOptions{AriaLabel: "pack totals"}))
	b.WriteString(`<div class="mt-4 flex flex-col gap-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "dialects", Title: "Dialect packs", SourceKey: "index layers",
		Question:    "What does each language's extractor deliver: which engine, how its call edges are typed, which inventory it feeds, and how precise it has been measured to be?",
		Verb:        verbReportPacks,
		Source:      "the extractor registry (internal/connector), each project's indexed files, and the published readings below",
		Denominator: plural(len(m.Dialects), "registered language"),
		Freshness:   knowledgeFreshness,
		Body:        renderDialectTable(m) + renderReadings(m),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "layer-packs", Title: "Layer packs", SourceKey: "layers",
		Question:    "Which layer packs exist, where does each come from, and in which projects is it active?",
		Verb:        verbReportPacks,
		Source:      storeFooter + " — and each project's .fathom/layers",
		Denominator: valuePlain(m.Claims) + " current claims across " + plural(len(m.Layers), "pack"),
		Freshness:   knowledgeFreshness,
		Body:        renderLayerPackTable(m),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

func renderDialectTable(m console.PacksView) string {
	var rows []table.Row
	for _, d := range m.Dialects {
		edges := d.TypedEdges
		if d.Producer != "" {
			edges += " (" + d.Producer + ")"
		}
		inv := strings.Join(d.Inventory, ", ")
		if inv == "" {
			inv = "—"
		}
		rows = append(rows, table.Row{
			ID:    "dialect-" + d.Language,
			Cells: []string{d.Name, d.Engine, edges, "", "", d.Integrity, inv, "", ""},
			HTMLCells: []templ.Component{
				templ.Raw(`<span class="font-medium text-ink">` + html.EscapeString(d.Name) + `</span> <span class="block font-mono text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(d.Extensions, " ")) + `</span>`),
				nil, nil, templ.Raw(valueText(d.Files)), templ.Raw(valueText(d.Projects)), nil, nil,
				templ.Raw(measuredText(d.Precision)), templ.Raw(measuredText(d.Recall)),
			},
			Muted: d.Files.State == console.StateZero,
		})
	}
	tbl := renderKitTable(table.TableParams{
		TableID: "dialect-matrix",
		Caption: "Languages × capabilities, most-indexed first (dimmed: registered, no file in this fleet)",
		Columns: []table.Column{
			{Key: "language", Label: "Language"}, {Key: "engine", Label: "Engine"}, {Key: "edges", Label: "Typed edges"},
			{Key: "files", Label: "Files", Align: "right"}, {Key: "projects", Label: "Projects", Align: "right"},
			{Key: "integrity", Label: "Integrity"}, {Key: "inventory", Label: "Inventory keys"},
			{Key: "precision", Label: "Precision", Align: "right"}, {Key: "recall", Label: "Recall", Align: "right"},
		},
		Rows: rows,
	})
	return tbl + `<p class="mt-2 text-xs text-ink-muted">Typed edges: native — fathom resolves the types itself; scip — from an imported SCIP index (<code class="font-mono">fathom ingest --scip</code>), otherwise the file-resolved fallback; syntax — call names only. Integrity: the conservation integrity plane checks Go tests and names other languages' tests unproven.</p>`
}

func renderReadings(m console.PacksView) string {
	var b strings.Builder
	b.WriteString(`<h3 class="mt-4 text-sm font-semibold text-ink">Published readings</h3><ul class="mt-1 flex flex-col gap-2 text-sm" data-readings>`)
	for _, r := range m.Readings {
		b.WriteString(`<li class="min-w-0"><span class="font-medium text-ink">` + html.EscapeString(console.LanguageName(r.Language)) + `, ` + html.EscapeString(r.Tier) + ` tier</span> · precision ` +
			fmt.Sprintf("%.2f", r.Precision) + ` · recall ` + fmt.Sprintf("%.2f", r.Recall) +
			`<span class="block text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(r.Measure+" — "+r.Source+" ("+r.Date+")") + `</span></li>`)
	}
	b.WriteString(`</ul><p class="mt-2 text-xs text-ink-muted">Every other language is unmeasured: no reading has scored its edges against an independent truth, so no number is shown.</p>`)
	return b.String()
}

func renderLayerPackTable(m console.PacksView) string {
	var rows []table.Row
	for _, l := range m.Layers {
		active := "none"
		if len(l.ActiveIn) > 0 {
			active = strings.Join(l.ActiveIn, ", ")
		}
		installed := "—"
		if len(l.InstalledIn) > 0 {
			installed = strings.Join(l.InstalledIn, ", ")
		}
		evaled := "not gated"
		if l.Evaled {
			evaled = "passed"
		}
		rows = append(rows, table.Row{
			ID:    "pack-" + l.Name,
			Cells: []string{l.Name, "", l.Stratum, l.Role, evaled, active, installed, ""},
			HTMLCells: []templ.Component{
				templ.Raw(knowledgeLink(layerHref(l.Name), l.Name)), templ.Raw(sourceBadge(l.Source)), nil, nil, nil,
				nil, nil,
				templ.Raw(claimsWord(l.Contributes)),
			},
		})
	}
	return renderKitTable(table.TableParams{
		TableID: "layer-packs",
		Caption: "Layer packs and where each is active",
		Columns: []table.Column{
			{Key: "layer", Label: "Layer"}, {Key: "source", Label: "Source"}, {Key: "stratum", Label: "Stratum"},
			{Key: "role", Label: "Role"}, {Key: "eval", Label: "Eval gate"}, {Key: "active", Label: "Active in"},
			{Key: "installed", Label: "Installed in"}, {Key: "claims", Label: "Current claims", Align: "right"},
		},
		Rows: rows,
	})
}
