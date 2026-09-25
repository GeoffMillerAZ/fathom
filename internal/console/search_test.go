package console_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

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
		Symbols: func(q string, limit int) ([]console.SearchDoc, error) {
			all := []console.SearchDoc{
				{Kind: "symbol", Title: "Charge", Service: "payment-service", Ref: "payment.go:12", Href: "/projects/payment-service"},
				{Kind: "symbol", Title: "validateCharge", Service: "payment-service", Ref: "payment.go:40", Href: "/projects/payment-service"},
				{Kind: "symbol", Title: "ChargeResult", Service: "payment-service", Ref: "payment.go:5", Href: "/projects/payment-service"},
			}
			var out []console.SearchDoc
			for _, d := range all {
				if strings.Contains(strings.ToLower(d.Title), strings.ToLower(q)) {
					out = append(out, d)
				}
			}
			return out, nil
		},
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

	// Group order is fixed, and a query spanning groups lists each once.
	res = console.Search(ix, "fleet", 8)
	var order []console.SearchGroup
	for _, g := range res.Groups {
		order = append(order, g.Group)
	}
	if want := []console.SearchGroup{console.GroupVerbs, console.GroupPages}; !reflect.DeepEqual(order, want) {
		t.Errorf("fleet groups = %v, want %v", order, want)
	}
	res = console.Search(ix, "o", 8) // matches nearly everything
	last := -1
	for _, g := range res.Groups {
		idx := -1
		for i, want := range console.SearchGroupOrder {
			if want == g.Group {
				idx = i
			}
		}
		if idx <= last {
			t.Fatalf("groups out of the fixed order: %v", res.Groups)
		}
		last = idx
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
	broken.Symbols = func(string, int) ([]console.SearchDoc, error) { return nil, errors.New("store unreadable") }
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
