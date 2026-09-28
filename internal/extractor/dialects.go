package extractor

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// dialectFS embeds every dialect directory's manifest, doc spec and queries
// (docs/design/language-breadth.md §2.1, ADR-048). Fixtures and goldens under
// testdata/ are test inputs and are not embedded.
//
//go:embed dialects/*/*.json dialects/*/*.scm
var dialectFS embed.FS

// Dialect is one internal/extractor/dialects/<id>/dialect.json (§2.2). The
// CUE schema (schema/dialect/dialect.cue, `task cue-vet`) owns the per-file
// shape; LoadDialects enforces the cross-dialect rules that need every
// manifest at once.
type Dialect struct {
	ID       string                 `json:"id"`
	Display  string                 `json:"display"`
	Kind     string                 `json:"kind"`
	Engine   string                 `json:"engine"`
	LangTag  string                 `json:"lang_tag"`
	Match    DialectMatch           `json:"match"`
	Grammar  *DialectGrammar        `json:"grammar,omitempty"`
	Variants []DialectVar           `json:"variants"`
	Queries  DialectQueries         `json:"queries"`
	Doc      string                 `json:"doc,omitempty"`
	Kinds    map[string]DialectKind `json:"kinds"`
	// RequiredKinds are keys of Kinds (§1.4).
	RequiredKinds []string                     `json:"required_kinds"`
	OracleKinds   map[string]map[string]string `json:"oracle_kinds"`
	// NameMatch is the harness's name comparison (§1.4): "bare" (default)
	// or "identity". Extraction never reads it.
	NameMatch   string           `json:"name_match,omitempty"`
	Qualify     DialectQualify   `json:"qualify"`
	Package     DialectPackage   `json:"package"`
	Imports     *DialectImports  `json:"imports,omitempty"`
	Signature   DialectSignature `json:"signature"`
	TargetLevel string           `json:"target_level"`
	QueueID     string           `json:"queue_id"`
	// Ctags names the dialect's own universal-ctags invocation (design
	// §13.4 step 18) for a tags dialect the langbench harness's
	// incumbents.json (the five pre-manifest languages) does not cover —
	// CSS onward. Omitted for a dialect ctags has no parser for.
	Ctags *DialectCtags `json:"ctags,omitempty"`
}

// DialectCtags is one dialect's tier-B oracle invocation: universal-ctags'
// own language name(s) (`ctags --list-languages`) and any extra flags its
// run needs (mirrors langbench.CtagsSpec, the shape incumbents.json already
// uses for the five migrated languages).
type DialectCtags struct {
	Languages []string `json:"languages"`
	Args      []string `json:"args,omitempty"`
}

// DialectMatch is how a file is classified to the dialect (§5).
type DialectMatch struct {
	Extensions []string      `json:"extensions"`
	Basenames  []string      `json:"basenames"`
	Globs      []string      `json:"globs"`
	Shebangs   []string      `json:"shebangs"`
	Sniff      *DialectSniff `json:"sniff"`
}

// DialectSniff splits an extension more than one dialect claims (§5).
type DialectSniff struct {
	Extensions []string `json:"extensions"`
	Pattern    string   `json:"pattern"`
	Priority   int      `json:"priority"`
}

// DialectGrammar names the tsengine blob and its default entry point.
type DialectGrammar struct {
	Blob       string `json:"blob"`
	EntryPoint string `json:"entry_point"`
}

// DialectVar is a second entry point in the same blob (TypeScript's tsx).
type DialectVar struct {
	ID         string   `json:"id"`
	EntryPoint string   `json:"entry_point"`
	Extensions []string `json:"extensions"`
}

// DialectQueries names the dialect's query files, relative to its directory.
type DialectQueries struct {
	Tags       string `json:"tags,omitempty"`
	Imports    string `json:"imports,omitempty"`
	Injections string `json:"injections,omitempty"`
}

// DialectKind maps one capture suffix ("definition.<key>") to fathom's
// taxonomy and the containment rules the engine qualifies by.
type DialectKind struct {
	SourceKind      string `json:"source_kind"`
	Canonical       string `json:"canonical"`
	Container       bool   `json:"container,omitempty"`
	Callable        bool   `json:"callable,omitempty"`
	QualifyAsTarget bool   `json:"qualify_as_target,omitempty"`
}

// DialectQualify is the containment-qualification rule. ChainContainers
// defaults to true, as in the schema.
type DialectQualify struct {
	ChainContainers *bool  `json:"chain_containers,omitempty"`
	ContainerSep    string `json:"container_sep,omitempty"`
	MethodSep       string `json:"method_sep,omitempty"`
	// ScopeSep is the language's own scope-resolution operator ("::" for
	// C++). When set, a definition name or callee that spells a scope path
	// with it is normalized to ContainerSep (template arguments dropped), so
	// an out-of-class definition "Widget::render" and an inline method share
	// one key (normalizeScopedName). Empty: names are taken as captured.
	ScopeSep string `json:"scope_sep,omitempty"`
}

