package console_test

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// fixtureSymbols is a SymbolLookup over docs, honoring its contract: every
// token matches, results ranked like every group, the total exact.
func fixtureSymbols(all []console.SearchDoc) console.SymbolLookup {
	return func(tokens []string, limit int) ([]console.SearchResult, int, error) {
		var out []console.SearchResult
		for _, d := range all {
			d.Group = console.GroupSymbols
			if sc := console.ScoreDoc(d, tokens); sc > 0 {
				out = append(out, console.SearchResult{SearchDoc: d, Score: sc})
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return console.RankLess(out[i], out[j]) })
		total := len(out)
		if limit > 0 && len(out) > limit {
			out = out[:limit]
		}
		return out, total, nil
	}
}

func searchFixture() console.SearchIndex {
	return console.SearchIndex{
		Docs: []console.SearchDoc{
			{Group: console.GroupPages, Kind: "page", Title: "Fleet", Subtitle: "How is every project doing?", Href: "/fleet"},
			{Group: console.GroupPages, Kind: "page", Title: "Impact", Subtitle: "Has fathom helped?", Href: "/impact"},
			{Group: console.GroupProjects, Kind: "service", Title: "payment-service", Subtitle: "/repos/payment-service", Href: "/projects/payment-service"},
			{Group: console.GroupProjects, Kind: "service", Title: "checkout-service", Subtitle: "/repos/checkout-service", Href: "/projects/checkout-service"},
			{Group: console.GroupPlans, Kind: "plan", Title: "observatory-console", Service: "fathom", Ref: "docs/plans/observatory-console.md", Command: "fathom plans status --service fathom --plan docs/plans/observatory-console.md"},
			{Group: console.GroupPlans, Kind: "row", Title: "OC-11 /search returns grouped results", Service: "fathom", Ref: "docs/plans/observatory-console.md:21"},
			{Group: console.GroupRecords, Kind: "decision", Title: "ADR-045 The Observatory becomes a console", Service: "fathom", Ref: "ADR.md:2396"},
			{Group: console.GroupRecords, Kind: "invariant", Title: "I31 store default resolves under workspace", Service: "fathom", Ref: "docs/intent/invariants.md:40"},
			{Group: console.GroupVerbs, Kind: "verb", Title: "fleet report", Subtitle: "Fleet operations", Command: "fathom fleet report --json", Href: "/fleet"},
			{Group: console.GroupVerbs, Kind: "verb", Title: "verify", Subtitle: "Verify plan files against the index", Command: "fathom verify"},
		},
		Symbols: fixtureSymbols([]console.SearchDoc{
			{Kind: "symbol", Title: "Charge", Service: "payment-service", Ref: "payment.go:12", Href: "/projects/payment-service"},
			{Kind: "symbol", Title: "validateCharge", Service: "payment-service", Ref: "payment.go:40", Href: "/projects/payment-service"},
			{Kind: "symbol", Title: "ChargeResult", Service: "payment-service", Ref: "payment.go:5", Href: "/projects/payment-service"},
		}),
		Notes: []string{"plans of 1 service could not be read"},
	}
}

