package gallery

import (
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/appkit/ui/palette"
)

// The command-palette specimen (catalog interaction.command-palette). /palette
// mounts the trigger, dialog and script; /palette/search is one app-owned route
// that answers palette.js with the results fragment and every other request with
// the full no-JavaScript search page. Its data is the Gallery's own navigation and
// the capability catalog, so each result is a real destination. A few reserved
// synthetic queries exercise states that live data would not produce on demand.
const (
	paletteRoute       = "/palette"
	paletteSearchRoute = "/palette/search"
	palettePerGroup    = 8
	paletteSlowDelay   = 1500 * time.Millisecond
	paletteStallDelay  = 7 * time.Second
	paletteTimeout     = 5 * time.Second
)

func registerPalette(mux *http.ServeMux) {
	mux.HandleFunc(paletteRoute, servePalettePage)
	mux.HandleFunc(paletteSearchRoute, servePaletteSearch)
}

func galleryPalette(theme string) palette.Palette {
	action := paletteSearchRoute
	if theme != "" {
		action += "?theme=" + theme
	}
	return palette.Palette{
		ID:          "gallery-palette",
		Action:      action,
		Label:       "Search the Gallery",
		Placeholder: "Pages and capabilities",
		IdleText:    "Search Gallery pages and kit capabilities by name or ID.",
		Timeout:     paletteTimeout,
	}
}

func withGalleryTheme(href, theme string) string {
	if theme == "" {
		return href
	}
	return href + "?theme=" + theme
}

// gallerySearch answers q from the Gallery's routes and the capability catalog.
// Reserved queries: "fail" (backend failure), "partial" (a source skipped),
// "unsearched" (no results, but a source skipped), "long" (long identifiers);
// "slow" and "stall" are delayed by the handler.
func gallerySearch(q, theme string) (palette.Results, int) {
	res := palette.Results{Query: q, Message: "Try a page name such as “overlays”, or a capability ID such as “interaction”."}
	needle := strings.ToLower(strings.TrimSpace(q))
	switch needle {
	case "":
		return palette.Results{Query: q}, http.StatusOK
	case "fail":
		res.Error = "The synthetic search backend refused this query. Nothing was searched."
		return res, http.StatusServiceUnavailable
	case "long":
		long := "internal/connector/extract/golang/callgraph/resolution/ReallyLongQualifiedSymbolNameWithoutSpaces"
		res.Groups = []palette.Group{{Heading: "Synthetic long identifiers", Results: []palette.Result{
			{Label: long, Href: withGalleryTheme(paletteRoute, theme), Detail: long + " · " + long, Meta: "Symbol"},
			{Label: "A short neighbour", Href: withGalleryTheme(paletteRoute, theme), Meta: "Symbol"},
		}}}
		return res, http.StatusOK
	case "partial":
		pages := palette.Group{Heading: "Gallery pages"}
		for _, link := range nav[:3] {
			pages.Results = append(pages.Results, palette.Result{Label: link.Label, Href: withGalleryTheme(link.Href, theme), Meta: "Page"})
		}
		res.Groups = []palette.Group{pages}
		res.Notice = "Synthetic partial answer: the capability catalog was not searched."
		return res, http.StatusOK
	case "unsearched":
		// Zero results while a source was skipped is not a definitive absence.
		res.Message = ""
		res.Notice = "Synthetic partial answer: no Gallery page matched and the capability catalog was not searched."
		return res, http.StatusOK
	}
	match := func(values ...string) bool {
		for _, v := range values {
			if strings.Contains(strings.ToLower(v), needle) {
				return true
			}
		}
		return false
	}
	pages := palette.Group{Heading: "Gallery pages"}
	for _, link := range nav {
		if len(pages.Results) < palettePerGroup && match(link.Label, link.Key) {
			pages.Results = append(pages.Results, palette.Result{Label: link.Label, Href: withGalleryTheme(link.Href, theme), Meta: "Page"})
		}
	}
	capabilities := palette.Group{Heading: "Capabilities"}
	if c, err := catalog.Default(); err == nil {
		for _, entry := range c.Entries {
			if len(capabilities.Results) < palettePerGroup && match(entry.ID, entry.Name) {
				capabilities.Results = append(capabilities.Results, palette.Result{
					Label: entry.Name, Href: withGalleryTheme("/catalog/"+url.PathEscape(entry.ID), theme),
					Detail: entry.ID, Meta: entry.Maturity,
				})
			}
		}
	}
	res.Groups = []palette.Group{pages, capabilities}
	return res, http.StatusOK
}