// DialectPackage selects how SymbolNode.Package is set: "dir" (the
// directory rule), "capture" (@scope.package, §3.5) or "none".
type DialectPackage struct {
	Strategy string `json:"strategy"`
}

// DialectImports names the L3 strategy (§6) and its parameters (LB-F11).
type DialectImports struct {
	Strategy string
	Params   ImportParams
}

// ImportParams are the parameters a manifest gives its import strategy
// (design §6). Each strategy reads the ones it documents
// (internal/fusion/resolvestrategy.go); an omitted one takes the strategy's
// default, which for relative-path and dotted-module is exactly the
// pre-LB-F11 TypeScript/JavaScript and Python rule.
type ImportParams struct {
	// SourceRoots are repo-relative directories a non-relative module is
	// looked up under, in order ("" is the repo root).
	SourceRoots []string `json:"source_roots,omitempty"`
	// Suffixes are tried after the module path, in order.
	Suffixes []string `json:"suffixes,omitempty"`
	// IndexFiles are file names tried inside a module path that is a
	// directory (index.ts, __init__.py, init.lua, mod.rs).
	IndexFiles []string `json:"index_files,omitempty"`
	// Separator splits a module name into path segments ("." for dotted
	// names, "\\" for PHP, "::" for Perl); "/" when empty.
	Separator string `json:"separator,omitempty"`
	// Relative resolves a spec without a ./ or ../ prefix against the
	// importing file's directory first (CSS @import, require_relative).
	Relative bool `json:"relative,omitempty"`
	// Case "snake" maps CamelCase segments to snake_case file names
	// (Elixir's MyApp.FooBar -> my_app/foo_bar).
	Case string `json:"case,omitempty"`
	// Labels "bazel" reads //pkg:file and :file labels (Starlark load).
	Labels string `json:"labels,omitempty"`
}

// importStrategies is design §6's closed set; go-package names Go's native
// resolution and resolves nothing here.
var importStrategies = map[string]bool{
	"relative-path": true, "dotted-module": true, "namespace-dir": true, "include-path": true,
	"crate-mod": true, "swift-module": true, "file-symbol": true, "go-package": true,
}

// DialectSignature caps Signature (§3.3).
type DialectSignature struct {
	MaxChars int `json:"max_chars"`
}

func (d *DialectImports) UnmarshalJSON(b []byte) error {
	var s struct {
		Strategy string `json:"strategy"`
		ImportParams
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // a parameter no strategy reads is a manifest error
	if err := dec.Decode(&s); err != nil {
		return err
	}
	d.Strategy, d.Params = s.Strategy, s.ImportParams
	return nil
}

// UnmarshalJSON decodes a manifest strictly (an unknown field is an error,
// as the closed CUE schema says) and applies the schema's defaults.
func (d *Dialect) UnmarshalJSON(b []byte) error {
	type plain Dialect
	var p plain
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return err
	}
	*d = Dialect(p)
	if d.Qualify.ChainContainers == nil {
		t := true
		d.Qualify.ChainContainers = &t
	}
	if d.Qualify.ContainerSep == "" {
		d.Qualify.ContainerSep = "."
	}
	if d.Qualify.MethodSep == "" {
		d.Qualify.MethodSep = "."
	}
	if d.Package.Strategy == "" {
		d.Package.Strategy = "dir"
	}
	if d.Signature.MaxChars == 0 {
		d.Signature.MaxChars = 200
	}
	return nil
}

// canonicalKinds is §2.3's closed list: language kinds, then tool-format
// kinds. schema/dialect/dialect.cue's #Canonical holds the same list, and
// TestCanonicalKindClosedList holds the two together.
var canonicalKinds = []string{
	"module", "namespace", "package", "class", "struct", "interface", "trait", "enum",
	"enum_member", "union", "type_alias", "function", "method", "constructor", "field",
	"property", "constant", "variable", "macro", "import", "test",
	"target", "job", "stage", "task", "step", "resource", "data_source", "output",
	"variable_decl", "rule", "policy", "service", "rpc", "message", "table", "view",
	"column", "selector", "mixin", "element", "section", "label", "route", "key",
}

// CanonicalKinds returns §2.3's closed list, sorted.
func CanonicalKinds() []string {
	out := append([]string(nil), canonicalKinds...)
	sort.Strings(out)
	return out
}

func isCanonicalKind(k string) bool {
	for _, c := range canonicalKinds {
		if c == k {
			return true
		}
	}
	return false
}

