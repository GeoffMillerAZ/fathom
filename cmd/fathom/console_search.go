package main

// console_search.go — OC-11's server side (docs/design/observatory-console.md
// §2). GET /search?q= answers the command palette's index — projects, plans
// and plan rows, symbols (from the store), rules and records (invariants,
// decisions, futures), verbs (the CLI registry) and pages (the console
// registry) — as a full page on the shell, as the ⌘K palette's results
// fragment (?format=fragment or the kit script's X-AppKit-Palette header;
// console_palette.go), and as JSON. The ranking is console.Search; this file
// only gathers the documents.

import (
	"bufio"
	"context"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/palette"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
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
// intent records) may be; symbols are keyed by each service's content.
const searchDocsTTL = 60 * time.Second

// searchGroupPageSize is how many results one group's own page shows
// (design §2: anything over 50 rows pages).
const searchGroupPageSize = 50

// consoleSearchCache holds the gathered documents and the symbol index.
type consoleSearchCache struct {
	mu      sync.Mutex
	docs    []console.SearchDoc
	notes   []string
	builtAt time.Time
	symbols map[string]consoleSymbolSet
	keys    contentKeyCache
}

// consoleSymbolSet is one service's symbol names. Entries stay compact —
// a SearchDoc is built only for a kept match — because the set is held for
// the daemon's lifetime (the fleet carries ~137k symbols; O108 bounds
// memory).
type consoleSymbolSet struct {
	key     string
	entries []consoleSymbolEntry
}

// consoleSymbolEntry is one symbol and its lowered search fields (a field
// that is already lower case shares the original string).
type consoleSymbolEntry struct {
	name, kind, pkg, ref     string
	lname, lkind, lpkg, lref string
}

func newSymbolEntry(name, kind, pkg, ref string) consoleSymbolEntry {
	return consoleSymbolEntry{
		name: name, kind: kind, pkg: pkg, ref: ref,
		lname: strings.ToLower(name), lkind: strings.ToLower(kind), lpkg: strings.ToLower(pkg), lref: strings.ToLower(ref),
	}
}

func newConsoleSearchCache() *consoleSearchCache {
	return &consoleSearchCache{symbols: map[string]consoleSymbolSet{}}
}

// searchService is one searchable project: its ID and recorded root.
type searchService struct{ name, path string }

// searchServices lists the projects from the daemon's own store — cheap
// reads, so a palette keystroke never waits on the fleet model (the survey:
// the first keystroke after 30 s idle rebuilt it, 1–5 s and ~600 MB of
// allocation). Without a store (handler tests) the model answers.
func searchServices(cr *consoleRequest) ([]searchService, string) {
	c := cr.deps.c
	if c == nil {
		m := cr.model()
		out := make([]searchService, 0, len(m.Services))
		for _, s := range m.Services {
			out = append(out, searchService{name: s.Service, path: s.Path})
		}
		return out, m.DegradedNote
	}
	ctx := cr.r.Context()
	names, err := c.Services(ctx)
	if err != nil {
		return nil, "the store's service list could not be read (" + err.Error() + ")"
	}
	sort.Strings(names)
	out := make([]searchService, 0, len(names))
	for _, name := range names {
		svc := searchService{name: name}
		if meta, found, merr := c.Store().GetServiceMeta(ctx, name); merr == nil && found {
			svc.path = meta.Path
		}
		out = append(out, svc)
	}
	return out, ""
}

// index returns the search index for this request.
func (sc *consoleSearchCache) index(cr *consoleRequest) console.SearchIndex {
	services, degraded := searchServices(cr)
	sc.mu.Lock()
	if sc.builtAt.IsZero() || cr.now.Sub(sc.builtAt) > searchDocsTTL || cr.now.Before(sc.builtAt) {
		sc.docs, sc.notes = gatherSearchDocs(cr.deps.reg, services, degraded)
		sc.builtAt = cr.now
	}
	docs, notes := sc.docs, sc.notes
	sc.mu.Unlock()
	names := make([]string, 0, len(services))
	for _, s := range services {
		names = append(names, s.name)
	}
	c := cr.deps.c
	ctx := cr.r.Context()
	return console.SearchIndex{
		Docs:  docs,
		Notes: notes,
		Symbols: func(tokens []string, limit int) ([]console.SearchResult, int, error) {
			return sc.lookupSymbols(ctx, c, names, tokens, limit)
		},
	}
}

// gatherSearchDocs collects every non-symbol document: pages, verbs,
// projects, and each local project's plans, rows and intent records.
func gatherSearchDocs(reg *consoleRegistry, services []searchService, degraded string) ([]console.SearchDoc, []string) {
	docs := reg.searchDocs()
	for _, v := range verbTable {
		d := console.SearchDoc{Group: console.GroupVerbs, Kind: "verb", Title: v.Name, Subtitle: clip(v.Summary, 160), Command: "fathom " + v.Name}
		if p, ok := reg.pageRendering(v.Name); ok {
			d.Href = p.Href
		}
		docs = append(docs, d)
	}
	var unreadable []string
	for _, s := range services {
		docs = append(docs, console.SearchDoc{Group: console.GroupProjects, Kind: "service", Title: s.name, Subtitle: s.path, Service: s.name, Href: projectHref(s.name)})
		if s.path == "" {
			continue
		}
		fileDocs, err := projectSearchDocs(s.name, s.path)
		if err != nil {
			unreadable = append(unreadable, s.name+" ("+err.Error()+")")
		}
		docs = append(docs, fileDocs...)
	}
	var notes []string
	if len(unreadable) > 0 {
		sort.Strings(unreadable)
		notes = append(notes, "plans and intent records could not be read for: "+strings.Join(unreadable, "; "))
	}
	if degraded != "" {
		notes = append(notes, "projects are not searchable: "+degraded)
	}
	return docs, notes
}

var (
	planRowRe   = regexp.MustCompile(`^\s*- \[([ xX-])\] (.*?)\s*\{id: ([A-Za-z0-9_.:-]+)`)
	adrHeadRe   = regexp.MustCompile(`^## (ADR-\d+):?\s+(.*)$`)
	invariantRe = regexp.MustCompile(`^\| (I\d+) \| (.*?) \|`)
	futureRe    = regexp.MustCompile(`^## (F\d+)\.\s+(.*)$`)
)

// idTitle is a row's or record's title: its ID, then its text without the
// ID a writer already put at its start ("RA-23 RA-23 Hostile …" read twice).
func idTitle(id, text string) string {
	text = strings.TrimSpace(text)
	if rest, ok := strings.CutPrefix(text, id); ok && (rest == "" || rest[0] == ' ' || rest[0] == ':' || rest[0] == '.') {
		text = strings.TrimLeft(rest, " :.")
	}
	if text == "" {
		return id
	}
	return id + " " + clip(text, 140)
}

// projectSearchDocs reads one project's plans, plan rows and intent
// records. Links point at the project page (the plan and record pages are
// later waves); Ref is file:line and Command the verb that shows it. A
// plans directory or record that exists and cannot be read is returned as
// the error (the page names it) — never read as "no plans" (I38).
func projectSearchDocs(service, root string) ([]console.SearchDoc, error) {
	var docs []console.SearchDoc
	var firstErr error
	var plans []string
	entries, err := os.ReadDir(filepath.Join(root, "docs", "plans"))
	if err != nil && !os.IsNotExist(err) {
		firstErr = err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			plans = append(plans, filepath.Join(root, "docs", "plans", e.Name()))
		}
	}
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
					Group: console.GroupPlans, Kind: "row", ID: mm[3], Title: idTitle(mm[3], mm[2]),
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
					Group: console.GroupRecords, Kind: rec.kind, ID: mm[1], Title: idTitle(mm[1], mm[2]),
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

// lookupSymbols answers the Symbols group (console.SymbolLookup): every
// service's symbols, cached per service content, filtered by EVERY token
// and ranked before anything is cut, with the exact match count.
func (sc *consoleSearchCache) lookupSymbols(ctx context.Context, c *client.Client, services []string, tokens []string, limit int) ([]console.SearchResult, int, error) {
	if c == nil || len(tokens) == 0 {
		return nil, 0, nil
	}
	sets := make(map[string][]consoleSymbolEntry, len(services))
	var firstErr error
	for _, svc := range services {
		set, err := sc.symbolSet(ctx, c, svc)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		sets[svc] = set.entries
	}
	rs, total := rankSymbolEntries(services, sets, tokens, limit)
	return rs, total, firstErr
}

// symbolCand is one matching symbol before it becomes a SearchResult.
type symbolCand struct {
	e     *consoleSymbolEntry
	svc   string
	href  string
	score int
}

// symbolCandLess is console.RankLess over candidates (score, then the
// shorter title, the title, the href, the ref) — the same order, so the
// kept top-k are exactly the results Search would rank first.
func symbolCandLess(a, b symbolCand) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if len(a.e.name) != len(b.e.name) {
		return len(a.e.name) < len(b.e.name)
	}
	if a.e.name != b.e.name {
		return a.e.name < b.e.name
	}
	if a.href != b.href {
		return a.href < b.href
	}
	return a.e.ref < b.e.ref
}

