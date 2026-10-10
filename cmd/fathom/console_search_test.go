package main

// console_search_test.go — /search's gathering and rendering (OC-11): every
// token filters the store's symbols before anything is cut, totals are
// exact, duplicate store rows show once, exact IDs lead, every match is
// reachable, the query is bounded, an unreadable plans directory is named,
// and a keystroke never rebuilds the observatory model.

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

func TestConsoleSearch_SymbolsMatchEveryTokenWithExactTotals(t *testing.T) {
	// 300 short names contain "test"; the one that also contains
	// "resolvecalllog" is long, so a cut by the first token and name length
	// before the other tokens were applied lost it (and token order changed
	// the answer).
	var entries []consoleSymbolEntry
	for i := 0; i < 300; i++ {
		entries = append(entries, newSymbolEntry(fmt.Sprintf("Test%03d", i), "GoFunc", "main", "x_test.go:1"))
	}
	entries = append(entries, newSymbolEntry("TestResolveCallLogPath_FallsBackWithNoConfig", "GoFunc", "main", "calllog_test.go:9"))
	sets := map[string]symbolSource{"fathom": entrySlice(entries)}
	for _, q := range []string{"resolveCallLog test", "test resolveCallLog", "Test resolvecalllog"} {
		rs, total := rankSymbolEntries([]string{"fathom"}, sets, strings.Fields(strings.ToLower(q)), 8)
		if total != 1 || len(rs) != 1 || rs[0].Title != "TestResolveCallLogPath_FallsBackWithNoConfig" {
			t.Errorf("%q = %d total, %v; want the one symbol carrying both tokens", q, total, rs)
		}
	}
	rs, total := rankSymbolEntries([]string{"fathom"}, sets, []string{"test"}, 8)
	if total != 301 || len(rs) != 8 {
		t.Errorf("test = %d shown of %d, want 8 of the exact 301", len(rs), total)
	}

	// Duplicate store rows (the same symbol at the same place, differing
	// only in ingested_at) are one result.
	obs := []core.Observation{
		{Symbol: "resolveCallLogCandidates", SourceKind: "GoFunc", Package: "main", SourceRef: "cmd/fathom/doctor.go", Line: 1198},
		{Symbol: "resolveCallLogCandidates", SourceKind: "GoFunc", Package: "main", SourceRef: "cmd/fathom/doctor.go", Line: 1198},
		{Symbol: "resolveCallLogCandidates", SourceKind: "GoFunc", Package: "main", SourceRef: "cmd/fathom/other.go", Line: 3},
		{Symbol: "Readme", SourceKind: "DocChunk", SourceRef: "README.md"},
	}
	nodes := make([]core.SymbolNode, len(obs))
	for i, o := range obs {
		nodes[i] = o.ToSymbolNode()
	}
	if got := memo.NewSnapshot(nodes).Palette(); len(got) != 2 {
		t.Errorf("palette = %v, want the duplicate row once and doc chunks dropped", got)
	}
}

func TestConsoleSearch_BoundedReachableAndHonest(t *testing.T) {
	f := newConsoleFixture(t)

	// A keystroke never rebuilds the fleet model (1–5 s and ~600 MB of
	// allocation on the real fleet after 30 s idle): the service list
	// comes from the store.
	resetObsCache()
	if code, _, _ := f.get(t, "/search?q=charge&format=fragment"); code != http.StatusOK {
		t.Fatalf("fragment = %d", code)
	}
	obsCache.Lock()
	built := !obsCache.at.IsZero()
	obsCache.Unlock()
	if built {
		t.Error("a search keystroke built the observatory model")
	}

	// Plural, and a capped group links to every match, paged.
	_, _, page := f.get(t, "/search?q=e")
	if strings.Contains(page, "matchs") || !strings.Contains(page, " matches for") {
		t.Error(`the summary must say "matches"`)
	}
	if !strings.Contains(page, `href="/search?group=symbols&amp;q=e"`) {
		t.Fatalf("a capped group must link to all of its matches:\n%.2000s", page)
	}
	env := f.pageJSON(t, "/search?q=e&group=symbols")
	var res console.SearchResults
	if err := json.Unmarshal(env.Model, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Groups) != 1 || res.Groups[0].Group != console.GroupSymbols || len(res.Groups[0].Results) != min(res.Groups[0].Total, searchGroupPageSize) || res.Groups[0].Total <= console.DefaultPerGroup {
		t.Errorf("the symbols group page = %+v, want every match of the one group, up to its page size", res.Groups)
	}

	// The kit's empty state, not a bare paragraph.
	_, _, none := f.get(t, "/search?q=zzzzqqq")
	if !strings.Contains(none, `rounded-lg border border-line bg-surface-sunken p-8 text-center`) || !strings.Contains(none, "Nothing matches") {
		t.Errorf("no-result state must be chrome.RenderEmptyState:\n%.1500s", none)
	}
	_, _, blank := f.get(t, "/search")
	if !strings.Contains(blank, `bg-surface-sunken p-8 text-center`) {
		t.Error("the no-query state must be chrome.RenderEmptyState")
	}

	// A query past the bounds is a 400 on the shell, never a scan.
	code, _, long := f.get(t, "/search?q="+url.QueryEscape(strings.Repeat("e ", 9)))
	if code != http.StatusBadRequest || !strings.Contains(long, `<main id="observatory-main"`) || !strings.Contains(long, "at most 8") {
		t.Errorf("an over-long query = %d:\n%.800s", code, long)
	}
}

