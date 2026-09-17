package extractor

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// tsQueryFS embeds every language's tree-sitter query set (.scm) — the only
// per-language "logic" TSExtractor needs beyond the shared engine (ADR-021
// §4 / docs/plans/extraction-treesitter.md: "adding a language = grammar
// blob + queries + fixtures, no new Go extraction logic").
//
//go:embed tsqueries/*.scm
var tsQueryFS embed.FS

// tsVariant is one (grammar blob, entry point, query file) triple a
// TSExtractor may parse a file with. A single TSExtractor can span several
// variants keyed by file extension — e.g. the "javascript" language spec
// parses .ts/.tsx with the "typescript" grammar and its richer query set,
// mirroring the incumbent JSExtractor's combined JS/TS/JSX/TSX scope.
type tsVariant struct {
	blob       string // tsengine grammar registry key
	entryPoint string // exported tree_sitter_<x> function
	queryFile  string // tsqueries/<name>.scm
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

// tsLangSpec is everything NewTSExtractor needs for one language.
type tsLangSpec struct {
	name  string
	kinds map[string]tsDefKind // capture suffix -> mapping

	// variantForExt selects the (grammar, entry point, query file) for a
	// given lowercased file extension (including the leading '.'). extOf
	// returning an extension absent from this map falls back to
	// defaultVariant.
	variantForExt  map[string]tsVariant
	defaultVariant tsVariant

	// chainContainers: true for languages where nested containers qualify
	// as "Outer::Inner" (Ruby's module/class nesting). false: only the
	// immediate container's bare name is used (Python, JS/TS) — matching
	// the incumbent CGO extractors' single-level scope tracking exactly.
	chainContainers bool
	containerSep    string // separator when chainContainers is true
	methodSep       string // separator between a container's qualified name and a callable's own name
}

var tsRegistry = map[string]*tsLangSpec{}

func init() {
	jsVariant := tsVariant{blob: "javascript", entryPoint: "tree_sitter_javascript", queryFile: "javascript.scm"}
	tsVariantTS := tsVariant{blob: "typescript", entryPoint: "tree_sitter_typescript", queryFile: "typescript.scm"}
	tsVariantTSX := tsVariant{blob: "typescript", entryPoint: "tree_sitter_tsx", queryFile: "typescript.scm"}

	tsRegistry["javascript"] = &tsLangSpec{
		name: "javascript",
		kinds: map[string]tsDefKind{
			"class":     {Kind: core.KindJSClass, Container: true},
			"function":  {Kind: core.KindJSFunc, Callable: true},
			"method":    {Kind: core.KindJSMethod, Callable: true},
			"interface": {Kind: core.KindTSInterface},
			"type":      {Kind: core.KindTSType},
			"enum":      {Kind: core.KindTSEnum},
		},
		variantForExt: map[string]tsVariant{
			".js":  jsVariant,
			".jsx": jsVariant,
			".mjs": jsVariant,
			".cjs": jsVariant,
			".ts":  tsVariantTS,
			".tsx": tsVariantTSX,
		},
		defaultVariant: jsVariant,
		methodSep:      ".",
	}

	tsRegistry["python"] = &tsLangSpec{
		name: "python",
		kinds: map[string]tsDefKind{
			"class":    {Kind: core.KindPyClass, Container: true},
			"function": {Kind: core.KindPyFunc, Callable: true},
			"variable": {Kind: core.KindPyVar},
		},
		defaultVariant: tsVariant{blob: "python", entryPoint: "tree_sitter_python", queryFile: "python.scm"},
		methodSep:      ".",
	}

	tsRegistry["ruby"] = &tsLangSpec{
		name: "ruby",
		kinds: map[string]tsDefKind{
			"module": {Kind: core.KindRubyModule, Container: true},
			"class":  {Kind: core.KindRubyClass, Container: true},
			"method": {Kind: core.KindRubyMethod, Callable: true},
		},
		defaultVariant:  tsVariant{blob: "ruby", entryPoint: "tree_sitter_ruby", queryFile: "ruby.scm"},
		chainContainers: true,
		containerSep:    "::",
		methodSep:       ".",
	}

	// Kotlin (wave P2, ADR-021 §4 new-language breadth — no CGO incumbent to
	// calibrate against; see grammars/PROVENANCE.md). .kt and .kts share the
	// one fwcd/tree-sitter-kotlin grammar (Kotlin scripts are valid Kotlin
	// syntax under the same parser). chainContainers is true (Kotlin nested
	// classes/objects are idiomatic, "Outer.Inner"), matching Java's own
	// incumbent extractor's qualifyJava behavior for the same shape.
	kotlinVariant := tsVariant{blob: "kotlin", entryPoint: "tree_sitter_kotlin", queryFile: "kotlin.scm"}
	tsRegistry["kotlin"] = &tsLangSpec{
		name: "kotlin",
		kinds: map[string]tsDefKind{
			"class":     {Kind: core.KindKotlinClass, Container: true},
			"interface": {Kind: core.KindKotlinInterface, Container: true},
			"object":    {Kind: core.KindKotlinObject, Container: true},
			"function":  {Kind: core.KindKotlinFunction, Callable: true},
			"property":  {Kind: core.KindKotlinProperty},
			"import":    {Kind: core.KindKotlinImport},
		},
		variantForExt: map[string]tsVariant{
			".kt":  kotlinVariant,
			".kts": kotlinVariant,
		},
		defaultVariant:  kotlinVariant,
		chainContainers: true,
		containerSep:    ".",
		methodSep:       ".",
	}
}

// TSExtractor implements extractor.Extractor for one language using the
// wazero tree-sitter engine (internal/extractor/tsengine, ADR-021 §4). It is
// parameterized entirely by a tsLangSpec: the grammar(s) to parse with and a
// tree-sitter query (.scm) expressing which nodes are symbol definitions —
// there is no per-language Go code beyond that data (see tsRegistry above
// and internal/extractor/tsqueries/*.scm).
//
// Performance: a wazero module instantiation (fresh linear memory, data
// segments copied in) costs far more than a CGO ts.NewParser() call, so
// TSExtractor does NOT create one per Extract call the way the CGO
// extractors create a fresh *ts.Parser per call (treesitter.WithTree).
// Instead each (grammar, entry point) variant gets exactly one live
// tsengine.Parser + compiled Query, created lazily on first use and reused
// for the process's lifetime — the same long-lived-resource shape as the
// CGO extractors' package-level `var pyLang = ts.NewLanguage(...)`. fusion's
// pass-3 extraction is documented as sequential/single-goroutine per
// Pipeline (internal/fusion/pipeline.go's extractAndStore), so e.mu is
// essentially uncontended in practice; it exists so a TSExtractor
// shared across concurrent callers (e.g. two ingests on one long-lived
// daemon Pipeline) degrades to serialized correctness rather than wasm
// linear-memory corruption, never the reverse.
type TSExtractor struct {
	spec    *tsLangSpec
	queries map[string]string // queryFile -> source text (read once from the embedded FS)

	mu        sync.Mutex
	parsers   map[string]*tsengine.Parser // variant key -> live parser
	tsQueries map[string]*tsengine.Query  // variant key -> compiled query bound to that parser

	// faults counts engine faults (parse timeout, wasm trap, query-run
	// failure) over this extractor's lifetime (critic M1). Every fault also
	// drops the affected cached parser so the next Extract transparently
	// re-creates it — recovery is ALWAYS parser re-creation, never a
	// fallback to the CGO incumbent extractors (fallback doctrine, plan M2
	// ruling: the incumbents are slated for deletion and must not be a
	// load-bearing path). Surfaced per-ingest as fusion.Report.TSEngineFaults.
	faults atomic.Int64
}

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

// noteFault records an engine fault and drops the (possibly dead) cached
// parser+query for variant so the next Extract rebuilds them. Caller must
// hold e.mu.
func (e *TSExtractor) noteFault(variant tsVariant) {
	e.faults.Add(1)
	key := variant.key()
	if p, ok := e.parsers[key]; ok {
		_ = p.Close() // best-effort; a timed-out module is already closed
		delete(e.parsers, key)
	}
	// The query object lives inside the same (now dropped) module instance —
	// no separate close possible or needed.
	delete(e.tsQueries, key)
}

// variantKey identifies one (blob, entry point) pair for the parser/query
// caches — two variants sharing a blob but different entry points (the
// "typescript" grammar's tree_sitter_typescript vs tree_sitter_tsx) need
// their own Parser, since a Parser is bound to one TSLanguage at creation.
func (v tsVariant) key() string { return v.blob + "|" + v.entryPoint }

// NewTSExtractor returns a TSExtractor for the named language ("javascript",
// "python", or "ruby" — see tsRegistry). "javascript" spans .js/.jsx/.ts/
// .tsx (the extensions fusion registers — the incumbent JSExtractor's
// combined scope); its variant table also maps .mjs/.cjs so the engine is
// ready if the connector ever learns those extensions, but they are NOT
// claimed as indexed today (see EngineProvenance).
func NewTSExtractor(name string) (*TSExtractor, error) {
	spec, ok := tsRegistry[name]
	if !ok {
		return nil, fmt.Errorf("tsextract: unknown language %q", name)
	}
	e := &TSExtractor{
		spec:      spec,
		queries:   map[string]string{},
		parsers:   map[string]*tsengine.Parser{},
		tsQueries: map[string]*tsengine.Query{},
	}
	seen := map[string]bool{}
	for _, v := range allVariants(spec) {
		if seen[v.queryFile] {
			continue
		}
		seen[v.queryFile] = true
		src, err := tsQueryFS.ReadFile("tsqueries/" + v.queryFile)
		if err != nil {
			return nil, fmt.Errorf("tsextract: read query %q: %w", v.queryFile, err)
		}
		e.queries[v.queryFile] = string(src)
	}
	return e, nil
}

// parserFor returns the cached (Parser, Query) pair for variant, creating
// both on first use. Caller must hold e.mu.
func (e *TSExtractor) parserFor(ctx context.Context, variant tsVariant) (*tsengine.Parser, *tsengine.Query, error) {
	key := variant.key()
	if p, ok := e.parsers[key]; ok {
		return p, e.tsQueries[key], nil
	}
	p, err := tsengine.NewParser(ctx, variant.blob, variant.entryPoint)
	if err != nil {
		return nil, nil, fmt.Errorf("tsextract[%s]: %w", e.spec.name, err)
	}
	q, err := p.CompileQuery(e.queries[variant.queryFile])
	if err != nil {
		p.Close()
		return nil, nil, fmt.Errorf("tsextract[%s]: compiling %s: %w", e.spec.name, variant.queryFile, err)
	}
	e.parsers[key] = p
	e.tsQueries[key] = q
	return p, q, nil
}

func allVariants(spec *tsLangSpec) []tsVariant {
	variants := []tsVariant{spec.defaultVariant}
	for _, v := range spec.variantForExt {
		variants = append(variants, v)
	}
	return variants
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

	ctx := context.Background()
	p, q, err := e.parserFor(ctx, variant)
	if err != nil {
		return nil, err
	}

	tree, err := p.Parse(src)
	if err != nil {
		// Engine fault (critic M1 — this was previously a silent nil,nil
		// that fingerprinted the file AND kept reusing a possibly-dead
		// parser): count it, drop the parser for re-creation, and surface a
		// typed error the fusion pipeline counts per-ingest. The pipeline's
		// existing non-fatal extract-error path fingerprints the file so it
		// is not retried until its bytes or the ExtractorEpoch change.
		e.noteFault(variant)
		return nil, fmt.Errorf("tsextract[%s] %s: parse: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	defer tree.Close()

	matches, err := q.Run(tree)
	if err != nil {
		e.noteFault(variant)
		return nil, fmt.Errorf("tsextract[%s] %s: query: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}

	defs, calls := splitTSMatches(matches, e.spec.kinds, src)
	resolveTSParents(defs)
	qualifyTSDefs(defs, e.spec)
	attachTSCalls(defs, calls)

	pkg := pkgFromPath(relPath)
	nodes := make([]core.SymbolNode, 0, len(defs))
	for _, d := range defs {
		nodes = append(nodes, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     d.qualName,
			Kind:     d.kind,
			FilePath: relPath,
			Line:     d.line,
			Tier:     core.TierVerified,
			Calls:    d.calls,
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
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
	parent     int // index into the defs slice, -1 if none
	qualName   string
	callSet    map[string]struct{}
	calls      []string
}

type tsCallSite struct {
	text       string
	start, end uint32
}

// splitTSMatches separates a query's flat match list into definitions (a
// "name" + "definition.<kind>" capture pair) and call sites (a bare
// "reference.call" capture) using src to resolve capture text.
func splitTSMatches(matches []tsengine.Match, kinds map[string]tsDefKind, src []byte) ([]*tsDef, []tsCallSite) {
	var defs []*tsDef
	var calls []tsCallSite
	for _, m := range matches {
		var nameNode *tsengine.Node
		var defNode *tsengine.Node
		var defSuffix string
		for i := range m.Captures {
			c := &m.Captures[i]
			switch {
			case c.Name == "name":
				n := c.Node
				nameNode = &n
			case c.Name == "reference.call":
				text := c.Node.Text(src)
				if text != "" {
					calls = append(calls, tsCallSite{text: text, start: c.Node.StartByte, end: c.Node.EndByte})
				}
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
		if name == "" {
			continue
		}
		defs = append(defs, &tsDef{
			kind:      dk.Kind,
			container: dk.Container,
			callable:  dk.Callable,
			name:      name,
			start:     defNode.StartByte,
			end:       defNode.EndByte,
			line:      defNode.Line(),
			parent:    -1,
		})
	}
	return defs, calls
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
func qualifyTSDefs(defs []*tsDef, spec *tsLangSpec) {
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
func attachTSCalls(defs []*tsDef, calls []tsCallSite) {
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
// claimed (critic M3): .pyi/.mjs/.cjs are parseable by this engine (the
// tsVariant table maps them) but the connector's ext->kind registry and
// fusion.New's dispatch don't know them, so no such file ever reaches an
// extractor — claiming them here would be provenance for an index entry
// that cannot exist. Registering those extensions is a deliberate
// scope-addition for a future slice (connector + fusion + tests together),
// at which point they join this map.
func EngineProvenance() map[string]string {
	return map[string]string{
		".py":  "tree-sitter-wasm",
		".js":  "tree-sitter-wasm",
		".jsx": "tree-sitter-wasm",
		".ts":  "tree-sitter-wasm",
		".tsx": "tree-sitter-wasm",
		".rb":  "tree-sitter-wasm",
		".kt":  "tree-sitter-wasm",
		".kts": "tree-sitter-wasm",
	}
}