// rankSymbolEntries scores every entry of every service against all tokens
// (console.ScoreFields: the name is the title; kind, package, service and
// ref are the rest), keeps the best limit and counts every match.
func rankSymbolEntries(services []string, sets map[string][]consoleSymbolEntry, tokens []string, limit int) ([]console.SearchResult, int) {
	var cands []symbolCand
	total := 0
	prune := func() {
		sort.SliceStable(cands, func(i, j int) bool { return symbolCandLess(cands[i], cands[j]) })
		if limit > 0 && len(cands) > limit {
			cands = cands[:limit]
		}
	}
	for _, svc := range services {
		lsvc := strings.ToLower(svc)
		href := projectHref(svc)
		entries := sets[svc]
		for i := range entries {
			e := &entries[i]
			rest := [4]string{e.lkind, e.lpkg, lsvc, e.lref}
			s := console.ScoreFields(e.lname, "", rest[:], tokens)
			if s == 0 {
				continue
			}
			total++
			cands = append(cands, symbolCand{e: e, svc: svc, href: href, score: s})
			if limit > 0 && len(cands) >= 4*limit+256 {
				prune()
			}
		}
	}
	prune()
	out := make([]console.SearchResult, 0, len(cands))
	for _, c := range cands {
		out = append(out, console.SearchResult{
			SearchDoc: console.SearchDoc{
				Group: console.GroupSymbols, Kind: "symbol", Title: c.e.name,
				Subtitle: c.e.kind + " in " + c.e.pkg, Service: c.svc, Ref: c.e.ref, Href: c.href,
			},
			Score: c.score,
		})
	}
	return out, total
}

