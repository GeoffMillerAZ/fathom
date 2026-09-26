package main

// console_project.go — the project workspace at /projects/{svc} (vision 35
// §3; design §3 "Project workspace"; OC-35). F0 moved the service detail
// onto the shell with a trail back to the fleet; an unknown project renders
// on the shell with the remedy instead of a bare text/plain 404 (OC-5).
//
// Wave E makes it a workspace whose tabs are URLs (chrome.Tabs): the
// overview here, and the index, intent, activity and wiring tabs as their
// own registered pages (console_project_<tab>.go) at /projects/{svc}/<tab>,
// so each tab has its own verbs, metrics, JSON and budget, and a tab is a
// link that works without JavaScript and survives a reload. Governance,
// plans, proof and refactor open the Knowledge and Proof pages filtered to
// the project (coordinated by URL: those pages own the views).

import (
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/internal/console"
)

// projectMetricIDs are the per-project shared metrics (OC-9).
var projectMetricIDs = []console.MetricID{console.MetricProjectSymbols, console.MetricProjectCalls, console.MetricProjectSessions}

func init() {
	declarePage(consolePage{
		ID: "project", Route: "/projects/{svc}", Group: "Fleet", Order: 10,
		Title: "Project", NavParent: "fleet", Aliases: []string{"service"},
		Question: "Everything about one repository: wiring, freshness, configuration and activity.",
		Verbs:    []string{"fathom fleet realization --service {svc} --json", "fathom fleet report --json"},
		Shows:    projectMetricIDs,
		Live:     []string{"index:{svc}", "layers:{svc}", "sessions:{svc}", "plans:{svc}"},
		Build:    buildProjectPage,
	})
}

// projectTab is one workspace tab. A tab with a Page renders here, at
// /projects/{svc} + Suffix; one without opens another page filtered to the
// project (Href).
type projectTab struct {
	ID, Label string
	Page      string // registered page ID, "" for a tab that opens another page
	Suffix    string // route suffix after /projects/{svc}
	Href      func(svc string) string
}

// projectTabList is the workspace's tab order (design §3).
var projectTabList = []projectTab{
	{ID: "overview", Label: "Overview", Page: "project"},
	{ID: "index", Label: "Index", Page: "project-index", Suffix: "/index"},
	{ID: "governance", Label: "Governance", Href: func(s string) string { return console.ProjectHref(s, "governance") }},
	{ID: "plans", Label: "Plans", Href: func(s string) string { return console.ProjectHref(s, "plans") }},
	{ID: "proof", Label: "Proof", Href: func(s string) string { return console.ProjectHref(s, "proof") }},
	{ID: "refactor", Label: "Refactor", Href: func(s string) string { return console.ProjectHref(s, "refactor") }},
	{ID: "intent", Label: "Intent", Page: "project-intent", Suffix: "/intent"},
	{ID: "activity", Label: "Activity", Page: "project-activity", Suffix: "/activity"},
	{ID: "wiring", Label: "Wiring", Page: "project-wiring", Suffix: "/wiring"},
}

// projectTabHref is a tab's URL for svc.
func projectTabHref(t projectTab, svc string) string {
	if t.Href != nil {
		return t.Href(svc)
	}
	return projectHref(svc) + t.Suffix
}

// projectTabs is the workspace's tabs for svc, the active one marked (OC-35):
// every tab is a URL, so a tab is bookmarkable, opens in a new window and
// works with no JavaScript at all.
func projectTabs(svc, active string) []chrome.Tab {
	tabs := make([]chrome.Tab, 0, len(projectTabList))
	for _, t := range projectTabList {
		tabs = append(tabs, chrome.Tab{Label: t.Label, Href: projectTabHref(t, svc), Active: t.ID == active})
	}
	return tabs
}

// renderProjectTabs renders the tab bar. Nine tabs do not fit a phone, so
// the bar scrolls sideways inside its own region (the page never does), and
// every tab keeps a 44 px target (the kit's tab is ~36 px — returned as
// FATHOM-E-KIT-01, docs/app-kit.md).
func renderProjectTabs(svc, active string) string {
	return `<div class="mb-4 overflow-x-auto [&_nav]:min-w-max [&_nav>a]:inline-flex [&_nav>a]:min-h-11 [&_nav>a]:items-center [&_nav>a]:whitespace-nowrap [&_nav>a]:focus-visible:outline-2 [&_nav>a]:focus-visible:outline-offset-[-2px] [&_nav>a]:focus-visible:outline-focus" data-project-tabs>` +
		chrome.Tabs(projectTabs(svc, active), svc+" workspace") + `</div>`
}

