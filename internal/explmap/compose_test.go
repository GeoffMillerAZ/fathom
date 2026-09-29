package explmap

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

func withEntries(m Map, es ...Entry) Map {
	m.Entries = es
	return m
}

func ent(path string, start, end int, hash, why string) Entry {
	return Entry{Anchor: path + "#L" + strconv.Itoa(start) + "-L" + strconv.Itoa(end), Path: path, Start: start, End: end, Hash: hash, Why: why}
}

// TestMintCompositeMapCarriesOnlyReferences pins XM-3: the handoff carries ONE
// id. Several maps become a minted composite made only of references (no entry
// is copied), minting is deterministic, and a sub-agent can pass its own
// composite down as an include of another composite.
func TestMintCompositeMapCarriesOnlyReferences(t *testing.T) {
	maps := Maps{
		"plan":  withEntries(eph("plan"), ent("a.go", 1, 3, "h1", "entry point")),
		"store": withEntries(eph("store"), ent("b.go", 4, 9, "h2", "the write path")),
	}

	c, err := mintCompositeMap([]string{"plan", "store"}, maps, "parent", t0)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if c.Kind != KindComposite || !strings.HasPrefix(c.ID, CompositePrefix) {
		t.Errorf("composite = kind %q id %q, want kind composite and id prefix %q", c.Kind, c.ID, CompositePrefix)
	}
	if len(c.Entries) != 0 {
		t.Errorf("a composite copied %d entries; it must carry references only", len(c.Entries))
	}
	if !reflect.DeepEqual(c.Includes, []string{"plan", "store"}) {
		t.Errorf("includes = %v, want [plan store]", c.Includes)
	}
	if c.CreatedBy != "parent" || !c.CreatedAt.Equal(t0) {
		t.Errorf("provenance = %q at %v", c.CreatedBy, c.CreatedAt)
	}

	// Deterministic, idempotent, duplicates collapsed; order is priority.
	again, _ := mintCompositeMap([]string{"plan", " plan", "store"}, maps, "someone-else", t0.Add(time.Hour))
	if again.ID != c.ID {
		t.Errorf("same list minted %q then %q; the id must depend on the references only", c.ID, again.ID)
	}
	swapped, _ := mintCompositeMap([]string{"store", "plan"}, maps, "parent", t0)
	if swapped.ID == c.ID {
		t.Errorf("a different priority order minted the same id %q", c.ID)
	}

	// Unknown references are refused, not minted around.
	if _, err := mintCompositeMap([]string{"plan", "gone"}, maps, "p", t0); !errors.Is(err, ErrUnknownMap) {
		t.Errorf("unknown include: err = %v, want ErrUnknownMap", err)
	}
	if _, err := mintCompositeMap(nil, maps, "p", t0); err == nil {
		t.Errorf("an empty composite was minted")
	}

	// A sub-agent passes its own composite down: a composite of a composite.
	maps[c.ID] = c
	maps["mine"] = withEntries(eph("mine"), ent("c.go", 2, 2, "h3", "what I found"))
	down, err := mintCompositeMap([]string{c.ID, "mine"}, maps, "agent-7", t0)
	if err != nil {
		t.Fatalf("composite of a composite: %v", err)
	}
	maps[down.ID] = down
	r, err := ResolveMap(down.ID, maps)
	if err != nil {
		t.Fatalf("resolving the passed-down composite: %v", err)
	}
	if got, want := nodeIDs(r), []string{down.ID, c.ID, "mine", "plan", "store"}; !reflect.DeepEqual(got, want) {
		t.Errorf("passed-down resolution = %v, want %v", got, want)
	}
	if len(down.Entries) != 0 {
		t.Errorf("the passed-down composite carries entries")
	}

	// Composing respects the depth cap: over a base already four levels deep
	// a composite would make five.
	deep := Maps{"d0": eph("d0", "d1"), "d1": eph("d1", "d2"), "d2": eph("d2", "d3"), "d3": eph("d3", "d4"), "d4": eph("d4")}
	if _, err := mintCompositeMap([]string{"d0", "d4"}, deep, "p", t0); !errors.Is(err, ErrTooDeep) {
		t.Errorf("composite over a four-deep base: err = %v, want ErrTooDeep", err)
	}

	// Compose with one id hands that id back: it already is one id.
	id, minted, err := Compose([]string{"plan"}, maps, "p", t0)
	if err != nil || id != "plan" || minted != nil {
		t.Errorf("Compose(one) = %q, %v, %v; want the id itself and nothing minted", id, minted, err)
	}
	id, minted, err = Compose([]string{"plan", "store"}, maps, "p", t0)
	if err != nil || minted == nil || id != c.ID {
		t.Errorf("Compose(two) = %q, %v, %v; want %q minted", id, minted, err, c.ID)
	}

	// A stored composite that somehow carries entries is invalid.
	bad := c
	bad.Entries = []Entry{ent("x.go", 1, 1, "h", "copied")}
	if err := Validate(bad); err == nil {
		t.Errorf("a composite with entries validated")
	}
}
