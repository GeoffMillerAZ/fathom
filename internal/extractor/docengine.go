package extractor

// docengine.go — the document-path engine (LB-F8, docs/design/
// language-breadth.md §4, ADR-048). Tool formats whose syntax is YAML,
// JSON/JSONC, TOML or XML are a schema problem, not a parsing problem: a
// doc.json names the decode, the entities to select, and the references
// between them, and this engine walks the decoded tree. It generalizes the
// walk the six bespoke config extractors (GitHub Actions, Compose,
// Kubernetes, Kustomize, Helm values, OpenAPI) each hand-code (P20):
// TestDocExtractorParity reproduces Compose's and GitHub Actions' symbols
// from a doc.json alone.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// DocSpec is a doc.json (§4).
type DocSpec struct {
	// Decode is "yaml", "json", "jsonc", "toml" or "xml".
	Decode string `json:"decode"`
	// MultiDocument: a YAML stream's documents are walked one by one
	// (Kubernetes manifests).
	MultiDocument bool          `json:"multi_document,omitempty"`
	Detect        DocDetect     `json:"detect"`
	Entities      []DocEntity   `json:"entities"`
	References    []DocRef      `json:"references,omitempty"`
	CrossFile     []DocCrossRef `json:"cross_file,omitempty"`
	// Labels names the label maps an entity carries for a label-selector
	// join (owner decision D4, 2026-09-28: Kubernetes Service -> workload).
	Labels []DocLabels `json:"labels,omitempty"`
}

// DocLabels declares a label set: for each entity of kind From, Select
// (relative to the entity's node) reaches a map of scalar labels; Scope,
// a value expression on the document root, names the join scope
// (ScopeDefault when it is empty).
type DocLabels struct {
	From string `json:"from"`
	// Where filters the origin entity's node (a Pod's own labels).
	Where        *DocWhere `json:"where,omitempty"`
	Select       string    `json:"select"`
	Scope        string    `json:"scope,omitempty"`
	ScopeDefault string    `json:"scope_default,omitempty"`
}

// DocLabelSet is one entity's label map.
type DocLabelSet struct {
	Entity string
	Scope  string
	Labels map[string]string
	Line   int
}

// DocDetect is how a file is recognized. Basenames, globs and the content
// sniff are classification's (connector.ClassifyDialect, §5); RequireKeys
// is enforced here, per document: a document whose root lacks one yields
// no entities.
type DocDetect struct {
	Basenames    []string `json:"basenames,omitempty"`
	Globs        []string `json:"globs,omitempty"`
	RequireKeys  []string `json:"require_keys,omitempty"`
	ContentSniff *string  `json:"content_sniff,omitempty"`
}

// DocEntity selects symbols: every node Select reaches from the document
// root that satisfies Where becomes one symbol of Kind (a canonical kind,
// §2.3) and SourceKind, named by the Name value expression and signed by
// the optional Signature one.
type DocEntity struct {
	Select     string   `json:"select"`
	Where      DocWhere `json:"where,omitempty"`
	Kind       string   `json:"kind"`
	SourceKind string   `json:"source_kind"`
	Name       string   `json:"name"`
	Signature  string   `json:"signature,omitempty"`
	// Files, when set, limits the entity to files whose basename is listed:
	// a spec shared by several document types (Helm's Chart.yaml and
	// values.yaml) says which selector reads which.
	Files []string `json:"files,omitempty"`
	// IncludeNull keeps a node written with no value (`key:`, `key: null`):
	// selectors drop those by default, since a null is usually an absent
	// field, but a key a document declares is an entity whatever it holds
	// (a Helm values.yaml's `rollingUpdate: null`).
	IncludeNull bool `json:"include_null,omitempty"`
	// Qualify prefixes the name with each non-empty value (evaluated on
	// the entity's node), "/"-joined: a Kubernetes object is
	// "<kind>/<namespace>/<name>", so a Service and a Deployment sharing a
	// name keep distinct symbol keys (the incumbent k8sextract's naming).
	Qualify []string `json:"qualify,omitempty"`
}

