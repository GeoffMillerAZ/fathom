package main

// console_search.go — OC-11's server side (docs/design/observatory-console.md
// §2). GET /search?q= answers the command palette's index — projects, plans
// and plan rows, symbols (from the store), rules and records (invariants,
// decisions, futures), verbs (the CLI registry) and pages (the console
// registry) — as a full page on the shell, as an HTML fragment for the
// palette (?format=fragment), and as JSON. The ranking is console.Search;
// this file only gathers the documents. The ⌘K dialog itself (OC-12) waits
// for the kit's command-palette contribution (FATHOM-KIT-01).

import (
	"bufio"
	"context"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func init() {
	declarePage(consolePage{
		ID: "search", Route: "/search", Group: "Now", Order: 90,
		Title:    "Search",
		Question: "Where is the project, plan, row, symbol, rule, verb or page I am thinking of?",
		Formats:  []string{"fragment"},
		Build:    buildSearchPage,
	})
}

// searchDocsTTL bounds how stale the file-backed documents (plans, rows,
// intent records) may be; symbols are keyed by each service's watermark.
const searchDocsTTL = 60 * time.Second

// consoleSearchCache holds the gathered documents and the symbol index.
type consoleSearchCache struct {
	mu      sync.Mutex
	docs    []console.SearchDoc
	notes   []string
	builtAt time.Time
	symbols map[string]consoleSymbolSet
}

// consoleSymbolSet is one service's symbol names. Entries stay compact —
// a SearchDoc is built only for a match — because the set is held for the
// daemon's lifetime (the fleet carries ~137k symbols; O108 bounds memory).
type consoleSymbolSet struct {
	watermark string
	entries   []consoleSymbolEntry
}

type consoleSymbolEntry struct {
	name, kind, pkg, ref string
}

func newConsoleSearchCache() *consoleSearchCache {
	return &consoleSearchCache{symbols: map[string]consoleSymbolSet{}}
}

// index returns the search index for this request.
func (sc *consoleSearchCache) index(cr *consoleRequest) console.SearchIndex {
	m := cr.model()
	sc.mu.Lock()
	if sc.builtAt.IsZero() || cr.now.Sub(sc.builtAt) > searchDocsTTL || cr.now.Before(sc.builtAt) {
		sc.docs, sc.notes = gatherSearchDocs(cr.deps.reg, m)
		sc.builtAt = cr.now
	}
	docs, notes := sc.docs, sc.notes
	sc.mu.Unlock()
	services := make([]string, 0, len(m.Services))
	for _, s := range m.Services {
		services = append(services, s.Service)
	}
	c := cr.deps.c
	ctx := cr.r.Context()
	return console.SearchIndex{
		Docs:  docs,
		Notes: notes,
		Symbols: func(q string, limit int) ([]console.SearchDoc, error) {
			return sc.lookupSymbols(ctx, c, services, q, limit)
		},
	}
}

// gatherSearchDocs collects every non-symbol document: pages, verbs,
// projects, and each local project's plans, rows and intent records.
func gatherSearchDocs(reg *consoleRegistry, m observatoryModel) ([]console.SearchDoc, []string) {
	docs := reg.searchDocs()
	for _, v := range verbTable {
		d := console.SearchDoc{Group: console.GroupVerbs, Kind: "verb", Title: v.Name, Subtitle: clip(v.Summary, 160), Command: "fathom " + v.Name}
		if p, ok := reg.pageRendering(v.Name); ok {
			d.Href = p.Href
		}
		docs = append(docs, d)
	}
	var unreadable []string
	for _, s := range m.Services {
		docs = append(docs, console.SearchDoc{Group: console.GroupProjects, Kind: "service", Title: s.Service, Subtitle: s.Path, Service: s.Service, Href: projectHref(s.Service)})
		if s.Path == "" {
			continue
		}
		fileDocs, err := projectSearchDocs(s.Service, s.Path)
		if err != nil {
			unreadable = append(unreadable, s.Service)
		}
		docs = append(docs, fileDocs...)
	}
	var notes []string
	if len(unreadable) > 0 {
		sort.Strings(unreadable)
		notes = append(notes, "plans and intent records could not be read for: "+strings.Join(unreadable, ", "))
	}
	if m.DegradedNote != "" {
		notes = append(notes, "projects are not searchable: "+m.DegradedNote)
	}
	return docs, notes
}

var (
	planRowRe   = regexp.MustCompile(`^\s*- \[([ xX-])\] (.*?)\s*\{id: ([A-Za-z0-9_.:-]+)`)
	adrHeadRe   = regexp.MustCompile(`^## (ADR-\d+):?\s+(.*)$`)
	invariantRe = regexp.MustCompile(`^\| (I\d+) \| (.*?) \|`)
	futureRe    = regexp.MustCompile(`^## (F\d+)\.\s+(.*)$`)
)

// projectSearchDocs reads one project's plans, plan rows and intent
// records. Links point at the project page (the plan and record pages are
// later waves); Ref is file:line and Command the verb that shows it.
func projectSearchDocs(service, root string) ([]console.SearchDoc, error) {
	var docs []console.SearchDoc
	var firstErr error
	plans, _ := filepath.Glob(filepath.Join(root, "docs", "plans", "*.md"))
	sort.Strings(plans)
	href := projectHref(service)
	for _, plan := range plans {
		rel, _ := filepath.Rel(root, plan)
		name := strings.TrimSuffix(filepath.Base(plan), ".md")
		if name == "README" {
			continue
		}
		docs = append(docs, console.SearchDoc{
			Group: console.GroupPlans, Kind: "plan", Title: name, Service: service, Ref: rel, Href: href,
			Command: "fathom plans status --service " + service + " --plan " + rel,
		})
		err := scanLines(plan, func(n int, line string) {
			if mm := planRowRe.FindStringSubmatch(line); mm != nil {
				docs = append(docs, console.SearchDoc{
					Group: console.GroupPlans, Kind: "row", Title: mm[3] + " " + clip(mm[2], 140),
					Subtitle: name, Service: service, Ref: rel + ":" + itoa(n), Href: href,
					Command: "fathom verify --service " + service + " --plan " + rel,
				})
			}
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	records := []struct {
		file string
		re   *regexp.Regexp
		kind string
	}{
		{"ADR.md", adrHeadRe, "decision"},
		{filepath.Join("docs", "intent", "invariants.md"), invariantRe, "invariant"},
		{filepath.Join("docs", "intent", "futures.md"), futureRe, "future"},
	}
	for _, rec := range records {
		path := filepath.Join(root, rec.file)
		err := scanLines(path, func(n int, line string) {
			if mm := rec.re.FindStringSubmatch(line); mm != nil {
				docs = append(docs, console.SearchDoc{
					Group: console.GroupRecords, Kind: rec.kind, Title: mm[1] + " " + clip(mm[2], 140),
					Service: service, Ref: rec.file + ":" + itoa(n), Href: href,
					Command: "fathom intent audit --json",
				})
			}
		})
		if err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	return docs, firstErr
}

func scanLines(path string, fn func(n int, line string)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	n := 0
	for sc.Scan() {
		n++
		fn(n, sc.Text())
	}
	return sc.Err()
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// lookupSymbols answers the Symbols group from the store: each service's
// symbol names, cached per service ingest (a fresh ingest invalidates it),
// filtered by substring. Documentation chunks are not symbols.
func (sc *consoleSearchCache) lookupSymbols(ctx context.Context, c *client.Client, services []string, q string, limit int) ([]console.SearchDoc, error) {
	if c == nil {
		return nil, nil
	}
	fields := strings.Fields(strings.ToLower(q))
	if len(fields) == 0 {
		return nil, nil
	}
	needle := fields[0]
	var out []console.SearchDoc
	var firstErr error
	for _, svc := range services {
		set, err := sc.symbolSet(ctx, c, svc)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		href := projectHref(svc)
		for _, e := range set.entries {
			if strings.Contains(strings.ToLower(e.name), needle) {
				out = append(out, console.SearchDoc{
					Group: console.GroupSymbols, Kind: "symbol", Title: e.name,
					Subtitle: e.kind + " in " + e.pkg, Service: svc, Ref: e.ref, Href: href,
				})
			}
		}
	}
	// Candidates only: Search ranks them. Keep the shortest names (exact and
	// prefix matches) when the candidate list is long.
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Title) != len(out[j].Title) {
			return len(out[i].Title) < len(out[j].Title)
		}
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].Ref < out[j].Ref
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, firstErr
}

func (sc *consoleSearchCache) symbolSet(ctx context.Context, c *client.Client, svc string) (consoleSymbolSet, error) {
	// The cache key is the service's last ingest (service_meta's commit and
	// time): cheap to read, and it changes on every ingest. (A full store
	// watermark walks the repository — far too slow for a keystroke.)
	meta, found, err := c.Store().GetServiceMeta(ctx, svc)
	if err != nil {
		return consoleSymbolSet{}, err
	}
	wm := "unrecorded"
	if found {
		wm = meta.IndexedCommit + "@" + itoa(int(meta.IndexedAt))
	}
	sc.mu.Lock()
	set, ok := sc.symbols[svc]
	sc.mu.Unlock()
	if ok && set.watermark == wm {
		return set, nil
	}
	obs, err := c.Store().QueryObservations(ctx, svc, "")
	if err != nil {
		return consoleSymbolSet{}, err
	}
	set = consoleSymbolSet{watermark: wm}
	for _, o := range obs {
		if o.Symbol == "" || o.SourceKind == "DocChunk" {
			continue
		}
		ref := o.SourceRef
		if o.Line > 0 && !strings.Contains(ref, ":") {
			ref += ":" + itoa(o.Line)
		}
		set.entries = append(set.entries, consoleSymbolEntry{name: o.Symbol, kind: string(o.SourceKind), pkg: o.Package, ref: ref})
	}
	sc.mu.Lock()
	sc.symbols[svc] = set
	sc.mu.Unlock()
	return set, nil
}

// buildSearchPage is GET /search.
func buildSearchPage(cr *consoleRequest) (consoleOut, error) {
	q := strings.TrimSpace(cr.r.URL.Query().Get("q"))
	res := console.Search(cr.deps.search.index(cr), q, console.DefaultPerGroup)
	title := "Search"
	if q != "" {
		title = "Search: " + q
	}
	return consoleOut{
		Title: title,
		Meta:  []string{searchSummary(res)},
		Model: res,
		Raw: func(w http.ResponseWriter) bool {
			if cr.r.URL.Query().Get("format") != "fragment" {
				return false
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(renderSearchResults(res)))
			return true
		},
		Body: func() string { return renderSearchResults(res) },
	}, nil
}

func searchSummary(res console.SearchResults) string {
	switch {
	case res.Query == "":
		return "type a project, plan, row ID, symbol, rule, verb or page"
	case res.Total == 0:
		return "no matches for “" + res.Query + "”"
	}
	return plural(res.Total, "match") + " for “" + res.Query + "”"
}

// renderSearchResults is the grouped result list — the palette's fragment
// and the no-JavaScript page body are the same markup.
func renderSearchResults(res console.SearchResults) string {
	var b strings.Builder
	b.WriteString(`<div data-console-search-results data-query="` + html.EscapeString(res.Query) + `">`)
	for _, n := range res.Notes {
		b.WriteString(`<p class="mb-2 text-xs text-caution">` + html.EscapeString(n) + `</p>`)
	}
	if res.Query == "" {
		b.WriteString(`<p class="text-ink-muted">Type in the search field above: a project, a plan or row ID (OC-11), a symbol, a rule (ADR-045, I31), a verb (fleet report) or a page.</p>`)
	} else if res.Total == 0 {
		b.WriteString(`<p class="text-ink-muted">Nothing matches “` + html.EscapeString(res.Query) + `”.</p>`)
	}
	for _, g := range res.Groups {
		b.WriteString(`<section class="mb-4" aria-labelledby="search-` + searchGroupSlug(g.Group) + `">`)
		b.WriteString(`<h2 id="search-` + searchGroupSlug(g.Group) + `" class="mb-1 text-xs font-semibold uppercase tracking-wider text-ink-muted">` + html.EscapeString(string(g.Group)))
		if g.Total > len(g.Results) {
			b.WriteString(` <span class="font-normal normal-case">(` + itoa(len(g.Results)) + ` of ` + itoa(g.Total) + `)</span>`)
		}
		b.WriteString(`</h2>`)
		if g.Note != "" {
			b.WriteString(`<p class="mb-1 text-xs text-caution">` + html.EscapeString(g.Note) + `</p>`)
		}
		b.WriteString(`<ul class="divide-y divide-line rounded-lg border border-line bg-surface-raised">`)
		for _, r := range g.Results {
			b.WriteString(`<li class="min-w-0" data-search-kind="` + html.EscapeString(r.Kind) + `">`)
			inner := `<span class="block font-medium text-ink [overflow-wrap:anywhere]">` + html.EscapeString(r.Title) + `</span>`
			var sub []string
			for _, s := range []string{r.Subtitle, r.Service, r.Ref} {
				if s != "" {
					sub = append(sub, s)
				}
			}
			if len(sub) > 0 {
				inner += `<span class="block text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(sub, " · ")) + `</span>`
			}
			if r.Href != "" {
				b.WriteString(`<a class="block min-h-11 px-3 py-2 hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="` + html.EscapeString(r.Href) + `">` + inner + `</a>`)
			} else {
				b.WriteString(`<div class="px-3 py-2">` + inner + `</div>`)
			}
			if r.Command != "" {
				b.WriteString(`<div class="px-3 pb-2">` + renderVerbChip(r.Command) + `</div>`)
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ul></section>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func searchGroupSlug(g console.SearchGroup) string {
	return strings.ReplaceAll(strings.ToLower(string(g)), " ", "-")
}
