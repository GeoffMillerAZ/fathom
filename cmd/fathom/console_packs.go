package main

// console_packs.go — /packs (vision 35 §4.5 "the dialect matrix", design
// §3 "Packs", plan OC-31): dialect packs as a language × capability matrix
// — extraction engine, typed-edge producer and what the fleet's typed
// edges actually are (their resolver-confirmed share), fleet use,
// integrity reach, inventory keys, and precision and recall only where a
// published reading measured them, with the tier it measured
// ("unmeasured" otherwise, never a number) — and layer packs with their
// source and where each is active. The page's JSON section is `fathom
// report packs --json`.

import (
	"fmt"
	"html"
	"strings"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
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
		Meta:     []string{plural(len(m.Dialects), "registered language"), fmt.Sprintf("%d indexed in this fleet", indexed), pluralValue(m.LayerCount, "layer pack")},
		Actions:  knowledgeActions([][2]string{{"Semantic layers", "/layers"}}),
		Metrics:  pageMetrics(cr.page, console.Metrics{console.MetricClaims: m.Claims}),
		Sections: map[string]any{verbReportPacks: m},
		Body:     func() string { return renderPacksPage(m, indexed) },
	}, nil
}

// measuredText renders a published precision or recall with the edge tier
// it measured — a file-tier reading is not the language's scip tier — or
// "unmeasured".
func measuredText(v console.Value[float64], tier string) string {
	if !v.Known() {
		return `<span class="italic text-ink-muted" data-truth-state="absent" title="` + html.EscapeString(v.Reason) + `">unmeasured</span>`
	}
	return fmt.Sprintf("%.2f", v.V) + ` <span class="block text-xs text-ink-muted" data-reading-tier>` + html.EscapeString(tier) + ` tier</span>`
}

// typedEdgesText says where a language's typed edges could come from and
// what the fleet holds: a SCIP-level language with no resolver-confirmed
// edge says no index was imported, never only the producer's name.
func typedEdgesText(d console.DialectPack) string {
	edges := html.EscapeString(d.TypedEdges)
	if d.Producer != "" {
		edges += " (" + html.EscapeString(d.Producer) + ")"
	}
	if d.TypedEdges == "scip" && d.ResolvedEdges() == 0 && d.TypedShare.State != console.StatePartial {
		edges += `<span class="block text-xs text-caution" data-scip-none>no SCIP index imported</span>`
	}
	return edges
}

// shareText renders a language's typed-edge share as a percentage with its
// counts (the full denominator in its title), or the named absence.
func shareText(d console.DialectPack) string {
	v := d.TypedShare
	switch v.State {
	case console.StateAbsent:
		return `<span class="italic text-ink-muted" data-truth-state="absent" title="` + html.EscapeString(v.Reason) + `">no typed edges</span>`
	case console.StateError:
		return `<span class="text-danger" data-truth-state="error">unreadable — ` + html.EscapeString(v.Reason) + `</span>`
	}
	pct := fmt.Sprintf("%.0f%%", v.V*100)
	title := v.Denominator
	if v.State == console.StatePartial {
		pct, title = "≈ "+pct, v.Reason
	}
	total := 0
	for _, n := range d.EdgeKinds {
		total += n
	}
	return `<span title="` + html.EscapeString(title) + `">` + pct + `</span> <span class="block text-xs text-ink-muted">` +
		html.EscapeString(fmtThousands(d.ResolvedEdges())+" of "+fmtThousands(total)+" resolved") + `</span>`
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
		valueStat("layer packs", m.LayerCount),
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
	notes := ""
	for _, n := range m.Notes {
		notes += `<p class="mt-2 text-xs text-caution [overflow-wrap:anywhere]" data-truth-line="partial">` + html.EscapeString(n) + `</p>`
	}
	state, reason := console.StatePresent, ""
	if m.LayerCount.State == console.StatePartial {
		state, reason = console.StatePartial, m.LayerCount.Reason
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "layer-packs", Title: "Layer packs", SourceKey: "layers",
		Question:    "Which layer packs exist, where does each come from, and in which projects is it active?",
		Verb:        verbReportPacks,
		Source:      storeFooter + " — and each project's .fathom/layers",
		Denominator: valuePlain(m.Claims) + " current claims across " + pluralValue(m.LayerCount, "pack"),
		Freshness:   knowledgeFreshness,
		State:       state, Reason: reason,
		Body: renderLayerPackTable(m) + notes,
	}))
	b.WriteString(`</div>`)
	return b.String()
}