func TestProjectSearchDocs_RowIDsAndUnreadablePlans(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "docs", "plans", "p.md"), "- [-] RA-23 Hostile Docker isolation is proven {id: RA-23}\n- [ ] OC-1 Page projections {id: OC-1}\n")
	docs, err := projectSearchDocs("svc", root)
	if err != nil {
		t.Fatal(err)
	}
	var titles, ids []string
	for _, d := range docs {
		if d.Kind == "row" {
			titles = append(titles, d.Title)
			ids = append(ids, d.ID)
		}
	}
	if strings.Join(titles, "|") != "RA-23 Hostile Docker isolation is proven|OC-1 Page projections" || strings.Join(ids, ",") != "RA-23,OC-1" {
		t.Errorf("rows = %q ids %q, want each ID once and carried as the row's ID", titles, ids)
	}
	// An unreadable plans directory is an error the page names, never an
	// empty plan list (I38).
	plans := filepath.Join(root, "docs", "plans")
	if err := os.Chmod(plans, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(plans, 0o755) })
	if _, err := os.ReadDir(plans); err == nil {
		t.Skip("running with privileges that read a mode-000 directory")
	}
	if _, err := projectSearchDocs("svc", root); err == nil {
		t.Error("an unreadable docs/plans must be reported, not read as no plans")
	}
}

// A plan result opens its plan page and a row result its row page — the
// routes the registry serves (plan, plan-row), never the project page
// (UX-M1 finding 1: even an exact row ID landed on /projects/{svc}). Rows
// are read with the plan parser the row page resolves them with, so an
// in-progress row, a starred bullet and an id that is not the first
// annotation key are rows too, and the plan result carries the plan's own
// title for search.
func TestProjectSearchDocs_PlansAndRowsLinkToTheirPages(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "docs", "plans", "p.md"), "# The P plan\n\n- [ ] OC-1 Page projections {id: OC-1}\n- [~] Nightly batch {symbol: NightlyBatch, id: NB-2}\n* [x] Starred row {id: ST-3}\n")
	mustWrite(t, filepath.Join(root, "docs", "plans", "README.md"), "- [ ] not a plan {id: R-1}\n")
	docs, err := projectSearchDocs("svc", root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]console.SearchDoc{}
	for _, d := range docs {
		got[d.Kind+" "+d.ID+d.Title] = d
	}
	plan, ok := got["plan p"]
	if !ok {
		t.Fatalf("no plan result: %+v", docs)
	}
	if plan.Href != planHref("svc", "p") || plan.Subtitle != "The P plan" {
		t.Errorf("plan result = href %q subtitle %q, want the plan page and the plan's title", plan.Href, plan.Subtitle)
	}
	for id, title := range map[string]string{"OC-1": "OC-1 Page projections", "NB-2": "NB-2 Nightly batch", "ST-3": "ST-3 Starred row"} {
		d, ok := got["row "+id+title]
		if !ok {
			t.Errorf("row %s is not indexed as %q: %+v", id, title, docs)
			continue
		}
		if d.Href != planRowHref("svc", "p", id) {
			t.Errorf("row %s href = %q, want its row page %q", id, d.Href, planRowHref("svc", "p", id))
		}
		if want := "fathom verify --service svc --plan docs/plans/p.md --row " + id + " --json"; d.Command != want {
			t.Errorf("row %s command = %q, want the row page's verb %q", id, d.Command, want)
		}
	}
	for _, d := range docs {
		if d.ID == "R-1" {
			t.Error("README.md is not a plan (the verbs' discovery excludes it)")
		}
	}
}