// symbolEntriesFrom turns a service's observations into its symbol
// entries: documentation chunks are not symbols, and a row the store holds
// twice (same symbol, kind, package and place — differing only in
// ingested_at) is one entry, never a duplicate result.
func symbolEntriesFrom(obs []core.Observation) []consoleSymbolEntry {
	seen := map[[4]string]bool{}
	lower := map[string]string{} // kinds and packages repeat: lower each once
	low := func(s string) string {
		if l, ok := lower[s]; ok {
			return l
		}
		l := strings.ToLower(s)
		lower[s] = l
		return l
	}
	var out []consoleSymbolEntry
	for _, o := range obs {
		if o.Symbol == "" || o.SourceKind == "DocChunk" {
			continue
		}
		ref := o.SourceRef
		if o.Line > 0 && !strings.Contains(ref, ":") {
			ref += ":" + itoa(o.Line)
		}
		k := [4]string{o.Symbol, string(o.SourceKind), o.Package, ref}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, consoleSymbolEntry{
			name: o.Symbol, kind: string(o.SourceKind), pkg: o.Package, ref: ref,
			lname: strings.ToLower(o.Symbol), lkind: low(string(o.SourceKind)), lpkg: low(o.Package), lref: strings.ToLower(ref),
		})
	}
	return out
}