// projectBody wraps a tab's panels. A panel's header is the kit widget's
// one flex row (title, then the verb chip), which never wraps: a long
// command — a --store or --root path — squeezed the title to a word per
// line at 390 px. The wrapper lets that row wrap so the chip drops under
// the title (returned as FATHOM-E-KIT-02, docs/app-kit.md).
func projectBody(panels string) string {
	return `<div class="[&_section>div:first-child]:flex-wrap" data-project-body>` + panels + `</div>`
}

// projectContext is one request's project: its model row, the model, and
// whether its registered root can be read on this machine.
type projectContext struct {
	svc      string
	m        observatoryModel
	row      obsServiceRow
	root     string // the registered root, "" when none
	rootNote string // why the root cannot be read here; "" when it can
}

// loadProjectContext resolves the request's project or answers OC-5's
// unknown-project page.
func loadProjectContext(cr *consoleRequest) (projectContext, error) {
	id := cr.r.PathValue("svc")
	m := cr.model()
	for _, s := range m.Services {
		if s.Service != id {
			continue
		}
		pc := projectContext{svc: id, m: m, row: s, root: s.Path}
		switch fi, err := os.Stat(s.Path); {
		case s.Path == "":
			pc.rootNote = "no local root is recorded for " + id + " — a remote corpus other repositories query; there is nothing on this machine to read"
		case err != nil:
			pc.rootNote = "the registered root " + s.Path + " is not readable on this machine (" + err.Error() + ")"
		case !fi.IsDir():
			pc.rootNote = "the registered root " + s.Path + " is not a directory on this machine"
		}
		return pc, nil
	}
	return projectContext{}, unknownProjectError(id, m, cr.deps.selfURL)
}

// facts is the project's row as the shared metric projections see it.
func (pc projectContext) facts() console.WorkspaceFacts {
	f := console.WorkspaceFacts{Name: pc.svc, SymbolsNote: pc.row.SymbolsNote, Calls: modelCallFacts(pc.m), CallsForProject: pc.row.CallsTotal, SessionsState: modelSessions(pc.m)}
	if pc.row.SymbolsNote == "" {
		n := pc.row.Symbols
		f.Symbols = &n
	}
	if pc.row.SessionCount != nil {
		n := *pc.row.SessionCount
		f.Sessions = &n
	}
	if t, err := time.Parse(time.RFC3339, pc.m.GeneratedAt); err == nil {
		f.AsOf = t
	}
	return f
}

// metrics is the page's declared share of the project's shared metrics.
func (pc projectContext) metrics(p *consolePage) console.Metrics {
	return pageMetrics(p, console.WorkspaceMetrics(pc.facts()))
}

// frame is the tab page's title, trail and meta line: "<svc> · <Tab>" as
// the h1 (the wiring steps' precedent), Observatory / Fleet / <svc> /
// <Tab> as the trail.
func (pc projectContext) frame(tabLabel string) consoleOut {
	out := consoleOut{Title: pc.svc, Meta: pc.meta()}
	if tabLabel == "" {
		out.Crumbs = []chrome.Crumb{{Label: "Fleet", Href: "/fleet"}, {Label: pc.svc}}
		return out
	}
	out.Title = pc.svc + " · " + tabLabel
	out.Crumbs = []chrome.Crumb{{Label: "Fleet", Href: "/fleet"}, {Label: pc.svc, Href: projectHref(pc.svc)}, {Label: tabLabel}}
	return out
}

// meta is the heading's meta line: path, symbols, freshness, generation.
func (pc projectContext) meta() []string {
	meta := []string{}
	if pc.row.Path != "" {
		meta = append(meta, pc.row.Path)
	}
	meta = append(meta, valuePlain(console.ProjectSymbols(pc.facts()))+" symbols", "index "+pc.row.Freshness)
	return append(meta, modelMeta(pc.m)...)
}

