package catalogkit

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
)

// stableKeyIndex checks the canonical catalog shape before indexing identities.
func stableKeyIndex(c catalog.Catalog) (map[string]catalog.Node, error) {
	// New validates slugs and sorts a detached slice; it leaves c untouched.
	canonical, err := catalog.New(c.Nodes)
	if err != nil {
		return nil, fmt.Errorf("catalogkit: malformed catalog: %w", err)
	}
	for i, n := range c.Nodes {
		if n.ID() != canonical.Nodes[i].ID() {
			return nil, fmt.Errorf("catalogkit: noncanonical node order")
		}
	}
	byKey := make(map[string]catalog.Node, len(c.Nodes))
	byID := make(map[string]struct{}, len(c.Nodes))
	for _, n := range c.Nodes {
		if !validStrings(reflect.ValueOf(row(n))) || !utf8.ValidString(n.Provenance.OverriddenBy) {
			return nil, fmt.Errorf("catalogkit: invalid UTF-8 in node %s", n.ID())
		}
		if strings.TrimSpace(n.Provenance.Source) == "" {
			return nil, fmt.Errorf("catalogkit: missing source at %s", n.ID())
		}
		if n.ParentID != "" {
			for _, segment := range strings.Split(n.ParentID, "/") {
				if err := catalog.ValidateSlug(segment); err != nil {
					return nil, fmt.Errorf("catalogkit: malformed parent of %s: %w", n.ID(), err)
				}
			}
		}
		if _, exists := byID[n.ID()]; exists {
			return nil, fmt.Errorf("catalogkit: duplicate node ID %q", n.ID())
		}
		byID[n.ID()] = struct{}{}
		if strings.TrimSpace(n.StableKey) == "" || !utf8.ValidString(n.StableKey) {
			return nil, fmt.Errorf("catalogkit: missing or invalid stable key at %s", n.ID())
		}
		if _, exists := byKey[n.StableKey]; exists {
			return nil, fmt.Errorf("catalogkit: ambiguous stable key %q", n.StableKey)
		}
		byKey[n.StableKey] = n
	}
	for _, n := range c.Nodes {
		if n.ParentID != "" {
			if _, exists := byID[n.ParentID]; !exists {
				return nil, fmt.Errorf("catalogkit: missing parent %q for %s", n.ParentID, n.ID())
			}
		}
	}
	return byKey, nil
}

// verifiedBundleIndex checks the recorded digest and canonical node set once.
func verifiedBundleIndex(b Bundle) (map[string]catalog.Node, error) {
	if b.Version != 1 || b.Digest == "" || b.Digest != bundleDigest(b) {
		return nil, fmt.Errorf("catalogkit: bundle integrity mismatch")
	}
	return stableKeyIndex(b.Catalog)
}

// LookupStableKey verifies the complete bundle and returns a detached node
// with the exact stable key. Hidden nodes remain discoverable.
func LookupStableKey(b Bundle, key string) (catalog.Node, error) {
	if key == "" || !utf8.ValidString(key) {
		return catalog.Node{}, fmt.Errorf("catalogkit: missing or invalid stable key")
	}
	index, err := verifiedBundleIndex(b)
	if err != nil {
		return catalog.Node{}, err
	}
	n, ok := index[key]
	if !ok {
		return catalog.Node{}, fmt.Errorf("catalogkit: stable key %q not found", key)
	}
	return cloneNode(n), nil
}
