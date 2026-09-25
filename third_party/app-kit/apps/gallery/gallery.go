// Package gallery is the kit's working catalog and experience laboratory
// (ADR-0020). It is one http.Handler, sharing capability records with the MCP.
// Component specimens and complete workflow evidence are explicitly distinct.
package gallery

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/geoffmilleraz/app-kit/apps/styleguide"
)

//go:embed ui/app.css
var assets embed.FS

// New returns the Gallery as a single http.Handler. It serves the catalog
// pages plus the built CSS at /static/app.css. Private references are disabled;
// a local host can select its own store with NewWithOptions.
func New() http.Handler {
	return NewWithOptions(Options{})
}

// Options selects optional local inputs. Licensed data is never embedded into
// the Gallery binary. Empty DesignAssetsRoot leaves first-party discovery usable.
type Options struct {
	DesignAssetsRoot string
	// ThemeLab is an explicit local-only guide created with NewThemeLab.
	// Nil leaves this tool unmounted and keeps ordinary Gallery setup unchanged.
	ThemeLab *styleguide.Handler
}

// NewWithOptions mounts the same first-party Gallery alongside an optional
// read-only private reference library. Reference routes enforce a local origin
// boundary even when the handler is mounted by a different host.
func NewWithOptions(options Options) http.Handler {
	mux := http.NewServeMux()

	css, cssErr := assets.ReadFile("ui/app.css")
	serveCSS := staticCSSHandler(css)
	mux.HandleFunc("/static/app.css", func(w http.ResponseWriter, r *http.Request) {
		if cssErr != nil {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "css not built — run `task gallery:css`", http.StatusInternalServerError)
			return
		}
		serveCSS(w, r)
	})

	registerCatalog(mux)
	registerQuality(mux)
	registerThemeWorkbench(mux, options.ThemeLab != nil)
	if options.ThemeLab != nil {
		mux.Handle("/theme-lab", options.ThemeLab)
		mux.Handle("/theme-lab/", options.ThemeLab)
	}
	registerReferences(mux, options.DesignAssetsRoot)
	registerAnalytics(mux)
	registerStaticContent(mux)
	registerShellDemo(mux)
	registerShellDemoAssets(mux)
	mux.HandleFunc("/overview", page("overview", "Specimen overview", buildOverview))
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
	{Key: "catalog", Label: "Catalog", Href: "/catalog"},
	{Key: "quality", Label: "Quality work", Href: "/quality"},
	{Key: "references", Label: "Reference library", Href: "/references"},
	{Key: "analytics", Label: "Analytics journey", Href: "/analytics"},
	{Key: "shell", Label: "Application shell", Href: "/shell"},
	{Key: "content", Label: "Static content", Href: "/content"},
	{Key: "overview", Label: "Overview", Href: "/overview"},
	{Key: "tokens", Label: "Tokens", Href: "/tokens"},
	{Key: "themes", Label: "Theme workbench", Href: "/themes"},
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
		if r.URL.Path != "/"+key {
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