func buildProjectPage(cr *consoleRequest) (consoleOut, error) {
	pc, err := loadProjectContext(cr)
	if err != nil {
		return consoleOut{}, err
	}
	body, _ := renderObsService(pc.m, pc.svc)
	out := pc.frame("")
	out.Model = pc.row
	out.Metrics = pc.metrics(cr.page)
	out.Verbs = []string{"fathom fleet report --json"}
	out.Sections = map[string]any{}
	out.Body = func() string { return renderProjectTabs(pc.svc, "overview") + obsContent("service", body, pc.m) }
	// OC-25 (RG-5): the realization ladder leads the overview.
	if l, ok := modelLadder(pc.m, pc.svc); ok {
		verb := realizationVerb(pc.svc)
		out.Verbs = append([]string{verb}, out.Verbs...)
		out.Sections[verb] = l
		// The ladder is a live panel, so the router no longer wraps the
		// page in its region: the rest of the page keeps its own.
		rest, _ := consoleLivePage(pc.svc, pageLiveSources(cr.page, cr), cr.now, body)
		out.Body = func() string {
			return renderProjectTabs(pc.svc, "overview") + obsContent("service", ladderPanel(l)+rest, pc.m)
		}
	}
	// The fleet report's section is computed for JSON only, as on the fleet
	// page: the report walks every repository and is not on the HTML render
	// path.
	if wantsJSON(cr.r) {
		out.Sections["fathom fleet report --json"] = fleetReportSection(cr)
	}
	return out, nil
}

// fillVerb fills a declared verb template's placeholders for one request.
// Each value is quoted when it holds anything a shell would split or expand,
// so the chip is a command that runs as written.
func fillVerb(template string, vals map[string]string) string {
	out := template
	for k, v := range vals {
		out = strings.ReplaceAll(out, "{"+k+"}", shellWord(v))
	}
	return out
}

// projectRootCallout names why a tab that reads the repository's files has
// nothing to read on this machine, in the kit's callout (doctrine rule 1):
// an info note for a remote corpus with no local root, a caution for a
// registered root this machine cannot read (unavailable configuration,
// docs/app-kit.md).
func projectRootCallout(pc projectContext) string {
	if pc.rootNote == "" {
		return ""
	}
	c := callout.Callout{Kind: callout.Info, Title: "Nothing to read on this machine", Body: pc.rootNote}
	if pc.root != "" {
		c.Kind, c.Title = callout.Caution, "The repository cannot be read here"
	}
	return `<div class="mb-3 max-w-3xl [overflow-wrap:anywhere]">` + callout.Render(c) + `</div>`
}

// unknownProjectError is OC-5's unknown-service page: the name that missed,
// the closest registered names, and the command that indexes a repository
// through THIS daemon (selfURL). The remedy is `fathom init`, doctor's own
// remedy for a repo missing from the index: it ingests through the daemon's
// ingest tool. A shell `fathom ingest` would write <repo>/workspace/db —
// a store this daemon never reads — so the project would stay unknown, and
// against the shared store it is the forbidden shell ingest (INC-3, SB-7).
func unknownProjectError(id string, m observatoryModel, selfURL string) *consoleError {
	if m.DegradedNote != "" {
		return &consoleError{
			Status: http.StatusServiceUnavailable, Title: "Store unreadable",
			Reason:      "The service registry could not be read, so project " + id + " cannot be looked up: " + m.DegradedNote,
			Remedy:      "fathom doctor",
			RemedyHref:  "/statusz",
			RemedyLabel: "Open the daemon status",
		}
	}
	var near []string
	for _, s := range m.Services {
		if strings.Contains(strings.ToLower(s.Service), strings.ToLower(id)) || strings.Contains(strings.ToLower(id), strings.ToLower(s.Service)) {
			near = append(near, s.Service)
		}
	}
	sort.Strings(near)
	reason := "No indexed project is named " + id + "."
	if len(near) > 0 {
		reason += " Did you mean: " + strings.Join(near, ", ") + "?"
	}
	return &consoleError{
		Status: http.StatusNotFound, Title: "Unknown project",
		Reason:     reason,
		Remedy:     "fathom init --path <repo> --service " + id + " --url " + selfURL,
		RemedyHref: "/fleet",
	}
}
