package extractor

// docengine.go — the document-path engine (LB-F8, docs/design/
// language-breadth.md §4, ADR-048). Tool formats whose syntax is YAML,
// JSON/JSONC, TOML or XML are a schema problem, not a parsing problem: a
// doc.json names the decode, the entities to select, and the references
// between them, and this engine walks the decoded tree. It generalizes the
// walk the six bespoke config extractors (GitHub Actions, Compose,
// Kubernetes, Kustomize, Helm values, OpenAPI) each hand-coded (P20):
// TestDocExtractorParity reproduces GitHub Actions' symbols from a doc.json
// alone (the Kustomize, Compose and OpenAPI extractors were retired by their
// dialects, each at lang-parity).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// DocSpec is a doc.json (§4).
type DocSpec struct {
	// Decode is "yaml", "json", "jsonc", "toml", "xml", "ini" or "compile-commands" (JSON, with the
	// include directories, macros and output of each entry's flags added as keys: docdecode_compdb.go).
	Decode string `json:"decode"`
	// DecodeFiles overrides Decode for the basenames it lists (LB-F125): one
	// dialect reads a YAML file and an INI file (Conan's conandata.yml and
	// conanfile.txt).
	DecodeFiles map[string]string `json:"decode_files,omitempty"`
	// MultiDocument: a YAML stream's documents are walked one by one
	// (Kubernetes manifests).
	MultiDocument bool `json:"multi_document,omitempty"`
	// YAMLTags names how a YAML local tag (`!Name value`) is read: unset,
	// it is dropped (the node reads as its untagged value, GitLab's
	// `!reference` a plain list); "cloudformation" reads a short-form
	// intrinsic as its long form (`!Ref X` is {Ref: X}, `!Sub s` is
	// {Fn::Sub: s}, `!GetAtt A.B` is {Fn::GetAtt: [A, B]}, as is a long-form
	// GetAtt written `A.B`), cfn-lint's rule (LB-Q-cloudformation).
	YAMLTags   string        `json:"yaml_tags,omitempty"`
	Detect     DocDetect     `json:"detect"`
	Entities   []DocEntity   `json:"entities"`
	References []DocRef      `json:"references,omitempty"`
	CrossFile  []DocCrossRef `json:"cross_file,omitempty"`
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

// DocDetect is how a file is recognized. Basenames and globs are
// classification's (connector.ClassifyDialect, §5). ContentSniff is a
// regular expression over the whole file, for a dialect no basename, glob or
// extension claims (a CloudFormation template is a .yaml, .yml or .json
// whose content says so): fusion's routing asks DetectsContent. RequireKeys
// is enforced here, per document: a document whose root lacks one yields no
// entities.
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
	// DeclaredOnly records a value only when it names an entity of ToKind
	// declared in the same document: a CloudFormation Ref to a pseudo
	// parameter (AWS::Region) or to a name the template never declares is
	// not a reference to one of its entities.
	DeclaredOnly bool `json:"declared_only,omitempty"`
	// Pointer reads the value as a local JSON Pointer (a fragment-only
	// `$ref`, "#/components/schemas/Pet"): the first Prefix that begins it
	// names the target kind, and the pointer's first segment after the
	// prefix, unescaped (`~1` is "/", `~0` is "~", then percent-decoding),
	// is the name recorded. A value no prefix begins (a file part, a URL, a
	// pointer elsewhere in the document) names nothing. With Pointer set,
	// ToKind is each prefix's, and From may list several kinds separated by
	// `|` (one rule serves every entity that can carry a `$ref`).
	Pointer []DocPointer `json:"pointer,omitempty"`
	// Scan (cloud-policy dialects, wave pc) reads every string under each
	// selected node, in document order, and records the first capture group
	// of each match of this regular expression as a value: an Azure Policy
	// parameter named inside `[parameters('x')]` wherever the rule writes it
	// (Value is then unused; "$" by convention).
	Scan string `json:"scan,omitempty"`
	// Pattern (LB-F145, wave iac) reads values out of strings: an RE2 with
	// exactly one capture group, applied to every scalar string Select
	// reaches — a selected map or sequence is walked for its strings (map
	// keys included) in document order, never into another entity's node (a
	// string inside a nested entity is that entity's) — and every non-overlapping match is
	// one value: its group, read as a scalar by Value (`$` is the group
	// itself; `fmt:[{$}]` wraps it); a value naming the referring entity
	// itself is none. An ARM template's
	// `[concat(parameters('a'), variables('b'))]` names a parameter and a
	// variable; a Serverless `${self:custom.x}` a custom key. Unset, Value
	// reads the selected node whole, as before.
	Pattern string `json:"pattern,omitempty"`
}

