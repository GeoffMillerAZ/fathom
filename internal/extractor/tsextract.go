package extractor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// tsVariant is one (grammar blob, entry point, dialect rules) triple a
// TSExtractor may parse a file with. A single TSExtractor can span several
// variants keyed by file extension — e.g. the "javascript" extractor parses
// .ts/.tsx with the typescript dialect's grammar and query set (see
// tsExtractorGroups), mirroring the incumbent JSExtractor's combined
// JS/TS/JSX/TSX scope.
type tsVariant struct {
	blob       string // tsengine grammar registry key
	entryPoint string // exported tree_sitter_<x> function
	rules      *tsDialectRules
}

// tsDefKind maps one query capture suffix (the text after "definition.", as
// written in the .scm files) to fathom's symbol taxonomy and to this
// engine's generic containment-based qualification rule:
//
//   - container: other definitions nested directly inside this one qualify
//     against it (e.g. a class contains methods).
//   - callable: this definition itself gets a qualified name ("Owner.name")
//     when its immediate structural parent is a container.
//
// A definition that is neither (TS interfaces/types/enums, Python module
// vars) is emitted with its bare name.
type tsDefKind struct {
	Kind      core.SourceKind
	Container bool
	Callable  bool
}

// tsDialectRules is one dialect manifest compiled for the engine: its
// capture-kind table, qualification rule, strategies and query source.
type tsDialectRules struct {
	id    string
	kinds map[string]tsDefKind // capture suffix -> mapping

	// chainContainers: true for dialects where nested containers qualify
	// as "Outer::Inner" (Ruby's module/class nesting). false: only the
	// immediate container's bare name is used (Python, JS/TS) — matching
	// the incumbent CGO extractors' single-level scope tracking exactly.
	chainContainers bool
	containerSep    string   // separator when chainContainers is true
	methodSep       string   // separator between a container's qualified name and a callable's own name
	scopeSep        string   // manifest qualify.scope_sep: the language's own scope operator ("::"), "" when none
	qualifyCalls    bool     // manifest calls.qualify (LB-F65): qualifiedCallee
	trimQuotes      bool     // manifest names.trim_quotes
	stripPrefix     []string // manifest names.strip_prefix
	cutAt           string   // manifest names.cut_at (LB-F124)

	importStrategy  string // manifest imports.strategy ("" when the dialect has none)
	keepAllRefs     bool   // manifest imports.type_paths: every reference is a type (LB-F74)
	packageStrategy string // manifest package.strategy
	sigMax          int    // manifest signature.max_chars

	queryFile string // dialects/<id>/<tags> (for messages and lang-probe)
	query     string // tags query then imports query

	injectionsQuery string // queries.injections (LB-F6), "" when none
	refsQuery       string // queries.references (LB-F74), "" when none
	localsQuery     string // queries.locals (LB-F254, tsextract_locals.go), "" when none
	marksMembers    bool   // the query writes a member capture (LB-F241, tsextract_members.go): scopeTSMembers runs

	// preprocess is the manifest's pre-parse step (tsPreprocessors), nil
	// when it names none.
	preprocess func(src []byte) [][]byte
}

// tsPreprocessors are the pre-parse steps a manifest's preprocess names:
// each returns the texts, in order, to parse a file as when it does not
// parse as written (none when a rewrite would change nothing).
var tsPreprocessors = map[string]func(src []byte) [][]byte{
	"csharp-conditionals": csharpConditionalTexts,
}

// tsErrorQuery captures every node a tree-sitter parse could not fit to
// the grammar.
const tsErrorQuery = "[(ERROR) (MISSING)] @error"

// tsLangSpec is everything NewTSExtractor needs for one extractor: the
// variant each extension selects.
type tsLangSpec struct {
	name string

	// variantForExt selects the variant for a lowercased file extension
	// (including the leading '.'); an extension absent from this map falls
	// back to defaultVariant.
	variantForExt  map[string]tsVariant
	defaultVariant tsVariant
}

// tsExtractorGroups are extractor names that span more than one dialect.
// fusion registers one "javascript" extractor for .js/.jsx/.mjs/.cjs AND
// .ts/.tsx/.mts/.cts (the incumbent JSExtractor's combined scope); the
// first member's grammar is the default for an unclaimed extension.
var tsExtractorGroups = map[string][]string{"javascript": {"javascript", "typescript"}}

var (
	tsRegistryOnce sync.Once
	tsRegistryMap  map[string]*tsLangSpec
	tsDialectMap   map[string]*tsLangSpec // one entry per tags dialect id
	tsRegistryErr  error
)

// tsRegistry is built from the embedded dialect manifests (LB-F1, §3.1):
// one spec per tags-engine dialect, plus the tsExtractorGroups.
func tsRegistry() (map[string]*tsLangSpec, error) {
	tsRegistryOnce.Do(func() {
		tsRegistryMap, tsDialectMap, tsRegistryErr = buildTSRegistry()
	})
	return tsRegistryMap, tsRegistryErr
}