// TestSearch_GroupsAndRanksResults pins OC-11: results group in the fixed
// order (projects, plans and rows, symbols, rules and records, verbs,
// pages), rank exact over prefix over word-start over substring over
// subtitle, cap per group while reporting the total, stay deterministic,
// and surface a symbol-lookup failure as a named note, never an empty group
// that reads as "no matches".
func TestSearch_GroupsAndRanksResults(t *testing.T) {
	ix := searchFixture()

	// Ranking inside the symbols group: exact beats prefix beats substring.
	res := console.Search(ix, "charge", 8)
	var syms []string
	for _, g := range res.Groups {
		if g.Group == console.GroupSymbols {
			for _, r := range g.Results {
				syms = append(syms, r.Title)
			}
		}
	}
	if want := []string{"Charge", "ChargeResult", "validateCharge"}; !reflect.DeepEqual(syms, want) {
		t.Errorf("symbol ranking = %v, want %v", syms, want)
	}
	if len(res.Notes) != 1 {
		t.Errorf("index notes must travel with the results: %v", res.Notes)
	}

	// Groups order by their best match — an exact page hit is never below
	// a group of weaker ones — and a query spanning groups lists each once.
	res = console.Search(ix, "fleet", 8)
	var order []console.SearchGroup
	for _, g := range res.Groups {
		order = append(order, g.Group)
	}
	if want := []console.SearchGroup{console.GroupPages, console.GroupVerbs}; !reflect.DeepEqual(order, want) {
		t.Errorf("fleet groups = %v, want %v (the exact page first)", order, want)
	}
	// Ties on the best score go to the smaller group, then the fixed order.
	res = console.Search(ix, "o", 8) // matches nearly everything
	for i := 1; i < len(res.Groups); i++ {
		a, b := res.Groups[i-1], res.Groups[i]
		if a.Results[0].Score < b.Results[0].Score {
			t.Fatalf("group %s (best %d) above %s (best %d)", a.Group, a.Results[0].Score, b.Group, b.Results[0].Score)
		}
		if a.Results[0].Score == b.Results[0].Score {
			if a.Total > b.Total || (a.Total == b.Total && groupIndex(a.Group) > groupIndex(b.Group)) {
				t.Fatalf("tied groups out of order (smaller first, then fixed): %v", res.Groups)
			}
		}
	}

	// Multi-token: every token must match; the phrase in order ranks first.
	res = console.Search(ix, "observatory console", 8)
	if len(res.Groups) == 0 || res.Groups[0].Group != console.GroupPlans || res.Groups[0].Results[0].Title != "observatory-console" {
		t.Errorf("plan search = %+v", res.Groups)
	}
	for _, g := range res.Groups {
		for _, r := range g.Results {
			lower := strings.ToLower(r.Title + " " + r.Subtitle + " " + r.Ref + " " + r.Command)
			if !strings.Contains(lower, "console") {
				t.Errorf("%q matched without every token", r.Title)
			}
		}
	}

	// Records are searchable by ID; projects by name prefix.
	if res := console.Search(ix, "ADR-045", 8); len(res.Groups) != 1 || res.Groups[0].Group != console.GroupRecords {
		t.Errorf("ADR search = %+v", res.Groups)
	}
	if res := console.Search(ix, "pay", 8); len(res.Groups) == 0 || res.Groups[0].Group != console.GroupProjects || res.Groups[0].Results[0].Href != "/projects/payment-service" {
		t.Errorf("project search = %+v", res.Groups)
	}

	// The per-group cap keeps the total honest.
	capped := console.Search(ix, "e", 1)
	for _, g := range capped.Groups {
		if len(g.Results) > 1 {
			t.Errorf("group %s exceeded the cap: %d", g.Group, len(g.Results))
		}
		if g.Total < len(g.Results) {
			t.Errorf("group %s total %d < shown %d", g.Group, g.Total, len(g.Results))
		}
	}

	// Deterministic: the same query twice is identical.
	if a, b := console.Search(ix, "e", 3), console.Search(ix, "e", 3); !reflect.DeepEqual(a, b) {
		t.Error("search is not deterministic")
	}

	// Empty query: no groups, never an error.
	if res := console.Search(ix, "   ", 8); len(res.Groups) != 0 || res.Total != 0 {
		t.Errorf("blank query = %+v", res)
	}

	// A failing symbol source is a named note in its group.
	broken := ix
	broken.Symbols = func([]string, int) ([]console.SearchResult, int, error) {
		return nil, 0, errors.New("store unreadable")
	}
	res = console.Search(broken, "charge", 8)
	found := false
	for _, g := range res.Groups {
		if g.Group == console.GroupSymbols {
			found = strings.Contains(g.Note, "store unreadable")
		}
	}
	if !found {
		t.Errorf("symbol failure must be named: %+v", res.Groups)
	}
}

func groupIndex(g console.SearchGroup) int {
	for i, want := range console.SearchGroupOrder {
		if want == g {
			return i
		}
	}
	return -1
}

