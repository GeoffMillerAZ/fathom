package main

// console_registry.go — OC-3 (docs/design/observatory-console.md §1): pages
// register in one table, and that table drives the router, the grouped
// sidebar, the command palette's page and verb index, and the JSON parity
// test. Each page lives in its own console_<page>.go and declares itself in
// init() (declarePage); registerPage validates each declaration and installs
// it into the registry every reader shares, so a page cannot be routable
// without being navigable and searchable, or vice versa.

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/fathom/internal/console"
)

// consoleGroups is the sidebar's fixed group order (vision 35 §3). A group
// is shown once a page registers in it; there are no placeholder pages.
var consoleGroups = []string{"Now", "Fleet", "Proof", "Knowledge", "Evidence", "System", "Learn"}

// consolePage is one registered page.
type consolePage struct {
	ID       string // stable key; also the legacy obsPage active key
	Route    string // http.ServeMux pattern, e.g. "/fleet", "/projects/{svc}"
	Href     string // destination for nav and search; "" when the route has parameters
	Group    string // one of consoleGroups
	Title    string // the page heading (h1)
	NavLabel string // sidebar label; defaults to Title
	Question string // the question the page answers
	Order    int    // order within its group
	Nav      bool   // listed in the sidebar
	// NavParent is the ID of the navigable page highlighted while this one
	// is open (a project page highlights Fleet).
	NavParent string
	// Aliases are older active keys that name this page (obsPage callers).
	Aliases []string
	// Verbs are the exact commands whose --json this page's JSON carries as
	// sections (P10); TestConsolePagesRenderTheirVerbs holds each page to
	// them. A page not yet rebuilt declares none.
	Verbs []string
	// Shows are the shared metrics the page renders (OC-9).
	Shows []console.MetricID
	// Live are the live-freshness sources the page's body follows when it
	// is not built from source-keyed panels (OC-14, console_live.go); a
	// "{svc}" in a key is the request's project.
	Live []string
	// Formats are the ?format= values the page answers besides HTML and
	// json ("md" for the brief, "fragment" for search); any other value is
	// a 400 on the shell naming the ones that work.
	Formats []string
	// Build computes the page for one request. A *consoleError return
	// renders on the shell with its reason and remedy (OC-5).
	Build func(cr *consoleRequest) (consoleOut, error)
}

// consoleOut is one built page.
type consoleOut struct {
	Title    string         // overrides the page title (a project page names its project)
	Crumbs   []chrome.Crumb // trail after Observatory; the current page is appended
	Meta     []string       // the heading's meta line
	Actions  string         // trusted heading actions HTML
	Model    any            // the page's own projection, served as JSON "model"
	Metrics  console.Metrics
	Sections map[string]any // verb → that verb's --json value
	// Verbs, when set, are this request's concrete commands for a page
	// whose registered Verbs are templates ("{svc}", "{plan}", "{id}"):
	// the heading's chips and the JSON's verbs name them, and each has
	// its section.
	Verbs []string
	// Chips, when non-nil, are the heading's verb chips instead of Verbs: a
	// fleet page rendering one verb per repository names each in its own
	// row rather than stacking them all in the heading (empty: none).
	Chips []string
	Body  func() string // HTML, rendered only for an HTML request
	// Raw, when set, answers the request itself (a download, a fragment).
	Raw func(w http.ResponseWriter) bool
}

// consoleRequest is one request to a page.
type consoleRequest struct {
	w    http.ResponseWriter
	r    *http.Request
	deps *consoleDeps
	page *consolePage
	now  time.Time
}

// consoleRegistry is the table.
type consoleRegistry struct {
	pages  []*consolePage
	byID   map[string]*consolePage
	routes map[string]*consolePage
}

func newConsoleRegistry() *consoleRegistry {
	return &consoleRegistry{byID: map[string]*consolePage{}, routes: map[string]*consolePage{}}
}

func consoleGroupIndex(g string) int {
	for i, name := range consoleGroups {
		if name == g {
			return i
		}
	}
	return -1
}

