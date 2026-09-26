package catalog

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestLayeredGraphStaysExperimentalL3a keeps FATHOM-KIT-02's discovery record
// honest: an L3a renderer whose originating consumer has selected, not
// integrated, it. Consumers lists observed integrations, so it stays empty
// until Fathom integrates; the gaps carry the selection. A later edit that
// promotes it, claims adoption or drops a gap fails here.
func TestLayeredGraphStaysExperimentalL3a(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := c.Get("analytics.graph")
	if !ok {
		t.Fatal("analytics.graph missing from discovery")
	}
	if e.Kind != "primitive" || e.Maturity != "experimental" || e.Preview != "/graph" ||
		!slices.Equal(e.Layers, []string{"L3a"}) || len(e.Consumers) != 0 {
		t.Fatalf("layered graph overstates tier, maturity or adoption: %+v", e)
	}
	gaps := strings.Join(e.Gaps, " ")
	for _, phrase := range []string{"Fathom selected it", "has not integrated it", "second consumer", "No screen-reader session", "pass behind intermediate nodes", "no render-time or memory budget"} {
		if !strings.Contains(gaps, phrase) {
			t.Errorf("gap disclosure omits %q", phrase)
		}
	}
	for _, phrase := range []string{"ErrInvalidGraph", "GraphLayout.Table", "GraphLayout.Notes", "class-free table", "44px"} {
		if !strings.Contains(e.Usage, phrase) {
			t.Errorf("usage omits %q", phrase)
		}
	}
}

// TestLayeredGraphFoundByConsumerWording: task search needs every query word
// in one declared phrase, so the phrases must cover the originating
// consumer's own words — US "neighborhood", "code neighborhood", "DAG" — not
// only the kit's British spelling.
func TestLayeredGraphFoundByConsumerWording(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"code neighborhood", "code neighbourhood", "call graph neighborhood", "call graph neighbourhood",
		"neighborhood", "DAG", "layered dag", "call graph", "callers and callees", "dependency graph",
	} {
		page, err := c.Search(Query{Text: q})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(page.Items, func(s Summary) bool { return s.ID == "analytics.graph" }) {
			t.Errorf("Search(%q) misses analytics.graph (total %d)", q, page.Total)
		}
	}
}

// TestScopeStatesCatalogSize keeps the compatibility-surface document's
// catalog count equal to the shipped catalog.
func TestScopeStatesCatalogSize(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	scope, err := os.ReadFile("../../docs/intent/scope.md")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\| Shared catalog \| (\d+) CUE-validated metadata records`).FindSubmatch(scope)
	if m == nil {
		t.Fatal("docs/intent/scope.md no longer states the shared catalog size")
	}
	if n, _ := strconv.Atoi(string(m[1])); n != len(c.Entries) {
		t.Errorf("docs/intent/scope.md states %d catalog records; the catalog has %d", n, len(c.Entries))
	}
}
