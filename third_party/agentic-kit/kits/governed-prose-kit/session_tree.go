package governedprosekit

import (
	"fmt"
	"sort"
	"strings"
)

func isContainer(kind string) bool {
	switch kind {
	case KindDocument, KindSection, KindList, KindTable:
		return true
	}
	return false
}

func isLeaf(kind string) bool {
	switch kind {
	case KindHeading, KindParagraph, KindSentence, KindSummary, KindListItem, KindCaption:
		return true
	}
	return false
}

// allowedChild reports whether kind may sit under a parent of parentKind.
func allowedChild(parentKind, kind string) bool {
	switch parentKind {
	case KindDocument, KindSection:
		return kind != KindListItem && kind != KindDocument
	case KindList:
		return kind == KindListItem
	case KindTable:
		return kind == KindCaption || kind == KindParagraph
	}
	return false
}

// docIndex is the validated, derived view of a DocumentTarget.
type docIndex struct {
	target   DocumentTarget
	nodes    map[string]NodeSpec
	kids     map[string][]string
	order    []string // pre-order, all nodes
	leaves   []string // pre-order, leaf nodes
	edges    []Edge
	deps     map[string][]string // leaf -> sorted dependency leaves
	topo     []string            // leaves, dependencies first
	facts    map[string]Fact
	goals    map[string]Goal
	exposed  map[string][]string // leaf -> goal IDs exposed
	sections map[string]SectionSpec
	section  map[string]string // leaf -> profile section ID ("" if none)
	context  map[string][]string
	bindings []GoalBinding
}

func impactErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrImpact, fmt.Sprintf(format, args...))
}

