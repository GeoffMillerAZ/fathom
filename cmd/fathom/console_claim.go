package main

// console_claim.go — one governed claim at /claims/{id} (design §3 "Claim
// page", plan OC-30): the statement, its citation excerpted through the
// kit's code viewer (code.RenderExcerpt, FATHOM-KIT-04: numbered lines, the
// cited range marked in words as well as tint, line anchors, a
// keyboard-scrollable viewport), scope, tier, status, provenance and every
// verdict recorded against it. The page's JSON section is `fathom claim
// show <id> --json`. A stale claim shows its compile-time anchor text,
// labeled, never the current lines as if they were its source (J2).

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/code"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/console"
)

const (
	verbClaimShow        = "fathom claim show {id} --json"
	verbClaimShowService = "fathom claim show {id} --service {svc} --json"
)

func init() {
	declarePage(consolePage{
		ID: "claim", Route: "/claims/{id}", Group: "Knowledge", Order: 2, NavParent: "layers",
		Title:    "Claim",
		Question: "What does this governed claim say, which lines does it cite, and what has happened to it since it was compiled?",
		Verbs:    []string{verbClaimShow, verbClaimShowService},
		Build:    buildClaimPage,
	})
}

// claimHref is a claim page, naming its service when one is given.
func claimHref(id, service string) string {
	h := "/claims/" + url.PathEscape(id)
	if service != "" {
		h += "?svc=" + url.QueryEscape(service)
	}
	return h
}

func buildClaimPage(cr *consoleRequest) (consoleOut, error) {
	id := cr.r.PathValue("id")
	svc := cr.r.URL.Query().Get("svc")
	src, err := knowledgeSourceFor(cr)
	if err != nil {
		return consoleOut{}, err
	}
	d, err := console.ClaimDetail(cr.r.Context(), src, id, svc)
	if err != nil {
		return consoleOut{}, err
	}
	if !d.Found && len(d.Matches) == 0 {
		reason := "No claim " + id + " is in this daemon's store."
		if svc != "" {
			reason = "Project " + svc + " holds no claim " + id + "."
		}
		return consoleOut{}, &consoleError{Status: http.StatusNotFound, Title: "Unknown claim", Reason: reason,
			Remedy: "fathom layers strata", RemedyHref: "/layers", RemedyLabel: "Open the semantic layers"}
	}
	verb := strings.ReplaceAll(verbClaimShow, "{id}", id)
	if svc != "" {
		verb = strings.ReplaceAll(strings.ReplaceAll(verbClaimShowService, "{id}", id), "{svc}", svc)
	}
	title := "Claim " + shortCommit(id)
	meta := []string{"claim id " + id}
	if d.Found {
		meta = append(meta, d.Layer+" layer", d.Service, "status "+d.Status)
	}
	return consoleOut{
		Title:    title,
		Meta:     meta,
		Verbs:    []string{verb},
		Sections: map[string]any{verb: d},
		Body:     func() string { return renderClaimPage(d, verb) },
	}, nil
}

