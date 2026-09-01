// Package gallery is the kit's living style guide (D12, VISION #2): a kit app,
// built on the kit, that renders every design token and every ui/viz primitive
// live through the token CSS. It honors the Loopback Invariant (D5) — the app
// is one http.Handler; a shell (web server, desktop wrapper) is just chrome
// around it. This v0 covers tokens + visualization; knob-driven component pages
// and service exercisers land with the Phase 1 spine.
package gallery

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed ui/app.css
var assets embed.FS

// New returns the Gallery as a single http.Handler. It serves the catalog
// pages plus the built CSS at /static/app.css. Any shell can mount it.
func New() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/static/app.css", func(w http.ResponseWriter, _ *http.Request) {
		b, err := assets.ReadFile("ui/app.css")
		if err != nil {
			http.Error(w, "css not built — run `task gallery:css`", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(b)
	})

	mux.HandleFunc("/", page("overview", "Overview", buildOverview))
	mux.HandleFunc("/tokens", page("tokens", "Tokens", buildTokens))
	mux.HandleFunc("/components", page("components", "Components", buildComponents))
	mux.HandleFunc("/layout", page("layout", "Layout", buildLayout))
	mux.HandleFunc("/forms", page("forms", "Forms", buildForms))
	mux.HandleFunc("/overlays", page("overlays", "Overlays", buildOverlays))
	mux.HandleFunc("/agentic", page("agentic", "Agentic", buildAgentic))
	mux.HandleFunc("/messaging", page("messaging", "Messaging", buildMessaging))
	mux.HandleFunc("/provenance", page("provenance", "Provenance", buildProvenance))
	mux.HandleFunc("/dashboard", page("dashboard", "Dashboard", buildDashboard))
	mux.HandleFunc("/motion", page("motion", "Motion", buildMotion))

	return mux
}

// nav is the catalog's top-level structure, shared by every page.
var nav = []navLink{
	{Key: "overview", Label: "Overview", Href: "/"},
	{Key: "tokens", Label: "Tokens", Href: "/tokens"},
	{Key: "components", Label: "Components", Href: "/components"},
	{Key: "layout", Label: "Layout", Href: "/layout"},
	{Key: "forms", Label: "Forms", Href: "/forms"},
	{Key: "overlays", Label: "Overlays", Href: "/overlays"},
	{Key: "agentic", Label: "Agentic", Href: "/agentic"},
	{Key: "messaging", Label: "Messaging", Href: "/messaging"},
	{Key: "provenance", Label: "Provenance", Href: "/provenance"},
	{Key: "dashboard", Label: "Dashboard", Href: "/dashboard"},
	{Key: "motion", Label: "Motion", Href: "/motion"},
}

// page builds a handler for one catalog page. build produces the page's inner
// content; the shell (nav, theme switch, footer) is shared. The "/" route is
// special-cased to 404 on any unknown path so the catalog stays tight.
func page(key, title string, build func() (template.HTML, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if key == "overview" && r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		content, err := build()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Theme is chosen via ?theme=; nav links round-trip the active page.
		theme := sanitizeTheme(r.URL.Query().Get("theme"))
		links := make([]navLink, len(nav))
		copy(links, nav)
		for i := range links {
			links[i].Active = links[i].Key == key
			if theme != "" {
				links[i].Href += "?theme=" + theme
			}
		}
		body, err := render(pageData{
			Title:   title,
			Nav:     links,
			Theme:   theme,
			Page:    key,
			Content: content,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}
}
