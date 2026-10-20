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
	NameMatch string         `json:"name_match,omitempty"`
	Qualify   DialectQualify `json:"qualify"`
	// Names rewrites every captured @name (a definition's, a callee's)
	// before qualification — the grammar's own spelling of a name (a
	// quoted string literal, a `.Values.` accessor prefix) is not the
	// symbol's key. Omitted: names are taken as captured.
	Names   *DialectNames  `json:"names,omitempty"`
	Package DialectPackage `json:"package"`
	// Preprocess names the dialect's pre-parse step (the closed set
	// tsPreprocessors, schema #Dialect.preprocess): a file whose tree has a
	// syntax error is parsed as each text the step gives, and the first tree
	// with none is read (O148). Omitted: the file is parsed as written.
	Preprocess string `json:"preprocess,omitempty"`
	// Calls is how call sites are stored (LB-F65). Omitted: every callee
	// is taken as captured.
	Calls       *DialectCalls    `json:"calls,omitempty"`
	Imports     *DialectImports  `json:"imports,omitempty"`
	Signature   DialectSignature `json:"signature"`
	TargetLevel string           `json:"target_level"`
	QueueID     string           `json:"queue_id"`
	// Ctags names the dialect's own universal-ctags invocation (design
	// §13.4 step 18) for a tags dialect the langbench harness's
	// incumbents.json (the five pre-manifest languages) does not cover —
	// CSS onward. Omitted for a dialect ctags has no parser for.
	Ctags *DialectCtags `json:"ctags,omitempty"`
	// ModuleSymbol (K18, ADR-048 amendment 2026-10-10) is the key of Kinds
	// the tags engine mints once per file whose top level executes code: the
	// file's module symbol (core.ModuleSymbolName, line 1 to the last line,
	// tsextract_module.go). Omitted: the dialect mints none, and its output
	// is unchanged. checkModuleSymbol holds the schema's rules at load.
	ModuleSymbol string `json:"module_symbol,omitempty"`
	// InjectionHost (F2, wave lf7) makes the file the owner of what its
	// injected ranges (queries.injections) import and run: each range's file
	// specs (imports, references, re-exports) become the host file's, at
	// host lines, resolved under the host's own imports strategy, and each
	// range's top-level calls (no enclosing definition of the range) are the
	// host's module symbol's. A Vue or Svelte component, an Astro page: the
	// component file is the module its <script> is part of. Omitted: an
	// injected range's imports and top-level calls are dropped, as for HTML's
	// <script> (checkInjectionHost holds the schema's rules at load).
	InjectionHost bool `json:"injection_host,omitempty"`
}

// checkInjectionHost holds a manifest's injection_host to the schema's rules:
// a host needs the injections query that finds its ranges, the module symbol
// that owns their top-level calls and the import strategy their specs
// resolve under.
func checkInjectionHost(d Dialect) error {
	if !d.InjectionHost {
		return nil
	}
	switch {
	case d.Queries.Injections == "":
		return fmt.Errorf("injection_host needs queries.injections: the ranges it hosts")
	case d.ModuleSymbol == "":
		return fmt.Errorf("injection_host needs module_symbol: the host's module symbol owns its ranges' top-level calls")
	case d.Imports == nil || d.Imports.Strategy == "":
		return fmt.Errorf("injection_host needs imports: its ranges' import specs resolve under the host's strategy")
	}
	return nil
}

// DialectCalls is a dialect's call-site rules (LB-F65). Qualify stores a
// call's text as its receiver or scope joined to the called name — "a.b",
// "self.x", "Foo::bar" — compacted (extractor.compactCallee): whitespace
// and argument contents dropped, and a callee that ends in no name (a
// subscript or a call's result called) not stored at all.
type DialectCalls struct {
	Qualify bool `json:"qualify,omitempty"`
	// Owner "module" (wave lf5, K19 for HTML's W1 references, design §1.8):
	// every call site the tags query captures is the file's module symbol's,
	// whatever definition encloses it. A markup document's ID reference is
	// resolved against the document, never an ancestor element, so the
	// document is what makes it. Owner "scope" (LB-Q-scss-rest, W5): a call
	// site is the smallest enclosing callable's, else the module symbol's, a
	// definition that is no callable (a variable, a selector) owning none, so
	// a reference inside `$z: f($a)` or `.x-#{$k}` is the enclosing @mixin's
	// or the file's. Both need module_symbol (checkCalls).
	Owner string `json:"owner,omitempty"`
	// Split "whitespace" (wave lf5): a call site's text is split on ASCII
	// whitespace into one callee per token, an attribute that holds a set
	// of IDs (aria-labelledby="a b") naming each of them.
	Split string `json:"split,omitempty"`
}