func renderDialectTable(m console.PacksView) string {
	var rows []table.Row
	for _, d := range m.Dialects {
		inv := strings.Join(d.Inventory, ", ")
		if inv == "" {
			inv = "—"
		}
		rows = append(rows, table.Row{
			ID:    "dialect-" + d.Language,
			Cells: []string{d.Name, d.Engine, "", "", "", "", d.Integrity, inv, "", ""},
			HTMLCells: []templ.Component{
				templ.Raw(`<span class="font-medium text-ink">` + html.EscapeString(d.Name) + `</span> <span class="block font-mono text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(d.Extensions, " ")) + `</span>`),
				nil, templ.Raw(typedEdgesText(d)), templ.Raw(shareText(d)), templ.Raw(valueText(d.Files)), templ.Raw(valueText(d.Projects)), nil, nil,
				templ.Raw(measuredText(d.Precision, d.ReadingTier)), templ.Raw(measuredText(d.Recall, d.ReadingTier)),
			},
			Muted: d.Files.State == console.StateZero,
		})
	}
	tbl := renderKitTable(table.TableParams{
		TableID: "dialect-matrix",
		Caption: "Languages × capabilities, most-indexed first (dimmed: registered, no file in this fleet)",
		Columns: []table.Column{
			{Key: "language", Label: "Language"}, {Key: "engine", Label: "Engine"}, {Key: "edges", Label: "Typed edges"},
			{Key: "share", Label: "Typed share"},
			{Key: "files", Label: "Files", Align: "right"}, {Key: "projects", Label: "Projects", Align: "right"},
			{Key: "integrity", Label: "Integrity"}, {Key: "inventory", Label: "Inventory keys"},
			{Key: "precision", Label: "Precision", Align: "right"}, {Key: "recall", Label: "Recall", Align: "right"},
		},
		Rows: rows,
	})
	return tbl + `<p class="mt-2 text-xs text-ink-muted">Typed edges: native — fathom resolves the types itself; scip — from an imported SCIP index (<code class="font-mono">fathom ingest --scip</code>), otherwise the file-resolved fallback; syntax — call names only. Typed share: of the call edges the store holds from this language's files, the share a resolver confirmed (a type checker or an imported SCIP index); file-resolved and unresolved edges are heuristic. Precision and recall name the tier their reading measured. Integrity: the conservation integrity plane checks Go tests and names other languages' tests unproven.</p>`
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
		// A pack that failed to load (or could not be looked up) is known
		// by name only: its stratum, role and gate are unknown, and the
		// reason travels with its name.
		unknown := `<span class="italic text-ink-muted">unknown</span>`
		stratum, role, gate := templ.Component(nil), templ.Component(nil), templ.Component(nil)
		name := knowledgeLink(layerHref(l.Name), l.Name)
		if l.Stratum == "" {
			stratum = templ.Raw(unknown)
		}
		if l.Role == "" {
			role, gate = templ.Raw(unknown), templ.Raw(unknown)
		}
		if l.Note != "" {
			name += `<span class="block text-xs text-caution [overflow-wrap:anywhere]" data-pack-note>` + html.EscapeString(l.Note) + `</span>`
		} else if l.Source == console.PackUnknown {
			name += `<span class="block text-xs text-caution [overflow-wrap:anywhere]" data-pack-note>` + html.EscapeString(l.Origin) + `</span>`
		}
		claims := claimsWord(l.Contributes)
		if l.Contributes.Known() {
			claims = knowledgeLink(claimsHref(l.Name, "", ""), valueText(l.Contributes))
		}
		rows = append(rows, table.Row{
			ID:    "pack-" + l.Name,
			Cells: []string{l.Name, "", l.Stratum, l.Role, evaled, active, installed, ""},
			HTMLCells: []templ.Component{
				templ.Raw(name), templ.Raw(sourceBadge(l.Source)), stratum, role, gate,
				nil, nil,
				templ.Raw(claims),
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