// Exact hits are never buried or unreachable (the survey: /search?q=OC-1
// listed OC-10..OC-19 and never OC-1; "fleet" put the Fleet page ~3,000 px
// down under 1,314 loosely matching plan rows). A row's ID is matched
// exactly; the in-order fuzzy match only counts inside a compact span; a
// capped group pages through every match; and a query is bounded.
func TestSearch_ExactHitsFirstAndEveryMatchReachable(t *testing.T) {
	var docs []console.SearchDoc
	docs = append(docs, console.SearchDoc{Group: console.GroupPlans, Kind: "row", ID: "OC-1", Title: "OC-1 Page projections carry a truth state per value (present, zero, absent, stale, partial, error) with reason, source, denominator and time"})
	for i := 10; i <= 19; i++ {
		id := fmt.Sprintf("OC-%d", i)
		docs = append(docs, console.SearchDoc{Group: console.GroupPlans, Kind: "row", ID: id, Title: id + " short"})
	}
	// Loose letter-order matches across a long title are not matches.
	docs = append(docs,
		console.SearchDoc{Group: console.GroupPlans, Kind: "row", ID: "RC-55", Title: "RC-55 Find local imports in each file, then export the results table"},
		console.SearchDoc{Group: console.GroupPages, Kind: "page", Title: "Fleet", Href: "/fleet"},
		console.SearchDoc{Group: console.GroupPlans, Kind: "row", ID: "FR-1", Title: "FR-1 fleet report names its call log"},
	)
	ix := console.SearchIndex{Docs: docs}
	res := console.Search(ix, "OC-1", 8)
	if len(res.Groups) == 0 || res.Groups[0].Results[0].ID != "OC-1" {
		t.Fatalf("OC-1 must rank first in its group: %+v", res.Groups)
	}
	res = console.Search(ix, "fleet", 8)
	if res.Groups[0].Group != console.GroupPages || res.Groups[0].Results[0].Title != "Fleet" {
		t.Errorf("the exact Fleet page must lead: %+v", res.Groups)
	}
	// An exact tie: many symbols named exactly "fleet" do not bury the one
	// exact page — the smaller group leads.
	crowd := make([]console.SearchDoc, 0, 30)
	for i := 0; i < 30; i++ {
		crowd = append(crowd, console.SearchDoc{Kind: "symbol", Title: "fleet", Service: fmt.Sprintf("svc%02d", i), Ref: "x.go:1"})
	}
	ix.Symbols = fixtureSymbols(crowd)
	if res := console.Search(ix, "fleet", 8); res.Groups[0].Group != console.GroupPages {
		t.Errorf("tied exact hits: the lone page must lead the crowd: %v", res.Groups[0].Group)
	}
	ix.Symbols = nil
	for _, g := range res.Groups {
		for _, r := range g.Results {
			if r.ID == "RC-55" {
				t.Errorf("a scattered letter-order match is not a fuzzy hit: %q", r.Title)
			}
		}
	}
	if res := console.Search(ix, "flet", 8); res.Total == 0 {
		t.Error("a compact fuzzy match (a typo) must still match")
	}

	// A capped group pages: offset 8 of the 11 OC rows reaches the rest.
	page2 := console.SearchWith(ix, "OC-1", console.SearchOptions{Group: console.GroupPlans, PerGroup: 8, Offset: 8})
	if len(page2.Groups) != 1 || page2.Groups[0].Total != 11 || len(page2.Groups[0].Results) != 3 {
		t.Fatalf("page 2 = %+v, want the last 3 of 11 in the plans group only", page2.Groups)
	}

	// Symbols: the lookup's total is the group total, never the candidate cap.
	many := make([]console.SearchDoc, 0, 300)
	for i := 0; i < 300; i++ {
		many = append(many, console.SearchDoc{Kind: "symbol", Title: fmt.Sprintf("TestThing%03d", i), Service: "svc", Ref: "x.go:1"})
	}
	res = console.Search(console.SearchIndex{Symbols: fixtureSymbols(many)}, "test", 8)
	if len(res.Groups) != 1 || res.Groups[0].Total != 300 || len(res.Groups[0].Results) != 8 {
		t.Errorf("symbols = %+v, want 8 shown of an exact 300", res.Groups)
	}

	// Bounded queries.
	if err := console.CheckQuery(strings.Repeat("e ", 9)); err == nil {
		t.Error("nine tokens must be refused")
	}
	if err := console.CheckQuery(strings.Repeat("x", console.MaxQueryRunes+1)); err == nil {
		t.Error("an over-long query must be refused")
	}
	if err := console.CheckQuery("resolveCallLog test"); err != nil {
		t.Errorf("an ordinary query refused: %v", err)
	}
}
