package catalogkit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
)

// DefaultLimits returns limits of 256 layers, 100,000 input nodes/patches,
// 256 hierarchy levels and 16 MiB of authored or resolved JSON.
func DefaultLimits() Limits { return Limits{256, 100000, 256, 16 << 20} }

func effectiveLimits(in Limits) (Limits, error) {
	d := DefaultLimits()
	for _, p := range []struct {
		in  int
		out *int
	}{{in.MaxLayers, &d.MaxLayers}, {in.MaxNodes, &d.MaxNodes}, {in.MaxDepth, &d.MaxDepth}, {in.MaxBytes, &d.MaxBytes}} {
		if p.in < 0 {
			return Limits{}, fmt.Errorf("catalogkit: negative resource limit")
		}
		if p.in > 0 {
			*p.out = p.in
		}
	}
	return d, nil
}

// LayerDigest hashes the JSON declaration with Pin omitted. Declaration order is
// significant for this source pin, even where resolved node order is equivalent.
func LayerDigest(l Layer) string { l.Pin = ""; return digest(l) }

func digest(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func normalizedFacets(in map[string][]string) map[string][]string {
	out := map[string][]string{}
	for k, vs := range in {
		values := append([]string{}, vs...)
		sort.Strings(values)
		out[k] = compact(values)
	}
	return out
}
func compact(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
func cloneNode(n catalog.Node) catalog.Node {
	n.Facets = normalizedFacets(n.Facets)
	n.Provenance.ContributingLayers = append([]string{}, n.Provenance.ContributingLayers...)
	return n
}
func row(n catalog.Node) Row {
	return Row{n.ID(), n.ParentID, n.Title, n.Synopsis, n.Order, n.StableKey, n.Hidden, normalizedFacets(n.Facets), n.Provenance.Source, n.Provenance.Extractor, n.Provenance.Watermark, append([]string{}, n.Provenance.ContributingLayers...)}
}

// NodeDigest covers every semantic and presentation field, unlike the legacy
// catalog.Node.Hash drift contract. Contribution bookkeeping is excluded.
func NodeDigest(n catalog.Node) string { r := row(n); r.Layers = nil; return digest(r) }

// Rows returns a detached, ID-sorted projection, including hidden nodes.
func Rows(b Bundle) []Row {
	out := make([]Row, 0, len(b.Catalog.Nodes))
	for _, n := range b.Catalog.Nodes {
		out = append(out, row(n))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func bundleDigest(b Bundle) string {
	return digest(struct {
		Version int
		Order   []string
		Sources []SourcePin
		Policy  string
		Catalog catalog.Catalog
		Changes []Change
	}{b.Version, b.Order, b.Sources, b.PolicyDigest, b.Catalog, b.Changes})
}

// VerifyBundle checks recorded integrity and canonical catalog shape. It does
// not authenticate the producer; consumers must compare an independently
// trusted expected digest when origin matters.
func VerifyBundle(b Bundle) error {
	_, err := verifiedBundleIndex(b)
	return err
}

// Resolve composes every supplied layer in dependency order (lexical tie breaks).
// Repeated IDs require explicit ancestry. Non-zero scalar assertions must agree;
// replacement needs a separately granted, reasoned, digest-checked Override.
// The whole operation refuses on failure and never returns a partial bundle.
func Resolve(spec Spec, policy Policy) (Bundle, error) {
	limits, err := effectiveLimits(policy.Limits)
	if err != nil {
		return Bundle{}, err
	}
	allNodes, err := validateLoadedSpec(spec, policy, limits)
	if err != nil {
		return Bundle{}, err
	}
	order, layers, ancestors, err := resolveGraph(spec.Layers)
	if err != nil {
		return Bundle{}, err
	}
	grants, err := validateGrants(policy.Grants, layers)
	if err != nil {
		return Bundle{}, err
	}
	acc := map[string]catalog.Node{}
	keys := map[string]string{}
	b := Bundle{Version: 1, Order: order, Sources: []SourcePin{}, Changes: []Change{}, PolicyDigest: policyDigest(policy)}
	usedGrants := map[string]bool{}
	for _, name := range order {
		l := layers[name]
		pin := LayerDigest(l)
		b.Sources = append(b.Sources, SourcePin{name, pin})
		for _, incoming := range allNodes[name] {
			id := incoming.ID()
			old, exists := acc[id]
			if exists {
				if err := requireAncestors(old, name, ancestors[name]); err != nil {
					return Bundle{}, err
				}
				merged, err := extend(old, incoming)
				if err != nil {
					return Bundle{}, err
				}
				incoming = merged
			} else {
				if strings.TrimSpace(incoming.StableKey) == "" || strings.TrimSpace(incoming.Provenance.Source) == "" {
					return Bundle{}, fmt.Errorf("catalogkit: %s needs stable_key and source", id)
				}
				if owner, ok := keys[incoming.StableKey]; ok && owner != id {
					return Bundle{}, fmt.Errorf("catalogkit: duplicate stable_key %s", incoming.StableKey)
				}
				keys[incoming.StableKey] = id
			}
			incoming.Provenance.ContributingLayers = append(incoming.Provenance.ContributingLayers, name)
			acc[id] = cloneNode(incoming)
		}
		seen := map[string]bool{}
		for _, o := range l.Overrides {
			if seen[o.ID] {
				return Bundle{}, fmt.Errorf("catalogkit: duplicate patch %s in %s", o.ID, name)
			}
			seen[o.ID] = true
			old, ok := acc[o.ID]
			if !ok {
				return Bundle{}, fmt.Errorf("catalogkit: unknown patch target %s", o.ID)
			}
			if err := requireAncestors(old, name, ancestors[name]); err != nil {
				return Bundle{}, err
			}
			before := NodeDigest(old)
			if o.ExpectedDigest != before || strings.TrimSpace(o.Reason) == "" {
				return Bundle{}, fmt.Errorf("catalogkit: stale or unjustified patch %s", o.ID)
			}
			fields := patchFields(o.Set)
			if len(fields) == 0 {
				return Bundle{}, fmt.Errorf("catalogkit: empty patch %s", o.ID)
			}
			for _, field := range fields {
				key := grantKey(name, o.ID, field)
				if !grants[key] {
					return Bundle{}, fmt.Errorf("catalogkit: ungranted %s patch of %s by %s", field, o.ID, name)
				}
				usedGrants[key] = true
			}
			n := applyPatch(old, o.Set)
			if len(n.Provenance.ContributingLayers) == 0 || n.Provenance.ContributingLayers[len(n.Provenance.ContributingLayers)-1] != name {
				n.Provenance.ContributingLayers = append(n.Provenance.ContributingLayers, name)
			}
			n.Provenance.Overridden = true
			n.Provenance.OverriddenBy = name
			acc[o.ID] = n
			b.Changes = append(b.Changes, Change{name, o.ID, fields, o.Reason, before, NodeDigest(n)})
		}
	}
	for key := range grants {
		if !usedGrants[key] {
			return Bundle{}, fmt.Errorf("catalogkit: unused override grant %q", key)
		}
	}
	nodes := make([]catalog.Node, 0, len(acc))
	for _, n := range acc {
		if n.ParentID != "" {
			if _, ok := acc[n.ParentID]; !ok {
				return Bundle{}, fmt.Errorf("catalogkit: orphan %s", n.ID())
			}
		}
		nodes = append(nodes, n)
	}
	b.Catalog, err = catalog.New(nodes)
	if err != nil {
		return Bundle{}, err
	}
	sort.Slice(b.Changes, func(i, j int) bool {
		a, c := b.Changes[i], b.Changes[j]
		if a.Layer != c.Layer {
			return indexOf(order, a.Layer) < indexOf(order, c.Layer)
		}
		return a.ID < c.ID
	})
	b.Digest = bundleDigest(b)
	out, _ := json.Marshal(b)
	if len(out) > limits.MaxBytes {
		return Bundle{}, fmt.Errorf("catalogkit: resolved byte limit exceeded")
	}
	return b, nil
}

// validateLoadedSpec checks every declaration before any profile selection.
// Composition rules still apply only to the layers selected for resolution.
func validateLoadedSpec(spec Spec, policy Policy, limits Limits) (map[string][]catalog.Node, error) {
	if spec.Version != 1 || len(spec.Layers) == 0 || len(spec.Layers) > limits.MaxLayers {
		return nil, fmt.Errorf("catalogkit: invalid version or layer count")
	}
	// Bound tree traversal before JSON marshaling or recursive library calls.
	allNodes := make(map[string][]catalog.Node, len(spec.Layers))
	total := 0
	for _, l := range spec.Layers {
		nodes, err := flattenBounded(l.Nodes, limits, &total)
		if err != nil {
			return nil, fmt.Errorf("catalogkit: layer %s: %w", l.Name, err)
		}
		total += len(l.Overrides)
		if total > limits.MaxNodes {
			return nil, fmt.Errorf("catalogkit: node/patch limit exceeded")
		}
		allNodes[l.Name] = nodes
	}
	if !validStrings(reflect.ValueOf(spec)) || !validStrings(reflect.ValueOf(policy)) {
		return nil, fmt.Errorf("catalogkit: invalid UTF-8 in authored values")
	}
	input, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("catalogkit: marshal authored values: %w", err)
	}
	if len(input) > limits.MaxBytes {
		return nil, fmt.Errorf("catalogkit: authored byte limit exceeded")
	}
	receiver, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("catalogkit: marshal receiver policy: %w", err)
	}
	if len(receiver) > limits.MaxBytes {
		return nil, fmt.Errorf("catalogkit: receiver policy byte limit exceeded")
	}
	for _, l := range spec.Layers {
		if l.Pin != "" && l.Pin != LayerDigest(l) {
			return nil, fmt.Errorf("catalogkit: layer %s pin mismatch", l.Name)
		}
	}
	return allNodes, nil
}

func validStrings(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return utf8.ValidString(v.String())
	case reflect.Pointer, reflect.Interface:
		return v.IsNil() || validStrings(v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !validStrings(v.Field(i)) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !validStrings(v.Index(i)) {
				return false
			}
		}
	case reflect.Map:
		it := v.MapRange()
		for it.Next() {
			if !validStrings(it.Key()) || !validStrings(it.Value()) {
				return false
			}
		}
	}
	return true
}

func indexOf(ss []string, s string) int {
	for i, x := range ss {
		if x == s {
			return i
		}
	}
	return -1
}

func flattenBounded(roots []catalog.AuthoredNode, limits Limits, total *int) ([]catalog.Node, error) {
	if len(roots) > limits.MaxNodes-*total {
		return nil, fmt.Errorf("node/depth limit exceeded")
	}
	type pending struct {
		node   catalog.AuthoredNode
		parent string
		depth  int
	}
	stack := []pending{}
	for _, n := range roots {
		stack = append(stack, pending{n, "", 1})
	}
	out := []catalog.Node{}
	seen := map[string]bool{}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		*total++
		if *total > limits.MaxNodes || p.depth > limits.MaxDepth {
			return nil, fmt.Errorf("node/depth limit exceeded")
		}
		a := p.node
		if err := catalog.ValidateSlug(a.Slug); err != nil {
			return nil, err
		}
		id := catalog.JoinID(p.parent, a.Slug)
		if seen[id] {
			return nil, fmt.Errorf("duplicate node %s", id)
		}
		seen[id] = true
		n := catalog.Node{ParentID: p.parent, Slug: a.Slug, Title: a.Title, Synopsis: a.Synopsis, Order: a.Order, StableKey: a.StableKey, Facets: normalizedFacets(a.Facets), Hidden: a.Hidden, Provenance: catalog.Provenance{Source: a.Source, Extractor: a.Extractor, Watermark: a.Watermark}}
		out = append(out, n)
		if len(a.Children) > limits.MaxNodes-*total-len(stack) {
			return nil, fmt.Errorf("node/depth limit exceeded")
		}
		for _, child := range a.Children {
			stack = append(stack, pending{child, id, p.depth + 1})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out, nil
}

func resolveGraph(input []Layer) ([]string, map[string]Layer, map[string]map[string]bool, error) {
	layers := map[string]Layer{}
	names := []string{}
	for _, l := range input {
		if err := catalog.ValidateSlug(l.Name); err != nil {
			return nil, nil, nil, err
		}
		if _, ok := layers[l.Name]; ok {
			return nil, nil, nil, fmt.Errorf("catalogkit: duplicate layer %s", l.Name)
		}
		layers[l.Name] = l
		names = append(names, l.Name)
	}
	sort.Strings(names)
	state := map[string]int{}
	anc := map[string]map[string]bool{}
	order := []string{}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("catalogkit: dependency cycle at %s", name)
		}
		if state[name] == 2 {
			return nil
		}
		l, ok := layers[name]
		if !ok {
			return fmt.Errorf("catalogkit: missing dependency %s", name)
		}
		state[name] = 1
		anc[name] = map[string]bool{}
		deps := append([]string{}, l.Requires...)
		sort.Strings(deps)
		for i, dep := range deps {
			if i > 0 && deps[i-1] == dep {
				return fmt.Errorf("catalogkit: duplicate dependency %s", dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
			anc[name][dep] = true
			for a := range anc[dep] {
				anc[name][a] = true
			}
		}
		state[name] = 2
		order = append(order, name)
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, nil, nil, err
		}
	}
	return order, layers, anc, nil
}

func requireAncestors(n catalog.Node, layer string, anc map[string]bool) error {
	for _, a := range n.Provenance.ContributingLayers {
		if a != layer && !anc[a] {
			return fmt.Errorf("catalogkit: layer %s must require contributor %s before touching %s", layer, a, n.ID())
		}
	}
	return nil
}

func extend(old, in catalog.Node) (catalog.Node, error) {
	n := cloneNode(old)
	for _, p := range []struct {
		a *string
		b string
	}{{&n.Title, in.Title}, {&n.Synopsis, in.Synopsis}, {&n.StableKey, in.StableKey}, {&n.Provenance.Source, in.Provenance.Source}, {&n.Provenance.Extractor, in.Provenance.Extractor}, {&n.Provenance.Watermark, in.Provenance.Watermark}} {
		if p.b != "" {
			if *p.a != "" && *p.a != p.b {
				return catalog.Node{}, fmt.Errorf("catalogkit: implicit scalar replacement of %s", old.ID())
			}
			*p.a = p.b
		}
	}
	if (in.Order != 0 && in.Order != n.Order) || (in.Hidden && !n.Hidden) {
		return catalog.Node{}, fmt.Errorf("catalogkit: implicit scalar replacement of %s", old.ID())
	}
	for axis, values := range in.Facets {
		n.Facets[axis] = append(n.Facets[axis], values...)
	}
	n.Facets = normalizedFacets(n.Facets)
	return n, nil
}

func patchFields(p Patch) []string {
	out := []string{}
	if p.Title != nil {
		out = append(out, "title")
	}
	if p.Synopsis != nil {
		out = append(out, "synopsis")
	}
	if p.Order != nil {
		out = append(out, "order")
	}
	if p.Hidden != nil {
		out = append(out, "hidden")
	}
	if p.Facets != nil {
		out = append(out, "facets")
	}
	sort.Strings(out)
	return out
}
func applyPatch(n catalog.Node, p Patch) catalog.Node {
	n = cloneNode(n)
	if p.Title != nil {
		n.Title = *p.Title
	}
	if p.Synopsis != nil {
		n.Synopsis = *p.Synopsis
	}
	if p.Order != nil {
		n.Order = *p.Order
	}
	if p.Hidden != nil {
		n.Hidden = *p.Hidden
	}
	if p.Facets != nil {
		n.Facets = normalizedFacets(*p.Facets)
	}
	return n
}
func grantKey(layer, id, field string) string { return layer + "\x00" + id + "\x00" + field }
func validateGrants(gs []Grant, layers map[string]Layer) (map[string]bool, error) {
	out := map[string]bool{}
	for _, g := range gs {
		if _, ok := layers[g.Layer]; !ok {
			return nil, fmt.Errorf("catalogkit: grant names unknown layer %s", g.Layer)
		}
		if g.ID == "" || len(g.Fields) == 0 {
			return nil, fmt.Errorf("catalogkit: empty grant")
		}
		for _, f := range g.Fields {
			if !strings.Contains("|title|synopsis|order|hidden|facets|", "|"+f+"|") || f == "" {
				return nil, fmt.Errorf("catalogkit: unknown grant field %s", f)
			}
			key := grantKey(g.Layer, g.ID, f)
			if out[key] {
				return nil, fmt.Errorf("catalogkit: duplicate grant")
			}
			out[key] = true
		}
	}
	return out, nil
}
func policyDigest(p Policy) string {
	p.Grants = append([]Grant{}, p.Grants...)
	for i := range p.Grants {
		p.Grants[i].Fields = append([]string{}, p.Grants[i].Fields...)
		sort.Strings(p.Grants[i].Fields)
	}
	sort.Slice(p.Grants, func(i, j int) bool {
		a, b := p.Grants[i], p.Grants[j]
		if a.Layer != b.Layer {
			return a.Layer < b.Layer
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return strings.Join(a.Fields, ",") < strings.Join(b.Fields, ",")
	})
	p.Limits, _ = effectiveLimits(p.Limits)
	return digest(p)
}

// EqualContent reports semantic equality independently of layer bookkeeping.
func EqualContent(a, b catalog.Node) bool {
	ra, rb := row(a), row(b)
	ra.Layers = nil
	rb.Layers = nil
	return reflect.DeepEqual(ra, rb)
}
