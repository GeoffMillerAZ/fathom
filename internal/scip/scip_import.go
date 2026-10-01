package scip

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Producer names the external indexer that produced an imported SCIP index
// (Index.Metadata.ToolInfo) — reported by `fathom ingest` as e.g.
// "scip-typescript 0.3.16", the same idea internal/fusion.TypedEdgeStatus
// already applies to the native Go pass, extended to an imported one.
type Producer struct {
	Name    string
	Version string
}

// String renders p the way ingest's summary line names it: "name version",
// "name" alone when no version was recorded, or "unknown" when the index
// carried no tool_info at all — never a blank producer line.
func (p Producer) String() string {
	switch {
	case p.Name == "":
		return "unknown"
	case p.Version == "":
		return p.Name
	default:
		return p.Name + " " + p.Version
	}
}

// Report summarises one ImportIndex call.
type Report struct {
	Producer Producer
	// Language is Document.Language of the first document that set one;
	// "" when no document in the index named a language.
	Language string
	// Documents is len(Index.Documents).
	Documents int
	// Edges is Direct + Unresolved (every occurrence ImportIndex turned
	// into an edge).
	Edges int
	// Direct is edges whose referenced symbol resolved to a definition
	// somewhere in this same index (core.EdgeDirect).
	Direct int
	// Unresolved is edges whose referenced symbol has no definition
	// anywhere in this index — external to it (core.EdgeUnresolved).
	Unresolved int
	// Unmapped is the count of SCIP definition occurrences (O115) that
	// could not be joined to any of the service's own extracted
	// core.SymbolNode values — neither by the definition's own position
	// (document path + its Range's start line matched against a node's
	// FilePath+Line) nor, when that match was ambiguous, by parsing the
	// SCIP symbol string's own descriptor suffix and matching its leaf
	// name within the same file. A definition counted here contributes NO
	// edge in either direction: never as a FROM (its own outgoing
	// references are dropped, tallied under Skipped["unmapped-endpoint"])
	// and never as a resolved TO (a reference to it is dropped the same
	// way rather than mis-typed as core.EdgeUnresolved, which means
	// "external to this index" — an unmapped definition is NOT external,
	// it is internal and unrecognised, a materially different, and more
	// alarming, condition that must never be silently folded into the
	// ordinary external-reference count). This is the disclosure O115
	// requires: a real producer that never sets SymbolInformation.
	// DisplayName (scip-typescript 0.4.0, scip-python 0.6.6, confirmed 0
	// of 23,583 / 5,551 — see .fathom/observations.md O115) used to make
	// EVERY definition's fathom key wrong, silently, joining nothing;
	// resolving by position instead makes the mapping right by
	// construction whenever the position exists, and this field reports
	// exactly the residue where it did not.
	Unmapped int
	// ResolvedViaFallback is the subset of successfully mapped
	// definitions (Unmapped's complement) whose position match was
	// ambiguous (zero or more than one core.SymbolNode at that exact
	// file+line) and so were instead resolved by the descriptor-suffix
	// fallback — informational: a large share here means the position
	// signal is noisy for this producer/language pair (e.g. a decorator
	// or export keyword shifting the def's own Range off the extractor's
	// idea of the definition's line) even though the fallback recovered
	// the correct symbol.
	ResolvedViaFallback int
	// EnclosingFallback (LB-F54) counts mapped definitions whose producer
	// set no EnclosingRange and that therefore took their fathom node's own
	// line span (Line..EndLine, whole lines) as their containment span —
	// see definitionSpan. scip-php, scip-clang and scip-dotnet set no
	// enclosing_range at all, so without it no reference fell inside any
	// definition and the import stored no typed edge. Unmapped definitions,
	// and mapped ones whose node has no EndLine, keep their name token.
	EnclosingFallback int
	// Skipped names, per reason, how many occurrences ImportIndex declined
	// to turn into an edge — never silently dropped. "no-enclosing-
	// definition" is a reference outside every definition's span (a
	// top-level import statement is the common case); "invalid-range" is
	// a malformed occurrence range; "unmapped-endpoint" is a reference
	// whose FROM (enclosing definition) or internally-defined TO could
	// not be mapped to a fathom symbol (O115, see Unmapped's doc comment);
	// "local-target" (LB-F55) is a reference to a "local N" symbol the
	// index defines but no extractor maps — a value with no fathom
	// identity (a local variable), so never a typed edge to miss, and
	// kept out of "unmapped-endpoint" (the harness's typed_recall
	// denominator) for that reason.
	Skipped map[string]int
}

