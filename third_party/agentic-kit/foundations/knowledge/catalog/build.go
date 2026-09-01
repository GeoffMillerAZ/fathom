package catalog

import (
	"fmt"
	"sort"
)

// LayerStableKeyCollisionError is returned by Build when a layer
// introduces a node whose StableKey is already claimed by a different
// node — most reachable when an overlay adds a node whose StableKey
// happens to already exist in the base (or an earlier overlay). Build
// refuses rather than accept it: a duplicate StableKey would silently
// corrupt the StableKey index Verify's Moved detection depends on (see
// DuplicateStableKeyError), so the collision is rejected loudly, at the
// point it is introduced, naming both the existing and incoming node and
// which layer each came from.
type LayerStableKeyCollisionError struct {
	StableKey     string
	ExistingID    string
	ExistingLayer string // the existing node's origin layer
	IncomingID    string
	IncomingLayer string // the layer whose node collides
}

// Error implements the error interface, naming the colliding StableKey,
// both node IDs, and both layer names.
func (e *LayerStableKeyCollisionError) Error() string {
	return fmt.Sprintf(
		"catalog: StableKey %q collision — %q (from layer %q) already claims it; layer %q cannot also claim it for %q",
		e.StableKey, e.ExistingID, e.ExistingLayer, e.IncomingLayer, e.IncomingID,
	)
}

// Layer is one input to Build: a named, ordered contribution to a
// catalog. A typical Build call is a base layer (shipped, embedded, or
// generated) plus zero or more project-local overlays — the same
// embedded-packs ∪ project-local-layers precedent this fleet already
// uses elsewhere. Name must be non-empty and should be stable across
// runs: it is recorded in every touched node's Provenance and in
// Verify's per-layer Drift (Drift.ForLayer).
type Layer struct {
	Name  string
	Nodes []Node
}

// Build merges layers, in order, into one Catalog. Per node ID:
//
//   - ADD: the first layer to mention an ID creates the node. Arbitrary
//     depth applies to overlays exactly as it does to a base catalog: an
//     overlay may introduce ID segments (intermediate groups) the base
//     never had, simply by including those group nodes in its own Nodes.
//   - EXTEND: a later layer mentioning an existing ID has its Facets
//     UNIONED into the node's Facets (per axis, deduplicated, sorted).
//     This never marks the node Overridden — adding a facet value is not
//     a claim that changes what an earlier layer already said.
//   - OVERRIDE: a later layer's non-zero Title/Synopsis/Order/StableKey/
//     Hidden/Provenance(Source/Extractor/Watermark) fields replace the
//     accumulated node's. This is allowed, but Provenance.Overridden is
//     set true and Provenance.OverriddenBy records which layer did it —
//     a silent override is exactly the class of lie the drift gate
//     exists to prevent.
//
// Every layer that mentions an ID — whether or not it changes anything —
// is appended to that node's Provenance.ContributingLayers, in Build's
// layer order; the first entry is always the node's origin layer.
//
// There is no delete: Build has no operation that removes a node an
// earlier layer contributed. The only way an overlay suppresses a node is
// Hidden — rendered out, but still present, and still checked by Verify.
//
// A later layer always wins on a scalar conflict, and facet-value unions
// are sorted regardless of which layer contributed which value, so Build
// is deterministic independent of how many layers touch a node or in
// what internal order a layer lists its nodes.
func Build(layers ...Layer) (Catalog, error) {
	acc := map[string]*Node{}
	keyOwner := map[string]string{} // StableKey -> the one ID currently allowed to claim it
	var order []string              // first-seen ID order; New re-sorts by ID anyway

	for _, layer := range layers {
		if layer.Name == "" {
			return Catalog{}, fmt.Errorf("catalog: layer has empty Name")
		}
		seenInLayer := map[string]bool{}
		for _, incoming := range layer.Nodes {
			if err := ValidateSlug(incoming.Slug); err != nil {
				return Catalog{}, fmt.Errorf("catalog: layer %q: %w", layer.Name, err)
			}
			id := incoming.ID()
			if seenInLayer[id] {
				return Catalog{}, fmt.Errorf("catalog: layer %q: duplicate node %q within one layer", layer.Name, id)
			}
			seenInLayer[id] = true

			existing, ok := acc[id]
			if !ok {
				n := incoming
				n.Provenance.ContributingLayers = []string{layer.Name}
				n.Provenance.Overridden = false
				n.Provenance.OverriddenBy = ""
				if err := claimStableKey(acc, keyOwner, id, n.StableKey, layer.Name); err != nil {
					return Catalog{}, err
				}
				acc[id] = &n
				order = append(order, id)
				continue
			}
			if err := mergeNode(acc, keyOwner, existing, incoming, layer.Name); err != nil {
				return Catalog{}, err
			}
		}
	}

	nodes := make([]Node, 0, len(order))
	for _, id := range order {
		nodes = append(nodes, *acc[id])
	}
	return New(nodes)
}

