package explmap

// Annotation is one why line for an entry, with its provenance.
type Annotation struct {
	Why string
	// MapID is the nearest map that carries this annotation; AgentID the agent
	// that appended it there ("" for the map's own session).
	MapID   string
	AgentID string
	// Depth is MapID's depth in the resolution (0 is the root).
	Depth int
	// Also are the farther maps carrying the same annotation, nearest first.
	Also []string
}

// Item is one deduplicated entry of an expansion.
type Item struct {
	// Key is the dedup key: anchor plus content hash (Entry.Key).
	Key string
	// Entry is the nearest map's copy: its anchor, span, hash, why and
	// provenance lead.
	Entry Entry
	// Annotations are the distinct why lines, nearest first. More than one is
	// a conflict, and every one is kept with its provenance.
	Annotations []Annotation
	// Maps are every map carrying the entry, nearest first.
	Maps []string
	// Fresh is the read-time freshness, checked once for the item.
	Fresh Freshness
}

// Conflict reports whether the maps disagree on why the entry matters.
func (it Item) Conflict() bool { return len(it.Annotations) > 1 }

// Expansion is a map expanded: its whole DAG as one deduplicated entry list.
type Expansion struct {
	Root string
	// Items are nearest first: the root's own entries, then each include's in
	// resolution order, each entry at its first (nearest) appearance.
	Items []Item
	// Maps is how many maps the resolution reached; Missing the include ids
	// that resolve to nothing.
	Maps    int
	Missing []string
	// The counts a reader needs before the items.
	Conflicts, Fresh, ReResolved, Stale int
}

// Expand resolves root's DAG and expands it, re-resolving every entry against
// src at read time (XM-4, XM-5): the one call a reader of a map makes.
func Expand(root string, maps Lookup, src Source) (Expansion, error) {
	res, err := ResolveMap(root, maps)
	if err != nil {
		return Expansion{}, err
	}
	return expandMap(res, maps, func(e Entry) Freshness { return resolveEntryFresh(e, src) }), nil
}

// expandMap is resolution as a union (XM-4): the entries of every map in res,
// deduplicated by anchor plus content hash. The nearest map's copy leads; a
// different why line from a farther map is kept beside it as another
// annotation with its provenance (never dropped, never merged into one line);
// the same why line from a farther map only adds that map to the annotation's
// provenance. check runs once per distinct entry however many maps include it,
// so a shared base costs one freshness check per entry, not one per route.
func expandMap(res Resolution, maps Lookup, check func(Entry) Freshness) Expansion {
	x := Expansion{Root: res.Root, Maps: len(res.Nodes), Missing: append([]string(nil), res.Missing...)}
	index := map[string]int{}
	for _, n := range res.Nodes {
		m, ok := maps.Map(n.ID)
		if !ok {
			continue
		}
		for _, e := range m.Entries {
			k := e.Key()
			i, seen := index[k]
			if !seen {
				index[k] = len(x.Items)
				x.Items = append(x.Items, Item{Key: k, Entry: e, Maps: []string{n.ID}})
				i = index[k]
			}
			it := &x.Items[i]
			if !containsString(it.Maps, n.ID) {
				it.Maps = append(it.Maps, n.ID)
			}
			it.addAnnotation(Annotation{Why: e.Why, MapID: n.ID, AgentID: e.AgentID, Depth: n.Depth})
			for _, note := range e.Notes {
				it.addAnnotation(Annotation{Why: note.Why, MapID: n.ID, AgentID: note.AgentID, Depth: n.Depth})
			}
		}
	}
	for i := range x.Items {
		it := &x.Items[i]
		it.Fresh = check(it.Entry)
		switch it.Fresh.State {
		case Fresh:
			x.Fresh++
		case ReResolved:
			x.ReResolved++
		default:
			x.Stale++
		}
		if it.Conflict() {
			x.Conflicts++
		}
	}
	return x
}

// addAnnotation records a why line for the item: the same why line from a
// farther map only adds that map to the annotation's provenance, a different one
// is kept beside it (never dropped, never merged into one line).
func (it *Item) addAnnotation(a Annotation) {
	for i := range it.Annotations {
		ann := &it.Annotations[i]
		if ann.Why == a.Why {
			if ann.MapID != a.MapID && !containsString(ann.Also, a.MapID) {
				ann.Also = append(ann.Also, a.MapID)
			}
			return
		}
	}
	it.Annotations = append(it.Annotations, a)
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