func servePaletteSearch(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != paletteSearchRoute {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query().Get("q")
	theme := sanitizeTheme(r.URL.Query().Get("theme"))
	w.Header().Add("Vary", palette.FragmentHeader)
	w.Header().Set("Cache-Control", "no-store")
	var delay time.Duration
	switch strings.ToLower(strings.TrimSpace(query)) {
	case "slow":
		delay = paletteSlowDelay
	case "stall":
		delay = paletteStallDelay
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-r.Context().Done(): // palette.js aborted a stale request
			timer.Stop()
			return
		}
	}
	results, status := gallerySearch(query, theme)
	if palette.WantsFragment(r) {
		if status != http.StatusOK {
			// palette.js shows its own failure template for any non-2xx answer.
			http.Error(w, "search failed", status)
			return
		}
		fragment, err := palette.RenderResults(results)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, fragment)
		return
	}
	results.HeadingLevel = 2
	form, err := palette.RenderSearchForm(galleryPalette(theme), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fragment, err := palette.RenderResults(results)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	content, err := htmlf("palette-search", paletteSearchHTML, struct {
		Form, Results template.HTML
		Back          string
	}{template.HTML(form), template.HTML(fragment), withGalleryTheme(paletteRoute, theme)})
	writeGalleryPage(w, r, "palette", "Search the Gallery", content, err, status)
}

func servePalettePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != paletteRoute {
		http.NotFound(w, r)
		return
	}
	theme := sanitizeTheme(r.URL.Query().Get("theme"))
	p := galleryPalette(theme)
	trigger, err := palette.RenderTrigger(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dialog, err := palette.RenderDialog(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	content, err := htmlf("palette", paletteHTML, struct {
		Trigger, Dialog template.HTML
		Script          template.JS
		SearchPage      string
	}{template.HTML(trigger), template.HTML(dialog), template.JS(palette.Script()), p.Action})
	writeGalleryPage(w, r, "palette", "Command palette", content, err, http.StatusOK)
}

// writeGalleryPage renders content in the shared shell with the section current
// and the theme links preserving the request's other parameters.
func writeGalleryPage(w http.ResponseWriter, r *http.Request, key, title string, content template.HTML, err error, status int) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	theme := sanitizeTheme(r.URL.Query().Get("theme"))
	links := append([]navLink(nil), nav...)
	for i := range links {
		links[i].Active = links[i].Key == key
		if theme != "" {
			links[i].Href += "?theme=" + theme
		}
	}
	withTheme := func(value string) string {
		q := r.URL.Query()
		q.Del("theme")
		if value != "" {
			q.Set("theme", value)
		}
		return r.URL.Path + "?" + q.Encode()
	}
	body, err := render(pageData{Title: title, Nav: links, Theme: theme, Page: key, Content: content, ThemeLight: withTheme("light"), ThemeDark: withTheme("dark"), ThemeSystem: withTheme("")})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

const paletteHTML = `
<h1 class="text-2xl font-semibold text-ink">Command palette</h1>
<p class="mt-2 max-w-2xl text-ink-muted"><code class="font-mono text-accent">appkit/ui/palette</code> — experimental <code class="font-mono">interaction.command-palette</code>. The trigger is a real link to a GET search page; the dependency-free <code class="font-mono">palette.js</code> (<code class="font-mono">window.AppKitPalette</code>) upgrades it into a native modal dialog built on <code class="font-mono">overlay.RenderDialog</code> whose results are server-rendered fragments from the same route.</p>

<div class="mt-6 flex flex-wrap items-center gap-3">{{.Trigger}}</div>

<h2 class="mt-8 text-lg font-medium text-ink">Keyboard contract</h2>
<ul class="mt-2 max-w-2xl list-disc space-y-1 pl-5 text-sm text-ink-muted">
  <li><kbd class="font-mono">Mod+K</kbd> (Command on Apple platforms, Control elsewhere) opens the palette from anywhere on this page and focuses the input.</li>
  <li>Typing fetches <code class="font-mono">{{.SearchPage}}</code> with <code class="font-mono">q=</code> after 150 ms; a newer query aborts the older request.</li>
  <li><kbd class="font-mono">↓</kbd>/<kbd class="font-mono">↑</kbd> move the highlighted result, <kbd class="font-mono">Enter</kbd> opens it, <kbd class="font-mono">Esc</kbd> closes and returns focus.</li>
  <li><kbd class="font-mono">Enter</kbd> with nothing current, or before new results arrive, opens the full search page for exactly what was typed.</li>
</ul>

<h2 class="mt-8 text-lg font-medium text-ink">States to try</h2>
<p class="mt-2 max-w-2xl text-sm text-ink-muted">Pages and capabilities are live Gallery data. These reserved queries are synthetic, so each honest state can be reproduced on demand:</p>
<dl class="mt-3 grid max-w-2xl gap-x-6 gap-y-2 text-sm sm:grid-cols-[auto_1fr]">
  <dt class="font-mono text-ink">overlay</dt><dd class="text-ink-muted">grouped results (pages and capabilities)</dd>
  <dt class="font-mono text-ink">zzz</dt><dd class="text-ink-muted">no results</dd>
  <dt class="font-mono text-ink">partial</dt><dd class="text-ink-muted">results with a partial-coverage notice</dd>
  <dt class="font-mono text-ink">unsearched</dt><dd class="text-ink-muted">no results while a source was not searched: partial, not a definitive absence</dd>
  <dt class="font-mono text-ink">long</dt><dd class="text-ink-muted">long identifiers that must wrap at narrow widths</dd>
  <dt class="font-mono text-ink">fail</dt><dd class="text-ink-muted">backend error (HTTP 503) and recovery by typing</dd>
  <dt class="font-mono text-ink">slow</dt><dd class="text-ink-muted">1.5 s response: busy indicator; keep typing to cancel it</dd>
  <dt class="font-mono text-ink">stall</dt><dd class="text-ink-muted">7 s response: the 5 s client timeout shows the error state</dd>
</dl>

<h2 class="mt-8 text-lg font-medium text-ink">Without JavaScript</h2>
<p class="mt-2 max-w-2xl text-sm text-ink-muted">The trigger navigates to <a href="{{.SearchPage}}" class="text-accent underline underline-offset-4">the full search page</a>, which renders <code class="font-mono">palette.RenderSearchForm</code> and the same <code class="font-mono">palette.RenderResults</code> fragment with <code class="font-mono">h2</code> group headings. <code class="font-mono">palette.WantsFragment</code> tells the two requests apart.</p>

{{.Dialog}}
<script>{{.Script}}</script>`

const paletteSearchHTML = `
<h1 class="text-2xl font-semibold text-ink">Search the Gallery</h1>
<p class="mt-2 max-w-2xl text-sm text-ink-muted">The command palette's no-JavaScript path: an ordinary GET form and the same server-rendered results. <a href="{{.Back}}" class="text-accent underline underline-offset-4">Back to the palette specimen</a>.</p>
<div class="mt-6 max-w-2xl">{{.Form}}</div>
<div class="mt-6 max-w-2xl">{{.Results}}</div>`
