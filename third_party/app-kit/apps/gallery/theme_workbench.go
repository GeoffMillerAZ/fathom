package gallery

import (
	"html/template"
	"net/http"
	"net/url"

	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/copper"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/geoffindigo"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/geoffsynthwave"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/lagoon"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/orchard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/synthwave"
)

type workbenchChoice struct{ ID, Name string }
type themeWorkbenchView struct {
	Palette, Theme, Specimen, Effect, Name, Description string
	DarkOnly                                            bool
	LabAvailable                                        bool
	Palettes, Specimens                                 []workbenchChoice
	Content                                             template.HTML
}

// The workbench selects first-party assets and real specimens only. It never
// parses uploaded CSS/CUE, writes app preferences, or loads website content.
func registerThemeWorkbench(mux *http.ServeMux, labAvailable bool) {
	for name, css := range map[string]string{
		"synthwave": synthwave.CSS(), "copper": copper.CSS(), "lagoon": lagoon.CSS(), "orchard": orchard.CSS(), "geoff-indigo": geoffindigo.CSS(), "geoff-synthwave": geoffsynthwave.CSS(),
	} {
		mux.HandleFunc("GET /static/themes/"+name+".css", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write([]byte(css))
		})
	}
	mux.Handle("GET /static/themes/geoff-neon.css", geoffsynthwave.NeonHandler())
	mux.HandleFunc("GET /themes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		// First-party SVG renderers use reviewed inline style attributes. This
		// permission is not shared with private-source/converted-document routes.
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; style-src 'self' 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.EscapedPath() != "/themes" || len(r.URL.RawQuery) > 2048 {
			http.Error(w, "Invalid workbench selection", 400)
			return
		}
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			http.Error(w, "Invalid workbench selection", 400)
			return
		}
		for k, v := range q {
			allowed := k == "palette" || k == "theme" || k == "specimen" || k == "effect"
			if len(v) != 1 || !allowed {
				http.Error(w, "Invalid workbench selection", 400)
				return
			}
		}
		view := themeWorkbenchView{Palette: q.Get("palette"), Theme: q.Get("theme"), Specimen: q.Get("specimen"), Effect: q.Get("effect"),
			LabAvailable: labAvailable,
			Palettes:     []workbenchChoice{{"geoff-synthwave", "Geoff Synthwave"}, {"geoff-indigo", "Geoff Indigo"}, {"synthwave", "Kit Synthwave"}, {"copper", "Copper"}, {"lagoon", "Lagoon"}, {"orchard", "Orchard"}, {"appkit", "App-kit default"}},
			Specimens:    []workbenchChoice{{"dashboard", "Dashboard"}, {"components", "Charts and components"}, {"forms", "Form controls"}, {"tokens", "Token swatches"}},
		}
		if view.Palette == "" {
			view.Palette = "geoff-synthwave"
		}
		if view.Specimen == "" {
			view.Specimen = "dashboard"
		}
		if view.Effect == "" {
			view.Effect = "none"
			if view.Palette == "geoff-synthwave" {
				view.Effect = "neon"
			}
		}
		if (view.Theme != "" && view.Theme != "light" && view.Theme != "dark") || (view.Effect != "none" && view.Effect != "neon") {
			http.Error(w, "Invalid workbench selection", 400)
			return
		}
		switch view.Palette {
		case "geoff-synthwave":
			view.Name, view.Description, view.DarkOnly = "Geoff Synthwave", "The older personal site's deep purple, hot pink, pale magenta and white-neon identity. Optional glow is separate from the palette.", true
		case "geoff-indigo":
			view.Name, view.Description, view.DarkOnly = "Geoff Indigo", "The newer dashboard's gray surfaces and indigo accents, with explicit semantic contrast adaptations.", true
		case "synthwave":
			view.Name, view.Description, view.DarkOnly = "Kit Synthwave", "The existing near-black violet and purple preset. Its identity is unchanged.", true
		case "copper":
			view.Name, view.Description = "Copper", "Warm mineral surfaces and a burnished orange action color, with distinct light and dark schemes."
		case "lagoon":
			view.Name, view.Description = "Lagoon", "Cool teal surfaces and clear blue-green actions, with distinct light and dark schemes."
		case "orchard":
			view.Name, view.Description = "Orchard", "Olive-green surfaces and leaf actions, with distinct light and dark schemes."
		case "appkit":
			view.Name, view.Description = "App-kit default", "The shared semantic palette, with distinct light and dark schemes."
		default:
			http.Error(w, "Unknown named theme", 400)
			return
		}
		var build func() (template.HTML, error)
		switch view.Specimen {
		case "dashboard":
			build = buildDashboard
		case "components":
			build = buildComponents
		case "forms":
			build = buildForms
		case "tokens":
			build = buildTokens
		default:
			http.Error(w, "Unknown specimen", 400)
			return
		}
		view.Content, err = build()
		if err != nil {
			http.Error(w, "Specimen unavailable", 500)
			return
		}
		content, err := htmlf("theme-workbench", themeWorkbenchHTML, view)
		if err != nil {
			http.Error(w, "Workbench unavailable", 500)
			return
		}
		selected := url.Values{"palette": {view.Palette}, "specimen": {view.Specimen}, "effect": {view.Effect}}
		link := func(theme string) string {
			next := selected
			next.Set("theme", theme)
			return "/themes?" + next.Encode()
		}
		links := append([]navLink(nil), nav...)
		for i := range links {
			links[i].Active = links[i].Key == "themes"
		}
		d := pageData{Title: "Theme workbench", Page: "themes", Theme: view.Theme, Nav: links, Content: content,
			ThemeLight: link("light"), ThemeDark: link("dark"), ThemeSystem: link("")}
		if view.Palette != "appkit" {
			d.ThemeStylesheet = "/static/themes/" + view.Palette + ".css"
		}
		if view.Effect == "neon" {
			d.EffectStylesheet = "/static/themes/geoff-neon.css"
		}
		body, err := render(d)
		if err != nil {
			http.Error(w, "Workbench unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	})
}

const themeWorkbenchHTML = `
<header class="max-w-3xl"><p class="text-sm font-medium text-accent">THEME WORKBENCH</p><h1 class="mt-3 text-3xl font-semibold {{if eq .Effect "neon"}}ak-neon-title{{end}}">{{.Name}}</h1><p class="mt-3 text-ink-muted">{{.Description}}</p></header>
{{if .LabAvailable}}<p class="mt-4 text-sm"><a href="/theme-lab" class="text-accent underline underline-offset-4">Open the experimental theme lab: edit the selected source, preview, review and export</a></p>{{end}}
<form method="get" action="/themes" class="mt-6 flex flex-wrap items-end gap-5 border-y border-line py-5" aria-label="Theme preview selection">
<div><label for="workbench-palette" class="block text-sm font-medium">Visual identity</label><select id="workbench-palette" name="palette" class="mt-2 rounded-md border border-line-strong bg-surface-raised p-2 text-ink">{{range .Palettes}}<option value="{{.ID}}" {{if eq $.Palette .ID}}selected{{end}}>{{.Name}}</option>{{end}}</select></div>
<div><label for="workbench-theme" class="block text-sm font-medium">Scheme request</label><select id="workbench-theme" name="theme" class="mt-2 rounded-md border border-line-strong bg-surface-raised p-2 text-ink"><option value="" {{if eq .Theme ""}}selected{{end}}>Follow system</option><option value="light" {{if eq .Theme "light"}}selected{{end}}>Light</option><option value="dark" {{if eq .Theme "dark"}}selected{{end}}>Dark</option></select></div>
<div><label for="workbench-specimen" class="block text-sm font-medium">Specimen</label><select id="workbench-specimen" name="specimen" class="mt-2 rounded-md border border-line-strong bg-surface-raised p-2 text-ink">{{range .Specimens}}<option value="{{.ID}}" {{if eq $.Specimen .ID}}selected{{end}}>{{.Name}}</option>{{end}}</select></div>
<div><label for="workbench-effect" class="block text-sm font-medium">Heading effect</label><select id="workbench-effect" name="effect" class="mt-2 rounded-md border border-line-strong bg-surface-raised p-2 text-ink"><option value="none" {{if eq .Effect "none"}}selected{{end}}>None</option><option value="neon" {{if eq .Effect "neon"}}selected{{end}}>Static neon glow</option></select></div>
<button class="rounded-md bg-accent px-4 py-2 font-medium text-accent-ink hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-focus">Preview selection</button><a href="/themes" class="py-2 text-accent underline">Reset preview</a></form>
<p class="mt-4 max-w-3xl text-sm text-ink-muted">{{if .DarkOnly}}This is a dark-only identity. Light and system requests keep its dark palette; no unreviewed light version is invented. {{end}}The selected stylesheet changes existing semantic colors, typography and corners. Fonts are declared, not downloaded. Glow affects the workbench title only and can be turned off.</p>
<details class="mt-4 max-w-3xl"><summary class="cursor-pointer text-sm font-medium text-accent">Adoption and evidence limits</summary><div class="mt-3 space-y-2 text-sm text-ink-muted"><p>These are real kit renderers with synthetic content, not a copied website. Form controls are specimens, not a profile/settings save workflow. Use the Application shell for that journey.</p><p>For portable adoption, select this identity with <code>appkit vendor --include tokens --theme {{.Palette}} -out ./vendor/appkit</code>. Regenerate and rebuild older utility CSS once to enable runtime font/radius bindings. A selected preset does not install fonts or migrate an app.</p><p>No app preferences or source files are changed. Browser fidelity, complete accessibility and real-consumer adoption remain unverified. The separate experimental theme lab requires an explicitly selected source at launch; this preset selector does not change that source.</p></div></details>
<section class="mt-8 border-t border-line pt-8" aria-label="Selected kit specimen">{{.Content}}</section>
`