// DocWhere filters selected nodes. Every set condition must hold.
type DocWhere struct {
	IsMap        *bool             `json:"is_map,omitempty"`
	IsSeq        *bool             `json:"is_seq,omitempty"`
	IsScalar     *bool             `json:"is_scalar,omitempty"`
	KeyIn        []string          `json:"key_in,omitempty"`
	NotKeyIn     []string          `json:"not_key_in,omitempty"`
	KeyPrefix    string            `json:"key_prefix,omitempty"`
	KeyNotPrefix string            `json:"key_not_prefix,omitempty"`
	HasKey       []string          `json:"has_key,omitempty"`
	Equals       map[string]string `json:"equals,omitempty"`
}

// DocRef is an intra-file reference: for every entity of kind From (the
// entity's kind, or its source_kind), Select (relative to the entity's
// node, "$") reaches nodes whose Value names an entity of ToKind; the name
// is recorded in the entity's Calls. Edge names the relation.
type DocRef struct {
	From   string `json:"from"`
	Select string `json:"select"`
	Value  string `json:"value"`
	ToKind string `json:"to_kind"`
	Edge   string `json:"edge"`
}

// DocCrossRef is a cross-file reference: Select from the document root
// (or, with From set, from each entity of that kind), Value the reference,
// resolved by the named strategy (repo-path, image-ref, module-source,
// service-name).
type DocCrossRef struct {
	// Files limits the reference to files whose basename is listed
	// (DocEntity.Files).
	Files []string `json:"files,omitempty"`
	From  string   `json:"from,omitempty"`
	// Where filters the origin (the entity, or the document root).
	Where   *DocWhere `json:"where,omitempty"`
	Select  string    `json:"select"`
	Value   string    `json:"value"`
	Resolve string    `json:"resolve"`
	Edge    string    `json:"edge"`
	// Scope and ScopeDefault name a label-selector's join scope, as
	// DocLabels does.
	Scope        string `json:"scope,omitempty"`
	ScopeDefault string `json:"scope_default,omitempty"`
}

// DocCrossFileRef is one resolved cross-file reference.
type DocCrossFileRef struct {
	// From is the referring entity's name ("" for a document-level one).
	From string
	Edge string
	// Raw is the value as written; Target the strategy's resolution: a
	// repo-relative path for repo-path and a relative module-source, the
	// value itself otherwise ("" when it cannot resolve, e.g. a path
	// climbing out of the repository).
	Raw, Target string
	Strategy    string
	Line        int
	// Scope and Labels: a label-selector's join scope and selector map
	// (Target is the map written canonically, "k=v,k2=v2").
	Scope  string
	Labels map[string]string
}

// DocResult is one file's extraction.
type DocResult struct {
	Nodes     []core.SymbolNode
	CrossFile []DocCrossFileRef
	// Labels is every declared label set (DocSpec.Labels).
	Labels []DocLabelSet
	// Roots names, in document order, the entities selected at a
	// document's root ("$") — what a file-level cross-file reference
	// resolves to.
	Roots []string
}

var docDecoders = map[string]bool{"yaml": true, "json": true, "jsonc": true, "toml": true, "xml": true}

var docResolveStrategies = map[string]bool{
	"repo-path": true, "image-ref": true, "module-source": true, "service-name": true,
	"repo-root": true, "label-selector": true,
}

// ParseDocSpec decodes a doc.json strictly (unknown fields refused).
func ParseDocSpec(b []byte) (DocSpec, error) {
	var s DocSpec
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return DocSpec{}, fmt.Errorf("doc spec: %w", err)
	}
	return s, nil
}

// DocExtractor is the document-path engine for one spec. It implements
// Extractor; ExtractDoc also returns the cross-file references.
type DocExtractor struct {
	spec     DocSpec
	entities []compiledEntity
	refs     []compiledRef
	cross    []compiledCross
	labels   []compiledLabels
}

type compiledEntity struct {
	DocEntity
	sel       docPath
	name, sig docValue
	qualify   []docValue
	equals    []docEquals
}

type compiledLabels struct {
	DocLabels
	sel    docPath
	scope  docValue
	equals []docEquals
}

type docEquals struct {
	p    docPath
	want string
}

type compiledRef struct {
	DocRef
	sel docPath
	val docValue
}

