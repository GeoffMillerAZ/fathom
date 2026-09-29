package explmap

import (
	"reflect"
	"testing"
	"time"
)

// TestExpandMapDedupNearestWinsFreshnessOnce pins XM-4: resolution is a union
// deduplicated by anchor; conflicting annotations are both kept with their
// provenance, the nearest include first; and freshness is checked once per
// entry however many maps include it.
func TestExpandMapDedupNearestWinsFreshnessOnce(t *testing.T) {
	shared := ent("x.go", 3, 5, "h1", "the entry point")
	other := ent("y.go", 1, 2, "h2", "the write path")

	near := shared
	near.Why = "where auth starts"
	near.AgentID = "agent-a1"

	same := shared // P2 files the same annotation base carries

	maps := Maps{
		"root": eph("root", "p1", "p2", "gone"),
		"p1":   withEntries(eph("p1", "base"), near),
		"p2":   withEntries(eph("p2", "base"), same),
		"base": withEntries(eph("base"), shared, other),
	}
	res, err := ResolveMap("root", maps)
	if err != nil {
		t.Fatalf("ResolveMap: %v", err)
	}

	checks := map[string]int{}
	check := func(e Entry) Freshness {
		checks[e.Key()]++
		if e.Key() == other.Key() {
			return Freshness{State: Stale, Anchor: e.SpanRef(), Added: 1, DiffKnown: true, Note: "changed"}
		}
		return Freshness{State: Fresh, Anchor: e.SpanRef(), DiffKnown: true}
	}
	x := expandMap(res, maps, check)

	if len(x.Items) != 2 {
		t.Fatalf("items = %d, want 2 (the shared entry once, the other once)", len(x.Items))
	}
	for k, n := range checks {
		if n != 1 {
			t.Errorf("freshness of %s checked %d times, want once", k, n)
		}
	}
	if len(checks) != 2 {
		t.Errorf("freshness checked for %d keys, want 2", len(checks))
	}

	first := x.Items[0]
	if first.Key != shared.Key() {
		t.Fatalf("first item = %s, want the shared entry (nearest first)", first.Key)
	}
	if first.Entry.Why != "where auth starts" || first.Entry.AgentID != "agent-a1" {
		t.Errorf("the winning entry = %q by %q, want the nearest include's (p1, agent-a1)", first.Entry.Why, first.Entry.AgentID)
	}
	wantAnn := []Annotation{
		{Why: "where auth starts", MapID: "p1", AgentID: "agent-a1", Depth: 1},
		{Why: "the entry point", MapID: "p2", Depth: 1, Also: []string{"base"}},
	}
	if !reflect.DeepEqual(first.Annotations, wantAnn) {
		t.Errorf("annotations = %+v\nwant %+v", first.Annotations, wantAnn)
	}
	if !reflect.DeepEqual(first.Maps, []string{"p1", "p2", "base"}) {
		t.Errorf("maps carrying it = %v, want [p1 p2 base]", first.Maps)
	}
	if !first.Conflict() {
		t.Errorf("two different annotations must read as a conflict")
	}

	second := x.Items[1]
	if second.Key != other.Key() || second.Fresh.State != Stale || second.Conflict() {
		t.Errorf("second = %+v, want the other entry, stale, no conflict", second)
	}

	if x.Maps != 4 || x.Conflicts != 1 || x.Fresh != 1 || x.Stale != 1 || x.ReResolved != 0 {
		t.Errorf("counts = maps %d conflicts %d fresh %d stale %d re-resolved %d; want 4 1 1 1 0",
			x.Maps, x.Conflicts, x.Fresh, x.Stale, x.ReResolved)
	}
	if !reflect.DeepEqual(x.Missing, []string{"gone"}) {
		t.Errorf("missing = %v, want [gone]", x.Missing)
	}

	// The root's own entry is nearest of all.
	own := shared
	own.Why = "root says so"
	maps["root"] = withEntries(maps["root"], own)
	res, _ = ResolveMap("root", maps)
	x = expandMap(res, maps, check)
	if x.Items[0].Entry.Why != "root says so" || x.Items[0].Annotations[0].Depth != 0 {
		t.Errorf("the root's own annotation must lead: %+v", x.Items[0].Annotations)
	}

	// Expand is ResolveMap plus expandMap over a Source.
	src := &fakeSource{files: map[string]string{}, filed: map[string]string{}}
	full, err := Expand("root", maps, src)
	if err != nil || len(full.Items) != 2 || full.Stale != 2 {
		t.Errorf("Expand = %d items, %d stale, %v; want 2 items both stale (no files)", len(full.Items), full.Stale, err)
	}
}