func buildTSRegistry() (map[string]*tsLangSpec, map[string]*tsLangSpec, error) {
	ds, err := LoadDialects()
	if err != nil {
		return nil, nil, err
	}
	all := map[string]*tsLangSpec{}
	single := map[string]*tsLangSpec{}
	for i := range ds {
		d := ds[i]
		if d.Engine != "tags" && d.Engine != "tags+doc" {
			continue
		}
		rules, err := compileDialectRules(d)
		if err != nil {
			return nil, nil, err
		}
		spec := &tsLangSpec{
			name:           d.ID,
			variantForExt:  map[string]tsVariant{},
			defaultVariant: tsVariant{blob: d.Grammar.Blob, entryPoint: d.Grammar.EntryPoint, rules: rules},
		}
		for _, e := range d.Match.Extensions {
			spec.variantForExt[strings.ToLower(e)] = spec.defaultVariant
		}
		for _, v := range d.Variants {
			vv := tsVariant{blob: d.Grammar.Blob, entryPoint: v.EntryPoint, rules: rules}
			for _, e := range v.Extensions {
				spec.variantForExt[strings.ToLower(e)] = vv
			}
		}
		all[d.ID] = spec
		single[d.ID] = spec
	}
	groups := make([]string, 0, len(tsExtractorGroups))
	for g := range tsExtractorGroups {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	for _, g := range groups {
		members := tsExtractorGroups[g]
		spec := &tsLangSpec{name: g, variantForExt: map[string]tsVariant{}}
		for i, m := range members {
			ms, ok := single[m]
			if !ok {
				return nil, nil, fmt.Errorf("tsextract: group %q names unknown tags dialect %q", g, m)
			}
			if i == 0 {
				spec.defaultVariant = ms.defaultVariant
			}
			for e, v := range ms.variantForExt {
				spec.variantForExt[e] = v
			}
		}
		all[g] = spec
	}
	return all, single, nil
}

func compileDialectRules(d Dialect) (*tsDialectRules, error) {
	query, err := dialectQuerySource(dialectFS, d)
	if err != nil {
		return nil, err
	}
	r := &tsDialectRules{
		id:              d.ID,
		kinds:           map[string]tsDefKind{},
		chainContainers: *d.Qualify.ChainContainers,
		containerSep:    d.Qualify.ContainerSep,
		methodSep:       d.Qualify.MethodSep,
		scopeSep:        d.Qualify.ScopeSep,
		packageStrategy: d.Package.Strategy,
		sigMax:          d.Signature.MaxChars,
		queryFile:       "dialects/" + d.ID + "/" + d.Queries.Tags,
		query:           query,
	}
	if d.Imports != nil {
		r.importStrategy = d.Imports.Strategy
		r.keepAllRefs = d.Imports.Params.TypePaths != ""
	}
	if d.Calls != nil {
		r.qualifyCalls = d.Calls.Qualify
	}
	if d.Names != nil {
		r.trimQuotes, r.stripPrefix, r.cutAt = d.Names.TrimQuotes, d.Names.StripPrefix, d.Names.CutAt
	}
	if d.Preprocess != "" {
		pre, ok := tsPreprocessors[d.Preprocess]
		if !ok {
			return nil, fmt.Errorf("dialect %s: unknown preprocess %q", d.ID, d.Preprocess)
		}
		r.preprocess = pre
	}
	if d.Queries.Injections != "" {
		b, err := fs.ReadFile(dialectFS, path.Join("dialects", d.ID, d.Queries.Injections))
		if err != nil {
			return nil, fmt.Errorf("dialect %s: %w", d.ID, err)
		}
		r.injectionsQuery = string(b)
	}
	if d.Queries.References != "" {
		b, err := fs.ReadFile(dialectFS, path.Join("dialects", d.ID, d.Queries.References))
		if err != nil {
			return nil, fmt.Errorf("dialect %s: %w", d.ID, err)
		}
		r.refsQuery = string(b)
	}
	if err := r.loadLocalsQuery(dialectFS, d); err != nil {
		return nil, err
	}
	r.marksMembers = strings.Contains(r.query, "@"+tsMemberCapture)
	for key, k := range d.Kinds {
		r.kinds[key] = tsDefKind{Kind: core.SourceKind(k.SourceKind), Container: k.Container, Callable: k.Callable}
	}
	return r, nil
}

// TSExtractor implements extractor.Extractor for one language using the
// wazero tree-sitter engine (internal/extractor/tsengine, ADR-021 §4). It is
// parameterized entirely by a tsLangSpec: the grammar(s) to parse with and a
// tree-sitter query (.scm) expressing which nodes are symbol definitions —
// there is no per-language Go code beyond that data (see tsRegistry above
// and internal/extractor/dialects/<id>/).
//
// Performance: a wazero module instantiation (fresh linear memory, data
// segments copied in) costs far more than a CGO ts.NewParser() call, so
// TSExtractor does NOT create one per Extract call the way the CGO
// extractors create a fresh *ts.Parser per call (treesitter.WithTree).
// Instead each (grammar, entry point) variant leases its Parser from the
// process-wide pool (tsengine.AcquireParser, LB-F15) under a key naming
// this extractor, with its compiled queries cached on that Parser: reused
// while the pool keeps it, closed when the pool's six-module LRU cap or its
// two-minute idle bound retires it, and re-instantiated on the next use. fusion's
// pass-3 extraction is documented as sequential/single-goroutine per
// Pipeline (internal/fusion/extract_sink.go's extractAndStore), so e.mu is
// essentially uncontended in practice; it exists so a TSExtractor
// shared across concurrent callers (e.g. two ingests on one long-lived
// daemon Pipeline) degrades to serialized correctness rather than wasm
// linear-memory corruption, never the reverse.
type TSExtractor struct {
	spec *tsLangSpec

	mu sync.Mutex
	// owner prefixes every pool key this extractor leases under, so its
	// Parsers (and the queries cached on them) are its own and Close
	// closes exactly them.
	owner string

	// faults counts engine faults (parse timeout, wasm trap, query-run
	// failure) over this extractor's lifetime (critic M1). Every fault also
	// drops the affected cached parser so the next Extract transparently
	// re-creates it — recovery is ALWAYS parser re-creation, never a
	// fallback to the CGO incumbent extractors (fallback doctrine, plan M2
	// ruling: the incumbents are slated for deletion and must not be a
	// load-bearing path). Surfaced per-ingest as fusion.Report.TSEngineFaults.
	faults atomic.Int64

	// typeRefs counts @reference.class/implementation/type captures over
	// this extractor's lifetime (LB-F4): measured in readings, not stored.
	typeRefs atomic.Int64

	// parses counts every source parse this extractor ran (extract, file
	// specs, injected ranges): the seam the one-parse test reads (EW-52).
	parses atomic.Int64
}

// Parses returns the source parses this extractor has run over its lifetime.
func (e *TSExtractor) Parses() int64 { return e.parses.Load() }

// tsParseTotal counts every tags-engine source parse in the process; ParseTotal
// reads it (EW-52, EW-53: a test counts the parses an ingest ran, whichever
// extractor instance ran them).
var tsParseTotal atomic.Int64

// ParseTotal is the number of tags-engine source parses this process has run.
func ParseTotal() int64 { return tsParseTotal.Load() }

// countParse records one source parse.
func (e *TSExtractor) countParse() { e.parses.Add(1); tsParseTotal.Add(1) }

// TypeRefs returns the type references (§3.4) this extractor's queries have
// captured over its lifetime; the harness reads it per file as a delta.
func (e *TSExtractor) TypeRefs() int64 { return e.typeRefs.Load() }

// Name is the registry key the extractor was built for (a tags dialect id,
// or an extractor group such as "javascript").
func (e *TSExtractor) Name() string { return e.spec.name }

// ErrTSEngineFault marks an Extract error as a tree-sitter engine fault
// (parse timeout on pathological input, a wasm trap, or a query-run
// failure) as opposed to a configuration error. fusion's extract loop
// counts errors matching this sentinel (errors.Is) on Report.TSEngineFaults;
// the affected file is fingerprinted like any other extract error (not
// retried until its content or the ExtractorEpoch changes), and the parser
// has already been dropped for re-creation by the time the error returns.
var ErrTSEngineFault = errors.New("tree-sitter engine fault")

// EngineFaults returns the number of engine faults over this extractor's
// lifetime. Zero on a healthy corpus; each fault corresponds to one file
// whose parse or query run was aborted and one parser re-creation.
func (e *TSExtractor) EngineFaults() int64 { return e.faults.Load() }

// noteFault records an engine fault and discards the (possibly dead)
// leased parser — and the queries cached on it — so the next Extract
// instantiates afresh. Caller must hold e.mu.
func (e *TSExtractor) noteFault(l *tsengine.Lease) {
	e.faults.Add(1)
	l.Discard() // best-effort; a timed-out module is already closed
}

// Close releases every grammar module this extractor leased (RT-107): its
// pooled parsers and the queries compiled inside them. The extractor stays
// usable — the next Extract leases afresh.
func (e *TSExtractor) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	tsengine.CloseParsers(e.owner)
}

