package catalogkit

import (
	"bytes"
	"encoding/json"
	"sort"
)

// ExportNDJSON returns ID-sorted analytics rows, including hidden entries. The
// entire bundle must verify before any bytes are returned. It is a projection,
// not an authoring format or a lossless replacement for the bundle audit trail.
func ExportNDJSON(b Bundle) ([]byte, error) {
	if err := VerifyBundle(b); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	e := json.NewEncoder(&out)
	e.SetEscapeHTML(false)
	for _, r := range Rows(b) {
		if err := e.Encode(r); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

// Difference reports semantic changes, including hidden/presentation changes
// omitted by the older catalog.Node.Hash contract. IDs are sorted.
type Difference struct {
	Before  string   `json:"before"`
	After   string   `json:"after"`
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
	Moved   []Move   `json:"moved"`
}

// Move records one logical node's old and current locations.
type Move struct {
	StableKey string `json:"stable_key"`
	BeforeID  string `json:"before_id"`
	AfterID   string `json:"after_id"`
}

// Compare checks both bundles and compares every node's semantic digest.
func Compare(before, after Bundle) (Difference, error) {
	d := Difference{Before: before.Digest, After: after.Digest, Added: []string{}, Removed: []string{}, Changed: []string{}, Moved: []Move{}}
	a, err := verifiedBundleIndex(before)
	if err != nil {
		return Difference{}, err
	}
	b, err := verifiedBundleIndex(after)
	if err != nil {
		return Difference{}, err
	}
	for key, current := range b {
		old, ok := a[key]
		if !ok {
			d.Added = append(d.Added, current.ID())
			continue
		}
		if old.ID() != current.ID() {
			d.Moved = append(d.Moved, Move{key, old.ID(), current.ID()})
		}
		// A path move changes ParentID and Slug but not the logical content.
		old.ParentID, old.Slug = current.ParentID, current.Slug
		if NodeDigest(old) != NodeDigest(current) {
			d.Changed = append(d.Changed, current.ID())
		}
	}
	for key, old := range a {
		if _, ok := b[key]; !ok {
			d.Removed = append(d.Removed, old.ID())
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	sort.Strings(d.Changed)
	sort.Slice(d.Moved, func(i, j int) bool {
		if d.Moved[i].StableKey != d.Moved[j].StableKey {
			return d.Moved[i].StableKey < d.Moved[j].StableKey
		}
		return d.Moved[i].AfterID < d.Moved[j].AfterID
	})
	return d, nil
}