// sharedSourceKindGroups lists the dialects that may emit the same
// SourceKind string: TypeScript kept JavaScript's JSClass/JSFunc/JSMethod
// when both moved to manifests (§2.2: existing languages keep their existing
// strings), and fusion still dispatches both through one "javascript"
// extractor (tsExtractorGroups).
var sharedSourceKindGroups = [][]string{{"javascript", "typescript"}}

func sameSourceKindGroup(a, b string) bool {
	for _, g := range sharedSourceKindGroups {
		ina, inb := false, false
		for _, id := range g {
			ina = ina || id == a
			inb = inb || id == b
		}
		if ina && inb {
			return true
		}
	}
	return false
}

var dialectIDRe = regexp.MustCompile(`^[a-z0-9-]+$`)

var (
	dialectsOnce sync.Once
	dialectsAll  []Dialect
	dialectsErr  error
)

// LoadDialects returns every embedded dialect manifest, sorted by id, after
// the cross-dialect rules of §2.2: the id is the directory name and the
// queue id; every source_kind is unique across dialects (bar
// sharedSourceKindGroups); every canonical kind is from the closed list;
// required kinds are declared kinds; every extension and basename has one
// owner (a sniffed extension excepted, §5); a tags-engine dialect names its
// grammar and a tags query that exists.
func LoadDialects() ([]Dialect, error) {
	dialectsOnce.Do(func() { dialectsAll, dialectsErr = loadDialects(dialectFS) })
	return dialectsAll, dialectsErr
}

