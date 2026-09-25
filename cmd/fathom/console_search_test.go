package main

// console_search_test.go — /search's gathering and rendering (OC-11): every
// token filters the store's symbols before anything is cut, totals are
// exact, duplicate store rows show once, exact IDs lead, every match is
// reachable, the query is bounded, an unreadable plans directory is named,
// and a keystroke never rebuilds the observatory model.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
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
	sets := map[string][]consoleSymbolEntry{"fathom": entries}
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
	if got := symbolEntriesFrom(obs); len(got) != 2 {
		t.Errorf("entries = %+v, want the duplicate row once and doc chunks dropped", got)
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
	if len(res.Groups) != 1 || res.Groups[0].Group != console.GroupSymbols || len(res.Groups[0].Results) != res.Groups[0].Total || res.Groups[0].Total <= console.DefaultPerGroup {
		t.Errorf("the symbols group page = %+v, want every match of the one group", res.Groups)
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