// PackageEdges is one fathom package's worth of imported typed edges.
// ImportIndex groups its output this way because store.PutTypedEdges
// (internal/store) replaces typed edges at exactly this granularity — see
// the package doc comment. Edges may be empty: a package with at least one
// mapped definition but no outgoing edges this run is still listed (O116,
// .fathom/observations.md) so a caller looping Packages to call
// store.PutTypedEdges per package still clears that package's stale rows
// from an earlier import rather than silently skipping it.
type PackageEdges struct {
	Package string
	Edges   []core.Edge
}

// Import is ImportIndex's result: edges grouped per fathom package, plus
// the run report.
type Import struct {
	Packages []PackageEdges
	Report   Report
}

// unresolvedMarker synthesizes a ToSymbol for an EdgeUnresolved edge —
// mirroring internal/fusion/typedcalls.go's own "<unresolved:%d>" marker
// for a call site the Go typed pass could not resolve, except keyed by the
// foreign SCIP symbol string (more informative here: unlike a Go call
// site, there is no ambiguity to hide behind a line number — the SCIP
// index already told us exactly which external symbol was referenced,
// just not one this index defines).
func unresolvedMarker(scipSymbol string) string {
	return fmt.Sprintf("<unresolved:%s>", scipSymbol)
}

// ImportIndex decodes raw SCIP protobuf bytes (as produced by an external
// indexer, or by BuildTSFixtureIndex+proto.Marshal offline) for service and
// returns the typed edges it implies. nodes is the SAME service's own
// already-extracted symbol table (internal/extractor's native or
// file-resolved pass output for the files this index covers — always
// available in practice, since `fathom ingest -scip` only imports after a
// syntax ingest has written the service, see ingest.go) — see the package
// doc comment (O115) for why every definition is mapped against it rather
// than against the index's own SymbolInformation.DisplayName.
func ImportIndex(service string, raw []byte, nodes []core.SymbolNode) (Import, error) {
	idx := &scippb.Index{}
	if err := proto.Unmarshal(raw, idx); err != nil {
		return Import{}, fmt.Errorf("scip: decode index: %w", err)
	}
	return fromIndex(service, idx, nodes), nil
}

// ImportIndexFile reads path and decodes it via ImportIndex.
func ImportIndexFile(service, path string, nodes []core.SymbolNode) (Import, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Import{}, fmt.Errorf("scip: read %s: %w", path, err)
	}
	return ImportIndex(service, raw, nodes)
}

// pos is a 0-based (line, character) position, comparable with <=.
type pos struct{ line, char int32 }

func lessEq(a, b pos) bool {
	if a.line != b.line {
		return a.line < b.line
	}
	return a.char <= b.char
}

func less(a, b pos) bool {
	if a.line != b.line {
		return a.line < b.line
	}
	return a.char < b.char
}

// span is a half-open source range, normalized from a SCIP Range/
// EnclosingRange array (3 or 4 int32 elements — see Occurrence.Range's own
// doc comment in third_party/scip/scip.proto).
type span struct {
	start, end pos
	ok         bool
}

func normalizeRange(r []int32) span {
	switch len(r) {
	case 3:
		return span{start: pos{r[0], r[1]}, end: pos{r[0], r[2]}, ok: true}
	case 4:
		return span{start: pos{r[0], r[1]}, end: pos{r[2], r[3]}, ok: true}
	default:
		return span{}
	}
}