func renderClaimPage(d console.ClaimPage, verb string) string {
	if len(d.Matches) > 0 {
		var list strings.Builder
		list.WriteString(`<ul class="flex flex-col gap-1">`)
		for _, m := range d.Matches {
			list.WriteString(`<li>` + knowledgeLink(claimHref(d.ClaimID, m.Service), m.Service) + ` <span class="text-sm text-ink-muted">` + html.EscapeString(m.Layer+" · "+m.Status) + `</span></li>`)
		}
		list.WriteString(`</ul>`)
		return renderPanel(consolePanel{
			ID: "matches", Title: "Held by several projects", SourceKey: "layers",
			Question: "A claim id is content-addressed, so two projects can hold the same one. Which project's copy?",
			Verb:     verb, Source: storeFooter,
			Denominator: plural(len(d.Matches), "project"),
			State:       console.StatePartial, Reason: "the id is ambiguous until a project is named",
			Body: list.String(),
		})
	}
	var b strings.Builder
	b.WriteString(`<div class="grid gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]"><div class="flex min-w-0 flex-col gap-4">`)
	statement := `<p class="text-base text-ink [overflow-wrap:anywhere]" data-claim-statement>` + html.EscapeString(d.Statement) + `</p>`
	if !d.Served {
		statement += `<p class="mt-2 text-sm font-medium text-caution">Not served: a ` + html.EscapeString(d.Status) + ` claim never reaches an agent again.</p>`
	} else if d.Status != "fresh" {
		statement += `<p class="mt-2 text-sm text-caution">Served flagged ` + html.EscapeString(d.Status) + `: its cited lines changed since compile.</p>`
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "statement", Title: "Statement", SourceKey: "layers:" + d.Service,
		Question:  "What does the agent read when this claim is served?",
		Verb:      verb,
		Source:    "semantic_claims",
		Freshness: "compiled " + d.CompiledAt,
		Body:      statement + renderClaimFacts(d),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "citation", Title: "Citation", SourceKey: "layers:" + d.Service,
		Question: "Which lines does the claim restate?",
		Verb:     verb,
		Source:   d.Excerpt.Source,
		State:    d.Excerpt.State, Reason: d.Excerpt.Reason,
		Body: renderClaimExcerpt(d),
	}))
	b.WriteString(`</div><div class="flex min-w-0 flex-col gap-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "events", Title: "Event history", SourceKey: "layers:" + d.Service,
		Question:    "What has happened to this claim since it was compiled?",
		Verb:        verb,
		Source:      "claim_events",
		Denominator: valuePlain(d.EventCount) + " recorded verdicts",
		State:       d.EventCount.State, Reason: d.EventCount.Reason,
		Body: renderClaimEvents(d),
	}))
	if len(d.Links) > 0 {
		var links strings.Builder
		links.WriteString(`<ul class="flex flex-col gap-2 text-sm">`)
		for _, l := range d.Links {
			target := `<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(l.Target) + `</span>`
			if l.Rel == "supersedes" {
				target = knowledgeLink(claimHref(l.Target, d.Service), shortCommit(l.Target))
			}
			links.WriteString(`<li class="min-w-0"><span class="font-semibold text-ink">` + html.EscapeString(l.Rel) + `</span> ` + target)
			if l.Note != "" && l.Rel != "cites" {
				links.WriteString(`<p class="text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(l.Note) + `</p>`)
			}
			links.WriteString(`</li>`)
		}
		links.WriteString(`</ul>`)
		b.WriteString(renderPanel(consolePanel{
			ID: "links", Title: "Links", SourceKey: "layers:" + d.Service, Question: "What does the claim scope, anchor to or supersede?",
			Verb: verb, Source: "claim_links", Denominator: plural(len(d.Links), "link"),
			Body: links.String(),
		}))
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

func renderClaimFacts(d console.ClaimPage) string {
	items := []chrome.DescItem{
		{Term: "project", DetailHTML: template.HTML(knowledgeLink(projectHref(d.Service), d.Service))},
		{Term: "layer", DetailHTML: template.HTML(knowledgeLink(layerHref(d.Layer), d.Layer))},
		{Term: "status", Detail: d.Status},
		{Term: "evidence tier", Detail: d.Tier},
		{Term: "scope", DetailHTML: template.HTML(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(d.Scope, ", ")) + `</span>`)},
		{Term: "subject", DetailHTML: template.HTML(`<span class="block min-w-0 font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(d.Subject) + `</span>`)},
	}
	if d.Model != "" {
		items = append(items, chrome.DescItem{Term: "compiled by", Detail: d.Model + " · " + d.PromptVersion})
	}
	if d.CompiledAtWatermark != "" {
		items = append(items, chrome.DescItem{Term: "corpus watermark", Detail: shortCommit(d.CompiledAtWatermark)})
	}
	valid := d.ValidFrom
	if d.ValidTo != "" {
		valid += " → " + d.ValidTo
	} else if valid != "" {
		valid += " → now"
	}
	if valid != "" {
		items = append(items, chrome.DescItem{Term: "valid", Detail: valid})
	}
	if d.AssertedBy != "" {
		items = append(items, chrome.DescItem{Term: "asserted by", DetailHTML: template.HTML(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(d.AssertedBy) + `</span>`)})
	}
	return `<div class="mt-3">` + chrome.RenderDescriptionList(items) + `</div>`
}

// renderClaimExcerpt draws the citation with the kit's code excerpt.
func renderClaimExcerpt(d console.ClaimPage) string {
	var b strings.Builder
	for _, c := range d.Citations {
		b.WriteString(`<p class="mb-2 font-mono text-xs text-ink-muted [overflow-wrap:anywhere]" data-citation>cites ` + html.EscapeString(c.Ref) + `</p>`)
	}
	ex := d.Excerpt
	if !ex.Known() {
		return b.String()
	}
	lines := make([]code.Line, 0, len(ex.V.Lines))
	for _, l := range ex.V.Lines {
		state := code.LineNormal
		if l.Cited {
			state = code.LineHighlighted
		}
		lines = append(lines, code.Line{Number: l.Number, Text: l.Text, State: state})
	}
	label := fmt.Sprintf("%s, lines %d to %d (%s)", ex.V.Path, ex.V.Start, ex.V.End, ex.V.From)
	excerpt, err := code.RenderExcerpt(code.Excerpt{
		Label: label, Caption: ex.V.Path + " · " + ex.V.From, AnchorPrefix: "cite-",
		Lines: lines, Labels: code.StateLabels{Highlighted: "Cited"},
	})
	if err != nil {
		b.WriteString(`<p class="text-sm text-danger" data-truth-line="error">the excerpt could not be drawn — ` + html.EscapeString(err.Error()) + `</p>`)
		return b.String()
	}
	if ex.V.Note != "" {
		b.WriteString(`<p class="mb-2 text-sm text-caution" data-excerpt-note>` + html.EscapeString(ex.V.Note) + `</p>`)
	}
	b.WriteString(excerpt)
	return b.String()
}

func renderClaimEvents(d console.ClaimPage) string {
	if len(d.Events) == 0 {
		if d.EventCount.Known() {
			return `<p class="text-sm text-ink-muted">No verdict recorded — the claim has not been re-anchored, contradicted, corroborated or refuted.</p>`
		}
		return ""
	}
	var rows []table.Row
	for i, e := range d.Events {
		note := e.Note
		if e.Resolution != "" {
			note = strings.TrimSpace(note + " (" + e.Resolution + " " + shortCommit(e.Commit) + ")")
		}
		rows = append(rows, table.Row{ID: fmt.Sprintf("event-%d", i), Cells: []string{e.At, e.Verdict, e.Author, note}})
	}
	return renderKitTable(table.TableParams{
		TableID: "claim-events", Caption: "Verdicts, oldest first",
		Columns: []table.Column{{Key: "at", Label: "At"}, {Key: "verdict", Label: "Verdict"}, {Key: "author", Label: "By"}, {Key: "note", Label: "Note"}},
		Rows:    rows,
	})
}