func (sc *consoleSearchCache) symbolSet(ctx context.Context, c *client.Client, svc string) (consoleSymbolSet, error) {
	// The cache key is the service's content (its commit and the store's
	// fingerprint digest, recomputed only when service_meta.indexed_at
	// moves): a hash-gated no-op re-ingest keeps it, so it no longer
	// reloads every symbol on the next keystroke.
	_, key, found, err := sc.keys.key(ctx, c.Store(), svc)
	if err != nil {
		return consoleSymbolSet{}, err
	}
	if !found {
		key = "unrecorded"
	}
	sc.mu.Lock()
	set, ok := sc.symbols[svc]
	sc.mu.Unlock()
	if ok && set.key == key {
		return set, nil
	}
	obs, err := c.Store().QueryObservations(ctx, svc, "")
	if err != nil {
		return consoleSymbolSet{}, err
	}
	set = consoleSymbolSet{key: key, entries: symbolEntriesFrom(obs)}
	sc.mu.Lock()
	sc.symbols[svc] = set
	sc.mu.Unlock()
	return set, nil
}

// searchGroupBySlug maps a ?group= slug to its group.
func searchGroupBySlug(slug string) (console.SearchGroup, bool) {
	for _, g := range console.SearchGroupOrder {
		if searchGroupSlug(g) == slug {
			return g, true
		}
	}
	return "", false
}

// searchHref is the search page for q, optionally one group's page.
func searchHref(q string, g console.SearchGroup, page int) string {
	v := url.Values{"q": {q}}
	if g != "" {
		v.Set("group", searchGroupSlug(g))
	}
	if page > 1 {
		v.Set("page", itoa(page))
	}
	return "/search?" + v.Encode()
}

// buildSearchPage is GET /search. A query past console's bounds is a 400
// on the shell, answered before any document is scored.
func buildSearchPage(cr *consoleRequest) (consoleOut, error) {
	query := cr.r.URL.Query()
	q := strings.TrimSpace(query.Get("q"))
	if err := console.CheckQuery(q); err != nil {
		if wantsPaletteFragment(cr.r) {
			// The palette shows the bound it crossed, not a generic failure.
			return consoleOut{Raw: func(w http.ResponseWriter) bool {
				writePaletteFragment(w, renderPaletteQueryError(q, err))
				return true
			}}, nil
		}
		return consoleOut{}, &consoleError{
			Status: http.StatusBadRequest, Title: "Search too long",
			Reason:      err.Error() + ".",
			Remedy:      "search with at most " + itoa(console.MaxQueryTokens) + " words and " + itoa(console.MaxQueryRunes) + " characters",
			RemedyHref:  "/search",
			RemedyLabel: "Start a new search",
		}
	}
	opts := console.SearchOptions{PerGroup: console.DefaultPerGroup}
	if slug := query.Get("group"); slug != "" {
		g, ok := searchGroupBySlug(slug)
		if !ok {
			return consoleOut{}, &consoleError{
				Status: http.StatusBadRequest, Title: "Unknown result group",
				Reason: "Search has no result group named " + clip(slug, 40) + ".", Remedy: "drop ?group=, or use one of: projects, plans-and-rows, symbols, rules-and-records, verbs, pages",
				RemedyHref: searchHref(q, "", 1), RemedyLabel: "Search every group",
			}
		}
		page := 1
		if n, err := strconv.Atoi(query.Get("page")); err == nil && n > 1 {
			page = n
		}
		opts = console.SearchOptions{Group: g, PerGroup: searchGroupPageSize, Offset: (page - 1) * searchGroupPageSize}
	}
	res := console.SearchWith(cr.deps.search.index(cr), q, opts)
	title := "Search"
	if q != "" {
		title = "Search: " + q
	}
	if res.Group != "" {
		title += " — " + string(res.Group)
	}
	return consoleOut{
		Title: title,
		Meta:  []string{searchSummary(res)},
		Model: res,
		Raw: func(w http.ResponseWriter) bool {
			// One URL answers the page, the palette's fragment and JSON.
			w.Header().Set("Vary", palette.FragmentHeader)
			if !wantsPaletteFragment(cr.r) {
				return false
			}
			writePaletteFragment(w, renderPaletteFragment(res, q))
			return true
		},
		// The no-JavaScript path: the palette trigger is a link here, so the
		// page carries its own search form above the same ranked results.
		Body: func() string {
			return `<div class="mb-4 max-w-2xl">` + consolePaletteSearchForm(q) + `</div>` + renderSearchResults(res)
		},
	}, nil
}