// claimStableKey registers id as the owner of stableKey (a no-op if
// stableKey is empty). If stableKey is already owned by a different ID,
// it returns a *LayerStableKeyCollisionError naming both nodes and both
// layers instead of letting the second claim silently win.
func claimStableKey(acc map[string]*Node, keyOwner map[string]string, id, stableKey, layerName string) error {
	if stableKey == "" {
		return nil
	}
	if ownerID, ok := keyOwner[stableKey]; ok && ownerID != id {
		ownerLayer := ""
		if owner, ok := acc[ownerID]; ok {
			ownerLayer = originLayer(*owner)
		}
		return &LayerStableKeyCollisionError{
			StableKey:     stableKey,
			ExistingID:    ownerID,
			ExistingLayer: ownerLayer,
			IncomingID:    id,
			IncomingLayer: layerName,
		}
	}
	keyOwner[stableKey] = id
	return nil
}

// mergeNode applies incoming's contribution (from layerName) onto an
// already-accumulated node (identified by id in acc), per Build's
// ADD/EXTEND/OVERRIDE rules. If incoming introduces or changes the
// node's StableKey to one already claimed by a different node, it
// returns a *LayerStableKeyCollisionError instead of applying the change.
func mergeNode(acc map[string]*Node, keyOwner map[string]string, existing *Node, incoming Node, layerName string) error {
	id := existing.ID()
	existing.Provenance.ContributingLayers = append(existing.Provenance.ContributingLayers, layerName)

	overrode := false
	if incoming.Title != "" && incoming.Title != existing.Title {
		existing.Title = incoming.Title
		overrode = true
	}
	if incoming.Synopsis != "" && incoming.Synopsis != existing.Synopsis {
		existing.Synopsis = incoming.Synopsis
		overrode = true
	}
	if incoming.Order != 0 && incoming.Order != existing.Order {
		existing.Order = incoming.Order
		overrode = true
	}
	if incoming.StableKey != "" && incoming.StableKey != existing.StableKey {
		if err := claimStableKey(acc, keyOwner, id, incoming.StableKey, layerName); err != nil {
			return err
		}
		existing.StableKey = incoming.StableKey
		overrode = true
	}
	if incoming.Hidden && !existing.Hidden {
		existing.Hidden = true
		overrode = true
	}
	if incoming.Provenance.Source != "" && incoming.Provenance.Source != existing.Provenance.Source {
		existing.Provenance.Source = incoming.Provenance.Source
		overrode = true
	}
	if incoming.Provenance.Extractor != "" && incoming.Provenance.Extractor != existing.Provenance.Extractor {
		existing.Provenance.Extractor = incoming.Provenance.Extractor
		overrode = true
	}
	if incoming.Provenance.Watermark != "" && incoming.Provenance.Watermark != existing.Provenance.Watermark {
		existing.Provenance.Watermark = incoming.Provenance.Watermark
		overrode = true
	}
	if !incoming.Provenance.ObservedAt.IsZero() {
		existing.Provenance.ObservedAt = incoming.Provenance.ObservedAt
	}

	// EXTEND: union facet values, per axis, sorted+deduplicated —
	// additive, never counted as an override.
	if len(incoming.Facets) > 0 {
		if existing.Facets == nil {
			existing.Facets = map[string][]string{}
		}
		for axis, vals := range incoming.Facets {
			existing.Facets[axis] = unionSorted(existing.Facets[axis], vals)
		}
	}

	if overrode {
		existing.Provenance.Overridden = true
		existing.Provenance.OverriddenBy = layerName
	}
	return nil
}

// unionSorted returns the sorted, deduplicated union of a and b.
func unionSorted(a, b []string) []string {
	set := map[string]bool{}
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		set[v] = true
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
