package main

// console_claims.go — /claims, the rows behind every claim count (vision
// 35 §5: "every number drills down. A count links to its rows"). The
// strata, the coverage matrix, the layer page and the packs page count
// governed claims; each count links here with its filter (layer, project,
// status) in the URL, and the list answers with the claims it counted, 50 a
// page, each opening its claim page. The page's JSON section is `fathom
// claim list --json` with the same filter.

import (
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/console"
)

// verbClaimList is the list verb; bracketed flags appear only when the
// request sets them (verbsFillTemplates reads the brackets as optional).
const verbClaimList = "fathom claim list [--layer {layer}] [--service {svc}] --status {status} [--page {page}] --json"

func init() {
	declarePage(consolePage{
		ID: "claims", Route: "/claims", Href: "/claims", Group: "Knowledge", Order: 2, NavParent: "layers",
		Title:    "Claims",
		Question: "Which governed claims does a count count — in this layer, this project, this status — and what does each say?",
		Verbs:    []string{verbClaimList},
		Build:    buildClaimsPage,
	})
}

// claimsHref is the claim list for a filter; empty parts are every layer,
// every project, and current claims.
func claimsHref(layer, service, status string) string {
	v := url.Values{}
	if layer != "" {
		v.Set("layer", layer)
	}
	if service != "" {
		v.Set("svc", service)
	}
	if status != "" && status != console.ClaimStatusCurrent {
		v.Set("status", status)
	}
	if len(v) == 0 {
		return "/claims"
	}
	return "/claims?" + v.Encode()
}

// claimListVerb is the exact command for a filter and page.
func claimListVerb(f console.ClaimFilter, page int) string {
	var b strings.Builder
	b.WriteString("fathom claim list")
	if f.Layer != "" {
		b.WriteString(" --layer " + shellWord(f.Layer))
	}
	if f.Service != "" {
		b.WriteString(" --service " + shellWord(f.Service))
	}
	b.WriteString(" --status " + f.Status)
	if page > 1 {
		b.WriteString(" --page " + strconv.Itoa(page))
	}
	return b.String() + " --json"
}

func buildClaimsPage(cr *consoleRequest) (consoleOut, error) {
	src, err := knowledgeSourceFor(cr)
	if err != nil {
		return consoleOut{}, err
	}
	q := cr.r.URL.Query()
	f := console.ClaimFilter{Layer: strings.TrimSpace(q.Get("layer")), Service: strings.TrimSpace(q.Get("svc")), Status: strings.TrimSpace(q.Get("status"))}
	if f.Status == "" {
		f.Status = console.ClaimStatusCurrent
	}
	if !console.ValidClaimStatus(f.Status) {
		return consoleOut{}, &consoleError{Status: http.StatusBadRequest, Title: "Unknown status",
			Reason: "status must be one of " + strings.Join(console.ClaimStatusFilters, ", ") + "; got " + f.Status + ".",
			Remedy: "fathom claim list --status current", RemedyHref: "/claims", RemedyLabel: "List current claims"}
	}
	page := 1
	if p, perr := strconv.Atoi(q.Get("page")); perr == nil && p > 1 {
		page = p
	}
	l, err := console.ListClaims(cr.r.Context(), src, f, page)
	if err != nil {
		return consoleOut{}, err
	}
	// A filter naming nothing the fleet knows is an error, never a
	// measured zero rows.
	if f.Layer != "" && !containsString(l.Layers, f.Layer) {
		return consoleOut{}, &consoleError{Status: http.StatusNotFound, Title: "Unknown layer",
			Reason: "No pack defines a layer named " + f.Layer + ", and the store holds no claim or compile run for it.",
			Remedy: "fathom layers strata", RemedyHref: "/layers", RemedyLabel: "Open the semantic layers"}
	}
	if f.Service != "" && !containsString(l.Services, f.Service) {
		return consoleOut{}, &consoleError{Status: http.StatusNotFound, Title: "Unknown project",
			Reason: "No project in this store is named " + f.Service + ".",
			Remedy: "fathom layers coverage", RemedyHref: "/claims", RemedyLabel: "List every project's claims"}
	}
	verb := claimListVerb(l.Filter, l.Page)
	return consoleOut{
		Meta:     []string{pluralValue(l.Total, "claim"), strings.TrimPrefix(l.Total.Denominator, "claims matching ")},
		Actions:  knowledgeActions([][2]string{{"Semantic layers", "/layers"}}),
		Verbs:    []string{verb},
		Sections: map[string]any{verb: l},
		Body:     func() string { return renderClaimsFilter(l) + renderClaimsList(l, verb) },
	}, nil
}