// matchCount is "1 match" / "1,522 matches".
func matchCount(n int) string {
	if n == 1 {
		return "1 match"
	}
	return fmtThousands(n) + " matches"
}

func searchSummary(res console.SearchResults) string {
	switch {
	case res.Query == "":
		return "type a project, plan, row ID, symbol, rule, verb or page"
	case res.Total == 0:
		return "no matches for “" + res.Query + "”"
	case res.Group != "":
		return matchCount(res.Total) + " in " + string(res.Group) + " for “" + res.Query + "”"
	}
	return matchCount(res.Total) + " for “" + res.Query + "”"
}

// renderSearchResults is the grouped result list — the palette's fragment
// and the no-JavaScript page body are the same markup. A capped group links
// to its own page, which shows every match 50 at a time.
func renderSearchResults(res console.SearchResults) string {
	var b strings.Builder
	b.WriteString(`<div data-console-search-results data-query="` + html.EscapeString(res.Query) + `">`)
	for _, n := range res.Notes {
		b.WriteString(`<p class="mb-2 text-xs text-caution">` + html.EscapeString(n) + `</p>`)
	}
	if res.Query == "" {
		b.WriteString(chrome.RenderEmptyState(chrome.EmptyState{
			Heading: "Search the console",
			Body:    "Type in the search field: a project, a plan or row ID (OC-11), a symbol, a rule (ADR-045, I31), a verb (fleet report) or a page.",
		}))
	} else if res.Total == 0 {
		b.WriteString(chrome.RenderEmptyState(chrome.EmptyState{
			Heading: "Nothing matches “" + res.Query + "”",
			Body:    "Every word must match. Try fewer words, a row or record ID (OC-11, ADR-045) or a project name.",
		}))
	}
	if res.Group != "" && res.Query != "" {
		b.WriteString(`<p class="mb-3 [&>a]:min-h-[44px]">` + form.RenderButton(form.Button{Label: "← every group", Href: searchHref(res.Query, "", 1), Variant: form.ButtonGhost}) + `</p>`)
	}
	for _, g := range res.Groups {
		b.WriteString(`<section class="mb-4" aria-labelledby="search-` + searchGroupSlug(g.Group) + `">`)
		b.WriteString(`<h2 id="search-` + searchGroupSlug(g.Group) + `" class="mb-1 text-xs font-semibold uppercase tracking-wider text-ink-muted">` + html.EscapeString(string(g.Group)))
		if g.Total > len(g.Results) {
			from := res.Offset + 1
			if len(g.Results) == 0 {
				from = res.Offset
			}
			span := itoa(len(g.Results))
			if res.Group != "" {
				span = itoa(from) + "–" + itoa(res.Offset+len(g.Results))
			}
			b.WriteString(` <span class="font-normal normal-case">(` + span + ` of ` + fmtThousands(g.Total) + `)</span>`)
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
		b.WriteString(`</ul>`)
		switch {
		case res.Group == "" && g.Total > len(g.Results):
			b.WriteString(`<p class="mt-1 [&>a]:min-h-[44px]">` + form.RenderButton(form.Button{Label: "Show all " + fmtThousands(g.Total) + " in " + string(g.Group), Href: searchHref(res.Query, g.Group, 1), Variant: form.ButtonGhost}) + `</p>`)
		case res.Group != "":
			pages := (g.Total + res.PerGroup - 1) / res.PerGroup
			group := g.Group
			b.WriteString(`<div class="mt-2 [&_a]:inline-flex [&_a]:min-h-11 [&_a]:items-center [&_span]:inline-flex [&_span]:min-h-11 [&_span]:items-center">` + chrome.RenderPagination(chrome.Pagination{
				Page: res.Offset/res.PerGroup + 1, TotalPages: pages, AriaLabel: string(g.Group) + " pages",
				HrefFor: func(n int) string { return searchHref(res.Query, group, n) },
			}) + `</div>`)
		}
		b.WriteString(`</section>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func searchGroupSlug(g console.SearchGroup) string {
	return strings.ReplaceAll(strings.ToLower(string(g)), " ", "-")
}
