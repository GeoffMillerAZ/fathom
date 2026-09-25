package gallery

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

type catalogView struct {
	Page        catalog.Page
	Query       catalog.Query
	Next        string
	Revision    string
	Theme       string
	DetailQuery string
	Reset       string
}

// registerCatalog uses one immutable catalog snapshot for HTML and JSON. An
// invalid embedded catalog is visible as an operational error, never an empty
// successful inventory. Paid corpora are neither needed nor read here.
func registerCatalog(mux *http.ServeMux) {
	c, loadErr := catalog.Default()
	ready := func(w http.ResponseWriter) bool {
		if loadErr != nil {
			http.Error(w, "catalog unavailable: "+loadErr.Error(), http.StatusInternalServerError)
			return false
		}
		return true
	}
	index := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/catalog" {
			http.NotFound(w, r)
			return
		}
		if !ready(w) {
			return
		}
		q, err := catalogQuery(r.URL.Query())
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}
		p, err := c.Search(q)
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}
		v := catalogView{Page: p, Query: q, Revision: c.Revision, Theme: sanitizeTheme(r.URL.Query().Get("theme")), DetailQuery: "?" + r.URL.Query().Encode(), Reset: previewURL("/catalog", sanitizeTheme(r.URL.Query().Get("theme")))}
		if p.NextCursor != "" {
			next := r.URL.Query()
			next.Set("cursor", p.NextCursor)
			v.Next = "/catalog?" + next.Encode()
		}
		content, err := htmlf("catalog", catalogIndexHTML, v)
		writeCatalogPage(w, r, "Capability catalog", content, err)
	}
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /catalog", index)
	mux.HandleFunc("GET /catalog/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		entry, ok := c.Get(r.PathValue("id"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		content, err := htmlf("capability", catalogEntryHTML, struct {
			Entry    catalog.Entry
			Revision string
			Back     string
			Preview  string
			Evidence string
			Quality  string
		}{entry, c.Revision, "/catalog?" + r.URL.Query().Encode(), previewURL(entry.Preview, sanitizeTheme(r.URL.Query().Get("theme"))), previewURL("/target-evidence?subject_id="+url.QueryEscape(entry.ID), sanitizeTheme(r.URL.Query().Get("theme"))), qualityCapabilityLink(entry.ID, sanitizeTheme(r.URL.Query().Get("theme")))})
		writeCatalogPage(w, r, entry.Name, content, err)
	})
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) {
		if ready(w) {
			writeCatalogJSON(w, r, c, c.Revision)
		}
	})
	mux.HandleFunc("GET /api/catalog/search", func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		q, err := catalogQuery(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		p, err := c.Search(q)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeCatalogJSON(w, r, p, "")
	})
	mux.HandleFunc("GET /api/catalog/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		e, ok := c.Get(r.PathValue("id"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeCatalogJSON(w, r, struct {
			Revision string        `json:"revision"`
			Entry    catalog.Entry `json:"entry"`
		}{c.Revision, e}, "")
	})
}

func catalogQuery(v url.Values) (catalog.Query, error) {
	q := catalog.Query{Text: v.Get("q"), Profile: v.Get("profile"), Layer: v.Get("layer"), Kind: v.Get("kind"), Maturity: v.Get("maturity"), Cursor: v.Get("cursor")}
	if v.Get("limit") != "" {
		n, err := strconv.Atoi(v.Get("limit"))
		if err != nil || n < 1 || n > 50 {
			return q, fmt.Errorf("limit must be an integer from 1 to 50")
		}
		q.Limit = n
	}
	return q, nil
}

func previewURL(path, theme string) string {
	if path == "" || theme == "" {
		return path
	}
	u, err := url.Parse(path)
	if err != nil {
		return ""
	}
	q := u.Query()
	q.Set("theme", theme)
	u.RawQuery = q.Encode()
	return u.String()
}

func writeCatalogJSON(w http.ResponseWriter, r *http.Request, v any, revision string) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "catalog encoding failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if revision != "" {
		tag := `"` + revision + `"`
		w.Header().Set("ETag", tag)
		w.Header().Set("Cache-Control", "no-cache")
		if r.Header.Get("If-None-Match") == tag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	_, _ = w.Write(body)
}