type compiledCross struct {
	DocCrossRef
	sel    docPath
	val    docValue
	scope  docValue
	equals []docEquals
}

func compileEquals(eqs map[string]string, where string) ([]docEquals, error) {
	keys := make([]string, 0, len(eqs))
	for k := range eqs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []docEquals
	for _, k := range keys {
		p, err := parseDocPath(k)
		if err != nil {
			return nil, fmt.Errorf("%s: where.equals %q: %w", where, k, err)
		}
		out = append(out, docEquals{p, eqs[k]})
	}
	return out, nil
}

func compileScope(expr string) (docValue, error) {
	if expr == "" {
		return docValue{}, nil
	}
	return parseDocValue(expr)
}

// NewDocExtractor compiles spec's selectors (§4's subset: `$`, `.key`,
// `.*`, `[*]`, `$key`, alternation `|`) and checks its decode, kinds and
// strategies; an error names the offending field.
func NewDocExtractor(spec DocSpec) (*DocExtractor, error) {
	if !docDecoders[spec.Decode] {
		return nil, fmt.Errorf("doc spec: decode %q is not yaml, json, jsonc, toml or xml", spec.Decode)
	}
	if spec.MultiDocument && spec.Decode != "yaml" {
		return nil, fmt.Errorf("doc spec: multi_document needs decode yaml")
	}
	if len(spec.Entities) == 0 {
		return nil, fmt.Errorf("doc spec: no entities")
	}
	x := &DocExtractor{spec: spec}
	kinds := map[string]bool{}
	for i, e := range spec.Entities {
		where := fmt.Sprintf("doc spec: entities[%d]", i)
		if !isCanonicalKind(e.Kind) {
			return nil, fmt.Errorf("%s: kind %q is not canonical (§2.3)", where, e.Kind)
		}
		if e.SourceKind == "" {
			return nil, fmt.Errorf("%s: empty source_kind", where)
		}
		ce := compiledEntity{DocEntity: e}
		var err error
		if ce.sel, err = parseDocPath(e.Select); err != nil {
			return nil, fmt.Errorf("%s: select: %w", where, err)
		}
		if ce.name, err = parseDocValue(e.Name); err != nil {
			return nil, fmt.Errorf("%s: name: %w", where, err)
		}
		if e.Signature != "" {
			if ce.sig, err = parseDocValue(e.Signature); err != nil {
				return nil, fmt.Errorf("%s: signature: %w", where, err)
			}
		}
		for j, q := range e.Qualify {
			qv, err := parseDocValue(q)
			if err != nil {
				return nil, fmt.Errorf("%s: qualify[%d]: %w", where, j, err)
			}
			ce.qualify = append(ce.qualify, qv)
		}
		if ce.equals, err = compileEquals(e.Where.Equals, where); err != nil {
			return nil, err
		}
		kinds[e.Kind], kinds[e.SourceKind] = true, true
		x.entities = append(x.entities, ce)
	}
	for i, r := range spec.References {
		where := fmt.Sprintf("doc spec: references[%d]", i)
		if !kinds[r.From] || !kinds[r.ToKind] {
			return nil, fmt.Errorf("%s: from %q / to_kind %q names no entity kind", where, r.From, r.ToKind)
		}
		cr := compiledRef{DocRef: r}
		var err error
		if cr.sel, err = parseDocPath(r.Select); err != nil {
			return nil, fmt.Errorf("%s: select: %w", where, err)
		}
		if cr.val, err = parseDocValue(r.Value); err != nil {
			return nil, fmt.Errorf("%s: value: %w", where, err)
		}
		x.refs = append(x.refs, cr)
	}
	for i, c := range spec.CrossFile {
		where := fmt.Sprintf("doc spec: cross_file[%d]", i)
		if !docResolveStrategies[c.Resolve] {
			return nil, fmt.Errorf("%s: resolve %q is not repo-path, repo-root, image-ref, module-source, service-name or label-selector", where, c.Resolve)
		}
		if c.From != "" && !kinds[c.From] {
			return nil, fmt.Errorf("%s: from %q names no entity kind", where, c.From)
		}
		cc := compiledCross{DocCrossRef: c}
		var err error
		if cc.sel, err = parseDocPath(c.Select); err != nil {
			return nil, fmt.Errorf("%s: select: %w", where, err)
		}
		if cc.val, err = parseDocValue(c.Value); err != nil {
			return nil, fmt.Errorf("%s: value: %w", where, err)
		}
		if cc.scope, err = compileScope(c.Scope); err != nil {
			return nil, fmt.Errorf("%s: scope: %w", where, err)
		}
		if c.Where != nil {
			if cc.equals, err = compileEquals(c.Where.Equals, where); err != nil {
				return nil, err
			}
		}
		x.cross = append(x.cross, cc)
	}
	for i, l := range spec.Labels {
		where := fmt.Sprintf("doc spec: labels[%d]", i)
		if !kinds[l.From] {
			return nil, fmt.Errorf("%s: from %q names no entity kind", where, l.From)
		}
		cl := compiledLabels{DocLabels: l}
		var err error
		if cl.sel, err = parseDocPath(l.Select); err != nil {
			return nil, fmt.Errorf("%s: select: %w", where, err)
		}
		if cl.scope, err = compileScope(l.Scope); err != nil {
			return nil, fmt.Errorf("%s: scope: %w", where, err)
		}
		if l.Where != nil {
			if cl.equals, err = compileEquals(l.Where.Equals, where); err != nil {
				return nil, err
			}
		}
		x.labels = append(x.labels, cl)
	}
	return x, nil
}

