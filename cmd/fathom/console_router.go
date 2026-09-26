package main

// console_router.go — mounts the registry (console_registry.go) on
// serve-http's mux: one handler per registered page, the event stream, the
// console script, the shell's 404, and the legacy /observatory* routes.
// Every page answers HTML on the shell or, for ?format=json / Accept:
// application/json, its projection in one envelope (consolePageJSON).

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/live"
	"github.com/geoffmilleraz/app-kit/appkit/ui/palette"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// consoleDeps is what every page handler reads.
type consoleDeps struct {
	reg       *consoleRegistry
	selfURL   string
	c         *client.Client // nil in handler tests without a store
	bus       *console.EventBus
	views     *consolePageViews
	startTime time.Time
	now       func() time.Time
	search    *consoleSearchCache
}

// newConsoleDeps builds the default dependencies for a handler: the
// registry, a fresh event bus, the durable page-view log and the search
// cache. serve-http then attaches the event feeders to deps.bus.
func newConsoleDeps(c *client.Client, selfURL string, startTime time.Time) *consoleDeps {
	return &consoleDeps{
		reg: defaultConsoleRegistry(), selfURL: selfURL, c: c,
		bus:       console.NewEventBus(console.BusOptions{}),
		views:     newConsolePageViews(consoleViewsLogPath()),
		startTime: startTime, now: time.Now,
		search: newConsoleSearchCache(),
	}
}

// sources names what this daemon's projections read (design §1).
func (d *consoleDeps) sources() console.Sources {
	return console.Sources{
		CallLog:       resolveConsoleCallLogFor(callLogQuery{BaseURL: d.selfURL}),
		SessionLedger: sessionLedgerPath(),
		VerifyLedger:  verifyLedgerPath(),
		Now:           d.now,
	}
}

// consolePageJSONSchema names the page envelope's wire version.
const consolePageJSONSchema = "fathom.console.page/v1"

// consolePageJSON is every page's JSON: its identity, the verbs it renders,
// the shared metrics it shows, each verb's --json as a section, and the
// page's own projection.
type consolePageJSON struct {
	Schema      string   `json:"schema"`
	Page        string   `json:"page"`
	Route       string   `json:"route"`
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	Group       string   `json:"group"`
	Question    string   `json:"question"`
	GeneratedAt string   `json:"generated_at"`
	Verbs       []string `json:"verbs"`
	// ProjectVerbs are the per-project verbs the page renders ("{svc}" is
	// each project); each one's section is keyed by service.
	ProjectVerbs []string        `json:"project_verbs,omitempty"`
	Metrics      console.Metrics `json:"metrics,omitempty"`
	Sections     map[string]any  `json:"sections,omitempty"`
	Model        any             `json:"model,omitempty"`
}

func consolePageJSONFor(p *consolePage, r *http.Request, out consoleOut, now time.Time) consolePageJSON {
	title := out.Title
	if title == "" {
		title = p.Title
	}
	verbs := p.Verbs
	if verbs == nil {
		verbs = []string{}
	}
	return consolePageJSON{
		Schema: consolePageJSONSchema, Page: p.ID, Route: p.Route, Path: r.URL.Path,
		Title: title, Group: p.Group, Question: p.Question,
		GeneratedAt: now.UTC().Format(time.RFC3339), Verbs: verbs, ProjectVerbs: p.ProjectVerbs,
		Metrics: out.Metrics, Sections: out.Sections, Model: out.Model,
	}
}

// pageHandler serves one registered page.
func (d *consoleDeps) pageHandler(p *consolePage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			consoleMethodNotAllowed(w, r, d.reg)
			return
		}
		if f := r.URL.Query().Get("format"); f != "" && f != "json" && !containsString(p.Formats, f) {
			allowed := append([]string{"json"}, p.Formats...)
			renderConsoleError(w, r, d.reg, &consoleError{
				Status: http.StatusBadRequest, Title: "Unsupported format",
				Reason: p.Title + " does not answer ?format=" + f + ".",
				Remedy: "ask for one of: ?format=" + strings.Join(allowed, ", ?format=") + " — or no format for the page itself",
			})
			return
		}
		now := d.now()
		cr := &consoleRequest{w: w, r: r, deps: d, page: p, now: now}
		out, err := p.Build(cr)
		if err != nil {
			var ce *consoleError
			if !errors.As(err, &ce) {
				ce = &consoleError{Status: http.StatusInternalServerError, Title: "Page failed", Reason: err.Error(),
					Remedy: "fathom doctor", RemedyHref: "/statusz", RemedyLabel: "Open the daemon status"}
			}
			renderConsoleError(w, r, d.reg, ce)
			return
		}
		if out.Raw != nil && out.Raw(w) {
			return
		}
		if wantsJSON(r) {
			writeObsJSON(w, consolePageJSONFor(p, r, out, now))
			return
		}
		// HEAD serves no page to anyone: answer the headers before the view
		// is counted (UX-M4 reads views as pages a person opened) and before
		// the body and the shell are rendered.
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			return
		}
		countPageView(d.views, p, now)
		title := out.Title
		if title == "" {
			title = p.Title
		}
		body := ""
		if out.Body != nil {
			body = out.Body()
		}
		// OC-14: a body built from source-keyed panels is already live; any
		// other body follows the page's declared sources as one region.
		live := strings.Contains(body, "data-live-source=")
		if !live {
			body, live = consoleLivePage(title, pageLiveSources(p, cr), now, body)
		}
		liveStatus := ""
		if live {
			liveStatus = consoleLiveStatus(d.bus.Cursor())
		}
		page := renderConsoleShell(d.reg, consoleShell{
			PageID: p.ID, Title: title, Crumbs: consoleCrumbs(d.reg, p.ID, title),
			Question: p.Question, Meta: out.Meta, Verbs: p.Verbs, Actions: out.Actions,
			Body: body, Live: liveStatus,
		})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}
}

// mountConsole registers every console route on mux.
func mountConsole(mux *http.ServeMux, d *consoleDeps, wiring http.Handler) {
	for _, p := range d.reg.pages {
		mux.Handle(p.Route, d.pageHandler(p))
	}
	mux.Handle("/", consoleNotFound(d.reg))
	mux.Handle("/assets/console.js", consoleAssetsJSHandler())
	mux.Handle(consolePaletteScriptPath, palette.Handler())
	mux.Handle(consoleLiveScriptPath, live.Handler())
	// GET /events (OC-13): the live-freshness stream — names and IDs only.
	mux.Handle("/events", d.bus)
	mountLegacyObservatory(mux, d.selfURL, wiring)
}

// model is the request's observatory model (cached, 30 s TTL).
func (cr *consoleRequest) model() observatoryModel {
	return cachedObservatoryModel(cr.deps.selfURL, cr.now)
}

// modelMeta is the heading meta line every model-backed page shares.
func modelMeta(m observatoryModel) []string {
	meta := []string{"generated " + m.GeneratedAt, "fathom " + m.FathomVersion}
	if m.DegradedNote != "" {
		meta = append(meta, "degraded: registry unreachable")
	}
	return meta
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// projectHref is a project's workspace route.
func projectHref(service string) string { return "/projects/" + url.PathEscape(service) }

// sessionHref is a session's replay route.
func sessionHref(id string) string { return "/sessions/" + url.PathEscape(id) }
