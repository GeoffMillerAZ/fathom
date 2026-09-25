package catalog

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func fixtureEntry(id string) Entry {
	return Entry{
		ID: id, Name: "Capability", Kind: "primitive", Summary: "Display a chart with semantic colors.",
		Tasks: []string{"Compare measurements", "bar chart"}, Layers: []string{"L3a"}, Profiles: []string{"go-web"},
		Dependencies: []string{}, Source: Source{Path: "appkit/ui/viz/line.go", Symbols: []string{"Line"}},
		Usage: "Render the chart in a Go template.", States: []string{"empty"}, AppOwns: []string{"data"},
		Maturity: "available", Proof: []string{"go test ./appkit/ui/viz"},
		Gaps:       []string{"No desktop lifecycle support."},
		Provenance: Provenance{Source: "first-party", License: "repository terms", Audience: "repository consumers"},
		Consumers:  []string{}, Preview: "/viz",
	}
}

func fixtureSource(t *testing.T, entries ...Entry) []byte {
	t.Helper()
	if entries == nil {
		entries = []Entry{}
	}
	b, err := json.Marshal(struct {
		Version int     `json:"schema_version"`
		Entries []Entry `json:"entries"`
	}{1, entries})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixtureCatalog(t *testing.T, entries ...Entry) *Catalog {
	t.Helper()
	c, err := Load(fixtureSource(t, entries...))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDefaultCatalog(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if c.SchemaVersion != 1 || len(c.Entries) == 0 || len(c.Revision) != 64 {
		t.Fatalf("invalid default catalog: version=%d entries=%d revision=%q", c.SchemaVersion, len(c.Entries), c.Revision)
	}
	for _, e := range c.Entries {
		if e.Provenance.Source != "first-party" {
			t.Errorf("embedded entry %q is not first-party: %q", e.ID, e.Provenance.Source)
		}
		if e.Maturity == "available" && len(e.Gaps) == 0 {
			t.Errorf("available entry %q must state its remaining evidence gaps", e.ID)
		}
	}
}

func TestLoadRejectsInvalidSource(t *testing.T) {
	good := string(fixtureSource(t, fixtureEntry("viz.chart")))
	cases := []struct{ name, source string }{
		{"syntax", "entries: ["},
		{"schema version", strings.Replace(good, `"schema_version":1`, `"schema_version":2`, 1)},
		{"missing field", strings.Replace(good, `"usage":"Render the chart in a Go template.",`, "", 1)},
		{"missing gap disclosure", strings.Replace(good, `"gaps":["No desktop lifecycle support."],`, "", 1)},
		{"missing empty dependencies", strings.Replace(good, `"dependencies":[],`, "", 1)},
		{"missing fixed schema version", strings.Replace(good, `"schema_version":1,`, "", 1)},
		{"missing entries", `{"schema_version":1}`},
		{"wrong type", strings.Replace(good, `"layers":["L3a"]`, `"layers":"L3a"`, 1)},
		{"empty layers", strings.Replace(good, `"layers":["L3a"]`, `"layers":[]`, 1)},
		{"unknown layer", strings.Replace(good, `"L3a"`, `"L9"`, 1)},
		{"unknown profile", strings.Replace(good, `"go-web"`, `"desktop"`, 1)},
		{"unknown kind", strings.Replace(good, `"primitive"`, `"framework"`, 1)},
		{"unknown maturity", strings.Replace(good, `"available"`, `"complete"`, 1)},
		{"empty name", strings.Replace(good, `"name":"Capability"`, `"name":" "`, 1)},
		{"invalid ID", strings.Replace(good, `"viz.chart"`, `"Viz Chart"`, 1)},
		{"unknown source body", strings.Replace(good, `"source":{"path":`, `"source":{"body":"licensed source","path":`, 1)},
		{"unknown top level", strings.Replace(good, `"schema_version":1`, `"secret":"value","schema_version":1`, 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load([]byte(tc.source)); err == nil {
				t.Fatalf("accepted invalid source: %s", tc.source)
			}
		})
	}
	t.Run("duplicate IDs", func(t *testing.T) {
		_, err := Load(fixtureSource(t, fixtureEntry("viz.chart"), fixtureEntry("viz.chart")))
		if err == nil || !strings.Contains(err.Error(), "duplicate entry ID") {
			t.Fatalf("duplicate IDs: %v", err)
		}
	})
	t.Run("terminal without web layer or preview", func(t *testing.T) {
		e := fixtureEntry("cli.command")
		e.Layers, e.Profiles, e.Preview = []string{}, []string{"cli"}, ""
		fixtureCatalog(t, e)
	})
}

func TestRevisionTracksContractNotSourceFormattingOrEntryOrder(t *testing.T) {
	a, b := fixtureEntry("viz.a"), fixtureEntry("viz.b")
	c := fixtureCatalog(t, a, b)
	ordered := fixtureCatalog(t, b, a)
	if c.Revision != ordered.Revision {
		t.Fatal("entry ordering changed canonical revision")
	}
	src := fixtureSource(t, a, b)
	formatted, err := Load(append([]byte("// unchanged metadata\n"), src...))
	if err != nil || formatted.Revision != c.Revision {
		t.Fatalf("formatting changed revision: %v", err)
	}
	b.Gaps = append(b.Gaps, "Keyboard behavior remains unverified.")
	changed := fixtureCatalog(t, a, b)
	if c.Revision == changed.Revision {
		t.Fatal("evidence gap update did not change revision")
	}
}

func TestGetAndSearchAreDetached(t *testing.T) {
	c := fixtureCatalog(t, fixtureEntry("viz.chart"))
	want, ok := c.Get("viz.chart")
	if !ok {
		t.Fatal("missing entry")
	}
	e, _ := c.Get(want.ID)
	e.Tasks[0], e.Layers[0], e.Profiles[0] = "mutated", "L1", "cli"
	e.Source.Symbols[0], e.States[0], e.AppOwns[0] = "mutated", "mutated", "mutated"
	e.Proof[0], e.Gaps[0] = "mutated", "mutated"
	c.Entries[0].Name, c.Entries[0].Source.Symbols[0], c.Revision = "mutated", "mutated", "mutated"
	page, err := c.Search(Query{})
	if err != nil {
		t.Fatal(err)
	}
	page.Items[0].Layers[0], page.Items[0].Profiles[0] = "L0", "portable"
	again, _ := c.Get(want.ID)
	if !reflect.DeepEqual(want, again) {
		t.Fatalf("returned data mutated snapshot: %+v", again)
	}
	second, err := c.Search(Query{})
	if err != nil || second.Revision == "mutated" || second.Items[0].Layers[0] != "L3a" {
		t.Fatalf("page mutation poisoned search: %+v %v", second, err)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("unknown ID found")
	}
}

func TestSearchFitRankingAndMissingCapability(t *testing.T) {
	primitive, reference, recipe := fixtureEntry("viz.a"), fixtureEntry("reference.a"), fixtureEntry("recipe.a")
	reference.Kind = "reference"
	recipe.Kind, recipe.Maturity = "recipe", "proven"
	react := fixtureEntry("react.chart")
	react.Profiles = []string{"react-web"}
	c := fixtureCatalog(t, primitive, reference, recipe, react)
	page, err := c.Search(Query{Text: "CHART", Profile: "go-web"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}
	if !slices.Equal(ids, []string{"recipe.a", "viz.a", "reference.a"}) {
		t.Fatalf("ranking/fit: %v", ids)
	}
	for _, q := range []Query{
		{Text: "desktop lifecycle"},
		{Text: "chart unavailable"},
		{Profile: "cli", Layer: "L3a"},
		{Kind: "token"},
	} {
		p, err := c.Search(q)
		if err != nil || p.Total != 0 || p.Items == nil || p.NextCursor != "" {
			t.Errorf("missing/incompatible query %+v: %+v %v", q, p, err)
		}
	}
	p, err := c.Search(Query{Layer: "L3a", Profile: "go-web", Kind: "recipe", Maturity: "proven"})
	if err != nil || p.Total != 1 || p.Items[0].ID != recipe.ID {
		t.Fatalf("combined exact filters: %+v %v", p, err)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	for _, fullField := range []string{`"source"`, `"usage"`, `"proof"`, `"gaps"`, `"dependencies"`} {
		if strings.Contains(string(encoded), fullField) {
			t.Errorf("search exposed full-contract field %s", fullField)
		}
	}
}

func TestSearchDeterministicTokenScore(t *testing.T) {
	a, b, c := fixtureEntry("viz.a"), fixtureEntry("viz.b"), fixtureEntry("viz.c")
	c.Name = "Chart"
	catalog := fixtureCatalog(t, b, c, a)
	p, err := catalog.Search(Query{Text: "chart"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].ID != c.ID || p.Items[1].ID != a.ID || p.Items[2].ID != b.ID {
		t.Fatalf("name match or ID tie order wrong: %+v", p.Items)
	}
}

func TestSearchRequiresCoherentPositiveTask(t *testing.T) {
	e := fixtureEntry("form.email")
	e.Name = "Email field adapter"
	e.Tasks = []string{"email field", "licensed design delivery"}
	e.Summary = "No email delivery service or target certification."
	e.Usage = "This is not an account backend; never apply an automatic source patch."
	c := fixtureCatalog(t, e)
	for _, text := range []string{"email delivery", "target certification", "account backend", "automatic source patch", "desktop lifecycle"} {
		p, err := c.Search(Query{Text: text})
		if err != nil || p.Total != 0 {
			t.Errorf("invented task %q: %+v %v", text, p, err)
		}
	}
	for _, text := range []string{"email field", "licensed design delivery", "FIELD email email", "email fie"} {
		p, err := c.Search(Query{Text: text})
		if err != nil || p.Total != 1 || p.Items[0].ID != e.ID {
			t.Errorf("lost supported task %q: %+v %v", text, p, err)
		}
	}
}

func TestSearchRejectsPriorMatcherCursor(t *testing.T) {
	c := fixtureCatalog(t, fixtureEntry("viz.a"), fixtureEntry("viz.b"))
	q := Query{Text: "chart", Limit: 1}
	p, err := c.Search(q)
	if err != nil {
		t.Fatal(err)
	}
	b, err := base64.RawURLEncoding.DecodeString(p.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	var cur continuation
	if err := json.Unmarshal(b, &cur); err != nil {
		t.Fatal(err)
	}
	cur.Version = 1 // Otherwise valid revision, query, page size and offset.
	b, err = json.Marshal(cur)
	if err != nil {
		t.Fatal(err)
	}
	q.Cursor = base64.RawURLEncoding.EncodeToString(b)
	if _, err := c.Search(q); err == nil || !strings.Contains(err.Error(), "restart search") {
		t.Fatalf("old matcher cursor accepted: %v", err)
	}
}

func TestSearchRejectsUnknownFiltersAndLimits(t *testing.T) {
	c := fixtureCatalog(t)
	for _, q := range []Query{{Layer: "L9"}, {Profile: "desktop"}, {Kind: "component"}, {Maturity: "production"}, {Limit: -1}, {Limit: 51}} {
		if _, err := c.Search(q); err == nil {
			t.Errorf("accepted invalid query %+v", q)
		}
	}
	for _, q := range []Query{{}, {Limit: 1}, {Limit: 50}} {
		if _, err := c.Search(q); err != nil {
			t.Errorf("rejected valid limit %+v: %v", q, err)
		}
	}
}

func TestSearchPaginationAndCursors(t *testing.T) {
	entries := make([]Entry, 23)
	for i := range entries {
		entries[i] = fixtureEntry(fmt.Sprintf("viz.item%02d", i))
	}
	c := fixtureCatalog(t, entries...)
	p, err := c.Search(Query{})
	if err != nil || len(p.Items) != 10 || p.Total != 23 || p.NextCursor == "" {
		t.Fatalf("default page: %+v %v", p, err)
	}
	seen := map[string]bool{}
	q := Query{Text: "chart", Limit: 3}
	for {
		p, err := c.Search(q)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range p.Items {
			if seen[item.ID] {
				t.Fatalf("duplicate on continuation: %s", item.ID)
			}
			seen[item.ID] = true
		}
		if p.NextCursor == "" {
			break
		}
		q.Cursor = p.NextCursor
		q.Text = " CHART chart " // Equivalent normalized text keeps its cursor.
	}
	if len(seen) != len(entries) {
		t.Fatalf("pagination returned %d entries, want %d", len(seen), len(entries))
	}
	first, err := c.Search(Query{Text: "chart", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []Query{
		{Text: "measurements", Limit: 3, Cursor: first.NextCursor},
		{Text: "chart", Limit: 4, Cursor: first.NextCursor},
		{Text: "chart", Limit: 3, Profile: "go-web", Cursor: first.NextCursor},
		{Cursor: "%%%"},
		{Cursor: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1}`))},
		{Cursor: strings.Repeat("a", 2049)},
	} {
		if _, err := c.Search(q); err == nil {
			t.Errorf("accepted incompatible/malformed cursor %+v", q)
		}
	}
	entries[0].Summary = "Changed contract."
	changed := fixtureCatalog(t, entries...)
	if _, err := changed.Search(Query{Text: "chart", Limit: 3, Cursor: first.NextCursor}); err == nil || !strings.Contains(err.Error(), "stale cursor") {
		t.Fatalf("stale cursor: %v", err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	var cur continuation
	if err := json.Unmarshal(decoded, &cur); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{-1, 0, 1, 23, 999} {
		cur.Offset = offset
		encoded, err := json.Marshal(cur)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Search(Query{Text: "chart", Limit: 3, Cursor: base64.RawURLEncoding.EncodeToString(encoded)}); err == nil || !strings.Contains(err.Error(), "offset out of range") {
			t.Errorf("offset %d: %v", offset, err)
		}
	}
}