func (s span) contains(p pos) bool {
	return s.ok && lessEq(s.start, p) && lessEq(p, s.end)
}

// narrower reports whether s's extent is strictly smaller than t's: fewer
// lines, or as many lines and fewer characters — used to pick the INNERMOST
// of several containing spans, never an arbitrary one. Lines compare first,
// so a whole-line fallback span (LB-F54, end character 1<<30) is still
// narrower than any producer EnclosingRange covering more lines.
func (s span) narrower(t span) bool {
	sl, tl := s.end.line-s.start.line, t.end.line-t.start.line
	if sl != tl {
		return sl < tl
	}
	return int64(s.end.char)-int64(s.start.char) < int64(t.end.char)-int64(t.start.char)
}

// wholeLine is the end character a line-span fallback uses: past the end
// of any real line, so the span covers its last line whole.
const wholeLine = 1 << 30

// definitionSpan returns the span a definition occurrence's containment
// tests use, and whether it came from the LB-F54 fallback:
//
//   - EnclosingRange when the producer set one (the real body extent);
//   - otherwise, when the definition mapped (resolveDefinition) to a fathom
//     node with EndLine >= Line > 0, that node's own lines: from
//     min(node.Line-1, Range's start line), character 0, through
//     EndLine-1 whole — the same fallback the language harness applies
//     (internal/langbench/scip.go, owner decision D5), so a producer that
//     sets no enclosing_range (scip-php, scip-clang, scip-dotnet) still
//     attributes the references inside a mapped body;
//   - otherwise Range (just the name token — still usable for containment,
//     only less generous): an unmapped definition, or one whose node has
//     no EndLine.
//
// The fallback never applies to a definition that carries an
// EnclosingRange, so such a producer's import is unchanged by it.
func definitionSpan(occ *scippb.Occurrence, node core.SymbolNode, mapped bool) (span, bool) {
	if sp := normalizeRange(occ.GetEnclosingRange()); sp.ok {
		return sp, false
	}
	rng := normalizeRange(occ.GetRange())
	if !rng.ok || !mapped || node.Line <= 0 || node.EndLine < node.Line {
		return rng, false
	}
	lo := min(int32(node.Line-1), rng.start.line)
	return span{start: pos{lo, 0}, end: pos{int32(node.EndLine - 1), wholeLine}, ok: true}, true
}

func isDefinition(occ *scippb.Occurrence) bool {
	return occ.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) != 0
}

// localSymbolPrefix is SCIP's own reserved marker (scip.proto's Symbol doc
// comment) for a "local" symbol: one with no cross-reference identity of
// its own, whose id is unique only WITHIN its enclosing document — freely
// reused, unrelated, by every other document (a file's first local
// variable is commonly "local 0" in every file the producer indexes).
const localSymbolPrefix = "local "

// scopedSymbolKey is the key ImportIndex indexes and looks up definitions
// by: docPath-scoped for a "local" symbol, the raw symbol string unchanged
// for every other kind (already globally unique — scheme, package and
// version are baked into it). A reference to a local symbol is always
// resolved against the SAME document's own definitions (SCIP's spec:
// locals never cross a document boundary), so using the occurrence's own
// document path as the scope is always correct, on both the write and the
// read side. Treating "local" ids as globally unique the way a flat
// scipSymbol->docDef map implicitly does lets an UNRELATED document's
// "local 5" silently satisfy a reference to a totally different
// document's own "local 5" — found by hand-verifying a real
// scip-typescript import: a method referencing only its own destructured
// local variable ended up wired to an unrelated type in a different test
// file that happened to reuse the same local id.
func scopedSymbolKey(docPath, scipSymbol string) string {
	if isLocalSymbol(scipSymbol) {
		return docPath + "\x00" + scipSymbol
	}
	return scipSymbol
}

// fileLine is a 1-based source location fathom's own extractors record on
// every core.SymbolNode (Line) — the join key O115's fix maps each SCIP
// definition occurrence against, via its OWN document path and its Range's
// (never EnclosingRange's — Range is the name token itself, the only part
// of a definition occurrence every SCIP producer is required to set, and
// the part that lines up with an extractor's own def-node line for the
// overwhelming majority of real definitions) start line.
type fileLine struct {
	file string
	line int
}