// renderClaimsFilter is the filter: a GET form, so every list is a URL.
func renderClaimsFilter(l console.ClaimList) string {
	layers := []form.Option{{Value: "", Label: "Every layer"}}
	for _, n := range l.Layers {
		layers = append(layers, form.Option{Value: n, Label: n})
	}
	projects := []form.Option{{Value: "", Label: "Every project"}}
	for _, s := range l.Services {
		projects = append(projects, form.Option{Value: s, Label: s})
	}
	statuses := make([]form.Option, 0, len(console.ClaimStatusFilters))
	for _, s := range console.ClaimStatusFilters {
		label := s
		switch s {
		case console.ClaimStatusCurrent:
			label = "current — fresh, stale, contradicted"
		case console.ClaimStatusAll:
			label = "any status"
		}
		statuses = append(statuses, form.Option{Value: s, Label: label})
	}
	field := func(label, id, name, value string, opts []form.Option) string {
		return form.RenderField(form.Field{Label: label, For: id, Control: template.HTML(form.RenderSelect(form.Select{ID: id, Name: name, Value: value, Options: opts}))})
	}
	return `<form method="get" action="/claims" class="mb-4 grid items-start gap-3 rounded-lg border border-line bg-surface-raised p-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_auto] [&_select]:min-h-11" aria-label="Filter claims" data-claims-form>` +
		field("Layer", "claims-layer", "layer", l.Filter.Layer, layers) +
		field("Project", "claims-svc", "svc", l.Filter.Service, projects) +
		field("Status", "claims-status", "status", l.Filter.Status, statuses) +
		`<div class="md:pt-6 [&>button]:min-h-11 [&>button]:w-full">` + form.RenderButton(form.Button{Label: "List claims"}) + `</div></form>`
}

// renderClaimsList is the page's table, paged.
func renderClaimsList(l console.ClaimList, verb string) string {
	var body strings.Builder
	if len(l.Claims) == 0 {
		body.WriteString(chrome.RenderEmptyState(chrome.EmptyState{Heading: "No claim matches", Body: "No " + l.Total.Denominator + " is in this store."}))
	} else {
		var rows []table.Row
		lo := (l.Page-1)*l.PerPage + 1
		for _, c := range l.Claims {
			cites := c.Cites
			if cites == "" {
				cites = "—"
			}
			rows = append(rows, table.Row{
				ID:    "claim-" + c.Service + "-" + c.ClaimID,
				Cells: []string{"", c.Service, c.Layer, c.Status, c.Tier, cites},
				HTMLCells: []templ.Component{
					templ.Raw(`<a href="` + html.EscapeString(claimHref(c.ClaimID, c.Service)) + `" class="inline-flex min-h-11 items-center rounded-sm text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(c.Statement) + `</a>`),
					templ.Raw(knowledgeLink(projectHref(c.Service), c.Service)),
					templ.Raw(knowledgeLink(layerHref(c.Layer), c.Layer)),
					nil, nil,
					templ.Raw(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(cites) + `</span>`),
				},
				Muted: c.Status == "superseded" || c.Status == "refuted",
			})
		}
		body.WriteString(renderKitTable(table.TableParams{
			TableID: "claims",
			Caption: "Claims " + fmtThousands(lo) + "–" + fmtThousands(lo+len(l.Claims)-1) + " of " + valuePlain(l.Total) + ", by project, layer and status",
			Columns: []table.Column{
				{Key: "statement", Label: "Statement"}, {Key: "project", Label: "Project"}, {Key: "layer", Label: "Layer"},
				{Key: "status", Label: "Status"}, {Key: "tier", Label: "Tier"}, {Key: "cites", Label: "Cites"},
			},
			Rows: rows,
		}))
		if l.Pages > 1 {
			// The kit's pager links are 44 px targets themselves
			// (FATHOM-F0-KIT-05); the div only spaces it.
			body.WriteString(`<div class="mt-3">` + chrome.RenderPagination(chrome.Pagination{
				Page: l.Page, TotalPages: l.Pages, AriaLabel: "claim pages",
				HrefFor: func(p int) string {
					h := claimsHref(l.Filter.Layer, l.Filter.Service, l.Filter.Status)
					sep := "?"
					if strings.Contains(h, "?") {
						sep = "&"
					}
					return h + sep + "page=" + strconv.Itoa(p) + "#claims"
				},
			}) + `</div>`)
		}
	}
	return renderPanel(consolePanel{
		ID: "claims", Title: "Claims", SourceKey: "layers",
		Question:    "Which claims are these? Each opens its claim page: the statement, the lines it cites and what happened to it since.",
		Verb:        verb,
		Source:      storeFooter,
		Denominator: valuePlain(l.Total) + " " + l.Total.Denominator,
		Freshness:   knowledgeFreshness,
		State:       l.Total.State, Reason: l.Total.Reason,
		Body: body.String(),
	})
}
