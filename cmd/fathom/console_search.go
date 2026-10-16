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
	"fmt"
	"html"
	"io"
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

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/pkg/client"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/palette"
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

// consoleSearchCache holds the gathered documents. Symbols come from the
// daemon's one symbol snapshot per (service, watermark) (internal/memo,
// RT-75) — the palette keeps no copy of its own.
type consoleSearchCache struct {
	mu      sync.Mutex
	docs    []console.SearchDoc
	notes   []string
	builtAt time.Time
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

// symbolSource is one service's palette rows: scored on the lowered
// fields, materialized as an entry only for a match.
type symbolSource interface {
	Len() int
	Lowered(i int) (lname, lkind, lpkg, lref string)
	Entry(i int) consoleSymbolEntry
}

// entrySlice is a symbolSource over entries already built.
type entrySlice []consoleSymbolEntry

func (e entrySlice) Len() int { return len(e) }
func (e entrySlice) Lowered(i int) (string, string, string, string) {
	return e[i].lname, e[i].lkind, e[i].lpkg, e[i].lref
}
func (e entrySlice) Entry(i int) consoleSymbolEntry { return e[i] }

// snapshotSource is a symbolSource over a service's shared snapshot.
type snapshotSource struct{ s *memo.Snapshot }

func (p snapshotSource) Len() int { return len(p.s.Palette()) }
func (p snapshotSource) Lowered(i int) (string, string, string, string) {
	l := p.s.Lower(int(p.s.Palette()[i]))
	return l.Name, l.Kind, l.Package, l.Ref
}
func (p snapshotSource) Entry(i int) consoleSymbolEntry {
	j := int(p.s.Palette()[i])
	n := &p.s.Nodes[j]
	l := p.s.Lower(j)
	return consoleSymbolEntry{name: n.Name, kind: string(n.Kind), pkg: n.Package, ref: p.s.Ref(j),
		lname: l.Name, lkind: l.Kind, lpkg: l.Package, lref: l.Ref}
}

func newConsoleSearchCache() *consoleSearchCache {
	return &consoleSearchCache{}
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
	for _, v := range verbs() {
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
// records. A plan links to its plan page and a row to its row page — the
// registry's plan and plan-row routes — so a search, the palette and the
// JSON all land where the answer is (UX-M1 finding 1: they sent both to the
// project page, even for an exact row ID). The plans are the verbs'
// discovery (activePlanFiles) and the rows the plan parser's, the same the
// row page resolves an ID with, so every result opens a page that exists:
// an in-progress [~] row or an id that is not the first annotation key is a
// row here too. Records still link to the project page. Ref is file:line
// and Command the verb that shows it. A plans directory or record that
// exists and cannot be read is returned as the error (the page names it) —
// never read as "no plans" (I38).
func projectSearchDocs(service, root string) ([]console.SearchDoc, error) {
	var docs []console.SearchDoc
	var firstErr error
	plans, err := activePlanFiles(root)
	if err != nil {
		firstErr = err
	}
	for _, plan := range plans {
		raw, err := readSearchPlan(plan)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		rel := filepath.ToSlash(filepath.Join("docs", "plans", filepath.Base(plan)))
		slug := console.PlanSlug(plan)
		docs = append(docs, console.SearchDoc{
			Group: console.GroupPlans, Kind: "plan", Title: slug, Subtitle: planTitle(raw, plan), Service: service, Ref: rel,
			Href:    planHref(service, slug),
			Command: "fathom plans status --service " + service + " --plan " + rel,
		})
		for _, t := range verify.ParseMarkdownPlan(service, raw).Tasks {
			docs = append(docs, console.SearchDoc{
				Group: console.GroupPlans, Kind: "row", ID: t.ID, Title: idTitle(t.ID, t.Title),
				Subtitle: slug, Service: service, Ref: rel + ":" + itoa(t.Line),
				Href:    planRowHref(service, slug, t.ID),
				Command: "fathom verify --service " + service + " --plan " + rel + " --row " + t.ID + " --json",
			})
		}
	}
	href := projectHref(service)
	type recordFile struct {
		file string
		re   *regexp.Regexp
		kind string
	}
	records := []recordFile{
		{"ADR.md", adrHeadRe, "decision"},
		{"docs/intent/invariants.md", invariantRe, "invariant"},
	}
	// futures.md is a router over docs/intent/futures/*.md (K8): the
	// family's files, in the order the corpus reads them, each scanned for
	// the `## F<n>.` headings that define a future — a router's listing
	// row is not one, so every future is found once, where it is defined.
	futures, err := intent.FamilyFiles(root, "futures")
	if err != nil && firstErr == nil {
		firstErr = err
	}
	for _, f := range futures {
		records = append(records, recordFile{f, futureRe, "future"})
	}
	for _, rec := range records {
		path := filepath.Join(root, filepath.FromSlash(rec.file))
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

// searchPlanMaxBytes caps one plan read by the search index: the largest
// plan in the fleet is under 100 KB, so a plan past 4 MiB is refused and
// named (the page's note), never parsed as if it were shorter (I38).
const searchPlanMaxBytes = 4 << 20

// readSearchPlan reads one plan for the search index under
// searchPlanMaxBytes, so the index rebuild's cost is bounded per plan
// rather than following a file's size (tenancy design §4.7).
func readSearchPlan(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, searchPlanMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > searchPlanMaxBytes {
		return nil, fmt.Errorf("%s exceeds the %d MiB plan cap", filepath.Base(path), searchPlanMaxBytes>>20)
	}
	return raw, nil
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
// service's symbols, from the daemon's shared snapshot (RT-75), filtered by
// EVERY token and ranked before anything is cut, with the exact match
// count.
func (sc *consoleSearchCache) lookupSymbols(ctx context.Context, c *client.Client, services []string, tokens []string, limit int) ([]console.SearchResult, int, error) {
	if c == nil || len(tokens) == 0 {
		return nil, 0, nil
	}
	sets := make(map[string]symbolSource, len(services))
	var firstErr error
	for _, svc := range services {
		snap, err := consoleSnapshot(ctx, c, svc)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		sets[svc] = snapshotSource{snap}
	}
	rs, total := rankSymbolEntries(services, sets, tokens, limit)
	return rs, total, firstErr
}

// symbolCand is one matching symbol before it becomes a SearchResult.
type symbolCand struct {
	e     consoleSymbolEntry
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
func rankSymbolEntries(services []string, sets map[string]symbolSource, tokens []string, limit int) ([]console.SearchResult, int) {
	var cands []symbolCand
	total := 0
	prune := func() {
		sort.SliceStable(cands, func(i, j int) bool { return symbolCandLess(cands[i], cands[j]) })
		if limit > 0 && len(cands) > limit {
			cands = cands[:limit]
		}
	}
	for _, svc := range services {
		src := sets[svc]
		if src == nil {
			continue
		}
		lsvc := strings.ToLower(svc)
		href := projectHref(svc)
		for i, n := 0, src.Len(); i < n; i++ {
			lname, lkind, lpkg, lref := src.Lowered(i)
			rest := [4]string{lkind, lpkg, lsvc, lref}
			s := console.ScoreFields(lname, "", rest[:], tokens)
			if s == 0 {
				continue
			}
			total++
			cands = append(cands, symbolCand{e: src.Entry(i), svc: svc, href: href, score: s})
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
			b.WriteString(consolePager(chrome.Pagination{
				Page: res.Offset/res.PerGroup + 1, TotalPages: pages, AriaLabel: string(g.Group) + " pages",
				HrefFor: func(n int) string { return searchHref(res.Query, group, n) },
			}))
		}
		b.WriteString(`</section>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func searchGroupSlug(g console.SearchGroup) string {
	return strings.ReplaceAll(strings.ToLower(string(g)), " ", "-")
}