// indexNodesByLocation groups nodes by their (FilePath, Line) — the
// PRIMARY join O115's fix performs. More than one node at the exact same
// file+line (rare, but occurs for e.g. a one-line arrow function bound to
// two names, or two extractor passes disagreeing) makes the position match
// ambiguous on purpose: resolveDefinition then falls back rather than
// guessing between them.
func indexNodesByLocation(nodes []core.SymbolNode) map[fileLine][]core.SymbolNode {
	idx := make(map[fileLine][]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		k := fileLine{file: n.FilePath, line: n.Line}
		idx[k] = append(idx[k], n)
	}
	return idx
}

// leafOf returns name's own last segment — the part after its last '.'
// (fathom's own "Owner.member" qualification, e.g. "Greeter.greet" ->
// "greet"), or name itself when it carries no '.'.
func leafOf(name string) string {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

// indexNodesByFileAndLeaf groups nodes by (FilePath, their own name's leaf
// segment) — the SECONDARY join the descriptor-suffix fallback uses. Never
// crosses files: two files can define same-named symbols, and only the
// SCIP document's own file is known to be the right one to search.
func indexNodesByFileAndLeaf(nodes []core.SymbolNode) map[string][]core.SymbolNode {
	idx := make(map[string][]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		k := n.FilePath + "\x00" + leafOf(n.Name)
		idx[k] = append(idx[k], n)
	}
	return idx
}

// descriptorLeafName parses sym per SCIP's own symbol grammar
// (third_party/scip/scip.proto's Symbol doc comment: "<scheme> ' '
// <manager> ' ' <package-name> ' ' <version> <descriptor>+") and returns
// the LAST descriptor's own name — the identifier a namespace, type, term
// or method descriptor carries, stripped of its kind suffix (the trailing
// '/', '#', '.', ':' or a method's "(<disambiguator>)." group) and of any
// backtick quoting a name containing one of those characters literally
// (e.g. a file name used as a namespace segment) — plus whether parsing
// succeeded. A symbol with fewer than the four leading package fields (the
// "local <id>" form SCIP uses for a symbol with no cross-reference
// identity at all, e.g. a function-local variable) has no descriptor path
// to parse and returns ok=false: there is no name to recover, which is
// correct — a local has no fathom-level identity to map to either.
func descriptorLeafName(sym string) (string, bool) {
	rest := sym
	for i := 0; i < 4; i++ {
		sp := strings.IndexByte(rest, ' ')
		if sp < 0 {
			return "", false
		}
		rest = rest[sp+1:]
	}
	if rest == "" {
		return "", false
	}

	// Trim exactly one trailing descriptor-kind suffix. A method's is
	// "(<disambiguator>)."; every other kind is a single trailing
	// character (namespace '/', type '#', term '.', meta ':';
	// type-parameter/parameter '[...]'/'(...)' are handled the same way
	// as a method's parens since only the LAST descriptor's kind matters
	// here).
	trimmed := strings.TrimSuffix(rest, ".")
	switch {
	case strings.HasSuffix(trimmed, ")"):
		if open := lastUnquotedByte(trimmed, '('); open >= 0 {
			trimmed = trimmed[:open]
		}
	default:
		trimmed = strings.TrimRight(trimmed, "#:]/")
	}

	// The leaf name is everything after the LAST unquoted separator
	// ('/', '#', '.', ':') — a backtick-quoted name (which may contain
	// any of those characters literally, e.g. a file name) is never split
	// on them.
	leaf := trimmed
	// ']' closes a descriptor as well for rust-analyzer, which writes an impl
	// block's members as `impl#[Type]name().`; every other producer keeps the
	// separator set its typed-edge readings were measured with.
	seps := "/#.:"
	if strings.HasPrefix(sym, "rust-analyzer ") {
		seps = "/#.:]"
	}
	if sepAt := lastUnquotedAny(trimmed, seps); sepAt >= 0 {
		leaf = trimmed[sepAt+1:]
	}
	leaf = strings.Trim(leaf, "`")
	if leaf == "" {
		return "", false
	}
	return leaf, true
}

// lastUnquotedByte returns the last index of b in s that is not inside a
// backtick-quoted (`...`) span, or -1.
func lastUnquotedByte(s string, b byte) int {
	inQuote := false
	last := -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '`':
			inQuote = !inQuote
		case b:
			if !inQuote {
				last = i
			}
		}
	}
	return last
}

