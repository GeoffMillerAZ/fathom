package gallery

import (
	"html/template"
	"net/http"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
)

// Each variant renders one real page heading, so the page keeps exactly one h1.
var pageHeadingVariants = []struct {
	Key, Label string
	Heading    chrome.PageHeading
}{
	{Key: "full", Label: "Full", Heading: chrome.PageHeading{
		ID:          "page-heading-title",
		Breadcrumbs: []chrome.Crumb{{Label: "Overview", Href: "/overview"}, {Label: "Specimens", Href: "/overview"}, {Label: "Page heading"}},
		Eyebrow:     "Project",
		Title:       "Plans for fathom",
		Meta:        []string{"12 proven", "3 open", "As of 12:03 UTC"},
		Actions: []form.Button{
			{Label: "Copy verify command", Type: "button", Variant: form.ButtonGhost},
			{Label: "Open ledger", Href: "/page-heading?variant=full#ledger"},
		},
	}},
	{Key: "minimal", Label: "Title only", Heading: chrome.PageHeading{Title: "Settings"}},
	{Key: "long", Label: "Long content", Heading: chrome.PageHeading{
		ID:          "page-heading-title",
		Breadcrumbs: []chrome.Crumb{{Label: "Overview", Href: "/overview"}, {Label: "A deliberately long breadcrumb label that must wrap", Href: "/overview"}, {Label: "Current"}},
		Eyebrow:     "Service · internal/semantic/obligations",
		Title:       "Governing rules for internal/semantic/obligations_rerank_windowed_served_tiers.go and its callers",
		Meta:        []string{"48 rules", "7 guides", "3 uncompiled", "Source: .fathom/layers/obligations.cue", "As of 2026-09-25 12:03 UTC"},
		Actions: []form.Button{
			{Label: "Re-rank", Type: "button"},
			{Label: "Compile guides", Type: "button", Variant: form.ButtonGhost},
			{Label: "Open evidence", Href: "/page-heading?variant=long#evidence", Variant: form.ButtonGhost},
			{Label: "Archive (unavailable)", Href: "javascript:void(0)", Variant: form.ButtonDanger},
		},
	}},
}

func servePageHeading(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/page-heading" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query()
	key := query.Get("variant")
	if key == "" {
		key = "full"
	}
	index := -1
	for i, v := range pageHeadingVariants {
		if v.Key == key {
			index = i
		}
	}
	if index < 0 || len(query["variant"]) > 1 {
		http.Error(w, "Unknown page-heading variant. Use full, minimal or long.", http.StatusBadRequest)
		return
	}
	heading, err := chrome.RenderPageHeading(pageHeadingVariants[index].Heading)
	if err != nil {
		http.Error(w, "The page-heading specimen could not be rendered.", http.StatusInternalServerError)
		return
	}
	theme := sanitizeTheme(query.Get("theme"))
	type choice struct {
		Label, Href string
		Current     bool
	}
	choices := make([]choice, 0, len(pageHeadingVariants))
	for i, v := range pageHeadingVariants {
		href := "/page-heading?variant=" + v.Key
		if theme != "" {
			href += "&theme=" + theme
		}
		choices = append(choices, choice{Label: v.Label, Href: href, Current: i == index})
	}
	content, err := htmlf("page-heading", pageHeadingHTML, struct {
		Heading template.HTML
		Choices []choice
	}{template.HTML(heading), choices})
	if err != nil {
		http.Error(w, "The page-heading specimen could not be rendered.", http.StatusInternalServerError)
		return
	}
	writeSpecimenPage(w, r, "page-heading", "Page heading", content, "variant="+key)
}

const pageHeadingHTML = `
{{.Heading}}
<nav aria-label="Heading variants" class="mt-8 flex flex-wrap gap-2 text-sm">
{{range .Choices}}<a href="{{.Href}}" class="inline-flex min-h-11 items-center rounded-md border border-line px-3 py-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus{{if .Current}} bg-surface-sunken font-semibold text-ink{{else}} text-ink-muted hover:bg-surface-sunken hover:text-ink{{end}}"{{if .Current}} aria-current="page"{{end}}>{{.Label}}</a>
{{end}}</nav>
<section aria-labelledby="page-heading-contract" class="mt-8 max-w-2xl">
<h2 id="page-heading-contract" class="text-lg font-medium text-ink">Contract</h2>
<p class="mt-2 text-sm text-ink-muted"><code class="font-mono">chrome.RenderPageHeading</code> renders the trail through <code class="font-mono">chrome.Breadcrumbs</code>, an optional eyebrow, the page's one <code class="font-mono">h1</code>, a wrapping meta list and up to four actions through <code class="font-mono">form.RenderButton</code>. Below the <code class="font-mono">sm</code> breakpoint the actions wrap onto their own row beneath the title. Every field is plain escaped text; an unsafe action destination renders the shared inert label. The actions here are specimens: they neither copy nor archive anything.</p>
<p id="ledger" class="mt-2 text-sm text-ink-muted">The ledger and evidence links only move to this paragraph.</p>
<p id="evidence" class="sr-only">Evidence anchor for the long variant.</p>
</section>`