func loadDialects(fsys fs.FS) ([]Dialect, error) {
	manifests, err := fs.Glob(fsys, "dialects/*/dialect.json")
	if err != nil {
		return nil, err
	}
	sort.Strings(manifests)
	var out []Dialect
	sourceOwner := map[string]string{}
	extOwner := map[string]string{}
	for _, mp := range manifests {
		dir := path.Base(path.Dir(mp))
		b, err := fs.ReadFile(fsys, mp)
		if err != nil {
			return nil, err
		}
		var d Dialect
		if err := json.Unmarshal(b, &d); err != nil {
			return nil, fmt.Errorf("%s: %w", mp, err)
		}
		if err := checkDialect(fsys, dir, d); err != nil {
			return nil, fmt.Errorf("%s: %w", mp, err)
		}
		for _, key := range sortedKindKeys(d) {
			sk := d.Kinds[key].SourceKind
			if prev, ok := sourceOwner[sk]; ok && prev != d.ID && !sameSourceKindGroup(prev, d.ID) {
				return nil, fmt.Errorf("%s: source_kind %q already emitted by dialect %q", mp, sk, prev)
			}
			sourceOwner[sk] = d.ID
		}
		sniffed := map[string]bool{}
		if d.Match.Sniff != nil {
			for _, e := range d.Match.Sniff.Extensions {
				sniffed[strings.ToLower(e)] = true
			}
		}
		claim := func(key string) error {
			if prev, ok := extOwner[key]; ok && prev != d.ID && !sniffed[strings.TrimPrefix(key, "ext:")] {
				return fmt.Errorf("%s: %s already claimed by dialect %q", mp, key, prev)
			}
			extOwner[key] = d.ID
			return nil
		}
		for _, e := range dialectExtensions(d) {
			if err := claim("ext:" + e); err != nil {
				return nil, err
			}
		}
		for _, base := range d.Match.Basenames {
			if err := claim("base:" + base); err != nil {
				return nil, err
			}
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func checkDialect(fsys fs.FS, dir string, d Dialect) error {
	switch {
	case !dialectIDRe.MatchString(d.ID):
		return fmt.Errorf("id %q is not [a-z0-9-]+", d.ID)
	case d.ID != dir:
		return fmt.Errorf("id %q is not its directory %q", d.ID, dir)
	case d.QueueID != d.ID:
		return fmt.Errorf("queue_id %q is not id %q", d.QueueID, d.ID)
	case len(d.Kinds) == 0:
		return fmt.Errorf("no kinds")
	}
	for _, key := range sortedKindKeys(d) {
		k := d.Kinds[key]
		if !isCanonicalKind(k.Canonical) {
			return fmt.Errorf("kind %q: canonical %q is not in the closed list (§2.3)", key, k.Canonical)
		}
		if k.SourceKind == "" {
			return fmt.Errorf("kind %q: empty source_kind", key)
		}
	}
	if d.NameMatch != "" && d.NameMatch != "bare" && d.NameMatch != "identity" {
		return fmt.Errorf("name_match %q is not bare or identity", d.NameMatch)
	}
	for _, rk := range d.RequiredKinds {
		found := false
		for key, k := range d.Kinds {
			if rk == key || rk == k.Canonical {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("required kind %q is not a declared kind", rk)
		}
	}
	if im := d.Imports; im != nil {
		if !importStrategies[im.Strategy] {
			return fmt.Errorf("imports.strategy %q is not in the closed set (§6)", im.Strategy)
		}
		if c := im.Params.Case; c != "" && c != "snake" {
			return fmt.Errorf("imports.case %q: only \"snake\"", c)
		}
		if l := im.Params.Labels; l != "" && l != "bazel" {
			return fmt.Errorf("imports.labels %q: only \"bazel\"", l)
		}
	}
	if s := d.Match.Sniff; s != nil {
		// connector.ClassifyDialect compiles it multi-line; one that does
		// not compile would silently never match.
		if _, err := regexp.Compile("(?m)" + s.Pattern); err != nil {
			return fmt.Errorf("match.sniff.pattern: %w", err)
		}
	}
	for oracle, m := range d.OracleKinds {
		for ok, canon := range m {
			if !isCanonicalKind(canon) {
				return fmt.Errorf("oracle_kinds.%s.%s: %q is not canonical", oracle, ok, canon)
			}
		}
	}
	if d.Engine == "tags" || d.Engine == "tags+doc" {
		if d.Grammar == nil || d.Grammar.Blob == "" || d.Grammar.EntryPoint == "" {
			return fmt.Errorf("tags engine without a grammar")
		}
		if d.Queries.Tags == "" {
			return fmt.Errorf("tags engine without queries.tags")
		}
	}
	for _, q := range []string{d.Queries.Tags, d.Queries.Imports, d.Queries.Injections} {
		if q == "" {
			continue
		}
		if _, err := fs.Stat(fsys, path.Join("dialects", d.ID, q)); err != nil {
			return fmt.Errorf("query %s: %w", q, err)
		}
	}
	if d.Engine == "doc" || d.Engine == "tags+doc" {
		if _, err := loadDocExtractor(fsys, d); err != nil {
			return err
		}
	}
	return nil
}

// LoadDocExtractor builds the document engine for a doc or tags+doc
// dialect from its embedded doc.json (LB-F8, design §4).
func LoadDocExtractor(d Dialect) (*DocExtractor, error) {
	return loadDocExtractor(dialectFS, d)
}

// NewDocDialectExtractor finds id's embedded manifest and builds its
// document engine (LB-F8, design §4) — the fusion dispatch table's
// (internal/fusion/pipeline.go) constructor for a "doc"/"tags+doc" dialect,
// the same role NewTSExtractor(id) plays for a "tags" one.
func NewDocDialectExtractor(id string) (*DocExtractor, error) {
	dialects, err := LoadDialects()
	if err != nil {
		return nil, err
	}
	for _, d := range dialects {
		if d.ID == id {
			return LoadDocExtractor(d)
		}
	}
	return nil, fmt.Errorf("dialect %q not found (doc engine)", id)
}

// loadDocExtractor reads and compiles d's doc spec and checks that every
// entity's source_kind is one the manifest declares — the manifest's kinds
// stay the one place the registry learns a dialect's SourceKinds.
func loadDocExtractor(fsys fs.FS, d Dialect) (*DocExtractor, error) {
	name := d.Doc
	if name == "" {
		name = "doc.json"
	}
	b, err := fs.ReadFile(fsys, path.Join("dialects", d.ID, name))
	if err != nil {
		return nil, fmt.Errorf("doc spec %s: %w", name, err)
	}
	spec, err := ParseDocSpec(b)
	if err != nil {
		return nil, err
	}
	declared := map[string]bool{}
	for _, k := range d.Kinds {
		declared[k.SourceKind] = true
	}
	for i, e := range spec.Entities {
		if !declared[e.SourceKind] {
			return nil, fmt.Errorf("doc spec: entities[%d] source_kind %q is not a kind the manifest declares", i, e.SourceKind)
		}
	}
	return NewDocExtractor(spec)
}

func sortedKindKeys(d Dialect) []string {
	keys := make([]string, 0, len(d.Kinds))
	for k := range d.Kinds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// dialectExtensions is every lowercased extension a dialect claims, its
// variants' included.
func dialectExtensions(d Dialect) []string {
	var out []string
	for _, e := range d.Match.Extensions {
		out = append(out, strings.ToLower(e))
	}
	for _, v := range d.Variants {
		for _, e := range v.Extensions {
			out = append(out, strings.ToLower(e))
		}
	}
	return out
}

// dialectQuerySource is the query text the tags engine compiles for d: its
// tags query, then its imports query, in that order (pattern indices, and so
// match order, follow the text).
func dialectQuerySource(fsys fs.FS, d Dialect) (string, error) {
	var b strings.Builder
	for _, q := range []string{d.Queries.Tags, d.Queries.Imports} {
		if q == "" {
			continue
		}
		src, err := fs.ReadFile(fsys, path.Join("dialects", d.ID, q))
		if err != nil {
			return "", fmt.Errorf("dialect %s: %w", d.ID, err)
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.Write(src)
	}
	return b.String(), nil
}