// buildIndex validates a target offline: digests, structure, exposure, expected
// edges/sections, cycles and required-goal coverage. Everything here fails closed
// before any source resolution or model call.
func buildIndex(limits DocumentLimits, maxFacts int, t DocumentTarget) (*docIndex, error) {
	if err := VerifyProfile(t.Profile); err != nil {
		return nil, err
	}
	if err := VerifySession(t.Session); err != nil {
		return nil, err
	}
	if t.Session.Profile != (ProfileRef{Key: t.Profile.Key, Version: t.Profile.Version, Digest: t.Profile.Digest}) {
		return nil, sessErr("session is pinned to a different profile")
	}
	if blank(t.Input.Principal) || len(t.Input.Facts) == 0 || len(t.Input.Facts) > maxFacts {
		return nil, sessErr("principal and bounded facts required")
	}
	ix := &docIndex{
		target: t, nodes: map[string]NodeSpec{}, kids: map[string][]string{}, deps: map[string][]string{}, facts: map[string]Fact{},
		goals: map[string]Goal{}, exposed: map[string][]string{}, sections: map[string]SectionSpec{}, section: map[string]string{}, context: map[string][]string{},
	}
	for _, f := range t.Input.Facts {
		if _, dup := ix.facts[f.Span.ID]; dup || blank(f.Span.ID) {
			return nil, sessErr("duplicate or blank fact ID")
		}
		ix.facts[f.Span.ID] = f
	}
	for _, s := range t.Profile.Sections {
		ix.sections[s.ID] = s
	}
	for _, g := range t.Session.Goals {
		ix.goals[g.ID] = g
	}
	if len(t.Spec.Nodes) == 0 || len(t.Spec.Nodes) > limits.MaxNodes || blank(t.Spec.ID) {
		return nil, sessErr("spec requires an ID and bounded nodes")
	}
	root := ""
	for _, n := range t.Spec.Nodes {
		if blank(n.ID) {
			return nil, sessErr("blank node ID")
		}
		if _, dup := ix.nodes[n.ID]; dup {
			return nil, sessErr("duplicate node ID %q", n.ID)
		}
		if !isContainer(n.Kind) && !isLeaf(n.Kind) {
			return nil, sessErr("node %q has unsupported kind %q", n.ID, n.Kind)
		}
		ix.nodes[n.ID] = n
		if n.Kind == KindDocument {
			if root != "" || n.ParentID != "" {
				return nil, sessErr("exactly one root document node required")
			}
			root = n.ID
		}
	}
	if root == "" {
		return nil, sessErr("document root node required")
	}
	orders := map[string]bool{}
	for _, n := range t.Spec.Nodes {
		if n.Kind == KindDocument {
			continue
		}
		parent, ok := ix.nodes[n.ParentID]
		if !ok || !isContainer(parent.Kind) || !allowedChild(parent.Kind, n.Kind) {
			return nil, sessErr("node %q has a missing or incompatible parent", n.ID)
		}
		key := fmt.Sprintf("%s\x00%d", n.ParentID, n.Order)
		if orders[key] {
			return nil, sessErr("node %q duplicates a sibling order", n.ID)
		}
		orders[key] = true
		ix.kids[n.ParentID] = append(ix.kids[n.ParentID], n.ID)
	}
	for p := range ix.kids {
		kids := ix.kids[p]
		sort.Slice(kids, func(i, j int) bool { return ix.nodes[kids[i]].Order < ix.nodes[kids[j]].Order })
	}
	seen := map[string]bool{}
	var walk func(id string, depth int) error
	walk = func(id string, depth int) error {
		if depth > limits.MaxDepth || seen[id] {
			return sessErr("tree too deep or cyclic at %q", id)
		}
		seen[id] = true
		ix.order = append(ix.order, id)
		if isLeaf(ix.nodes[id].Kind) {
			ix.leaves = append(ix.leaves, id)
		}
		for _, k := range ix.kids[id] {
			if err := walk(k, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, 1); err != nil {
		return nil, err
	}
	if len(seen) != len(ix.nodes) {
		return nil, sessErr("unreachable nodes")
	}
	for _, id := range ix.leaves {
		n := ix.nodes[id]
		if blank(n.Instruction) {
			return nil, sessErr("leaf %q requires an instruction", id)
		}
		if len(n.FactIDs) == 0 && !n.WholeInput && n.Kind != KindHeading {
			return nil, sessErr("factual leaf %q needs fact exposure", id)
		}
		for _, fid := range n.FactIDs {
			if _, ok := ix.facts[fid]; !ok {
				return nil, sessErr("leaf %q exposes unknown fact %q", id, fid)
			}
		}
		if n.Kind == KindSummary && len(n.Summarizes) == 0 {
			return nil, sessErr("summary %q must declare contributing nodes", id)
		}
		ctx := dedupSorted(append(append([]string{}, n.ContextNodeIDs...), n.Summarizes...))
		for _, c := range ctx {
			if cn, ok := ix.nodes[c]; !ok || !isLeaf(cn.Kind) || c == id {
				return nil, impactErr("leaf %q depends on missing, non-leaf or self node %q", id, c)
			}
		}
		for _, c := range n.Summarizes {
			ix.edges = append(ix.edges, Edge{From: id, To: c, Kind: EdgeSummary})
		}
		for _, c := range dedupSorted(n.ContextNodeIDs) {
			if !contains(n.Summarizes, c) {
				ix.edges = append(ix.edges, Edge{From: id, To: c, Kind: EdgeContext})
			}
		}
		ix.context[id] = ctx
		ix.deps[id] = ctx
		for _, g := range n.GoalIDs {
			if _, ok := ix.goals[g]; !ok {
				return nil, sessErr("leaf %q names unknown goal %q", id, g)
			}
		}
	}
	if len(ix.edges) > limits.MaxEdges {
		return nil, sessErr("too many edges")
	}
	have := map[Edge]bool{}
	for _, e := range ix.edges {
		have[e] = true
	}
	for _, e := range t.Spec.ExpectedEdges {
		if _, ok := ix.nodes[e.From]; !ok {
			return nil, impactErr("expected edge from missing node %q", e.From)
		}
		if _, ok := ix.nodes[e.To]; !ok {
			return nil, impactErr("expected edge to missing node %q", e.To)
		}
		if !have[e] {
			return nil, impactErr("declared dependencies omit expected edge %s -> %s (%s)", e.From, e.To, e.Kind)
		}
	}
	state := map[string]int{}
	var visit func(id string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return impactErr("dependency cycle through %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		for _, d := range ix.deps[id] {
			if err := visit(d); err != nil {
				return err
			}
		}
		state[id] = 2
		ix.topo = append(ix.topo, id)
		return nil
	}
	for _, id := range ix.leaves {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	for _, want := range t.Session.ExpectedSectionIDs {
		if n, ok := ix.nodes[want]; !ok || n.Kind != KindSection {
			return nil, sessErr("expected section %q is missing from the document", want)
		}
	}
	for id, n := range ix.nodes {
		if n.Kind == KindSection {
			if _, ok := ix.sections[id]; !ok && !contains(t.Session.ExpectedSectionIDs, id) && !t.Profile.PermitExtra {
				return nil, sessErr("section %q is outside the profile and extras are not permitted", id)
			}
		}
	}
	for _, id := range ix.leaves {
		n := ix.nodes[id]
		anc := ix.ancestors(id)
		ix.section[id] = n.SectionID
		if n.SectionID == "" {
			for _, a := range anc {
				if _, ok := ix.sections[a]; ok {
					ix.section[id] = a
					break
				}
			}
		} else if _, ok := ix.sections[n.SectionID]; !ok {
			return nil, sessErr("leaf %q names unknown profile section %q", id, n.SectionID)
		}
		var exp []string
		for gid, g := range ix.goals {
			if g.Scope == ScopeDocument || contains(anc, g.Scope) || contains(n.GoalIDs, gid) {
				exp = append(exp, gid)
			}
		}
		ix.exposed[id] = dedupSorted(exp)
	}
	for _, gid := range sortedGoalIDs(ix.goals) {
		b := GoalBinding{GoalID: gid, Class: ix.goals[gid].Class, Nodes: []string{}}
		for _, id := range ix.leaves {
			if contains(ix.exposed[id], gid) {
				b.Nodes = append(b.Nodes, id)
			}
		}
		if b.Class == GoalRequired && len(b.Nodes) == 0 {
			return nil, sessErr("required goal %q is bound to no generated node", gid)
		}
		ix.bindings = append(ix.bindings, b)
	}
	return ix, nil
}

func sortedGoalIDs(m map[string]Goal) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (ix *docIndex) ancestors(id string) []string {
	var out []string
	for p := ix.nodes[id].ParentID; p != ""; p = ix.nodes[p].ParentID {
		out = append(out, p)
	}
	return out
}

func leafContentDigest(n DocNode) (string, error) {
	return digestJSON(struct {
		ID         string       `json:"id"`
		Kind       string       `json:"kind"`
		Text       string       `json:"text"`
		Claims     []ProseClaim `json:"claims"`
		Ranges     []ClaimRange `json:"ranges"`
		NonFactual string       `json:"nf"`
	}{n.ID, n.Kind, n.Text, n.Claims, n.Ranges, n.NonFactual})
}

func sealManifest(m ReadManifest) (ReadManifest, error) {
	m.Digest = ""
	d, err := digestJSON(m)
	m.Digest = d
	return m, err
}

// sealNode computes the leaf content digest and its provenance-inclusive Version.
func sealNode(n DocNode) (DocNode, error) {
	var err error
	if n.ContentDigest, err = leafContentDigest(n); err != nil {
		return n, err
	}
	gen, rev := "", ""
	if n.GenManifest != nil {
		gen = n.GenManifest.Digest
	}
	if n.ReviewMan != nil {
		rev = n.ReviewMan.Digest
	}
	n.Version, err = digestJSON(struct {
		C, G, R string
		S       map[string]SupportDecision
		Gen     *GenerationRecord
	}{n.ContentDigest, gen, rev, n.Support, n.Generation})
	return n, err
}

// sealContainer rolls children into the container's content digest and Version.
func sealContainer(n DocNode, children []DocNode) (DocNode, error) {
	type pair struct{ ID, C, V string }
	var cs []pair
	for _, c := range children {
		cs = append(cs, pair{c.ID, c.ContentDigest, c.Version})
	}
	var err error
	cc := make([]pair, len(cs))
	for i, c := range cs {
		cc[i] = pair{c.ID, c.C, ""}
	}
	if n.ContentDigest, err = digestJSON(struct {
		ID, Kind string
		Kids     []pair
	}{n.ID, n.Kind, cc}); err != nil {
		return n, err
	}
	n.Version, err = digestJSON(struct {
		ID, Kind string
		Kids     []pair
	}{n.ID, n.Kind, cs})
	return n, err
}

func documentDigest(v DocumentVersion) (string, error) {
	v.Receipt.DocumentDigest = ""
	return digestJSON(v)
}

// VerifyDocument recomputes the profile, session, node content, container rollup
// and document digests. It detects mutation of a returned version; it does not
// re-resolve sources (Render does) and is not a signature.
func VerifyDocument(v DocumentVersion) error {
	if v.Receipt.Version != sessionDigestVersion || v.Receipt.Integrity != "pass" {
		return sessErr("document receipt not passing")
	}
	d, err := documentDigest(v)
	if err != nil || d != v.Receipt.DocumentDigest {
		return sessErr("document digest mismatch")
	}
	if VerifyProfile(v.Target.Profile) != nil || VerifySession(v.Target.Session) != nil {
		return sessErr("session or profile digest mismatch")
	}
	if v.Receipt.SessionDigest != v.Target.Session.Digest || v.Receipt.ProfileDigest != v.Target.Profile.Digest {
		return sessErr("receipt does not bind the session/profile")
	}
	byID := map[string]DocNode{}
	for _, n := range v.Nodes {
		byID[n.ID] = n
	}
	kids := map[string][]DocNode{}
	for _, n := range v.Nodes {
		if n.ParentID != "" {
			kids[n.ParentID] = append(kids[n.ParentID], n)
		}
	}
	for _, n := range v.Nodes {
		var got DocNode
		if isLeaf(n.Kind) {
			if got, err = sealNode(n); err != nil || got.ContentDigest != n.ContentDigest || got.Version != n.Version {
				return sessErr("node %q digest mismatch", n.ID)
			}
			continue
		}
		cs := kids[n.ID]
		sort.Slice(cs, func(i, j int) bool { return cs[i].Order < cs[j].Order })
		if got, err = sealContainer(n, cs); err != nil || got.ContentDigest != n.ContentDigest || got.Version != n.Version {
			return sessErr("container %q digest mismatch", n.ID)
		}
	}
	return nil
}

func nodeOrderIndex(v DocumentVersion) (map[string]DocNode, map[string][]DocNode) {
	byID := map[string]DocNode{}
	kids := map[string][]DocNode{}
	for _, n := range v.Nodes {
		byID[n.ID] = n
		if n.ParentID != "" {
			kids[n.ParentID] = append(kids[n.ParentID], n)
		}
	}
	for k := range kids {
		cs := kids[k]
		sort.Slice(cs, func(i, j int) bool { return cs[i].Order < cs[j].Order })
	}
	return byID, kids
}

// RenderMarkdown renders a verified version deterministically with a node span
// map. It performs no source checks; use DocumentEngine.Render for publication.
func RenderMarkdown(v DocumentVersion) (RenderedDocument, error) {
	if err := VerifyDocument(v); err != nil {
		return RenderedDocument{}, err
	}
	byID, kids := nodeOrderIndex(v)
	var root string
	for _, n := range v.Nodes {
		if n.Kind == KindDocument {
			root = n.ID
		}
	}
	var b strings.Builder
	var spans []SpanMapEntry
	leaf := func(n DocNode, prefix, suffix string) {
		b.WriteString(prefix)
		start := b.Len()
		b.WriteString(n.Text)
		spans = append(spans, SpanMapEntry{NodeID: n.ID, Start: start, End: b.Len()})
		b.WriteString(suffix)
	}
	var render func(id string, sections int)
	render = func(id string, sections int) {
		n := byID[id]
		sep := "\n\n"
		switch n.Kind {
		case KindList, KindTable:
			sep = "\n"
		case KindSection:
			sections++
		}
		for i, k := range kids[id] {
			if i > 0 {
				b.WriteString(sep)
			}
			c := byID[k.ID]
			switch c.Kind {
			case KindHeading:
				leaf(c, strings.Repeat("#", min(6, sections+1))+" ", "")
			case KindListItem:
				leaf(c, "- ", "")
			case KindCaption:
				leaf(c, "*", "*")
			case KindParagraph, KindSentence, KindSummary:
				leaf(c, "", "")
			default:
				render(c.ID, sections)
			}
		}
	}
	render(root, 0)
	md := b.String()
	return RenderedDocument{Markdown: md, MarkdownDigest: ContentDigest(md), DocumentDigest: v.Receipt.DocumentDigest, SpanMap: spans}, nil
}

// InspectNode returns a node with its semantic dependencies, dependents and the
// goals it was exposed to. It copies; callers cannot mutate the version.
func InspectNode(v DocumentVersion, id string) (NodeInspection, error) {
	for _, n := range v.Nodes {
		if n.ID != id {
			continue
		}
		owned, err := cloneValue(n)
		if err != nil {
			return NodeInspection{}, err
		}
		out := NodeInspection{Node: owned, Dependencies: []Edge{}, Dependents: []Edge{}, Goals: []string{}}
		for _, e := range v.Edges {
			if e.From == id {
				out.Dependencies = append(out.Dependencies, e)
			}
			if e.To == id {
				out.Dependents = append(out.Dependents, e)
			}
		}
		for _, g := range v.Goals {
			if contains(g.Nodes, id) {
				out.Goals = append(out.Goals, g.GoalID)
			}
		}
		return out, nil
	}
	return NodeInspection{}, sessErr("unknown node %q", id)
}