// checkCalls holds a manifest's calls block to the schema's rules: owner is
// "module", "scope" or absent, and either needs the module symbol it names; split
// is "whitespace" or absent.
func checkCalls(d Dialect) error {
	if d.Calls == nil {
		return nil
	}
	switch d.Calls.Owner {
	case "":
	case "module", "scope":
		if d.ModuleSymbol == "" {
			return fmt.Errorf("calls.owner %q needs module_symbol: the file's module symbol owns the calls", d.Calls.Owner)
		}
	default:
		return fmt.Errorf("calls.owner %q is not \"module\" or \"scope\"", d.Calls.Owner)
	}
	if d.Calls.Split != "" && d.Calls.Split != "whitespace" {
		return fmt.Errorf("calls.split %q is not \"whitespace\"", d.Calls.Split)
	}
	return nil
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
	// ScanBytes widens the window the pattern sees past the default 1 KiB,
	// for a language whose files open with a long licence header
	// (Objective-C); 0 is the default.
	ScanBytes int `json:"scan_bytes,omitempty"`
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
	// Queries is the variant's own queries (O732, LB-F385): its tags query
	// is compiled before the dialect's, for the variant's extensions alone —
	// the patterns only its grammar can satisfy (TypeScript's tsx: JSX).
	Queries *DialectVarQueries `json:"queries,omitempty"`
}

// DialectVarQueries names a variant's query files, relative to the
// dialect's directory.
type DialectVarQueries struct {
	Tags string `json:"tags,omitempty"`
}

// DialectQueries names the dialect's query files, relative to its directory.
// References (LB-F74) is the file resolver's own query: the names a file
// references outside a call (@ref.*) and the modules it re-exports
// (@export.*), compiled and run apart from Tags and Imports
// (TSExtractor.ExtractFileSpecs), so Extract's matches never see it.
type DialectQueries struct {
	Tags       string `json:"tags,omitempty"`
	Imports    string `json:"imports,omitempty"`
	Injections string `json:"injections,omitempty"`
	References string `json:"references,omitempty"`
	// Locals (LB-F254) is the dialect's locals query: @local.scope and
	// @local.definition, read by the engine's locals rule
	// (tsextract_locals.go) for the tags patterns that capture their
	// @reference.call as @local.reference — tree-sitter-tags'
	// `(#is-not? local)`, which the engine's predicates cannot express.
	Locals string `json:"locals,omitempty"`
}