// lastUnquotedAny returns the last index in s of any byte in chars that is
// not inside a backtick-quoted span, or -1.
func lastUnquotedAny(s, chars string) int {
	inQuote := false
	last := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '`' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && strings.IndexByte(chars, c) >= 0 {
			last = i
		}
	}
	return last
}

// docDef is one definition occurrence within a document, positioned so
// containment (nesting, and a reference's enclosing definition) can be
// recovered without re-walking any syntax tree, and resolved (O115) to the
// fathom core.SymbolNode it names — or not, in which case mapped is false
// and node is the zero value.
type docDef struct {
	scipSymbol  string
	sp          span // containment span: see definitionSpan
	parent      int  // index into the same document's []docDef, -1 if none
	node        core.SymbolNode
	mapped      bool
	viaFallback bool
	// lineSpan: sp is the mapped node's own lines (LB-F54), not a range
	// the producer gave.
	lineSpan bool
}

// descriptorIsParameterLike reports whether scipSymbol's LAST descriptor
// (per SCIP's own grammar) is a Parameter ("(name)") or TypeParameter
// ("[name]") — a per-declaration value with NO cross-reference identity of
// its own, which no fathom extractor ever emits as a top-level
// core.SymbolNode. Such a definition must be refused BEFORE position
// matching is attempted: on the overwhelming majority of real signatures a
// parameter's own Range sits on the EXACT SAME LINE as its enclosing
// function's own definition (a Python/TypeScript signature normally fits
// one line), so a naive position join would silently collapse "this
// parameter" onto "the function that declares it" — and every later
// reference to that parameter inside the function's own body (its `self`
// in Python alone is nearly every line of nearly every method) would then
// mint a spurious edge from the function TO ITSELF. Found by hand-
// verifying a real scip-python import: flask's `raise_routing_exception`
// never calls itself, yet produced exactly this self-loop, once per
// reference to its own `request`/`self` parameters, before this guard.
// A Method descriptor is distinguished from a Parameter by its mandatory
// trailing '.': "name(<disambiguator>)." vs. a parameter's bare "(name)".
func descriptorIsParameterLike(scipSymbol string) bool {
	rest := scipSymbol
	for i := 0; i < 4; i++ {
		sp := strings.IndexByte(rest, ' ')
		if sp < 0 {
			return false
		}
		rest = rest[sp+1:]
	}
	switch {
	case strings.HasSuffix(rest, "]"):
		return true // TypeParameter
	case strings.HasSuffix(rest, ")") && !strings.HasSuffix(rest, ")."):
		return true // Parameter (a Method descriptor always ends ").")
	default:
		return false
	}
}