// scopeOf evaluates a join scope on a document root.
func scopeOf(v docValue, def string, root *docNode) string {
	if s := v.eval(docMatch{node: root}); s != "" {
		return s
	}
	return def
}

// labelMap is a map node's scalar entries, and its canonical "k=v,…"
// form; ok is false for anything else or an empty map.
func labelMap(n *docNode) (map[string]string, string, bool) {
	if n == nil || n.kind != docMap || len(n.keys) == 0 {
		return nil, "", false
	}
	m := map[string]string{}
	for i, k := range n.keys {
		if n.vals[i].kind != docScalar {
			return nil, "", false
		}
		m[k] = n.vals[i].scalar
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + m[k]
	}
	return m, strings.Join(parts, ","), true
}

// Extract implements Extractor: the entities, with intra-file references
// in Calls. Malformed input yields no nodes and no error (as the bespoke
// config extractors do).
func (x *DocExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := x.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// entityHit is one selected entity before it becomes a SymbolNode.
type entityHit struct {
	ent  *compiledEntity
	node *docNode
	name string
	sig  string
}

// ExtractDoc decodes src and walks every document.
func (x *DocExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	docs, err := decodeDoc(x.spec.Decode, src, x.spec.MultiDocument)
	if err != nil || len(docs) == 0 {
		return DocResult{}, nil
	}
	pkg := pkgFromPath(relPath)
	base := path.Base(relPath)
	var res DocResult
	for _, root := range docs {
		if !hasKeys(root, x.spec.Detect.RequireKeys) {
			continue
		}
		var hits []entityHit
		for i := range x.entities {
			ce := &x.entities[i]
			if len(ce.Files) > 0 && !containsStr(ce.Files, base) {
				continue
			}
			for _, m := range ce.sel.evalKeep(root, "", ce.IncludeNull) {
				if !ce.matches(m) {
					continue
				}
				name := ce.name.eval(m)
				if name == "" {
					continue
				}
				for j := len(ce.qualify) - 1; j >= 0; j-- {
					if q := ce.qualify[j].eval(m); q != "" {
						name = q + "/" + name
					}
				}
				if ce.sel.isRoot() {
					res.Roots = append(res.Roots, name)
				}
				h := entityHit{ent: ce, node: m.node, name: name}
				if ce.Signature != "" {
					h.sig = ce.sig.eval(m)
				}
				hits = append(hits, h)
			}
		}
		for _, h := range hits {
			calls := map[string]bool{}
			for i := range x.refs {
				cr := &x.refs[i]
				if cr.From != h.ent.Kind && cr.From != h.ent.SourceKind {
					continue
				}
				for _, m := range cr.sel.eval(h.node, "") {
					if v := cr.val.eval(m); v != "" {
						calls[v] = true
					}
				}
			}
			res.Nodes = append(res.Nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      h.name,
				Kind:      core.SourceKind(h.ent.SourceKind),
				FilePath:  relPath,
				Line:      h.node.line,
				EndLine:   h.node.endLine,
				Signature: h.sig,
				Calls:     sortedSet(calls),
				Tier:      core.TierVerified,
			})
		}
		for i := range x.cross {
			cc := &x.cross[i]
			if len(cc.Files) > 0 && !containsStr(cc.Files, base) {
				continue
			}
			type origin struct {
				from string
				node *docNode
			}
			origins := []origin{{"", root}}
			if cc.From != "" {
				origins = origins[:0]
				for _, h := range hits {
					if h.ent.Kind == cc.From || h.ent.SourceKind == cc.From {
						origins = append(origins, origin{h.name, h.node})
					}
				}
			}
			for _, o := range origins {
				if cc.Where != nil && !whereMatches(*cc.Where, cc.equals, docMatch{node: o.node}) {
					continue
				}
				for _, m := range cc.sel.eval(o.node, "") {
					if cc.Resolve == "label-selector" {
						labels, canon, ok := labelMap(m.node)
						if !ok {
							continue
						}
						res.CrossFile = append(res.CrossFile, DocCrossFileRef{
							From: o.from, Edge: cc.Edge, Raw: canon, Target: canon, Strategy: cc.Resolve,
							Line: m.node.line, Scope: scopeOf(cc.scope, cc.ScopeDefault, root), Labels: labels,
						})
						continue
					}
					raw := cc.val.eval(m)
					if raw == "" {
						continue
					}
					res.CrossFile = append(res.CrossFile, DocCrossFileRef{
						From: o.from, Edge: cc.Edge, Raw: raw, Strategy: cc.Resolve,
						Target: resolveDocRef(cc.Resolve, relPath, raw), Line: m.node.line,
					})
				}
			}
		}
		for i := range x.labels {
			cl := &x.labels[i]
			for _, h := range hits {
				if h.ent.Kind != cl.From && h.ent.SourceKind != cl.From {
					continue
				}
				if cl.Where != nil && !whereMatches(*cl.Where, cl.equals, docMatch{node: h.node}) {
					continue
				}
				for _, m := range cl.sel.eval(h.node, "") {
					if labels, _, ok := labelMap(m.node); ok {
						res.Labels = append(res.Labels, DocLabelSet{Entity: h.name, Scope: scopeOf(cl.scope, cl.ScopeDefault, root), Labels: labels, Line: m.node.line})
					}
				}
			}
		}
	}
	sort.SliceStable(res.Nodes, func(i, j int) bool {
		a, b := res.Nodes[i], res.Nodes[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Kind < b.Kind
	})
	sort.SliceStable(res.CrossFile, func(i, j int) bool {
		a, b := res.CrossFile[i], res.CrossFile[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Raw < b.Raw
	})
	return res, nil
}

func sortedSet(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func hasKeys(n *docNode, keys []string) bool {
	for _, k := range keys {
		if n.get(k) == nil {
			return false
		}
	}
	return true
}

func (ce *compiledEntity) matches(m docMatch) bool { return whereMatches(ce.Where, ce.equals, m) }

// whereMatches applies a where filter (equals compiled) to a match.
func whereMatches(w DocWhere, equals []docEquals, m docMatch) bool {
	n := m.node
	if w.IsMap != nil && (n.kind == docMap) != *w.IsMap {
		return false
	}
	if w.IsSeq != nil && (n.kind == docSeq) != *w.IsSeq {
		return false
	}
	if w.IsScalar != nil && (n.kind == docScalar) != *w.IsScalar {
		return false
	}
	if len(w.KeyIn) > 0 && !containsStr(w.KeyIn, m.key) {
		return false
	}
	if containsStr(w.NotKeyIn, m.key) {
		return false
	}
	if w.KeyPrefix != "" && !strings.HasPrefix(m.key, w.KeyPrefix) {
		return false
	}
	if w.KeyNotPrefix != "" && strings.HasPrefix(m.key, w.KeyNotPrefix) {
		return false
	}
	for _, k := range w.HasKey {
		if n.get(k) == nil {
			return false
		}
	}
	for _, eq := range equals {
		ok := false
		for _, v := range eq.p.eval(n, m.key) {
			if v.node.kind == docScalar && v.node.scalar == eq.want {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// resolveDocRef applies a cross-file strategy (§4, §6). repo-path: a
// leading "/" is the repository root, anything else is relative to the
// referring file's directory; a path that climbs out of the repository
// does not resolve. module-source: a "./" or "../" source resolves like a
// relative repo-path, any other (a registry or VCS address) is kept as
// written. image-ref and service-name are names, kept as written.
func resolveDocRef(strategy, relPath, raw string) string {
	switch strategy {
	case "repo-path":
		return repoPath(relPath, raw)
	case "module-source":
		if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
			return repoPath(relPath, raw)
		}
		return raw
	case "repo-root":
		// GitHub Actions' `uses: ./path`: relative to the repository
		// root, whatever directory the referring file sits in (D4).
		if raw == "." || raw == "./" || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
			p := path.Clean(raw)
			if p == ".." || strings.HasPrefix(p, "../") {
				return ""
			}
			return p
		}
		return raw
	}
	return raw
}

func repoPath(relPath, raw string) string {
	var p string
	if strings.HasPrefix(raw, "/") {
		p = path.Clean(strings.TrimPrefix(raw, "/"))
	} else {
		p = path.Clean(path.Join(path.Dir(relPath), raw))
	}
	if p == ".." || strings.HasPrefix(p, "../") || p == "." {
		return ""
	}
	return p
}

// ── the decoded tree ──────────────────────────────────────────────────────

type docKind int

const (
	docNull docKind = iota
	docScalar
	docMap
	docSeq
)

// docNode is one decoded value, whatever the syntax: a scalar, an ordered
// map, or a sequence, with its 1-based first and last line (0: unknown).
type docNode struct {
	kind          docKind
	scalar        string
	keys          []string
	vals          []*docNode
	items         []*docNode
	line, endLine int
	// xmlRepeat marks a sequence the XML decoder built from repeated
	// sibling elements.
	xmlRepeat bool
}

func (n *docNode) get(k string) *docNode {
	if n == nil || n.kind != docMap {
		return nil
	}
	for i, key := range n.keys {
		if key == k {
			return n.vals[i]
		}
	}
	return nil
}

// set adds or replaces a map entry, keeping first-seen order.
func (n *docNode) set(k string, v *docNode) {
	for i, key := range n.keys {
		if key == k {
			n.vals[i] = v
			return
		}
	}
	n.keys = append(n.keys, k)
	n.vals = append(n.vals, v)
}

// fixEndLines sets every node's endLine to the last line in its subtree
// (at least its own line).
func fixEndLines(n *docNode) int {
	if n == nil {
		return 0
	}
	end := n.line
	if n.endLine > end {
		end = n.endLine
	}
	for _, v := range n.vals {
		if e := fixEndLines(v); e > end {
			end = e
		}
	}
	for _, it := range n.items {
		if e := fixEndLines(it); e > end {
			end = e
		}
	}
	n.endLine = end
	return end
}

// ── selectors ─────────────────────────────────────────────────────────────

type docStepKind int

const (
	stepKey docStepKind = iota
	stepAllValues
	stepAllItems
)

type docStep struct {
	kind docStepKind
	key  string
}

// docPath is a compiled selector: `$` (the context node) followed by
// `.key`, `.*` (every value of a map, with its key) and `[*]` (every item
// of a sequence; any other non-null node is its own single item, so a
// field written as either one value or a list reads the same, and a
// single repeated XML element reads as a one-item list).
type docPath struct{ steps []docStep }

type docMatch struct {
	node *docNode
	key  string // the map key the node was reached by ("" for an item)
	// path is the dotted keys from the selection's start to this node
	// ("image.repository"); a sequence item adds none. `$path` reads it.
	path string
}

// joinDocPath appends key to a dotted path.
func joinDocPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

var docKeyRe = regexp.MustCompile(`^[^.\[\]|*\s$]+`)

func parseDocPath(s string) (docPath, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return docPath{}, fmt.Errorf("empty selector")
	case strings.HasPrefix(s, "$"):
		s = s[1:]
	case strings.HasPrefix(s, "."):
	default:
		return docPath{}, fmt.Errorf("selector %q must start with $ or .", s)
	}
	var p docPath
	for s != "" {
		switch {
		case strings.HasPrefix(s, "[*]"):
			p.steps = append(p.steps, docStep{kind: stepAllItems})
			s = s[3:]
		case strings.HasPrefix(s, ".*"):
			p.steps = append(p.steps, docStep{kind: stepAllValues})
			s = s[2:]
		case strings.HasPrefix(s, "."):
			k := docKeyRe.FindString(s[1:])
			if k == "" {
				return docPath{}, fmt.Errorf("selector: bad key at %q", s)
			}
			p.steps = append(p.steps, docStep{kind: stepKey, key: k})
			s = s[1+len(k):]
		default:
			return docPath{}, fmt.Errorf("selector: unexpected %q (the subset is $ .key .* [*])", s)
		}
	}
	return p, nil
}

// isRoot reports whether the path selects the document root itself ("$").
func (p docPath) isRoot() bool { return len(p.steps) == 0 }

func (p docPath) eval(ctx *docNode, ctxKey string) []docMatch {
	return p.evalKeep(ctx, ctxKey, false)
}

// evalKeep is eval that, with keepNull, also returns the null nodes the
// selector reaches (DocEntity.IncludeNull).
func (p docPath) evalKeep(ctx *docNode, ctxKey string, keepNull bool) []docMatch {
	cur := []docMatch{{node: ctx, key: ctxKey}}
	for _, st := range p.steps {
		var next []docMatch
		for _, m := range cur {
			n := m.node
			switch st.kind {
			case stepKey:
				if v := n.get(st.key); v != nil {
					next = append(next, docMatch{node: v, key: st.key, path: joinDocPath(m.path, st.key)})
				}
			case stepAllValues:
				if n != nil && n.kind == docMap {
					for i, k := range n.keys {
						next = append(next, docMatch{node: n.vals[i], key: k, path: joinDocPath(m.path, k)})
					}
				}
			case stepAllItems:
				switch {
				case n == nil || n.kind == docNull:
				case n.kind == docSeq:
					for _, it := range n.items {
						next = append(next, docMatch{node: it, path: m.path})
					}
				default:
					next = append(next, docMatch{node: n, key: m.key, path: m.path})
				}
			}
		}
		cur = next
	}
	var out []docMatch
	for _, m := range cur {
		if m.node != nil && (keepNull || m.node.kind != docNull) {
			out = append(out, m)
		}
	}
	return out
}

// docValue is a value expression: alternatives separated by `|`, each a
// selector relative to the match (`$` is the matched node), `$key` (the
// key it was reached by) or `$path` (every key from the selection's start
// down to the node, dot-joined — a nested map's "image.repository"). The
// first alternative that reaches a non-empty scalar is the value.
type docValue struct{ alts []docAlt }

type docAlt struct {
	key  bool
	path bool
	p    docPath
}

func parseDocValue(s string) (docValue, error) {
	var v docValue
	for _, a := range strings.Split(s, "|") {
		a = strings.TrimSpace(a)
		if a == "$key" {
			v.alts = append(v.alts, docAlt{key: true})
			continue
		}
		if a == "$path" {
			v.alts = append(v.alts, docAlt{path: true})
			continue
		}
		p, err := parseDocPath(a)
		if err != nil {
			return docValue{}, err
		}
		v.alts = append(v.alts, docAlt{p: p})
	}
	return v, nil
}

func (v docValue) eval(m docMatch) string {
	for _, a := range v.alts {
		if a.key {
			if m.key != "" {
				return m.key
			}
			continue
		}
		if a.path {
			if m.path != "" {
				return m.path
			}
			continue
		}
		for _, r := range a.p.eval(m.node, m.key) {
			if r.node.kind == docScalar && r.node.scalar != "" {
				return r.node.scalar
			}
		}
	}
	return ""
}