// DialectKind maps one capture suffix ("definition.<key>") to fathom's
// taxonomy and the containment rules the engine qualifies by.
type DialectKind struct {
	SourceKind      string `json:"source_kind"`
	Canonical       string `json:"canonical"`
	Container       bool   `json:"container,omitempty"`
	Callable        bool   `json:"callable,omitempty"`
	QualifyAsTarget bool   `json:"qualify_as_target,omitempty"`
	// Titled names a definition of this kind by its string title, quoted,
	// and one inside a titled definition of the same kind by both titles
	// (TQ-27: a jest test block inside a describe block is "sum" "adds", the
	// runner's full name). The quotes keep a title ("sum", the function a
	// describe block tests) from reading as a name a call resolves to.
	Titled bool `json:"titled,omitempty"`
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

// DialectNames is the manifest's names block (LB-Q-helm): StripPrefix drops
// the first listed prefix the captured text starts with (`.Values.` leaves the
// values key the accessor reads; a quoted string starting the same way is not
// touched), then TrimQuotes drops one matching pair of surrounding double
// quotes, single quotes or backticks (a Go-template `define "chart.name"` is
// the symbol chart.name). Both run before scope_sep.
//
// CutAt (LB-F124) keeps only the text before its first occurrence, after the
// other two and before scope_sep: a Conan reference `zlib/1.3.1#rev` is a use of
// the symbol `zlib`.
type DialectNames struct {
	TrimQuotes  bool     `json:"trim_quotes,omitempty"`
	StripPrefix []string `json:"strip_prefix,omitempty"`
	CutAt       string   `json:"cut_at,omitempty"`
	// PascalCase (wave lf7, LB-Q-vue) camelizes and capitalizes the name as
	// Vue's resolveComponent resolves a component tag, capitalize(camelize(
	// tag)): el-input names ElInput, keep-alive KeepAlive. It runs last.
	PascalCase bool `json:"pascal_case,omitempty"`
}

// DialectPackage selects how SymbolNode.Package is set: "dir" (the
// directory rule: the file's stem), "file" (the file's base name with its
// extension, LB-F519), "capture" (@scope.package, §3.5) or "none".
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
	// names, "\\" for PHP, "::" for Perl); "/" when empty. Under
	// file-symbol a "\\" separator is a path's second separator beside "/"
	// (PowerShell reads both on every platform): every backslash in a spec
	// is a "/" before any other rule reads it.
	Separator string `json:"separator,omitempty"`
	// DirVars are variables that name the importing file's own directory
	// (PowerShell's automatic $PSScriptRoot): under file-symbol a spec that
	// opens with one, in any letter case and with or without braces
	// (`${PSScriptRoot}`), then a separator resolves against that directory
	// alone, as a ./ spec does; a spec naming any other variable resolves
	// nothing (LB-Q-powershell, wave lf6).
	DirVars []string `json:"dir_vars,omitempty"`
	// Relative resolves a spec without a ./ or ../ prefix against the
	// importing file's directory first (CSS @import, require_relative).
	Relative bool `json:"relative,omitempty"`
	// Case "snake" maps CamelCase segments to snake_case file names
	// (Elixir's MyApp.FooBar -> my_app/foo_bar).
	Case string `json:"case,omitempty"`
	// Labels "bazel" reads //pkg:file and :file labels (Starlark load).
	Labels string `json:"labels,omitempty"`
	// TypeWildcard "members": a wildcard import whose module names a type
	// brings that type's members into scope, not the names of its file
	// (C#'s `using static N.T;`, namespace-dir).
	TypeWildcard string `json:"type_wildcard,omitempty"`
	// PackageScope "nested": a namespace's code sees its own and every
	// enclosing namespace's names unqualified (C#). A namespace-dir dialect
	// whose package comes from a clause (package.strategy "capture") sees
	// its own package's names without it (LB-F74).
	PackageScope string `json:"package_scope,omitempty"`
	// Constants "lexical": a constant reference is looked up through the
	// lexical scopes around its reference, innermost first, before the
	// names a require brought into scope (Ruby, LB-F76).
	Constants string `json:"constants,omitempty"`
	// ModuleRoots "buf": a module path is also looked up under the module
	// roots buf.yaml (v1: its directory; v2: modules[].path) and
	// buf.work.yaml (directories) name (protobuf, LB-F77).
	ModuleRoots string `json:"module_roots,omitempty"`
	// TypePaths "package-dir": a dotted type reference is package-qualified
	// (`other_package.Embed.Enumerated`, `.buf.validate.FieldRules`), its
	// leading segments the package, which names the defining file's
	// directory (buf's PACKAGE_DIRECTORY_MATCH); every type reference is
	// kept, whatever its case (protobuf, LB-F77).
	TypePaths string `json:"type_paths,omitempty"`
	// PartialPrefix "_": relative-path also tries a candidate's file name
	// with this prefix, after the name itself and before a directory's
	// index files (Sass's partials: `@use "x"` loads `_x.scss`, LB-F465).
	PartialPrefix string `json:"partial_prefix,omitempty"`
	// Scope names, under global-name, the files a type name is looked up
	// among: "service" every file of the dialect (a GraphQL schema spans the
	// service), "folder" the files of the importer's own directory (a Prisma
	// schema folder); "service" when empty (LB-F580).
	Scope string `json:"scope,omitempty"`
	// Defines are, under global-name, the keys of kinds whose top-level
	// definitions a name binds to (a GraphQL type, never an extension of
	// one); every top-level definition when empty.
	Defines []string `json:"defines,omitempty"`
}

