package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// slugPattern is the allowed shape for a single path segment: lowercase
// ASCII letters, digits, and internal hyphens (kebab-case), matching the
// kit's own directory-naming convention (CLAUDE.md: "Dirs are
// kebab-case").
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSlug reports an error if slug is not a valid path segment.
func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return fmt.Errorf("catalog: invalid slug %q: must be non-empty lowercase kebab-case", slug)
	}
	return nil
}

// Provenance records where a node came from and whether it is still
// trustworthy: the Source it was scanned or authored from, the Extractor
// that produced it, and a Watermark identifying the scan (a commit SHA, a
// timestamp string, a version tag — whatever the source of truth
// considers current). ObservedAt is the wall-clock instant the source was
// read; callers supply it explicitly (via their own injected clock) —
// Provenance itself never calls time.Now().
//
// A node without a non-empty Source cannot fully participate in the drift
// gate: Verify still runs, but a caller building a catalog for real
// governance should always set it.
//
// ContributingLayers, Overridden, and OverriddenBy are merge bookkeeping,
// populated only by Build (see build.go); they are empty/false for a
// Catalog assembled directly via New.
type Provenance struct {
	Source     string
	Extractor  string
	Watermark  string
	ObservedAt time.Time

	// ContributingLayers names, in order, every Build layer that touched
	// this node; the first entry is always the node's origin layer.
	ContributingLayers []string

	// Overridden is true once a later layer has replaced one or more of
	// this node's own scalar fields after the origin layer created it.
	// Extending Facets never sets Overridden — that is additive, not a
	// claim that changes what an earlier layer already said.
	Overridden bool

	// OverriddenBy is the layer Name of the last layer that overrode a
	// scalar field. Empty if Overridden is false.
	OverriddenBy string
}

// Fresh reports whether p is still within maxAge of now. Callers supply
// now explicitly (never time.Now() inside this package) so freshness
// checks stay deterministic and testable.
func (p Provenance) Fresh(now time.Time, maxAge time.Duration) bool {
	if p.ObservedAt.IsZero() {
		return false
	}
	d := now.Sub(p.ObservedAt)
	return d >= 0 && d <= maxAge
}

// Node is one entry in a catalog: a section, group, subgroup, or item —
// the same type at every depth, because depth is data, never a type
// (design §2.1). A node's identity is its ID, computed by
// JoinID(ParentID, Slug); callers set ParentID and Slug and never set an
// ID field directly, so a Node can never disagree with its own identity.
type Node struct {
	// ParentID is "" for a top-level node, otherwise another node's ID.
	ParentID string
	// Slug is this node's own path segment; required, kebab-case, unique
	// among siblings (see DuplicateSlugs).
	Slug string

	Title    string
	Synopsis string // indexed by Search; not required

	// Order is an explicit sort hint among siblings; ties — including the
	// default zero value shared by every unordered sibling — break on
	// Slug, so an entirely unordered catalog still renders deterministically.
	Order int

	// StableKey, if set, survives renames: Verify uses it to distinguish
	// a moved node (same StableKey, different ID) from an unrelated
	// delete-then-add pair.
	StableKey string

	// Facets is the axis->allowed-values map for an item (design §2.2).
	// Facets are metadata, never tree levels: Variants expands them into
	// the item's variant set without creating any child Node.
	Facets map[string][]string

	// Hidden marks a node rendered out (by the renderers in this
	// package) while leaving it fully present for Verify and every
	// compute function — the only suppression an overlay may apply;
	// Build has no delete operation.
	Hidden bool

	Provenance Provenance
}

// ID returns the node's identity: JoinID(n.ParentID, n.Slug).
func (n Node) ID() string { return JoinID(n.ParentID, n.Slug) }

// JoinID computes a node ID from its parent's ID and its own slug: the
// slash-joined path of ancestor slugs (design §3's stable node identity).
// parentID == "" means a top-level node, whose ID is simply its slug.
func JoinID(parentID, slug string) string {
	if parentID == "" {
		return slug
	}
	return parentID + "/" + slug
}

// Depth returns a node's depth from its ID: the number of path segments.
// A top-level node ("foundations") has depth 1; "" (no node) has depth 0.
func Depth(id string) int {
	if id == "" {
		return 0
	}
	return strings.Count(id, "/") + 1
}

// Hash returns the node's content-addressed fingerprint over (id, title,
// facets, provenance) — design §3's "did this entry change?" answer
// without reading the whole tree. Facet axes and values are sorted
// before hashing so the result never depends on map iteration order.
//
// Deliberately excluded: Synopsis, Order, Hidden, StableKey, and
// Provenance.ObservedAt/ContributingLayers/Overridden/OverriddenBy — all
// presentation or merge bookkeeping, not identity of what the entry IS.
// A node's freshness ticking forward, or a fourth layer contributing to
// it with no actual change, must never register as Changed drift.
func (n Node) Hash() string {
	h := sha256.New()
	fmt.Fprintf(h, "id:%s\ntitle:%s\n", n.ID(), n.Title)
	fmt.Fprintf(h, "provenance.source:%s\nprovenance.extractor:%s\nprovenance.watermark:%s\n",
		n.Provenance.Source, n.Provenance.Extractor, n.Provenance.Watermark)

	axes := make([]string, 0, len(n.Facets))
	for axis := range n.Facets {
		axes = append(axes, axis)
	}
	sort.Strings(axes)
	for _, axis := range axes {
		vals := append([]string(nil), n.Facets[axis]...)
		sort.Strings(vals)
		fmt.Fprintf(h, "facet:%s=%s\n", axis, strings.Join(vals, ","))
	}
	return hex.EncodeToString(h.Sum(nil))
}
