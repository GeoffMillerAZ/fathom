package explmap

import (
	"reflect"
	"testing"
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