// importStrategies is design §6's closed set; go-package names Go's native
// resolution and resolves nothing here. rego-package maps a Rego import path
// to the files declaring its package (LB-F132); global-name binds a type name
// to the one file of its scope defining it (LB-F580); cue-module maps a CUE
// import path to the files of the package the cue binary loads (LB-F583).
var importStrategies = map[string]bool{
	"relative-path": true, "dotted-module": true, "namespace-dir": true, "include-path": true,
	"crate-mod": true, "swift-module": true, "file-symbol": true, "go-package": true,
	"rego-package": true, "global-name": true, "cue-module": true,
}

// DialectSignature caps Signature (§3.3).
type DialectSignature struct {
	MaxChars int `json:"max_chars"`
	// Kinds are keys of Kinds whose definitions are signed beside the
	// callables (LB-F290: a C# class's header and base list).
	Kinds []string `json:"kinds,omitempty"`
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
var sharedSourceKindGroups = [][]string{
	{"javascript", "typescript"},
	// An injection host's manifest names the kinds its <script> blocks'
	// definitions carry (F2, wave lf7): a Vue or Svelte component's
	// functions and constants are TypeScript's or JavaScript's.
	{"javascript", "typescript", "vue", "svelte"},
}

// contractSourceKinds are source kinds a cross-dialect contract defines, so
// any dialect may declare them: TelemetryIdentity is ADR-062's TL-5 identity
// contract (an OpenTelemetry service.name, a New Relic app name, a Lambda
// function name, a Prometheus job, whichever format declares it), read by
// kind at the join, never by the dialect that wrote it.
var contractSourceKinds = []string{"TelemetryIdentity"}

func isContractSourceKind(sk string) bool {
	for _, k := range contractSourceKinds {
		if k == sk {
			return true
		}
	}
	return false
}

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
			if prev, ok := sourceOwner[sk]; ok && prev != d.ID && !sameSourceKindGroup(prev, d.ID) && !isContractSourceKind(sk) {
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
	if d.NameMatch != "" && d.NameMatch != "bare" && d.NameMatch != "identity" && d.NameMatch != "selector" {
		return fmt.Errorf("name_match %q is not bare, identity or selector", d.NameMatch)
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
		if w := im.Params.TypeWildcard; w != "" && w != "members" {
			return fmt.Errorf("imports.type_wildcard %q: only \"members\"", w)
		}
		if v := im.Params.PackageScope; v != "" && v != "nested" {
			return fmt.Errorf("imports.package_scope %q: only \"nested\"", v)
		}
		if v := im.Params.Constants; v != "" && v != "lexical" {
			return fmt.Errorf("imports.constants %q: only \"lexical\"", v)
		}
		if v := im.Params.ModuleRoots; v != "" && v != "buf" {
			return fmt.Errorf("imports.module_roots %q: only \"buf\"", v)
		}
		if v := im.Params.TypePaths; v != "" && v != "package-dir" {
			return fmt.Errorf("imports.type_paths %q: only \"package-dir\"", v)
		}
		if err := checkGlobalName(d, im); err != nil {
			return err
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
	if err := checkModuleSymbol(d); err != nil {
		return err
	}
	if err := checkCalls(d); err != nil {
		return err
	}
	if err := checkInjectionHost(d); err != nil {
		return err
	}
	if d.Engine == "tags" || d.Engine == "tags+doc" {
		if d.Grammar == nil || d.Grammar.Blob == "" || d.Grammar.EntryPoint == "" {
			return fmt.Errorf("tags engine without a grammar")
		}
		if d.Queries.Tags == "" {
			return fmt.Errorf("tags engine without queries.tags")
		}
	}
	for _, q := range []string{d.Queries.Tags, d.Queries.Imports, d.Queries.Injections, d.Queries.References, d.Queries.Locals} {
		if q == "" {
			continue
		}
		if _, err := fs.Stat(fsys, path.Join("dialects", d.ID, q)); err != nil {
			return fmt.Errorf("query %s: %w", q, err)
		}
	}
	for _, v := range d.Variants {
		if v.Queries == nil || v.Queries.Tags == "" {
			continue
		}
		if _, err := fs.Stat(fsys, path.Join("dialects", d.ID, v.Queries.Tags)); err != nil {
			return fmt.Errorf("variant %s: query %s: %w", v.ID, v.Queries.Tags, err)
		}
	}
	if d.Engine == "doc" || d.Engine == "tags+doc" {
		if _, err := loadDocExtractor(fsys, d); err != nil {
			return err
		}
	}
	return nil
}

// checkGlobalName holds the global-name parameters to the schema's rules
// (LB-F580): scope is "service" or "folder", each defines entry is a key of
// the manifest's kinds, and neither is read by any other strategy.
func checkGlobalName(d Dialect, im *DialectImports) error {
	if v := im.Params.Scope; v != "" && v != "service" && v != "folder" {
		return fmt.Errorf("imports.scope %q: only \"service\" or \"folder\"", v)
	}
	for _, k := range im.Params.Defines {
		if _, ok := d.Kinds[k]; !ok {
			return fmt.Errorf("imports.defines %q is not a declared kind", k)
		}
	}
	if im.Strategy != "global-name" && (im.Params.Scope != "" || len(im.Params.Defines) > 0) {
		return fmt.Errorf("imports.scope and imports.defines are global-name's parameters, not %s's", im.Strategy)
	}
	return nil
}

// checkModuleSymbol holds a manifest's module_symbol to the schema's rules
// (K18, schema #Dialect.module_symbol): it names a kind of the manifest whose
// canonical kind is "module", neither a container (the file's definitions
// are not its members, they keep their names) nor a callable (it has no
// signature); the engine minting it is the tags engine; no oracle kind maps
// to "module" (no L1 oracle tags a file as a definition, so the symbol stays
// outside the kind intersection); and the package is the file's own name
// (strategy "dir", its stem, or "file", its base name with the extension,
// LB-F519), so each file's module symbol has a key of its own.
func checkModuleSymbol(d Dialect) error {
	key := d.ModuleSymbol
	if key == "" {
		return nil
	}
	k, ok := d.Kinds[key]
	switch {
	case !ok:
		return fmt.Errorf("module_symbol %q is not a kind of the manifest", key)
	case k.Canonical != "module":
		return fmt.Errorf("module_symbol %q: its kind is canonical %q, not canonical \"module\"", key, k.Canonical)
	case k.Container || k.Callable:
		return fmt.Errorf("module_symbol %q: the module kind is neither a container nor a callable", key)
	case d.Engine != "tags" && d.Engine != "tags+doc":
		return fmt.Errorf("module_symbol %q: the tags engine mints it, not engine %q", key, d.Engine)
	case d.Package.Strategy != "dir" && d.Package.Strategy != "file":
		return fmt.Errorf("module_symbol %q: needs package strategy \"dir\" or \"file\" (the file's own name keys it), not %q", key, d.Package.Strategy)
	}
	var inside []string
	for oracle, m := range d.OracleKinds {
		for ok, canon := range m {
			if canon == "module" {
				inside = append(inside, oracle+"."+ok)
			}
		}
	}
	if len(inside) > 0 {
		sort.Strings(inside)
		return fmt.Errorf("module_symbol %q: oracle_kinds.%s maps to \"module\"; the module symbol stays outside oracle_kinds", key, strings.Join(inside, ", oracle_kinds."))
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
// (internal/fusion/register.go) constructor for a "doc"/"tags+doc" dialect,
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