// registerPage validates p and installs it: a unique ID and route, a known
// group, a title and a question, a Build, an Href for every navigable page,
// and verbs that are exact fathom commands. Pages stay sorted by group
// order, then Order, then ID, so every reader sees one deterministic order.
func registerPage(reg *consoleRegistry, p consolePage) error {
	switch {
	case p.ID == "":
		return fmt.Errorf("console: a page needs an ID")
	case reg.byID[p.ID] != nil:
		return fmt.Errorf("console: page %q registered twice", p.ID)
	case p.Route == "" || !strings.HasPrefix(p.Route, "/"):
		return fmt.Errorf("console: page %q needs a rooted route, got %q", p.ID, p.Route)
	case reg.routes[p.Route] != nil:
		return fmt.Errorf("console: route %s claimed by both %q and %q", p.Route, reg.routes[p.Route].ID, p.ID)
	case consoleGroupIndex(p.Group) < 0:
		return fmt.Errorf("console: page %q names unknown group %q", p.ID, p.Group)
	case p.Title == "" || p.Question == "":
		return fmt.Errorf("console: page %q needs a title and the question it answers", p.ID)
	case p.Build == nil:
		return fmt.Errorf("console: page %q has no Build", p.ID)
	case p.Nav && p.Href == "":
		return fmt.Errorf("console: navigable page %q needs an Href", p.ID)
	case p.Href != "" && strings.Contains(p.Href, "{"):
		return fmt.Errorf("console: page %q Href %q has an unfilled parameter", p.ID, p.Href)
	}
	for _, v := range p.Verbs {
		if !strings.HasPrefix(v, "fathom ") {
			return fmt.Errorf("console: page %q verb %q is not an exact fathom command", p.ID, v)
		}
	}
	for _, a := range p.Aliases {
		if reg.byID[a] != nil {
			return fmt.Errorf("console: page %q alias %q collides", p.ID, a)
		}
	}
	if p.NavLabel == "" {
		p.NavLabel = p.Title
	}
	cp := p
	reg.pages = append(reg.pages, &cp)
	reg.byID[cp.ID] = &cp
	for _, a := range cp.Aliases {
		reg.byID[a] = &cp
	}
	reg.routes[cp.Route] = &cp
	sort.SliceStable(reg.pages, func(i, j int) bool {
		a, b := reg.pages[i], reg.pages[j]
		if ga, gb := consoleGroupIndex(a.Group), consoleGroupIndex(b.Group); ga != gb {
			return ga < gb
		}
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.ID < b.ID
	})
	return nil
}

// consolePageDecls collects init()-time declarations.
var consolePageDecls []consolePage

// declarePage is what each console_<page>.go calls from init().
func declarePage(p consolePage) { consolePageDecls = append(consolePageDecls, p) }

var (
	consoleRegOnce sync.Once
	consoleReg     *consoleRegistry
)

// defaultConsoleRegistry registers every declared page once; a declaration
// that registerPage refuses is a programming error and panics at startup
// (and in every test that builds a handler), never a silently missing page.
func defaultConsoleRegistry() *consoleRegistry {
	consoleRegOnce.Do(func() {
		reg := newConsoleRegistry()
		for _, p := range consolePageDecls {
			if err := registerPage(reg, p); err != nil {
				panic(err)
			}
		}
		consoleReg = reg
	})
	return consoleReg
}

// page returns the page registered under id or alias.
func (reg *consoleRegistry) page(id string) (*consolePage, bool) {
	p, ok := reg.byID[id]
	return p, ok
}

// navGroups renders the registry as the sidebar's groups.
func (reg *consoleRegistry) navGroups() []dashboard.NavGroup {
	var groups []dashboard.NavGroup
	for _, g := range consoleGroups {
		var items []dashboard.NavItem
		for _, p := range reg.pages {
			if p.Group == g && p.Nav {
				items = append(items, dashboard.NavItem{Href: p.Href, Label: p.NavLabel})
			}
		}
		if len(items) > 0 {
			groups = append(groups, dashboard.NavGroup{Heading: g, Items: items})
		}
	}
	return groups
}

// navCurrentPath is the nav destination highlighted for the page with id.
func (reg *consoleRegistry) navCurrentPath(id string) string {
	p, ok := reg.page(id)
	if !ok {
		return ""
	}
	if p.NavParent != "" {
		if parent, ok := reg.page(p.NavParent); ok {
			return parent.Href
		}
	}
	if p.Nav {
		return p.Href
	}
	return ""
}

// searchDocs is the registry's contribution to the palette index: every
// page with a destination, and every verb a page renders (linked to it).
func (reg *consoleRegistry) searchDocs() []console.SearchDoc {
	var docs []console.SearchDoc
	for _, p := range reg.pages {
		if p.Href == "" {
			continue
		}
		docs = append(docs, console.SearchDoc{
			Group: console.GroupPages, Kind: "page", ID: p.ID, Title: p.Title,
			Subtitle: p.Question, Keywords: append([]string{p.Group, p.NavLabel}, p.Aliases...), Href: p.Href,
		})
	}
	return docs
}

// pageRendering returns the first page whose Verbs include the command
// (verb results in the palette link to the page that renders them).
func (reg *consoleRegistry) pageRendering(verbName string) (*consolePage, bool) {
	for _, p := range reg.pages {
		if p.Href == "" {
			continue
		}
		for _, v := range p.Verbs {
			if v == "fathom "+verbName || strings.HasPrefix(v, "fathom "+verbName+" ") {
				return p, true
			}
		}
	}
	return nil, false
}