// TestEntryKeyIgnoresTheServicePrefixOfASymbol: `svc::pkg.Name` and `pkg.Name`
// spell one symbol, so entries filed under either dedupe as one (XM-33, O147: a
// wave map carried the same symbol twice for want of it).
func TestEntryKeyIgnoresTheServicePrefixOfASymbol(t *testing.T) {
	a := Entry{Symbol: "internal/fusion.ResolveImportSpecs", Hash: "h1", Path: "internal/fusion/x.go", Start: 55, End: 97}
	b := a
	b.Symbol = "fathom::internal/fusion.ResolveImportSpecs"
	if a.Key() != b.Key() {
		t.Errorf("keys differ by the service prefix: %q vs %q", a.Key(), b.Key())
	}
	c := a
	c.Hash = "h2"
	if a.Key() == c.Key() {
		t.Errorf("the same symbol in another content state is another entry, keys equal: %q", a.Key())
	}
}

// TestExpandMapCarriesEntryNotesAsAnnotations pins XM-33 in the model: a later
// why filed on an entry already on the map is a Note on that entry, and an
// expansion serves it as another annotation with its provenance, after the
// entry's own; a note that repeats a why already served adds nothing.
func TestExpandMapCarriesEntryNotesAsAnnotations(t *testing.T) {
	e := ent("x.go", 3, 5, "h1", "where auth starts")
	e.Notes = []Note{
		{Why: "swallows the error", AgentID: "agent-7", FiledAt: t0},
		{Why: "where auth starts", AgentID: "agent-8", FiledAt: t0},
	}
	maps := Maps{"m": withEntries(eph("m"), e)}
	x, err := Expand("m", maps, &fakeSource{files: map[string]string{}, filed: map[string]string{}})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(x.Items) != 1 {
		t.Fatalf("items = %d, want 1: notes are not entries", len(x.Items))
	}
	want := []Annotation{
		{Why: "where auth starts", MapID: "m", Depth: 0},
		{Why: "swallows the error", MapID: "m", AgentID: "agent-7", Depth: 0},
	}
	if got := x.Items[0].Annotations; !reflect.DeepEqual(got, want) {
		t.Errorf("annotations = %+v\nwant %+v", got, want)
	}
	if x.Conflicts != 1 {
		t.Errorf("conflicts = %d, want 1 (two different whys)", x.Conflicts)
	}
}

// TestMapEntryLookupByKeyAndWhy: the append's dedupe reads the map's own
// entries by key, and asks whether a why is already carried by the entry or one
// of its notes.
func TestMapEntryLookupByKeyAndWhy(t *testing.T) {
	e := ent("x.go", 3, 5, "h1", "the handler")
	e.ID = "th-1"
	e.Notes = []Note{{Why: "swallows the error", AgentID: "agent-7", FiledAt: time.Time{}}}
	m := withEntries(eph("m"), e, ent("y.go", 1, 2, "h2", "other"))

	got, ok := m.OwnEntry(e.Key())
	if !ok || got.ID != "th-1" {
		t.Fatalf("OwnEntry(%q) = %+v, %v; want th-1", e.Key(), got, ok)
	}
	if _, ok := m.OwnEntry("span:z.go#L1-L1@h9"); ok {
		t.Errorf("OwnEntry found a key nothing carries")
	}
	if !got.CarriesWhy("the handler") || !got.CarriesWhy("  swallows the error ") || got.CarriesWhy("something new") {
		t.Errorf("CarriesWhy over the entry's why and its notes is wrong: %+v", got)
	}
}