func writeCatalogPage(w http.ResponseWriter, r *http.Request, title string, content template.HTML, err error) {
	writeCatalogStatus(w, r, title, content, err, http.StatusOK)
}

func writeCatalogError(w http.ResponseWriter, r *http.Request, problem error) {
	content, err := htmlf("catalog-error", `<h1 class="text-2xl font-semibold">This catalog request needs a reset</h1><p class="mt-3 text-ink-muted">{{.}}</p><p class="mt-3 text-ink-muted">A catalog update can invalidate an old page cursor. Your application has not changed.</p><a href="/catalog" class="mt-5 inline-flex min-h-11 items-center text-accent underline">Clear filters and return to the catalog</a>`, problem.Error())
	writeCatalogStatus(w, r, "Reset catalog request", content, err, http.StatusBadRequest)
}

func writeCatalogStatus(w http.ResponseWriter, r *http.Request, title string, content template.HTML, err error, status int) {
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	theme := sanitizeTheme(r.URL.Query().Get("theme"))
	links := append([]navLink(nil), nav...)
	for i := range links {
		links[i].Active = links[i].Key == "catalog"
		if theme != "" {
			links[i].Href += "?theme=" + theme
		}
	}
	page := pageData{Title: title, Nav: links, Theme: theme, Page: "catalog", Content: content}
	withTheme := func(value string) string {
		q := r.URL.Query()
		q.Del("theme")
		if value != "" {
			q.Set("theme", value)
		}
		return r.URL.Path + "?" + q.Encode()
	}
	page.ThemeLight = withTheme("light")
	page.ThemeDark = withTheme("dark")
	page.ThemeSystem = withTheme("")
	body, err := render(page)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

const catalogIndexHTML = `
<div class="max-w-3xl">
  <p class="text-sm font-medium text-accent">FIRST-PARTY CAPABILITIES</p>
  <h1 class="mt-2 text-3xl font-semibold text-ink">What are you building?</h1>
  <p class="mt-3 text-ink-muted">Find a useful starting point. Check its runtime costs, see what your app still owns, and inspect the evidence before adopting.</p>
</div>
<form action="/catalog" method="get" role="search" aria-label="Capability catalog" class="mt-8 grid gap-4 rounded-lg border border-line bg-surface-raised p-4 sm:grid-cols-2 lg:grid-cols-4">
  {{if .Theme}}<input type="hidden" name="theme" value="{{.Theme}}">{{end}}
  <div class="sm:col-span-2"><label for="catalog-query" class="block text-sm font-medium">Task or capability</label><input id="catalog-query" name="q" type="search" value="{{.Query.Text}}" placeholder="Try navigation, analytics, or CLI" class="mt-1 min-h-11 w-full rounded-md border border-line bg-surface p-2 text-ink focus-visible:outline-2 focus-visible:outline-focus"></div>
  <div><label for="catalog-profile" class="block text-sm font-medium">Runtime profile</label><select id="catalog-profile" name="profile" class="mt-1 min-h-11 w-full rounded-md border border-line bg-surface p-2 text-ink focus-visible:outline-2 focus-visible:outline-focus"><option value="">Any profile</option>{{range profiles}}<option value="{{.}}"{{if eq . $.Query.Profile}} selected{{end}}>{{.}}</option>{{end}}</select></div>
  <div><label for="catalog-layer" class="block text-sm font-medium">Adoption layer</label><select id="catalog-layer" name="layer" class="mt-1 min-h-11 w-full rounded-md border border-line bg-surface p-2 text-ink focus-visible:outline-2 focus-visible:outline-focus"><option value="">Any layer</option>{{range layers}}<option value="{{.}}"{{if eq . $.Query.Layer}} selected{{end}}>{{.}}</option>{{end}}</select></div>
  {{if .Query.Kind}}<input type="hidden" name="kind" value="{{.Query.Kind}}">{{end}}{{if .Query.Maturity}}<input type="hidden" name="maturity" value="{{.Query.Maturity}}">{{end}}
  <div class="flex flex-wrap items-center gap-4 sm:col-span-2 lg:col-span-4"><button class="inline-flex min-h-11 items-center rounded-md bg-accent px-4 py-2 font-medium text-accent-ink focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Find capabilities</button><a href="{{.Reset}}" class="inline-flex min-h-11 items-center text-sm text-ink-muted underline underline-offset-4">Reset filters</a><span class="text-xs text-ink-muted">Web layers do not describe terminal runtime costs.</span></div>
</form>
<section class="mt-8" aria-labelledby="catalog-results">
  <div class="flex flex-wrap items-baseline justify-between gap-2 border-b border-line pb-3"><h2 id="catalog-results" class="text-lg font-medium">{{.Page.Total}} matching capabilities</h2><span class="text-xs text-ink-muted">Same records as the appkit MCP</span></div>
  {{range .Page.Items}}<article class="grid gap-3 border-b border-line py-5 sm:grid-cols-3">
    <div class="sm:col-span-2"><h3 class="font-semibold"><a href="{{capabilityURL .ID $.DetailQuery}}" class="inline-flex min-h-11 items-center text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-focus">{{.Name}}</a></h3><p class="mt-1 text-sm text-ink-muted">{{.Summary}}</p><p class="mt-2 break-all font-mono text-xs text-ink-muted">{{.ID}}</p></div>
    <div class="text-sm"><p>{{.Kind}} · {{.Maturity}}</p><p class="mt-1 text-ink-muted">{{range .Layers}}{{.}} {{end}}</p><p class="mt-1 text-ink-muted">{{range .Profiles}}{{.}} {{end}}</p></div>
  </article>{{else}}<div class="py-10"><h3 class="font-semibold">No compatible capability found</h3><p class="mt-2 max-w-2xl text-ink-muted">Try fewer task words or another profile. An empty result is useful evidence: do not force an incompatible component into your app. The current catalog does not yet describe every kit package.</p><a href="{{.Reset}}" class="mt-4 inline-flex min-h-11 items-center text-accent underline">Clear filters and explore the catalog</a></div>{{end}}
  {{if .Next}}<a href="{{.Next}}" class="mt-5 inline-flex min-h-11 items-center rounded-md border border-line px-4 py-2 text-accent">Next page →</a>{{end}}
</section>
<aside class="mt-8 rounded-lg bg-surface-raised p-5 text-sm" aria-label="Evidence and commercial references"><h2 class="font-medium">Choose the design reference as well as the renderer</h2><p class="mt-2 text-ink-muted">Available means an implementation exists. Proven requires named workflow evidence. A useful primitive does not replace reviewing a complete design.</p><p class="mt-3 text-ink-muted">Open the <a href="{{referenceLibraryURL .Theme}}" class="text-accent underline underline-offset-4">private reference library</a> for imported Tailwind Plus projects, dependencies and original source. MatDash and the separate snippet corpus remain available through MCP <code>search_components</code>. Check source fit and rights before adapting.</p><p class="mt-3 text-ink-muted">For bounded starter and selected on-demand planning, browse <a href="/quality{{if .Theme}}?theme={{.Theme}}{{end}}" class="inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">quality work</a>. Planning records are not target certification.</p></aside>
<details class="mt-6 text-xs text-ink-muted"><summary class="min-h-11 cursor-pointer py-3">Catalog revision and machine-readable inventory</summary><p class="mt-2 break-all font-mono">{{.Revision}}</p><a href="/api/catalog" class="mt-2 inline-flex min-h-11 items-center underline">Read the shared JSON snapshot</a></details>
`

const catalogEntryHTML = `
<a href="{{.Back}}" class="inline-flex min-h-11 items-center text-sm text-accent underline underline-offset-4">← Capability catalog</a>
<div class="mt-5 max-w-3xl"><p class="text-sm text-ink-muted">{{.Entry.Kind}} · {{.Entry.Maturity}} · first-party</p><h1 class="mt-2 text-3xl font-semibold">{{.Entry.Name}}</h1><p class="mt-3 text-lg text-ink-muted">{{.Entry.Summary}}</p><p class="mt-3 break-all font-mono text-xs text-ink-muted">{{.Entry.ID}}</p></div>
{{if .Preview}}<a href="{{.Preview}}" class="mt-5 inline-flex min-h-11 items-center rounded-md bg-accent px-4 py-2 font-medium text-accent-ink">Open live specimen →</a>{{else}}<p class="mt-5 text-sm text-ink-muted">No browser specimen is linked yet. Use the source and package proof entrypoints below; terminal behavior needs terminal evidence.</p>{{end}}
<p class="mt-3 text-sm"><a href="{{.Quality}}" class="inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Browse recorded quality work for this capability</a><span class="block text-ink-muted">No matching record is a planning result, not an adoption or certification claim.</span></p>
{{if eq .Entry.Kind "reference"}}<p class="mt-3 text-sm"><a href="{{.Evidence}}" class="inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus">Inspect retained target evidence and captures</a><span class="block max-w-3xl text-ink-muted">Requires the private library. Records may be absent; historical captures are not live specimens or certified targets.</span></p>{{end}}
<div class="mt-8 grid gap-8 lg:grid-cols-2">
<section aria-labelledby="cap-fit"><h2 id="cap-fit" class="text-lg font-semibold">Fit and integration</h2><dl class="mt-4 space-y-3 text-sm"><div><dt class="font-medium">Profiles</dt><dd class="text-ink-muted">{{range .Entry.Profiles}}{{.}} {{end}}</dd></div><div><dt class="font-medium">Web layers</dt><dd class="text-ink-muted">{{range .Entry.Layers}}{{.}} {{else}}Not applicable to this terminal capability{{end}}</dd></div><div><dt class="font-medium">Dependencies</dt><dd><ul class="mt-1 list-disc space-y-1 pl-5 text-ink-muted">{{range .Entry.Dependencies}}<li>{{.}}</li>{{end}}</ul></dd></div></dl><p class="mt-5 text-sm text-ink-muted">{{.Entry.Usage}}</p><h3 class="mt-5 font-medium">Source</h3><p class="mt-2 break-all font-mono text-xs">{{.Entry.Source.Path}}</p><p class="mt-2 font-mono text-xs text-ink-muted">{{range .Entry.Source.Symbols}}{{.}} {{end}}</p></section>
<section aria-labelledby="cap-boundary"><h2 id="cap-boundary" class="text-lg font-semibold">Your application owns</h2><ul class="mt-4 list-disc space-y-2 pl-5 text-sm text-ink-muted">{{range .Entry.AppOwns}}<li>{{.}}</li>{{end}}</ul><h3 class="mt-5 font-medium">States described</h3><ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-ink-muted">{{range .Entry.States}}<li>{{.}}</li>{{end}}</ul></section>
</div>
<section class="mt-8 border-t border-line pt-6" aria-labelledby="cap-evidence"><h2 id="cap-evidence" class="text-lg font-semibold">Evidence and known gaps</h2><p class="mt-2 text-sm text-ink-muted">These commands are reproducible entrypoints, not a claim of a current pass on your app or host.</p>{{range .Entry.Proof}}<pre class="mt-3 overflow-x-auto rounded-md bg-surface-sunken p-3 text-sm"><code>{{.}}</code></pre>{{end}}<ul class="mt-4 list-disc space-y-2 pl-5 text-sm text-ink-muted">{{range .Entry.Gaps}}<li>{{.}}</li>{{end}}</ul>{{if .Entry.Consumers}}<p class="mt-4 text-sm">Observed consumers: {{range .Entry.Consumers}}{{.}} · {{end}}<span class="text-ink-muted">integration history, not a fresh target certification.</span></p>{{end}}</section>
<details class="mt-6 text-xs text-ink-muted"><summary class="min-h-11 cursor-pointer py-3">Provenance and revision</summary><p class="mt-2">{{.Entry.Provenance.Source}} · {{.Entry.Provenance.Audience}}</p><p class="mt-2">{{.Entry.Provenance.License}}</p><p class="mt-2 break-all font-mono">{{.Revision}}</p></details>
`