// resolveDefinition maps one SCIP definition occurrence to fathom's own
// core.SymbolNode (O115's fix): first by its DEFINITION position (docPath
// + the occurrence's Range start line, 1-based, matched against byLoc);
// when that match is ambiguous (zero or more than one candidate at that
// exact file+line), falls back to parsing scipSymbol's own descriptor
// suffix (descriptorLeafName) and resolving the leaf name against byLeaf,
// scoped to the SAME file. mapped is false — and node the zero value —
// when NEITHER resolves to exactly one candidate, or when scipSymbol
// names a Parameter/TypeParameter (descriptorIsParameterLike): the
// definition is then reported unmapped and contributes no edge, in either
// direction, rather than guessing (I44-style honesty: never emit an edge
// whose endpoint cannot be mapped).
func resolveDefinition(byLoc map[fileLine][]core.SymbolNode, byLeaf map[string][]core.SymbolNode, docPath string, rangeStartLine0 int32, scipSymbol string) (node core.SymbolNode, mapped, viaFallback bool) {
	if descriptorIsParameterLike(scipSymbol) {
		return core.SymbolNode{}, false, false
	}
	loc := fileLine{file: docPath, line: int(rangeStartLine0) + 1}
	if cands := byLoc[loc]; len(cands) == 1 {
		return cands[0], true, false
	}
	leaf, ok := descriptorLeafName(scipSymbol)
	if !ok {
		return core.SymbolNode{}, false, false
	}
	if cands := byLeaf[docPath+"\x00"+leaf]; len(cands) == 1 {
		return cands[0], true, true
	}
	return core.SymbolNode{}, false, false
}

// buildDocDefs collects every definition occurrence in doc, resolves each
// to a fathom symbol (resolveDefinition), and sorts by containment span
// start (widest-first on a tie) so a stack-based parent pass — identical
// in shape to tsextract.go's resolveTSParents — can recover nesting in one
// linear scan. Nesting is pure geometry and is computed for every
// definition regardless of whether it mapped, since an UNMAPPED outer
// definition must still be able to correctly refuse containment to an
// inner one that mapped fine (rare, but geometry does not care whether
// naming succeeded).
func buildDocDefs(doc *scippb.Document, byLoc map[fileLine][]core.SymbolNode, byLeaf map[string][]core.SymbolNode) []docDef {
	var defs []docDef
	for _, occ := range doc.GetOccurrences() {
		if !isDefinition(occ) {
			continue
		}
		// The definition's own position (Range, else EnclosingRange) is the
		// join key; a definition with neither is skipped as before.
		rng := normalizeRange(occ.GetRange())
		if !rng.ok {
			rng = normalizeRange(occ.GetEnclosingRange())
		}
		if !rng.ok {
			continue
		}
		node, mapped, viaFallback := resolveDefinition(byLoc, byLeaf, doc.GetRelativePath(), rng.start.line, occ.GetSymbol())
		sp, lineSpan := definitionSpan(occ, node, mapped)
		defs = append(defs, docDef{
			scipSymbol:  occ.GetSymbol(),
			sp:          sp,
			parent:      -1,
			node:        node,
			mapped:      mapped,
			viaFallback: viaFallback,
			lineSpan:    lineSpan,
		})
	}
	sort.SliceStable(defs, func(i, j int) bool {
		if defs[i].sp.start != defs[j].sp.start {
			return less(defs[i].sp.start, defs[j].sp.start)
		}
		return defs[j].sp.narrower(defs[i].sp) // wider first when tied
	})

	var stack []int
	for idx := range defs {
		d := &defs[idx]
		for len(stack) > 0 && !defs[stack[len(stack)-1]].sp.contains(d.sp.start) {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			d.parent = stack[len(stack)-1]
		}
		stack = append(stack, idx)
	}
	return defs
}

// enclosingDef returns the index into defs of the innermost definition
// whose span contains p, or -1 when none does.
func enclosingDef(defs []docDef, p pos) int {
	best := -1
	for i := range defs {
		if !defs[i].sp.contains(p) {
			continue
		}
		if best == -1 || defs[i].sp.narrower(defs[best].sp) {
			best = i
		}
	}
	return best
}

// isLocalSymbol reports whether sym is SCIP's "local N" form
// (localSymbolPrefix).
func isLocalSymbol(sym string) bool { return strings.HasPrefix(sym, localSymbolPrefix) }