// tsOwnerSeq numbers extractors for their pool keys.
var tsOwnerSeq atomic.Uint64

func newTSExtractorFromSpec(spec *tsLangSpec) *TSExtractor {
	return &TSExtractor{spec: spec, owner: fmt.Sprintf("tsx%d|", tsOwnerSeq.Add(1))}
}

// key identifies one (dialect, blob, entry point) triple for the
// parser/query caches — two variants sharing a blob but different entry
// points (the "typescript" grammar's tree_sitter_typescript vs
// tree_sitter_tsx) need their own Parser, since a Parser is bound to one
// TSLanguage at creation, and two dialects on one grammar compile different
// queries.
func (v tsVariant) key() string { return v.rules.id + "|" + v.blob + "|" + v.entryPoint }

// NewTSExtractor returns a TSExtractor for a tags-engine dialect id (every
// embedded dialects/<id>/dialect.json with engine "tags") or an extractor
// group (tsExtractorGroups: "javascript" spans the javascript and typescript
// dialects, the extensions fusion registers for the incumbent JSExtractor's
// combined scope).
func NewTSExtractor(name string) (*TSExtractor, error) {
	reg, err := tsRegistry()
	if err != nil {
		return nil, fmt.Errorf("tsextract: dialect registry: %w", err)
	}
	spec, ok := reg[name]
	if !ok {
		return nil, fmt.Errorf("tsextract: unknown language %q", name)
	}
	return newTSExtractorFromSpec(spec), nil
}

// leaseFor leases variant's pooled Parser and returns it with its compiled
// tags query, compiling the query on the Parser's first use. The caller
// releases the lease (or noteFault discards it). Caller must hold e.mu.
func (e *TSExtractor) leaseFor(ctx context.Context, variant tsVariant) (*tsengine.Lease, *tsengine.Query, error) {
	l, err := tsengine.AcquireParser(ctx, e.owner+variant.key(), variant.blob, variant.entryPoint)
	if err != nil {
		return nil, nil, fmt.Errorf("tsextract[%s]: %w", e.spec.name, err)
	}
	p := l.Parser()
	if q, ok := p.Cached("tags").(*tsengine.Query); ok {
		return l, q, nil
	}
	q, err := p.CompileQuery(variant.rules.query)
	if err != nil {
		l.Discard()
		return nil, nil, fmt.Errorf("tsextract[%s]: compiling %s: %w", e.spec.name, variant.rules.queryFile, err)
	}
	p.SetCached("tags", q)
	return l, q, nil
}

// extOf returns the lowercased extension (including the leading '.') of
// relPath, or "" if there is none.
func extOf(relPath string) string {
	for i := len(relPath) - 1; i >= 0; i-- {
		switch relPath[i] {
		case '.':
			return strings.ToLower(relPath[i:])
		case '/', '\\':
			return ""
		}
	}
	return ""
}

func (e *TSExtractor) variantFor(relPath string) tsVariant {
	if v, ok := e.spec.variantForExt[extOf(relPath)]; ok {
		return v
	}
	return e.spec.defaultVariant
}

// Extract parses src with this language's tree-sitter grammar, runs its
// query set, and turns the flat matches into core.SymbolNode via generic
// containment-based qualification (see tsDef/resolveParents/qualifyTSDefs
// below) — the same mechanism for every language, driven only by which
// capture kinds are marked Container/Callable in tsRegistry. Output is
// sorted by Line then Name for determinism, matching every other extractor.
func (e *TSExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	variant := e.variantFor(relPath)

	e.mu.Lock()
	defer e.mu.Unlock()

	nodes, _, err := e.extractNodesLocked(variant, service, relPath, src, nil)
	return nodes, err
}