// A plan past searchPlanMaxBytes is refused and named, never indexed as if
// it were shorter (I38); the plans beside it are still indexed.
func TestProjectSearchDocs_RefusesAPlanPastTheCap(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "docs", "plans", "a.md"), "# A\n\n- [ ] Row {id: A-1}\n")
	big := "# Big\n\n- [ ] First row {id: BIG-1}\n" + strings.Repeat("x", searchPlanMaxBytes)
	mustWrite(t, filepath.Join(root, "docs", "plans", "big.md"), big)
	docs, err := projectSearchDocs("svc", root)
	if err == nil || !strings.Contains(err.Error(), "big.md exceeds the 4 MiB plan cap") {
		t.Errorf("err = %v, want big.md named past the cap", err)
	}
	var a, bigRow bool
	for _, d := range docs {
		a = a || d.ID == "A-1"
		bigRow = bigRow || d.ID == "BIG-1"
	}
	if !a || bigRow {
		t.Errorf("indexed A-1 %v, BIG-1 %v: want the plan under the cap indexed and the one past it not", a, bigRow)
	}
}

// resultHrefRe reads a search page's result links, in order; paletteHrefRe
// the palette fragment's options.
var (
	resultHrefRe  = regexp.MustCompile(`<li class="min-w-0" data-search-kind="[^"]*"><a [^>]*href="([^"]*)"`)
	paletteHrefRe = regexp.MustCompile(`<a href="([^"]*)" data-palette-option`)
)

// Search, the ⌘K fragment and the JSON agree on where a plan or row result
// goes, and an exact row ID leads: the first result is that row's page, in
// all three renderings, and every such destination is a page that answers.
func TestConsoleSearch_PlanAndRowResultsOpenTheirPages(t *testing.T) {
	f := newConsoleFixture(t)
	for _, tc := range []struct{ q, want string }{
		{"S-5", "/plans/proof-service/settlement/rows/S-5"},
		{"s-6", "/plans/proof-service/settlement/rows/S-6"}, // an in-progress [~] row
		{"Settlement ledger", "/plans/proof-service/settlement"},
		{"Audit (fixture plan)", "/plans/proof-service/audit"}, // the plan's title, not its file name
	} {
		env := f.pageJSON(t, "/search?q="+url.QueryEscape(tc.q))
		var res console.SearchResults
		if err := json.Unmarshal(env.Model, &res); err != nil {
			t.Fatal(err)
		}
		var jsonHrefs []string
		for _, g := range res.Groups {
			for _, r := range g.Results {
				jsonHrefs = append(jsonHrefs, r.Href)
			}
		}
		if len(res.Groups) == 0 || res.Groups[0].Group != console.GroupPlans || len(jsonHrefs) == 0 || jsonHrefs[0] != tc.want {
			t.Errorf("%q: JSON leads with %v, want %s first", tc.q, jsonHrefs, tc.want)
			continue
		}
		_, _, page := f.get(t, "/search?q="+url.QueryEscape(tc.q))
		var pageHrefs []string
		for _, m := range resultHrefRe.FindAllStringSubmatch(page, -1) {
			pageHrefs = append(pageHrefs, html.UnescapeString(m[1]))
		}
		_, _, frag := f.get(t, "/search?format=fragment&q="+url.QueryEscape(tc.q))
		var fragHrefs []string
		for _, m := range paletteHrefRe.FindAllStringSubmatch(frag, -1) {
			if h := html.UnescapeString(m[1]); !strings.HasPrefix(h, "/search?") || !strings.Contains(h, "group=") {
				fragHrefs = append(fragHrefs, h) // not a "Show all" link
			}
		}
		if strings.Join(pageHrefs, " ") != strings.Join(jsonHrefs, " ") || strings.Join(fragHrefs, " ") != strings.Join(jsonHrefs, " ") {
			t.Errorf("%q: the renderings disagree:\n json %v\n page %v\n palette %v", tc.q, jsonHrefs, pageHrefs, fragHrefs)
		}
		if code, _, _ := f.get(t, tc.want); code != http.StatusOK {
			t.Errorf("%q: %s = %d", tc.q, tc.want, code)
		}
	}
}

// Doctrine rule 1: the brief's heading action is the kit's link-mode ghost
// button, never a hand-rolled look-alike anchor.
func TestConsoleBrief_DownloadIsTheKitButton(t *testing.T) {
	f := newConsoleFixture(t)
	_, _, page := f.get(t, "/brief")
	want := form.RenderButton(form.Button{Label: "Download markdown", Href: "/brief?format=md", Variant: form.ButtonGhost})
	if !strings.Contains(page, want) {
		t.Errorf("the brief's download action must be form.RenderButton's ghost link:\n%s", want)
	}
}