// enclosingFrom returns the definition a reference whose innermost
// enclosing definition is defs[di] is attributed FROM (LB-F55). An
// UNMAPPED "local N" definition (a local variable whose span covers its
// own initializer, as scip-dart writes it) is no fathom symbol, so the
// reference climbs its parent chain to the nearest mapped definition — the
// function the local lives in. The climb passes only through unmapped
// locals: it stops at the first definition that is mapped or is not a
// local, so a reference inside an unmapped method, or a Parameter or
// TypeParameter (descriptorIsParameterLike — handled exactly as before
// LB-F55: unmapped, its references counted as "unmapped-endpoint"), is
// never re-attributed to an outer definition. A local with no mapped
// ancestor returns itself, still unmapped.
func enclosingFrom(defs []docDef, di int) docDef {
	d := defs[di]
	for !d.mapped && isLocalSymbol(d.scipSymbol) && d.parent >= 0 {
		d = defs[d.parent]
	}
	return d
}

// fromIndex is ImportIndex's pure core.
func fromIndex(service string, idx *scippb.Index, nodes []core.SymbolNode) Import {
	rep := Report{Documents: len(idx.GetDocuments()), Skipped: map[string]int{}}
	if ti := idx.GetMetadata().GetToolInfo(); ti != nil {
		rep.Producer = Producer{Name: ti.GetName(), Version: ti.GetVersion()}
	}
	for _, doc := range idx.GetDocuments() {
		if doc.GetLanguage() != "" && rep.Language == "" {
			rep.Language = doc.GetLanguage()
		}
	}

	byLoc := indexNodesByLocation(nodes)
	byLeaf := indexNodesByFileAndLeaf(nodes)

	// Pass 1: every document's definitions, resolved to a fathom symbol
	// (or not) and nested — plus a GLOBAL scipSymbol -> docDef map for
	// MAPPED definitions only, since a reference in one document commonly
	// targets a definition in ANOTHER (an imported function) and an
	// unmapped definition must never be joined to as if it were a real
	// key (O115).
	type docState struct {
		doc  *scippb.Document
		defs []docDef
	}
	states := make([]docState, 0, len(idx.GetDocuments()))
	bySym := map[string]docDef{}
	// definedRaw names every raw SCIP symbol this index defines ANYWHERE,
	// mapped or not — it is what tells an unmapped-but-internal reference
	// apart from a genuinely external one (Unresolved), so the two are
	// never conflated.
	definedRaw := map[string]bool{}

	for _, doc := range idx.GetDocuments() {
		defs := buildDocDefs(doc, byLoc, byLeaf)
		for _, d := range defs {
			key := scopedSymbolKey(doc.GetRelativePath(), d.scipSymbol)
			definedRaw[key] = true
			if d.mapped {
				bySym[key] = d
				if d.viaFallback {
					rep.ResolvedViaFallback++
				}
				if d.lineSpan {
					rep.EnclosingFallback++
				}
			} else {
				rep.Unmapped++
			}
		}
		states = append(states, docState{doc: doc, defs: defs})
	}

	// Pass 2: every non-definition occurrence becomes an edge FROM its
	// innermost enclosing definition (same document) TO the referenced
	// symbol's fathom key (any document) — or core.EdgeUnresolved when no
	// document in this index defines that symbol at all. A reference
	// whose FROM is unmapped, or whose TO names a symbol this index DOES
	// define but could not itself map, contributes no edge (O115) and is
	// counted under Skipped["unmapped-endpoint"] rather than silently
	// dropped or mis-typed as Unresolved — except a TO that is an unmapped
	// "local N" (LB-F55), counted under Skipped["local-target"]: a local
	// variable has no fathom identity, so it is no typed edge missed. A
	// reference whose innermost definition is an unmapped local (inside a
	// local variable's initializer) climbs to that local's nearest mapped
	// enclosing definition (enclosingFrom).
	edgesByPkg := map[string][]core.Edge{}
	for _, st := range states {
		for _, occ := range st.doc.GetOccurrences() {
			if isDefinition(occ) {
				continue
			}
			sp := normalizeRange(occ.GetRange())
			if !sp.ok {
				rep.Skipped["invalid-range"]++
				continue
			}
			di := enclosingDef(st.defs, sp.start)
			if di == -1 {
				rep.Skipped["no-enclosing-definition"]++
				continue
			}
			toSym := occ.GetSymbol()
			// A reference to a "local" symbol only ever resolves within
			// its OWN document (scopedSymbolKey) — the doc that this
			// reference occurrence itself sits in, per SCIP's spec.
			toKey := scopedSymbolKey(st.doc.GetRelativePath(), toSym)
			if isLocalSymbol(toSym) && definedRaw[toKey] && !bySym[toKey].mapped {
				rep.Skipped["local-target"]++
				continue
			}
			fromDef := enclosingFrom(st.defs, di)
			if !fromDef.mapped {
				rep.Skipped["unmapped-endpoint"]++
				continue
			}
			fromKey := fromDef.node.Key()
			fromPkg := fromDef.node.PackagePath()

			var edge core.Edge
			switch {
			case bySym[toKey].mapped:
				edge = core.Edge{FromService: service, FromFile: st.doc.GetRelativePath(), FromSymbol: fromKey, ToSymbol: bySym[toKey].node.Key(), Kind: core.EdgeDirect}
				rep.Direct++
			case definedRaw[toKey]:
				// Defined somewhere in this index, but that definition's
				// OWN mapping failed — never emit an edge to a key that
				// does not exist (O115).
				rep.Skipped["unmapped-endpoint"]++
				continue
			default:
				edge = core.Edge{FromService: service, FromFile: st.doc.GetRelativePath(), FromSymbol: fromKey, ToSymbol: unresolvedMarker(toSym), Kind: core.EdgeUnresolved}
				rep.Unresolved++
			}
			edgesByPkg[fromPkg] = append(edgesByPkg[fromPkg], edge)
		}
	}
	rep.Edges = rep.Direct + rep.Unresolved
	if len(rep.Skipped) == 0 {
		rep.Skipped = nil
	}

	// O116 (.fathom/observations.md): Packages must list every package this
	// import KNOWS ABOUT — every package holding at least one mapped
	// definition — not merely the packages that happened to accumulate an
	// edge. edgesByPkg alone under-reports: a package whose sole definition
	// is never called FROM anywhere in this index (like "math" in the TS
	// fixture, or any package whose calls all turned out unresolved-FROM or
	// were simply absent) never gets a key there, so a caller looping
	// Packages to call store.PutTypedEdges per package never even SEES it —
	// and if an earlier, buggier import once wrote edges for that same
	// package, those stale rows survive forever, because nothing ever calls
	// PutTypedEdges with an empty slice for it (store.PutTypedEdges itself
	// already clears correctly the instant it IS called that way — the gap
	// was entirely about which packages get called at all). pkgSet is the
	// union of every package with a mapped definition (bySym's values) and
	// edgesByPkg's own keys (defensive: every edge's fromPkg already comes
	// from a mapped definition, so this union should already be a no-op,
	// but a package set derived from the SAME data an edge's fromPkg came
	// from can never miss an edge-bearing package this way).
	pkgSet := make(map[string]bool, len(bySym))
	for _, d := range bySym {
		pkgSet[d.node.PackagePath()] = true
	}
	for p := range edgesByPkg {
		pkgSet[p] = true
	}
	pkgs := make([]string, 0, len(pkgSet))
	for p := range pkgSet {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	out := make([]PackageEdges, 0, len(pkgs))
	for _, p := range pkgs {
		edges := edgesByPkg[p]
		sortEdges(edges)
		out = append(out, PackageEdges{Package: p, Edges: edges})
	}

	return Import{Packages: out, Report: rep}
}

// sortEdges sorts edges by (FromSymbol, ToSymbol, Kind) for deterministic
// storage order — the same order internal/fusion/typedcalls.go's own
// sortEdges keeps typed edges in.
func sortEdges(edges []core.Edge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromSymbol != edges[j].FromSymbol {
			return edges[i].FromSymbol < edges[j].FromSymbol
		}
		if edges[i].ToSymbol != edges[j].ToSymbol {
			return edges[i].ToSymbol < edges[j].ToSymbol
		}
		return edges[i].Kind < edges[j].Kind
	})
}