// ExtractWithFileSpecs is Extract and ExtractFileSpecs from one parse and one
// tags-query run (EW-52, O361): nodes is exactly what Extract returns, and
// specs, when ok, exactly what ExtractFileSpecs returns for the same bytes.
// ok is false when specs could not be taken from this parse (a dialect
// preprocessing step swapped the tree, or the references query faulted); the
// caller then asks ExtractFileSpecs, which reproduces the answer or the fault.
func (e *TSExtractor) ExtractWithFileSpecs(service, relPath string, src []byte) (nodes []core.SymbolNode, specs FileSpecs, ok bool, err error) {
	variant := e.variantFor(relPath)

	e.mu.Lock()
	defer e.mu.Unlock()

	var out tsSpecsOut
	nodes, _, err = e.extractNodesLocked(variant, service, relPath, src, &out)
	return nodes, out.specs, out.ok, err
}

// tsSpecsOut receives the file specs extractLocked can read from its own
// parse.
type tsSpecsOut struct {
	specs FileSpecs
	ok    bool
}

// extractNodesLocked is Extract's body: the variant's symbols plus its
// injected ranges', sorted. Caller holds e.mu.
func (e *TSExtractor) extractNodesLocked(variant tsVariant, service, relPath string, src []byte, sp *tsSpecsOut) ([]core.SymbolNode, []tsInjection, error) {
	nodes, injected, err := e.extractLocked(variant, service, relPath, src, "", 0, true, sp)
	if err != nil {
		return nil, nil, err
	}
	if len(injected) > 0 {
		nodes = append(nodes, e.extractInjections(injected, service, relPath, src, nodes)...)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, injected, nil
}

// tsInjection is one embedded-language range an injections query found.
type tsInjection struct {
	dialect    string
	start, end uint32
	row        int // 0-based host row the range starts on
}

// extractLocked runs one variant over src and returns its symbols (unsorted)
// and, when withInjections, the injection ranges its injections query found.
// pkgOverride replaces the dialect's package rule when non-empty (an
// injected range takes its host file's package); lineOffset shifts every
// line (an injected range's first row in the host). Caller holds e.mu.
func (e *TSExtractor) extractLocked(variant tsVariant, service, relPath string, src []byte, pkgOverride string, lineOffset int, withInjections bool, sp *tsSpecsOut) ([]core.SymbolNode, []tsInjection, error) {
	ctx := context.Background()
	l, q, err := e.leaseFor(ctx, variant)
	if err != nil {
		return nil, nil, err
	}
	defer l.Release() // a no-op after noteFault's Discard
	p := l.Parser()

	e.countParse()
	tree, err := p.Parse(src)
	if err != nil {
		// Engine fault (critic M1 — this was previously a silent nil,nil
		// that fingerprinted the file AND kept reusing a possibly-dead
		// parser): count it, drop the parser for re-creation, and surface a
		// typed error the fusion pipeline counts per-ingest. The pipeline's
		// existing non-fatal extract-error path fingerprints the file so it
		// is not retried until its bytes or the ExtractorEpoch change.
		e.noteFault(l)
		return nil, nil, fmt.Errorf("tsextract[%s] %s: parse: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	defer func() { tree.Close() }() // the tree preprocessing swaps in, when it does

	rules := variant.rules
	swapped := false
	if rules.preprocess != nil {
		raw := tree
		if tree, src, err = tsPreprocessed(p, tree, src, rules.preprocess); err != nil {
			e.noteFault(l)
			return nil, nil, fmt.Errorf("tsextract[%s] %s: preprocess: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
		}
		swapped = tree != raw
		if swapped {
			e.countParse()
		}
	}

	matches, err := q.Run(tree)
	if err != nil {
		e.noteFault(l)
		return nil, nil, fmt.Errorf("tsextract[%s] %s: query: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	if rules.localsQuery != "" {
		matches = keepNonLocalCalls(q, src, matches) // LB-F254: a bare identifier is a call only where no local binds it
	}
	if sp != nil && !swapped {
		// The tree is the one ExtractFileSpecs would parse (src as written),
		// so its tags matches are that call's: take the specs before the
		// symbol pass reads them.
		if fs, serr := e.fileSpecsLocked(l, tree, rules, relPath, src, matches); serr == nil {
			sp.specs, sp.ok = fs, true
		}
	}

	var injected []tsInjection
	if withInjections && rules.injectionsQuery != "" {
		injected, err = e.injectionRanges(variant, l, tree, src)
		if err != nil {
			return nil, nil, err
		}
	}

	defs, calls, attaches, typeRefs := splitTSMatchesWith(matches, rules.kinds, src, rules.nameNormalizer(), rules.qualifyCalls)
	e.typeRefs.Add(int64(typeRefs))
	resolveTSParents(defs)
	qualifyTSDefs(defs, rules)
	defs = scopeTSMembers(matches, defs, src, rules) // LB-F241: @member marks (tsextract_members.go); no-op without them
	attachTSCalls(defs, calls, attaches...)

	pkg := pkgFromPath(relPath)
	switch {
	case pkgOverride != "":
		pkg = pkgOverride
	case rules.packageStrategy == "none":
		pkg = ""
	case rules.packageStrategy == "capture":
		tsPackageFromCapture(matches, src, defs, pkg)
	}
	nodes := make([]core.SymbolNode, 0, len(defs))
	for _, d := range defs {
		p := pkg
		if d.pkg != "" {
			p = d.pkg
		}
		n := core.SymbolNode{
			Service:  service,
			Package:  p,
			Name:     d.qualName,
			Kind:     d.kind,
			FilePath: relPath,
			Line:     d.line + lineOffset,
			EndLine:  tsEndLine(src, d) + lineOffset,
			Tier:     core.TierVerified,
			Calls:    d.calls,
		}
		if d.callable {
			n.Signature = tsSignature(src, d, rules.sigMax)
		}
		nodes = append(nodes, n)
	}
	return nodes, injected, nil
}

// tsPreprocessed applies a dialect's pre-parse step (O148): when tree has a
// syntax error, each text pre gives is parsed in turn, and the first tree
// with none replaces tree, its text src. A file that parses as written is
// read as written. Every tree not returned is closed.
func tsPreprocessed(p *tsengine.Parser, tree *tsengine.Tree, src []byte, pre func([]byte) [][]byte) (*tsengine.Tree, []byte, error) {
	bad, err := tsHasError(p, tree)
	if err != nil || !bad {
		return tree, src, err
	}
	for _, text := range pre(src) {
		t2, err := p.Parse(text)
		if err != nil {
			return tree, src, err
		}
		if bad, err = tsHasError(p, t2); err != nil || bad {
			t2.Close()
			if err != nil {
				return tree, src, err
			}
			continue
		}
		tree.Close()
		return t2, text, nil
	}
	return tree, src, nil
}

// tsHasError reports whether tree holds an ERROR or MISSING node (the
// query is compiled once per Parser and cached on it).
func tsHasError(p *tsengine.Parser, tree *tsengine.Tree) (bool, error) {
	q, ok := p.Cached("errors").(*tsengine.Query)
	if !ok {
		var err error
		if q, err = p.CompileQuery(tsErrorQuery); err != nil {
			return false, err
		}
		p.SetCached("errors", q)
	}
	ms, err := q.Run(tree)
	return len(ms) > 0, err
}

// injectionRanges runs variant's injections query over tree: every
// @injection.content capture whose pattern sets a static
// injection.language naming a registered tags dialect, sorted by start.
// Caller holds e.mu.
func (e *TSExtractor) injectionRanges(variant tsVariant, l *tsengine.Lease, tree *tsengine.Tree, src []byte) ([]tsInjection, error) {
	p := l.Parser()
	iq, ok := p.Cached("injections").(*tsengine.Query)
	if !ok {
		var err error
		iq, err = p.CompileQuery(variant.rules.injectionsQuery)
		if err != nil {
			return nil, fmt.Errorf("tsextract[%s]: compiling injections of %s: %w", e.spec.name, variant.rules.id, err)
		}
		p.SetCached("injections", iq)
	}
	matches, err := iq.Run(tree)
	if err != nil {
		e.noteFault(l)
		return nil, fmt.Errorf("tsextract[%s]: injections query: %v: %w", e.spec.name, err, ErrTSEngineFault)
	}
	if _, err := tsRegistry(); err != nil {
		return nil, err
	}
	var out []tsInjection
	for _, m := range matches {
		lang := iq.PatternSetting(m.PatternIndex, "injection.language")
		if _, known := tsDialectMap[lang]; !known {
			continue
		}
		for _, c := range m.Captures {
			if c.Name == "injection.content" && c.Node.EndByte > c.Node.StartByte && int(c.Node.EndByte) <= len(src) {
				out = append(out, tsInjection{dialect: lang, start: c.Node.StartByte, end: c.Node.EndByte, row: int(c.Node.StartRow)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].start != out[j].start {
			return out[i].start < out[j].start
		}
		return out[i].dialect < out[j].dialect
	})
	return out, nil
}

// extractInjections (LB-F6, design §3.6) runs each injected dialect over
// its byte range as its own document: lines offset into the host, the host
// file's package, one level deep (an injected range's own injections are
// not run), and no qualification across the boundary — injected symbols
// never nest under host definitions. A range whose extraction faults is
// skipped (the fault is counted); the host's symbols stand. Caller holds
// e.mu.
func (e *TSExtractor) extractInjections(injected []tsInjection, service, relPath string, src []byte, host []core.SymbolNode) []core.SymbolNode {
	pkg := pkgFromPath(relPath)
	if len(host) > 0 && host[0].Package != "" {
		pkg = host[0].Package
	}
	var out []core.SymbolNode
	for _, inj := range injected {
		v := tsDialectMap[inj.dialect].defaultVariant
		nodes, _, err := e.extractLocked(v, service, relPath, src[inj.start:inj.end], pkg, inj.row, false, nil)
		if err != nil {
			continue
		}
		out = append(out, nodes...)
	}
	return out
}

// tsDef is one query-captured symbol definition, positioned by byte range so
// containment (nesting) can be recovered without ever walking the tree
// directly — the query already told us exactly which nodes matter.
type tsDef struct {
	kind       core.SourceKind
	container  bool
	callable   bool
	name       string
	start, end uint32
	line       int
	// defLine is the DEFINITION node's own start line (defNode.Line()),
	// independent of `line` above (nameNode.Line() -- see splitTSMatchesWith).
	// tsEndLine adds its within-range newline count to THIS, never to
	// `line`: the two diverge exactly when N names share one definition
	// (CSS's grouped selector list is the only case today), and using
	// `line` there double-counts the gap between the name and the
	// definition's own start, both for group members after the first.
	defLine   int
	parent    int    // index into the defs slice, -1 if none
	bodyStart uint32 // start of the @body capture, when hasBody
	hasBody   bool
	pkg       string // @scope.package's answer (LB-F5), "" for the file default
	qualName  string
	callSet   map[string]struct{}
	calls     []string
}

type tsCallSite struct {
	text       string
	start, end uint32
}

// tsCallAttach gives the calls inside from (a decorator, a module-level
// variable's statement) to the definition whose node spans exactly to — a
// call no callable definition encloses still has an owner (LB-F65). A query
// says so with one match capturing @calls.to (that definition's node) and
// @calls.from; a dialect whose query has neither is unaffected.
type tsCallAttach struct {
	toStart, toEnd     uint32
	fromStart, fromEnd uint32
}

// typeRefCaptures are the reference captures §3.4 counts but does not store
// until a consumer exists.
var typeRefCaptures = map[string]bool{"reference.class": true, "reference.implementation": true, "reference.type": true}

// tsCalleeName is a call site's callee (LB-F4, design §3.4): the @name
// captured inside the match's @reference.call when the query has one (the
// tags convention), else the @reference.call node's own text (the five
// pre-manifest queries capture the callee expression itself). ok is false
// when the match has no @reference.call.
func tsCalleeName(m tsengine.Match, src []byte) (callee string, site tsengine.Node, ok bool) {
	var name *tsengine.Node
	for i := range m.Captures {
		c := &m.Captures[i]
		switch c.Name {
		case "reference.call":
			site, ok = c.Node, true
		case "name":
			n := c.Node
			name = &n
		}
	}
	if !ok {
		return "", site, false
	}
	if name != nil && name.StartByte >= site.StartByte && name.EndByte <= site.EndByte {
		return name.Text(src), site, true
	}
	return site.Text(src), site, true
}

// qualifiedCallee is a call site's callee under the manifest's
// calls.qualify (LB-F65): the called name joined to its receiver
// (@call.receiver, by ".") or its scope (@call.scope, by "::") when the
// match captures one inside @reference.call, else the whole callee
// expression compacted (compactCallee). Either way the text after the last
// '.' or ':' is the called name, as BareName and the store's CalleeName
// read it.
func qualifiedCallee(m tsengine.Match, src []byte, callee string, site tsengine.Node) string {
	var named bool
	var recv, scope *tsengine.Node
	for i := range m.Captures {
		c := &m.Captures[i]
		if c.Node.StartByte < site.StartByte || c.Node.EndByte > site.EndByte {
			continue
		}
		switch c.Name {
		case "name":
			named = true
		case "call.receiver":
			n := c.Node
			recv = &n
		case "call.scope":
			n := c.Node
			scope = &n
		}
	}
	if !named {
		return compactCallee(callee)
	}
	switch {
	case scope != nil:
		if q, ok := compactExpr(scope.Text(src), true); ok && q != "" {
			return q + "::" + callee
		}
	case recv != nil:
		if q, ok := compactExpr(recv.Text(src), true); ok && q != "" && q != "()" {
			return q + "." + callee
		}
		return "()." + callee // a receiver with no name of its own: (new Foo)->bar()
	}
	return callee
}

// compactCallee is a whole callee expression as stored call text (LB-F65):
// compactExpr's, unless it ends in no name (a subscript or a call's result
// called), when it names no callee: "". A text whose groups the compaction
// cannot follow (a quote or bracket inside a comment or a regular
// expression) is kept as written, whitespace collapsed: its last segment is
// still the called name.
func compactCallee(text string) string {
	out, ok := compactExpr(text, false)
	if !ok {
		return strings.Join(strings.Fields(text), "")
	}
	if strings.HasSuffix(out, ")") || strings.HasSuffix(out, "]") || strings.HasSuffix(out, "}") {
		return ""
	}
	return out
}

// compactExpr is a callee, receiver or scope expression compacted (LB-F65):
// whitespace dropped, the contents of every (), [] and {} group dropped
// (string literals inside them skipped; a ' after a letter or digit is an
// apostrophe, not a quote), `->` and `?->` (PHP) and `?.` (TypeScript) read
// as '.', TypeScript's non-null `!` after a name or group dropped (LB-F172),
// and with sigils PHP's `$` before a name dropped. ok is false when a group
// never closes.
func compactExpr(text string, sigils bool) (string, bool) {
	var b strings.Builder
	depth := 0
	var quote byte
	for i := 0; i < len(text); i++ {
		c := text[i]
		if depth > 0 {
			switch {
			case quote != 0:
				if c == '\\' {
					i++
				} else if c == quote {
					quote = 0
				}
			case c == '"' || c == '`' || (c == '\'' && (i == 0 || !isIdentByte(text[i-1]))):
				quote = c
			case c == '(' || c == '[' || c == '{':
				depth++
			case c == ')' || c == ']' || c == '}':
				depth--
				if depth == 0 {
					b.WriteByte(c)
				}
			}
			continue
		}
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
		case c == '(' || c == '[' || c == '{':
			depth++
			b.WriteByte(c)
		case sigils && c == '$' && i+1 < len(text) && isIdentByte(text[i+1]):
		case c == '?' && i+1 < len(text) && text[i+1] == '.':
		case c == '!' && i > 0 && (isIdentByte(text[i-1]) || text[i-1] == '$' || text[i-1] == ')' || text[i-1] == ']') && (i+1 == len(text) || text[i+1] != '='):
			// TypeScript's non-null assertion (`a!.b`, `f!()`) is no part of a name.
		case c == '?' && i+2 < len(text) && text[i+1] == '-' && text[i+2] == '>':
		case c == '-' && i+1 < len(text) && text[i+1] == '>':
			b.WriteByte('.')
			i++
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), depth == 0
}

// splitTSMatchesWith separates a query's flat match list into definitions (a
// "name" + "definition.<kind>" capture pair) and call sites (a
// "reference.call" capture, its callee per tsCalleeName, qualified under the
// manifest's calls.qualify) using src to resolve capture text, returns the
// query's call attachments (@calls.to, @calls.from), and counts type
// references (typeRefCaptures).
func splitTSMatchesWith(matches []tsengine.Match, kinds map[string]tsDefKind, src []byte, norm func(string) string, qualify bool) ([]*tsDef, []tsCallSite, []tsCallAttach, int) {
	var defs []*tsDef
	var calls []tsCallSite
	var attaches []tsCallAttach
	typeRefs := 0
	for _, m := range matches {
		if callee, site, ok := tsCalleeName(m, src); ok {
			if qualify {
				callee = qualifiedCallee(m, src, callee, site)
			}
			if norm != nil && callee != "" {
				callee = norm(callee)
			}
			if callee != "" {
				calls = append(calls, tsCallSite{text: callee, start: site.StartByte, end: site.EndByte})
			}
			continue
		}
		if a, ok := tsCallAttachOf(m); ok {
			attaches = append(attaches, a)
			continue
		}
		var nameNode, defNode, bodyNode, endNode *tsengine.Node
		var defSuffix string
		for i := range m.Captures {
			c := &m.Captures[i]
			switch {
			case typeRefCaptures[c.Name]:
				typeRefs++
			case c.Name == "name":
				n := c.Node
				nameNode = &n
			case c.Name == "body":
				n := c.Node
				bodyNode = &n
			case c.Name == "end":
				n := c.Node
				endNode = &n
			case strings.HasPrefix(c.Name, "definition."):
				n := c.Node
				defNode = &n
				defSuffix = strings.TrimPrefix(c.Name, "definition.")
			}
		}
		if defNode == nil || nameNode == nil {
			continue
		}
		dk, ok := kinds[defSuffix]
		if !ok {
			continue
		}
		name := nameNode.Text(src)
		if norm != nil {
			name = norm(name)
		}
		if name == "" {
			continue
		}
		d := &tsDef{
			kind:      dk.Kind,
			container: dk.Container,
			callable:  dk.Callable,
			name:      name,
			start:     defNode.StartByte,
			end:       defNode.EndByte,
			// The definition's OWN name's row, not the (possibly
			// shared) definition node's row: for a single name per
			// definition -- every kind but one -- the two coincide, so
			// this changes nothing. CSS's grouped selector list is the
			// exception a query alone cannot express: one rule_set
			// (@definition.selector, needed whole for a correct
			// EndLine -- the rule's closing brace) produces N matches,
			// one per comma-separated @name, each on its OWN source
			// line when the group spans multiple lines. Before this,
			// every selector past the first inherited the rule_set's
			// STARTING line -- up to many lines wrong, both in the
			// store (a wrong find_symbol/references location) and
			// against ctags, which reports each selector's real line
			// (design section 1.4's matching tolerance is |line delta| <= 2).
			line:    nameNode.Line(),
			defLine: defNode.Line(),
			parent:  -1,
		}
		// @end: a grammar that puts a callable's body in a sibling of its
		// signature (tree-sitter-dart) has no node spanning both, so the match
		// captures the sibling and the definition reaches its end. Never shorter.
		if endNode != nil && endNode.StartByte >= d.start && endNode.EndByte > d.end {
			d.end = endNode.EndByte
		}
		if bodyNode != nil && bodyNode.StartByte >= d.start && bodyNode.StartByte <= d.end {
			d.bodyStart, d.hasBody = bodyNode.StartByte, true
		}
		defs = append(defs, d)
	}
	return defs, calls, attaches, typeRefs
}

// tsCallAttachOf reads a match capturing @calls.to and @calls.from.
func tsCallAttachOf(m tsengine.Match) (tsCallAttach, bool) {
	var to, from *tsengine.Node
	for i := range m.Captures {
		c := &m.Captures[i]
		switch c.Name {
		case "calls.to":
			n := c.Node
			to = &n
		case "calls.from":
			n := c.Node
			from = &n
		}
	}
	if to == nil || from == nil {
		return tsCallAttach{}, false
	}
	return tsCallAttach{toStart: to.StartByte, toEnd: to.EndByte, fromStart: from.StartByte, fromEnd: from.EndByte}, true
}

// tsPackageFromCapture applies @scope.package (LB-F5, design §3.5) for a
// manifest with package.strategy "capture". A capture whose match has no
// @scope range is a file-wide package clause (Kotlin, Java, PHP): it
// answers every definition. One whose match also captures @scope (a block
// namespace, C#'s shape) answers the definitions inside that range, nested
// ranges joined with "."; the innermost range chain wins over the file-wide
// clause. A file with no capture keeps fallback (pkgFromPath).
func tsPackageFromCapture(matches []tsengine.Match, src []byte, defs []*tsDef, fallback string) {
	type scoped struct {
		name       string
		start, end uint32
	}
	fileWide := ""
	var scopes []scoped
	for _, m := range matches {
		var name, scope *tsengine.Node
		for i := range m.Captures {
			c := &m.Captures[i]
			switch c.Name {
			case "scope.package":
				n := c.Node
				name = &n
			case "scope":
				n := c.Node
				scope = &n
			}
		}
		if name == nil {
			continue
		}
		text := strings.Join(strings.Fields(name.Text(src)), "")
		if text == "" {
			continue
		}
		if scope == nil {
			if fileWide == "" {
				fileWide = text
			}
			continue
		}
		scopes = append(scopes, scoped{name: text, start: scope.StartByte, end: scope.EndByte})
	}
	sort.SliceStable(scopes, func(i, j int) bool { return scopes[i].start < scopes[j].start })
	for _, d := range defs {
		var parts []string
		for _, s := range scopes {
			if d.start >= s.start && d.end <= s.end {
				parts = append(parts, s.name) // outer ranges sort first
			}
		}
		switch {
		case len(parts) > 0:
			d.pkg = strings.Join(parts, ".")
		case fileWide != "":
			d.pkg = fileWide
		default:
			d.pkg = fallback
		}
	}
}

// tsEndLine is the 1-based line of the definition node's last byte (§3.3).
func tsEndLine(src []byte, d *tsDef) int {
	if d.end <= d.start || int(d.end) > len(src) {
		return d.line
	}
	// defLine, not line: they diverge when N names share one definition
	// (see tsDef.defLine's doc) and the newline count below is counted
	// from the DEFINITION's own start (d.start), never the name's.
	return d.defLine + bytes.Count(src[d.start:d.end-1], []byte{'\n'})
}

// tsSignature is a callable's signature (LB-F3, design §3.3): the
// definition's source from its start to the start of its @body capture, or
// its first line when the query captured no body; whitespace runs collapsed
// to one space, trimmed, and capped at maxChars runes with a trailing "…".
func tsSignature(src []byte, d *tsDef, maxChars int) string {
	end := d.end
	if d.hasBody {
		end = d.bodyStart
	} else if nl := bytes.IndexByte(src[d.start:d.end], '\n'); nl >= 0 {
		end = d.start + uint32(nl)
	}
	if end <= d.start || int(end) > len(src) {
		return ""
	}
	sig := strings.Join(strings.Fields(string(src[d.start:end])), " ")
	if maxChars <= 0 {
		maxChars = 200
	}
	if utf8.RuneCountInString(sig) > maxChars {
		r := []rune(sig)
		sig = strings.TrimRight(string(r[:maxChars-1]), " ") + "…"
	}
	return sig
}

// resolveTSParents computes each def's immediate structural parent (the
// smallest def whose byte range strictly contains it) via a standard
// ascending-start / stack-based interval containment pass — O(n log n).
func resolveTSParents(defs []*tsDef) {
	order := make([]int, len(defs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool {
		da, db := defs[order[a]], defs[order[b]]
		if da.start != db.start {
			return da.start < db.start
		}
		return da.end > db.end // wider range first when tied, so it's seen as the outer one
	})

	var stack []int
	for _, idx := range order {
		d := defs[idx]
		for len(stack) > 0 && d.start >= defs[stack[len(stack)-1]].end {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			d.parent = stack[len(stack)-1]
		}
		stack = append(stack, idx)
	}
}

// qualifyTSDefs computes each def's qualified name. Containers qualify
// against their own enclosing container only when the language chains
// nested containers (Ruby's "Outer::Inner"); callables ("function"/
// "method") qualify as "Owner.name" ONLY when their immediate parent is
// itself a container — a function nested inside another function never
// inherits a class name, matching every incumbent CGO extractor's scope
// rules exactly (see pyextract.go/jsextract.go/rubyextract.go).
func qualifyTSDefs(defs []*tsDef, spec *tsDialectRules) {
	resolved := make([]bool, len(defs))
	var resolve func(i int) string
	resolve = func(i int) string {
		if resolved[i] {
			return defs[i].qualName
		}
		d := defs[i]
		var q string
		switch {
		case d.container:
			if spec.chainContainers && d.parent >= 0 && defs[d.parent].container {
				q = resolve(d.parent) + spec.containerSep + d.name
			} else {
				q = d.name
			}
		case d.callable:
			if d.parent >= 0 && defs[d.parent].container {
				q = resolve(d.parent) + spec.methodSep + d.name
			} else {
				q = d.name
			}
		default:
			q = d.name
		}
		defs[i].qualName = q
		resolved[i] = true
		return q
	}
	for i := range defs {
		resolve(i)
	}
}

// attachTSCalls assigns each call site to its smallest enclosing callable
// def (the naive same-file collection the incumbent extractors already do
// via treesitter.CollectCalls — not a typed/resolved edge; ADR-021 §3's
// typed Go enrichment pass is unaffected). Deduplicated and sorted, matching
// CollectCalls's contract exactly.
//
// An attachment (@calls.to/@calls.from, LB-F65) gives a call inside its
// from range to its definition when no callable definition encloses the
// call more tightly than that range: a decorator's call goes to the
// definition it decorates, a module-level assignment's to its variable.
func attachTSCalls(defs []*tsDef, calls []tsCallSite, attaches ...tsCallAttach) {
	for _, c := range calls {
		best := -1
		for i, d := range defs {
			if !d.callable {
				continue
			}
			if c.start >= d.start && c.end <= d.end {
				if best == -1 || (defs[best].end-defs[best].start) > (d.end-d.start) {
					best = i
				}
			}
		}
		var span uint32
		if best >= 0 {
			span = defs[best].end - defs[best].start
		}
		for _, a := range attaches {
			if c.start < a.fromStart || c.end > a.fromEnd || (best >= 0 && a.fromEnd-a.fromStart >= span) {
				continue
			}
			for i, d := range defs {
				if d.start == a.toStart && d.end == a.toEnd {
					best, span = i, a.fromEnd-a.fromStart
					break
				}
			}
		}
		if best < 0 {
			continue
		}
		d := defs[best]
		if d.callSet == nil {
			d.callSet = map[string]struct{}{}
		}
		d.callSet[c.text] = struct{}{}
	}
	for _, d := range defs {
		if len(d.callSet) == 0 {
			continue
		}
		out := make([]string, 0, len(d.callSet))
		for text := range d.callSet {
			out = append(out, text)
		}
		sort.Strings(out)
		d.calls = out
	}
}

// EngineProvenance reports, for every extension this migration touched, which
// extraction engine actually runs — "tree-sitter-wasm" (this package,
// wazero-executed, ADR-021 §4) once a language's calibration score beat its
// incumbent (docs/plans/extraction-treesitter.md XT-2/XT-3/XT-4), or
// "cgo-tree-sitter" (ADR-013) for a language whose incumbent was kept because
// it wasn't beaten. Surfaced via the `capabilities` MCP tool (XT-5) so a
// caller can tell which evidence tier's mechanism produced a given file's
// symbols without reading source. This is deliberately scoped to the
// extensions this plan migrated, not a full 24-language registry — Go
// (go/parser), the config/IaC formats, and the languages still on their
// original heuristic/CGO path are out of scope for this map.
//
// Only extensions the connector actually discovers AND fusion registers are
// claimed (critic M3): .pyi is parseable by this engine (the tsVariant
// table maps it) but the connector's ext->kind registry and fusion.New's
// dispatch don't know it, so no such file ever reaches an extractor —
// claiming it here would be provenance for an index entry that cannot
// exist. Registering it is a deliberate scope-addition for a future slice.
//
// .mjs/.cjs/.mts/.cts (J1, app-kit O18/O32/O33) joined this map once THAT
// future slice landed: connector.extClassMap classifies them, fusion.New
// dispatches them to this same "javascript" TSExtractor, and
// TestTSExtractor_MjsExports/_CjsModuleExports/_MtsCtsRouting plus
// TestIngest_MjsCjsMtsCtsSymbolsIndexed (internal/fusion) pin real symbols
// indexed end to end.
func EngineProvenance() map[string]string {
	out := map[string]string{}
	ds, err := LoadDialects()
	if err != nil {
		return out
	}
	for _, d := range ds {
		if d.Engine != "tags" && d.Engine != "tags+doc" {
			continue
		}
		for _, e := range dialectExtensions(d) {
			out[e] = "tree-sitter-wasm"
		}
	}
	return out
}
