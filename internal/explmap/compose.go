package explmap

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Compose returns the ONE id a handoff carries for ids (XM-3). A single map is
// already one id and is handed back as it is, with nothing minted. Several maps
// become a minted composite made only of references; minted is that composite,
// for the caller to store (minting the same references again yields the same
// id, so storing it twice is harmless). by and at are the provenance: the
// minting agent or session, and the caller's clock.
func Compose(ids []string, maps Lookup, by string, at time.Time) (id string, minted *Map, err error) {
	clean := uniqueIDs(ids)
	if len(clean) == 1 {
		if _, ok := maps.Map(clean[0]); !ok {
			return "", nil, fmt.Errorf("%w %q", ErrUnknownMap, clean[0])
		}
		return clean[0], nil, nil
	}
	c, err := mintCompositeMap(clean, maps, by, at)
	if err != nil {
		return "", nil, err
	}
	return c.ID, &c, nil
}

// mintCompositeMap mints the composite over ids: a map whose only content is
// the ordered include references (never a copied entry), whose id is derived
// from those references alone (so re-minting is idempotent and two composites
// with the same references are the same map), and which is checked as a DAG
// before it exists (a composite over a base already MaxDepth deep is refused).
// A composite may include another composite: that is how a sub-agent passes
// its own composite down a level.
func mintCompositeMap(ids []string, maps Lookup, by string, at time.Time) (Map, error) {
	clean := uniqueIDs(ids)
	if len(clean) == 0 {
		return Map{}, errors.New("a composite needs at least one map to reference")
	}
	for _, id := range clean {
		if _, ok := maps.Map(id); !ok {
			return Map{}, fmt.Errorf("%w %q: a composite references only maps that exist", ErrUnknownMap, id)
		}
	}
	h := sha256.Sum256([]byte("composite\x00" + strings.Join(clean, "\x00")))
	c := Map{
		ID:        CompositePrefix + hex.EncodeToString(h[:])[:10],
		Kind:      KindComposite,
		Includes:  clean,
		CreatedBy: by,
		CreatedAt: at,
		LastUsed:  at,
	}
	if _, err := ResolveMap(c.ID, With(maps, c)); err != nil {
		return Map{}, err
	}
	if err := Validate(c); err != nil {
		return Map{}, err
	}
	return c, nil
}

// uniqueIDs trims ids and drops blanks and repeats, keeping first occurrence:
// the order is the priority.
func uniqueIDs(ids []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
