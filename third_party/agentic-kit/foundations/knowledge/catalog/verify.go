package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// DriftEntry is one node-level finding in a Drift report.
type DriftEntry struct {
	ID    string
	Title string
	// Layer is the node's origin layer (Provenance.ContributingLayers[0])
	// on whichever side (built or published) the entry was found on.
	// Empty if that catalog was assembled directly via New rather than
	// Build.
	Layer string
}

// MovedEntry reports a node whose StableKey is shared by a node in both
// catalogs but whose ID differs between them — a rename, not a
// delete-then-add.
type MovedEntry struct {
	StableKey string
	From      string // ID in published
	To        string // ID in built
	Layer     string // built node's origin layer
}

// Drift is the return of Verify: a value, not a bool (design §4), so a
// clean run still reports what it checked. Each list is sorted for
// determinism (by ID; Moved by To).
type Drift struct {
	Missing []DriftEntry // published claims it, built (source) lacks it
	Extra   []DriftEntry // built has it, published omits it
	Changed []DriftEntry // present in both, Hash differs
	Moved   []MovedEntry // same StableKey, different ID

	CheckedBuilt     int
	CheckedPublished int
}

// Clean reports whether Verify found no drift at all.
func (d Drift) Clean() bool {
	return len(d.Missing) == 0 && len(d.Extra) == 0 && len(d.Changed) == 0 && len(d.Moved) == 0
}

// ForLayer filters d down to entries attributed to layer, so "the base
// catalog drifted" and "this project's overlay drifted" are
// distinguishable findings instead of one aggregate. CheckedBuilt/
// CheckedPublished are carried through unfiltered (they describe the
// whole run, not one layer).
func (d Drift) ForLayer(layer string) Drift {
	out := Drift{CheckedBuilt: d.CheckedBuilt, CheckedPublished: d.CheckedPublished}
	for _, e := range d.Missing {
		if e.Layer == layer {
			out.Missing = append(out.Missing, e)
		}
	}
	for _, e := range d.Extra {
		if e.Layer == layer {
			out.Extra = append(out.Extra, e)
		}
	}
	for _, e := range d.Changed {
		if e.Layer == layer {
			out.Changed = append(out.Changed, e)
		}
	}
	for _, e := range d.Moved {
		if e.Layer == layer {
			out.Moved = append(out.Moved, e)
		}
	}
	return out
}

// originLayer returns n's origin (first-contributing) layer, or "" if n
// carries no layer bookkeeping (i.e. its catalog was built via New, not Build).
func originLayer(n Node) string {
	if len(n.Provenance.ContributingLayers) == 0 {
		return ""
	}
	return n.Provenance.ContributingLayers[0]
}

// DuplicateStableKeyError is returned by Verify when either the built or
// the published catalog contains more than one node sharing a StableKey.
// Verify's Moved detection depends on StableKey uniquely identifying one
// node per catalog; an ambiguous key means any Moved/Missing/Extra
// classification built from it would be a guess — possibly reported
// against the wrong node, possibly missing a genuine rename entirely.
// That is exactly the "catalog that can lie" failure this package exists
// to prevent, so Verify refuses to run rather than silently pick one
// colliding node and drop the rest.
type DuplicateStableKeyError struct {
	Side       string // "built" or "published"
	Duplicates []DuplicateStableKey
}

// Error implements the error interface, naming the offending side
// ("built" or "published") and every colliding StableKey with the node
// IDs that claim it.
func (e *DuplicateStableKeyError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "catalog: refusing to Verify — %s catalog has %d duplicate StableKey group(s): ", e.Side, len(e.Duplicates))
	for i, d := range e.Duplicates {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%q claimed by %s", d.StableKey, strings.Join(d.IDs, ", "))
	}
	return b.String()
}

// Verify compares a freshly built Catalog against a previously published
// one and reports every disagreement (design §4 — the reason this kit
// exists). A node deleted from one ID and re-added under another, sharing
// a StableKey, is reported as exactly one Moved entry — never as a
// Missing plus an Extra.
//
// Verify first checks both catalogs for duplicate StableKeys
// (DuplicateStableKeys) and returns a *DuplicateStableKeyError, naming
// the colliding keys and node IDs, instead of guessing which node a
// shared key belongs to.
func Verify(built, published Catalog) (Drift, error) {
	if dups := DuplicateStableKeys(built); len(dups) > 0 {
		return Drift{}, &DuplicateStableKeyError{Side: "built", Duplicates: dups}
	}
	if dups := DuplicateStableKeys(published); len(dups) > 0 {
		return Drift{}, &DuplicateStableKeyError{Side: "published", Duplicates: dups}
	}

	d := Drift{CheckedBuilt: len(built.Nodes), CheckedPublished: len(published.Nodes)}

	builtByID := make(map[string]Node, len(built.Nodes))
	for _, n := range built.Nodes {
		builtByID[n.ID()] = n
	}
	pubByID := make(map[string]Node, len(published.Nodes))
	for _, n := range published.Nodes {
		pubByID[n.ID()] = n
	}
	// Safe to key by StableKey alone now: both catalogs were just proven
	// collision-free above.
	builtByKey := map[string]Node{}
	for _, n := range built.Nodes {
		if n.StableKey != "" {
			builtByKey[n.StableKey] = n
		}
	}
	pubByKey := map[string]Node{}
	for _, n := range published.Nodes {
		if n.StableKey != "" {
			pubByKey[n.StableKey] = n
		}
	}

	for _, pn := range published.Nodes {
		if _, ok := builtByID[pn.ID()]; ok {
			continue // present in both by ID; handled by the Changed pass below
		}
		if pn.StableKey != "" {
			if bn, ok := builtByKey[pn.StableKey]; ok && bn.ID() != pn.ID() {
				d.Moved = append(d.Moved, MovedEntry{
					StableKey: pn.StableKey,
					From:      pn.ID(),
					To:        bn.ID(),
					Layer:     originLayer(bn),
				})
				continue
			}
		}
		d.Missing = append(d.Missing, DriftEntry{ID: pn.ID(), Title: pn.Title, Layer: originLayer(pn)})
	}

	for _, bn := range built.Nodes {
		pn, ok := pubByID[bn.ID()]
		if !ok {
			if bn.StableKey != "" {
				if p, ok := pubByKey[bn.StableKey]; ok && p.ID() != bn.ID() {
					continue // already recorded as Moved above
				}
			}
			d.Extra = append(d.Extra, DriftEntry{ID: bn.ID(), Title: bn.Title, Layer: originLayer(bn)})
			continue
		}
		if bn.Hash() != pn.Hash() {
			d.Changed = append(d.Changed, DriftEntry{ID: bn.ID(), Title: bn.Title, Layer: originLayer(bn)})
		}
	}

	sortDriftEntries(d.Missing)
	sortDriftEntries(d.Extra)
	sortDriftEntries(d.Changed)
	sort.Slice(d.Moved, func(i, j int) bool { return d.Moved[i].To < d.Moved[j].To })

	return d, nil
}

func sortDriftEntries(es []DriftEntry) {
	sort.Slice(es, func(i, j int) bool { return es[i].ID < es[j].ID })
}