// DocPointer is one local-pointer prefix of a DocRef and the entity kind a
// pointer beginning with it names.
type DocPointer struct {
	Prefix string `json:"prefix"`
	ToKind string `json:"to_kind"`
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
	// TargetMap (resource-type only, wave pc) maps the value as written to
	// its Target, "" for one the map does not name (an unmapped type; an
	// empty map maps nothing): a Cloud Custodian `resource: aws.s3` to its
	// CloudFormation type. Unset, Target is the value as written.
	TargetMap map[string]string `json:"target_map,omitempty"`
	// Pattern reads the references out of strings, as DocRef.Pattern does
	// (a Serverless `${file(./x.yml)}`, an ARM templateLink.uri relative to
	// the template's own link); not with label-selector, which reads a map.
	Pattern string `json:"pattern,omitempty"`
	// Via reads a reference through a declared entity (LB-F146): each value
	// Pattern gives names an entity of Via.Kind declared in the document,
	// and Via.Pattern, applied to that entity's strings as Pattern is, gives
	// the reference (an ARM deployment's templateLink.uri written as
	// `[variables('netUri')]`, the variable holding the uri). A name no such
	// entity declares gives nothing.
	Via *DocVia `json:"via,omitempty"`
}

// DocVia is a cross-file reference's indirection (DocCrossRef.Via).
type DocVia struct {
	Kind    string `json:"kind"`
	Pattern string `json:"pattern"`
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

var docDecoders = map[string]bool{"yaml": true, "json": true, "jsonc": true, "toml": true, "xml": true, "compile-commands": true}

// docYAMLTagSchemes are the yaml_tags schemes (DocSpec.YAMLTags).
var docYAMLTagSchemes = map[string]bool{yamlTagsCloudFormation: true}

var docResolveStrategies = map[string]bool{
	"repo-path": true, "image-ref": true, "module-source": true, "service-name": true,
	"repo-root": true, "label-selector": true, "route-host": true,
	// policy_ref strategies (wave pc): the value is kept as written, and
	// fusion's policy-reference resolver says what it names.
	"aws-arn": true, "resource-type": true,
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
	sniff    *regexp.Regexp // detect.content_sniff, nil when the spec has none
	// patterned: some reference or cross-file reference has a pattern.
	patterned bool
}

// DetectsContent reports whether src is a file this spec's
// detect.content_sniff claims — the content classification of a dialect no
// basename, glob or extension claims (design §4, §5). A spec without a
// sniff detects nothing.
func (x *DocExtractor) DetectsContent(src []byte) bool {
	return x.sniff != nil && x.sniff.Match(src)
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
	froms []string // From split on `|`
	sel   docPath
	val   docValue
	scan  *regexp.Regexp // DocRef.Scan, nil when unset
	re    *regexp.Regexp // DocRef.Pattern, nil when unset
}

type compiledCross struct {
	DocCrossRef
	sel    docPath
	val    docValue
	scope  docValue
	equals []docEquals
	re     *regexp.Regexp // DocCrossRef.Pattern, nil when unset
	viaRe  *regexp.Regexp // DocCrossRef.Via.Pattern, nil when Via is unset
}

// target is a cross-file value's Target: the strategy's resolution, or, with
// TargetMap set (resource-type only), the map's entry ("" for a value the map
// does not name).
func (cc *compiledCross) target(relPath, raw string) string {
	if cc.TargetMap != nil {
		return cc.TargetMap[raw]
	}
	return resolveDocRef(cc.Resolve, relPath, raw)
}

// compilePattern compiles a reference's pattern (DocRef.Pattern): RE2 with
// exactly one capture group, or nil when unset.
func compilePattern(p string) (*regexp.Regexp, error) {
	if p == "" {
		return nil, nil
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, err
	}
	if n := re.NumSubexp(); n != 1 {
		return nil, fmt.Errorf("%q has %d capture groups; it needs exactly one capture group", p, n)
	}
	return re, nil
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
// `.*`, `[*]`, `[N]`, `..key`, `$key`, alternation `|`) and its content
// sniff, and checks its decode, tag scheme, kinds and strategies; an error
// names the offending field.
func NewDocExtractor(spec DocSpec) (*DocExtractor, error) {
	if !docDecoders[spec.Decode] {
		return nil, fmt.Errorf("doc spec: decode %q is not yaml, json, jsonc, toml or xml", spec.Decode)
	}
	for base, f := range spec.DecodeFiles {
		if !docDecoders[f] {
			return nil, fmt.Errorf("doc spec: decode_files[%q]: %q is not a decode", base, f)
		}
	}
	if spec.MultiDocument && spec.Decode != "yaml" {
		return nil, fmt.Errorf("doc spec: multi_document needs decode yaml")
	}
	if spec.YAMLTags != "" {
		if spec.Decode != "yaml" {
			return nil, fmt.Errorf("doc spec: yaml_tags needs decode yaml")
		}
		if !docYAMLTagSchemes[spec.YAMLTags] {
			return nil, fmt.Errorf("doc spec: yaml_tags %q is not cloudformation", spec.YAMLTags)
		}
	}
	if len(spec.Entities) == 0 {
		return nil, fmt.Errorf("doc spec: no entities")
	}
	x := &DocExtractor{spec: spec}
	if cs := spec.Detect.ContentSniff; cs != nil {
		re, err := regexp.Compile(*cs)
		if err != nil {
			return nil, fmt.Errorf("doc spec: detect.content_sniff: %w", err)
		}
		x.sniff = re
	}
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
		cr := compiledRef{DocRef: r, froms: strings.Split(r.From, "|")}
		for _, f := range cr.froms {
			if !kinds[f] {
				return nil, fmt.Errorf("%s: from %q / to_kind %q names no entity kind", where, r.From, r.ToKind)
			}
		}
		if len(r.Pointer) == 0 {
			if !kinds[r.ToKind] {
				return nil, fmt.Errorf("%s: from %q / to_kind %q names no entity kind", where, r.From, r.ToKind)
			}
		} else {
			if r.ToKind != "" {
				return nil, fmt.Errorf("%s: pointer sets each prefix's kind; to_kind %q must be empty", where, r.ToKind)
			}
			for j, pt := range r.Pointer {
				if pt.Prefix == "" || !kinds[pt.ToKind] {
					return nil, fmt.Errorf("%s: pointer[%d]: prefix %q / to_kind %q names no entity kind", where, j, pt.Prefix, pt.ToKind)
				}
			}
		}
		var err error
		if cr.sel, err = parseDocPath(r.Select); err != nil {
			return nil, fmt.Errorf("%s: select: %w", where, err)
		}
		if cr.val, err = parseDocValue(r.Value); err != nil {
			return nil, fmt.Errorf("%s: value: %w", where, err)
		}
		if r.Scan != "" {
			re, err := regexp.Compile(r.Scan)
			if err != nil {
				return nil, fmt.Errorf("%s: scan: %w", where, err)
			}
			if re.NumSubexp() < 1 {
				return nil, fmt.Errorf("%s: scan %q has no capture group", where, r.Scan)
			}
			cr.scan = re
		}
		if cr.re, err = compilePattern(r.Pattern); err != nil {
			return nil, fmt.Errorf("%s: pattern: %w", where, err)
		}
		x.patterned = x.patterned || cr.re != nil
		x.refs = append(x.refs, cr)
	}
	for i, c := range spec.CrossFile {
		where := fmt.Sprintf("doc spec: cross_file[%d]", i)
		if !docResolveStrategies[c.Resolve] {
			return nil, fmt.Errorf("%s: resolve %q is not repo-path, repo-root, image-ref, module-source, service-name, label-selector, route-host, aws-arn or resource-type", where, c.Resolve)
		}
		if c.From != "" && !kinds[c.From] {
			return nil, fmt.Errorf("%s: from %q names no entity kind", where, c.From)
		}
		if c.TargetMap != nil && c.Resolve != "resource-type" {
			return nil, fmt.Errorf("%s: target_map belongs to resolve resource-type, not %q", where, c.Resolve)
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
		if c.Pattern != "" && c.Resolve == "label-selector" {
			return nil, fmt.Errorf("%s: pattern reads strings; label-selector reads a map", where)
		}
		if cc.re, err = compilePattern(c.Pattern); err != nil {
			return nil, fmt.Errorf("%s: pattern: %w", where, err)
		}
		if c.Via != nil {
			if cc.re == nil {
				return nil, fmt.Errorf("%s: via needs a pattern naming the entity", where)
			}
			if !kinds[c.Via.Kind] {
				return nil, fmt.Errorf("%s: via kind %q names no entity kind", where, c.Via.Kind)
			}
			if cc.viaRe, err = compilePattern(c.Via.Pattern); err != nil || cc.viaRe == nil {
				if err == nil {
					err = fmt.Errorf("empty")
				}
				return nil, fmt.Errorf("%s: via pattern: %w", where, err)
			}
		}
		x.patterned = x.patterned || cc.re != nil
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
	format := x.spec.Decode
	if f, ok := x.spec.DecodeFiles[path.Base(relPath)]; ok {
		format = f
	}
	docs, err := decodeDoc(format, src, x.spec.MultiDocument, x.spec.YAMLTags)
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
				m.file = base
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
		// declared: the names this document declares, per entity kind and
		// source kind (a declared_only reference's filter).
		declared := map[string]map[string]bool{}
		// declaredNode: the first node declaring each name, per kind (a
		// cross-file reference's via).
		declaredNode := map[string]map[string]*docNode{}
		// entityNodes: the nodes the entities were selected at, which a
		// pattern's walk does not enter from another entity (DocRef.Pattern).
		var entityNodes map[*docNode]bool
		if x.patterned {
			entityNodes = make(map[*docNode]bool, len(hits))
			for _, h := range hits {
				entityNodes[h.node] = true
			}
		}
		for _, h := range hits {
			for _, k := range []string{h.ent.Kind, h.ent.SourceKind} {
				if declared[k] == nil {
					declared[k] = map[string]bool{}
					declaredNode[k] = map[string]*docNode{}
				}
				declared[k][h.name] = true
				if declaredNode[k][h.name] == nil {
					declaredNode[k][h.name] = h.node
				}
			}
		}
		for _, h := range hits {
			calls := map[string]bool{}
			for i := range x.refs {
				cr := &x.refs[i]
				if !containsStr(cr.froms, h.ent.Kind) && !containsStr(cr.froms, h.ent.SourceKind) {
					continue
				}
				for _, m := range cr.sel.eval(h.node, "") {
					m.file = base
					if cr.scan != nil {
						for _, v := range scanStrings(m.node, cr.scan, nil) {
							if !cr.DeclaredOnly || declared[cr.ToKind][v] {
								calls[v] = true
							}
						}
						continue
					}
					for _, lv := range matchValues(cr.re, cr.val, m, h.node, entityNodes) {
						v, toKind := lv.v, cr.ToKind
						if len(cr.Pointer) > 0 {
							v, toKind = localPointerTarget(cr.Pointer, v)
						}
						if v == "" || (cr.DeclaredOnly && !declared[toKind][v]) {
							continue
						}
						if cr.re != nil && v == h.name && (toKind == h.ent.Kind || toKind == h.ent.SourceKind) {
							// A pattern reads every string of the entity: one
							// naming the entity itself (an ARM resource's own
							// resourceId in its properties) is no reference.
							continue
						}
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
					m.file = base
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
					for _, lv := range matchValues(cc.re, cc.val, m, o.node, entityNodes) {
						if lv.v == "" {
							continue
						}
						if cc.Via != nil {
							// the value names an entity: its strings hold the reference
							vn := declaredNode[cc.Via.Kind][lv.v]
							if vn == nil {
								continue
							}
							for _, vv := range matchValues(cc.viaRe, docValue{alts: []docAlt{{}}}, docMatch{node: vn, file: base}, vn, entityNodes) {
								res.CrossFile = append(res.CrossFile, DocCrossFileRef{
									From: o.from, Edge: cc.Edge, Raw: vv.v, Strategy: cc.Resolve,
									Target: cc.target(relPath, vv.v), Line: lv.line,
								})
							}
							continue
						}
						res.CrossFile = append(res.CrossFile, DocCrossFileRef{
							From: o.from, Edge: cc.Edge, Raw: lv.v, Strategy: cc.Resolve,
							Target: cc.target(relPath, lv.v), Line: lv.line,
						})
					}
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

// scanStrings appends, in document order, the first capture group of every
// match of re in every scalar string under n (DocRef.Scan).
func scanStrings(n *docNode, re *regexp.Regexp, out []string) []string {
	if n == nil {
		return out
	}
	switch n.kind {
	case docScalar:
		for _, m := range re.FindAllStringSubmatch(n.scalar, -1) {
			if m[1] != "" {
				out = append(out, m[1])
			}
		}
	case docMap:
		for _, v := range n.vals {
			out = scanStrings(v, re, out)
		}
	case docSeq:
		for _, it := range n.items {
			out = scanStrings(it, re, out)
		}
	}
	return out
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

// lineValue is one reference value and the line of the string it was read
// from.
type lineValue struct {
	v    string
	line int
}

// matchValues is what one selected match gives a reference: without a
// pattern, its value expression's string (one value, "" for none); with
// one, every non-overlapping match of the pattern in the match's strings —
// the match itself when it is a scalar, else every map key and scalar under
// it in document order, never entering an entity node other than origin's
// (entityNodes) — each match's group read by the value expression as a
// scalar of its own.
func matchValues(re *regexp.Regexp, val docValue, m docMatch, origin *docNode, entityNodes map[*docNode]bool) []lineValue {
	if re == nil {
		return []lineValue{{val.eval(m), m.node.line}}
	}
	var out []lineValue
	scan := func(text string, at *docNode) {
		for _, g := range re.FindAllStringSubmatch(text, -1) {
			if g[1] == "" {
				continue
			}
			cm := m
			cm.node = &docNode{kind: docScalar, scalar: g[1], line: at.line, endLine: at.endLine}
			if v := val.eval(cm); v != "" {
				out = append(out, lineValue{v, at.line})
			}
		}
	}
	var walk func(n *docNode)
	walk = func(n *docNode) {
		if n == nil || (n != origin && entityNodes[n]) {
			return
		}
		switch n.kind {
		case docScalar:
			scan(n.scalar, n)
		case docMap:
			for i, v := range n.vals {
				if v != origin && entityNodes[v] {
					continue // the entry is another entity's, its key too
				}
				scan(n.keys[i], v) // a key is a string too (an ARM tag name `[parameters('n')]`)
				walk(v)
			}
		case docSeq:
			for _, it := range n.items {
				walk(it)
			}
		}
	}
	walk(m.node)
	return out
}

// localPointerTarget reads value as a local JSON Pointer against pointers
// (DocRef.Pointer): the name its first segment after the matching prefix
// gives and that prefix's kind, or "" when no prefix begins it.
func localPointerTarget(pointers []DocPointer, value string) (name, toKind string) {
	for _, pt := range pointers {
		rest, ok := strings.CutPrefix(value, pt.Prefix)
		if !ok {
			continue
		}
		seg, _, _ := strings.Cut(rest, "/")
		seg = strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")
		if dec, err := url.PathUnescape(seg); err == nil {
			seg = dec
		}
		return seg, pt.ToKind
	}
	return "", ""
}

// fileOfRef is a `$ref` value's file part for the `@file` value transform:
// the text before a fragment, "" for a fragment-only (local) reference and
// for a URL (a scheme://, not a file of the repository).
func fileOfRef(v string) string {
	f, _, _ := strings.Cut(v, "#")
	if f == "" || docURLRe.MatchString(f) {
		return ""
	}
	return f
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
// leading "/" (or a backslash) is the repository root, anything else is
// relative to the referring file's directory, and either separator is one; a
// path that climbs out of the repository does not resolve. module-source: a
// "./" or "../" source resolves like a relative repo-path, any other (a
// registry or VCS address) is kept as written. image-ref and service-name
// are names, kept as written.
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
	// A `\` is the separator of a Windows-authored path (an MSBuild
	// ProjectReference, `..\lib\lib.csproj`); no repository file is named
	// with one, so it reads as `/`.
	raw = strings.ReplaceAll(raw, `\`, "/")
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
	stepDescend
	stepIndex
	// stepDescendWhere is `..{key=value}`: every map at any depth (the node
	// itself included) whose entry key is the scalar value.
	stepDescendWhere
)

type docStep struct {
	kind  docStepKind
	key   string
	index int
	want  string // stepDescendWhere's value
}

// docPath is a compiled selector: `$` (the context node) followed by
// `.key`, `.*` (every value of a map, with its key), `[*]` (every item
// of a sequence; any other non-null node is its own single item, so a
// field written as either one value or a list reads the same, and a
// single repeated XML element reads as a one-item list), `[N]` (a
// sequence's item N, from 0; nothing for anything else) and `..key`
// (recursive descent: the value of every entry named key at any depth under
// the node, through maps and sequences, in document order — a
// CloudFormation `Ref` wherever a resource's properties nest it).
type docPath struct{ steps []docStep }

type docMatch struct {
	node *docNode
	key  string // the map key the node was reached by ("" for an item)
	// parent is the key of the match this one was reached from, one step up
	// (`$parent`): an operation reached by `$.paths.*.*` has its path as its
	// parent; "" after a `..key` descent.
	parent string
	// path is the dotted keys from the selection's start to this node
	// ("image.repository"); a sequence item adds none. `$path` reads it.
	path string
	// file is the base name of the file being read, extension included
	// (`$file`); ExtractDoc sets it on every match before a value is read.
	file string
	// index is a sequence item's position, from 0 (`$index`); a node that is
	// not a sequence is its own one item, index 0. "" for a match no `[*]`
	// or `[N]` step reached.
	index string
}

// joinDocPath appends key to a dotted path.
func joinDocPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

var (
	docKeyRe   = regexp.MustCompile(`^[^.\[\]|*\s]+`)
	docURLRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)
	docIndexRe = regexp.MustCompile(`^\[([0-9]+)\]`)
)

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
		case docIndexRe.MatchString(s):
			m := docIndexRe.FindStringSubmatch(s)
			i, err := strconv.Atoi(m[1])
			if err != nil {
				return docPath{}, fmt.Errorf("selector: bad index at %q", s)
			}
			p.steps = append(p.steps, docStep{kind: stepIndex, index: i})
			s = s[len(m[0]):]
		case strings.HasPrefix(s, "..{"):
			end := strings.IndexByte(s, '}')
			if end < 0 {
				return docPath{}, fmt.Errorf("selector: unclosed `..{` at %q", s)
			}
			k, want, ok := strings.Cut(s[3:end], "=")
			if !ok || k == "" || want == "" {
				return docPath{}, fmt.Errorf("selector: `..{key=value}` needs a key and a value at %q", s)
			}
			p.steps = append(p.steps, docStep{kind: stepDescendWhere, key: k, want: want})
			s = s[end+1:]
		case strings.HasPrefix(s, ".."):
			k := docKeyRe.FindString(s[2:])
			if k == "" {
				return docPath{}, fmt.Errorf("selector: `..` needs a key at %q", s)
			}
			p.steps = append(p.steps, docStep{kind: stepDescend, key: k})
			s = s[2+len(k):]
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
			return docPath{}, fmt.Errorf("selector: unexpected %q (the subset is $ .key .* [*] [N] ..key)", s)
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
					next = append(next, docMatch{node: v, key: st.key, parent: m.key, path: joinDocPath(m.path, st.key)})
				}
			case stepAllValues:
				if n != nil && n.kind == docMap {
					for i, k := range n.keys {
						next = append(next, docMatch{node: n.vals[i], key: k, parent: m.key, path: joinDocPath(m.path, k)})
					}
				}
			case stepAllItems:
				switch {
				case n == nil || n.kind == docNull:
				case n.kind == docSeq:
					for i, it := range n.items {
						next = append(next, docMatch{node: it, parent: m.key, path: m.path, index: strconv.Itoa(i)})
					}
				default:
					next = append(next, docMatch{node: n, key: m.key, parent: m.parent, path: m.path, index: "0"})
				}
			case stepIndex:
				if n != nil && n.kind == docSeq && st.index < len(n.items) {
					next = append(next, docMatch{node: n.items[st.index], parent: m.key, path: m.path, index: strconv.Itoa(st.index)})
				}
			case stepDescend:
				next = descend(next, n, m.path, st.key)
			case stepDescendWhere:
				next = descendWhere(next, n, m.key, m.path, st.key, st.want)
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

// descend appends, in document order, the value of every map entry named
// key at any depth under n (a match's own subtree follows it), each with
// the dotted path of the keys walked from the selection's start.
func descend(out []docMatch, n *docNode, path, key string) []docMatch {
	if n == nil {
		return out
	}
	switch n.kind {
	case docMap:
		for i, k := range n.keys {
			p := joinDocPath(path, k)
			if k == key {
				out = append(out, docMatch{node: n.vals[i], key: k, path: p})
			}
			out = descend(out, n.vals[i], p, key)
		}
	case docSeq:
		for _, it := range n.items {
			out = descend(out, it, path, key)
		}
	}
	return out
}

// descendWhere appends, in document order, every map at or under n whose
// entry key is the scalar want (each reached by the key it sits under, with
// the dotted path of the keys walked), then descends into it.
func descendWhere(out []docMatch, n *docNode, key, path, entry, want string) []docMatch {
	if n == nil {
		return out
	}
	switch n.kind {
	case docMap:
		if v := n.get(entry); v != nil && v.kind == docScalar && v.scalar == want {
			out = append(out, docMatch{node: n, key: key, path: path})
		}
		for i, k := range n.keys {
			out = descendWhere(out, n.vals[i], k, joinDocPath(path, k), entry, want)
		}
	case docSeq:
		for _, it := range n.items {
			out = descendWhere(out, it, "", path, entry, want)
		}
	}
	return out
}

// docValue is a value expression: alternatives separated by `|`, each a
// selector relative to the match (`$` is the matched node), `$key` (the
// key it was reached by), `$parent` (the key of the match one step up: an
// operation's path, reached by `$.paths.*.*`), `$path` (every key from the
// selection's start down to the node, dot-joined — a nested map's
// "image.repository") or `$file` (the base name of the file being read,
// extension included: the name of a document that carries none, an MSBuild
// project). The first alternative that reaches a non-empty scalar is the
// value. An alternative may end in transforms, each a space and `@name`:
// `@upper` (upper-case) and `@file` (a `$ref`'s file part: the text before
// its `#`, "" for a local reference or a URL), and last in ` cut:<marker>`
// (LB-F126: the text before the marker's first occurrence, after the
// transforms; a lockfile's `zlib/1.3.1#rev` is `zlib` by `$ cut:/`). A value
// that begins `fmt:` is a template: literal text with `{expr}` placeholders,
// each a value expression of its own, and empty when any placeholder is
// (`fmt:{$key @upper} {$parent}` is an operation's "GET /pets").
type docValue struct {
	alts []docAlt
	tmpl []docTmplPart // set for a `fmt:` template; alts is then empty
}

type docAlt struct {
	key    bool
	parent bool
	path   bool
	file   bool
	index  bool
	p      docPath
	xform  []string
	// cut (LB-F126): keep the text before its first occurrence (` cut:/`).
	cut string
}

// docCutSuffix ends a value alternative that is cut at a marker.
const docCutSuffix = " cut:"

// docTmplPart is one piece of a `fmt:` template: literal text, or (v set)
// a placeholder.
type docTmplPart struct {
	lit string
	v   *docValue
}

var docTransforms = map[string]bool{"upper": true, "file": true}

func parseDocValue(s string) (docValue, error) {
	if rest, ok := strings.CutPrefix(strings.TrimSpace(s), "fmt:"); ok {
		return parseDocTemplate(rest)
	}
	var v docValue
	for _, a := range strings.Split(s, "|") {
		a = strings.TrimSpace(a)
		cut := ""
		if i := strings.Index(a, docCutSuffix); i >= 0 {
			a, cut = strings.TrimSpace(a[:i]), a[i+len(docCutSuffix):]
			if cut == "" {
				return docValue{}, fmt.Errorf("value %q: cut: names no marker", s)
			}
		}
		fields := strings.Fields(a)
		if len(fields) == 0 {
			return docValue{}, fmt.Errorf("empty value alternative in %q", s)
		}
		alt := docAlt{cut: cut}
		for _, f := range fields[1:] {
			name, ok := strings.CutPrefix(f, "@")
			if !ok || !docTransforms[name] {
				return docValue{}, fmt.Errorf("value %q: unknown transform %q (the subset is @upper @file)", s, f)
			}
			alt.xform = append(alt.xform, name)
		}
		switch fields[0] {
		case "$key":
			alt.key = true
		case "$parent":
			alt.parent = true
		case "$path":
			alt.path = true
		case "$file":
			alt.file = true
		case "$index":
			alt.index = true
		default:
			p, err := parseDocPath(fields[0])
			if err != nil {
				return docValue{}, err
			}
			alt.p = p
		}
		v.alts = append(v.alts, alt)
	}
	return v, nil
}

// parseDocTemplate reads the body of a `fmt:` value.
func parseDocTemplate(s string) (docValue, error) {
	var v docValue
	for s != "" {
		open := strings.IndexByte(s, '{')
		if open < 0 {
			v.tmpl = append(v.tmpl, docTmplPart{lit: s})
			break
		}
		if open > 0 {
			v.tmpl = append(v.tmpl, docTmplPart{lit: s[:open]})
		}
		end := strings.IndexByte(s[open:], '}')
		if end < 0 {
			return docValue{}, fmt.Errorf("fmt: template has an unclosed {")
		}
		inner, err := parseDocValue(s[open+1 : open+end])
		if err != nil {
			return docValue{}, err
		}
		v.tmpl = append(v.tmpl, docTmplPart{v: &inner})
		s = s[open+end+1:]
	}
	if len(v.tmpl) == 0 {
		return docValue{}, fmt.Errorf("fmt: template is empty")
	}
	return v, nil
}

func (a docAlt) apply(s string) string {
	for _, x := range a.xform {
		switch x {
		case "upper":
			s = strings.ToUpper(s)
		case "file":
			s = fileOfRef(s)
		}
	}
	return a.cutText(s)
}

func (v docValue) eval(m docMatch) string {
	if len(v.tmpl) > 0 {
		var b strings.Builder
		for _, part := range v.tmpl {
			if part.v == nil {
				b.WriteString(part.lit)
				continue
			}
			x := part.v.eval(m)
			if x == "" {
				return ""
			}
			b.WriteString(x)
		}
		return b.String()
	}
	for _, a := range v.alts {
		switch {
		case a.key:
			if r := a.apply(m.key); r != "" {
				return r
			}
			continue
		case a.parent:
			if r := a.apply(m.parent); r != "" {
				return r
			}
			continue
		case a.path:
			if r := a.apply(m.path); r != "" {
				return r
			}
			continue
		case a.file:
			if r := a.apply(m.file); r != "" {
				return r
			}
			continue
		case a.index:
			if r := a.apply(m.index); r != "" {
				return r
			}
			continue
		}
		for _, r := range a.p.eval(m.node, m.key) {
			if r.node.kind == docScalar && r.node.scalar != "" {
				if x := a.apply(r.node.scalar); x != "" {
					return x
				}
			}
		}
	}
	return ""
}

// cutText applies the alternative's ` cut:` marker: the text before its first
// occurrence, unless the text starts with it.
func (a docAlt) cutText(s string) string {
	if a.cut == "" {
		return s
	}
	if i := strings.Index(s, a.cut); i > 0 {
		return s[:i]
	}
	return s
}
