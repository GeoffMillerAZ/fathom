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
	// Skipped names, per reason, how many occurrences ImportIndex declined
	// to turn into an edge — never silently dropped. The one reason this
	// package currently produces is "no-enclosing-definition": a reference
	// outside every definition's span (a top-level import statement is the
	// common case).
	Skipped map[string]int
}

// PackageEdges is one fathom package's worth of imported typed edges.
// ImportIndex groups its output this way because store.PutTypedEdges
// (internal/store) replaces typed edges at exactly this granularity — see
// the package doc comment.
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
// returns the typed edges it implies.
func ImportIndex(service string, raw []byte) (Import, error) {
	idx := &scippb.Index{}
	if err := proto.Unmarshal(raw, idx); err != nil {
		return Import{}, fmt.Errorf("scip: decode index: %w", err)
	}
	return fromIndex(service, idx), nil
}

// ImportIndexFile reads path and decodes it via ImportIndex.
func ImportIndexFile(service, path string) (Import, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Import{}, fmt.Errorf("scip: read %s: %w", path, err)
	}
	return ImportIndex(service, raw)
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

// size is a comparable measure of s's extent — used to pick the INNERMOST
// of several containing spans (the smallest one), never an arbitrary one.
func (s span) size() int64 {
	return int64(s.end.line-s.start.line)*1_000_000 + int64(s.end.char-s.start.char)
}

// docDef is one definition occurrence within a document, positioned so
// containment (nesting, and a reference's enclosing definition) can be
// recovered without re-walking any syntax tree — the SCIP index already
// carries exactly the geometry needed (EnclosingRange, falling back to
// Range when a producer omits it).
type docDef struct {
	scipSymbol string
	display    string
	sp         span
	parent     int // index into the same document's []docDef, -1 if none
	name       string
	resolved   bool
}

// pkgFromPath mirrors internal/extractor/pyextract.go's pkgFromPath EXACTLY
// (basename, minus extension) — kept as an independent copy rather than an
// import of that unexported helper, since the "same convention" this
// package must honor is a NAME convention (documented in tsextract.go/
// pyextract.go's own Package assignment), not a code dependency. See the
// package doc comment for why this, joined with core.QualifiedPackage,
// reproduces the same fathom SymbolKey the language's own extractor mints.
func pkgFromPath(relPath string) string {
	base := relPath
	for i := len(relPath) - 1; i >= 0; i-- {
		if relPath[i] == '/' || relPath[i] == '\\' {
			base = relPath[i+1:]
			break
		}
	}
	if dot := strings.LastIndex(base, "."); dot >= 0 {
		base = base[:dot]
	}
	return base
}

// definitionSpan returns the span a definition occurrence's containment
// tests use: EnclosingRange when the producer set one (the real body
// extent), falling back to Range (just the name token — still usable for
// containment, only less generous) otherwise.
func definitionSpan(occ *scippb.Occurrence) span {
	if sp := normalizeRange(occ.GetEnclosingRange()); sp.ok {
		return sp
	}
	return normalizeRange(occ.GetRange())
}

func isDefinition(occ *scippb.Occurrence) bool {
	return occ.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) != 0
}

// displayNameFor returns a definition occurrence's display name: the
// matching SymbolInformation's DisplayName when the document declares one
// (SCIP's own field for exactly this), falling back to the raw SCIP symbol
// string when a producer omitted it — never an empty name.
func displayNameFor(occ *scippb.Occurrence, infoBySym map[string]*scippb.SymbolInformation) string {
	if info, ok := infoBySym[occ.GetSymbol()]; ok && info.GetDisplayName() != "" {
		return info.GetDisplayName()
	}
	return occ.GetSymbol()
}

// buildDocDefs collects every definition occurrence in doc, sorted by span
// start (widest-first on a tie) so a stack-based parent pass — identical
// in shape to tsextract.go's resolveTSParents — can recover nesting in one
// linear scan.
func buildDocDefs(doc *scippb.Document) []docDef {
	infoBySym := make(map[string]*scippb.SymbolInformation, len(doc.GetSymbols()))
	for _, info := range doc.GetSymbols() {
		infoBySym[info.GetSymbol()] = info
	}

	var defs []docDef
	for _, occ := range doc.GetOccurrences() {
		if !isDefinition(occ) {
			continue
		}
		sp := definitionSpan(occ)
		if !sp.ok {
			continue
		}
		defs = append(defs, docDef{
			scipSymbol: occ.GetSymbol(),
			display:    displayNameFor(occ, infoBySym),
			sp:         sp,
			parent:     -1,
		})
	}
	sort.SliceStable(defs, func(i, j int) bool {
		if defs[i].sp.start != defs[j].sp.start {
			return less(defs[i].sp.start, defs[j].sp.start)
		}
		return defs[i].sp.size() > defs[j].sp.size() // wider first when tied
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

// qualifiedName resolves defs[i]'s fathom-facing name: its own display
// name, prefixed by its enclosing definition's own qualified name (dot
// separated) when it has one — recursively, exactly mirroring tsextract's
// qualifyTSDefs "Owner.name" convention (see the package doc comment).
func qualifiedName(defs []docDef, i int) string {
	d := &defs[i]
	if d.resolved {
		return d.name
	}
	if d.parent >= 0 {
		d.name = qualifiedName(defs, d.parent) + "." + d.display
	} else {
		d.name = d.display
	}
	d.resolved = true
	return d.name
}

// enclosingDef returns the index into defs of the innermost definition
// whose span contains p, or -1 when none does.
func enclosingDef(defs []docDef, p pos) int {
	best := -1
	for i := range defs {
		if !defs[i].sp.contains(p) {
			continue
		}
		if best == -1 || defs[i].sp.size() < defs[best].sp.size() {
			best = i
		}
	}
	return best
}

// fromIndex is ImportIndex's pure core.
func fromIndex(service string, idx *scippb.Index) Import {
	rep := Report{Documents: len(idx.GetDocuments()), Skipped: map[string]int{}}
	if ti := idx.GetMetadata().GetToolInfo(); ti != nil {
		rep.Producer = Producer{Name: ti.GetName(), Version: ti.GetVersion()}
	}
	for _, doc := range idx.GetDocuments() {
		if doc.GetLanguage() != "" && rep.Language == "" {
			rep.Language = doc.GetLanguage()
		}
	}

	// Pass 1: every document's definitions, nested and named — and a
	// GLOBAL scipSymbol -> fathomKey map, since a reference in one document
	// commonly targets a definition in ANOTHER (an imported function).
	type docState struct {
		doc  *scippb.Document
		defs []docDef
		pkg  string
	}
	states := make([]docState, 0, len(idx.GetDocuments()))
	fathomKeyBySym := map[string]string{}

	for _, doc := range idx.GetDocuments() {
		defs := buildDocDefs(doc)
		pkg := core.QualifiedPackage(doc.GetRelativePath(), pkgFromPath(doc.GetRelativePath()))
		for i := range defs {
			name := qualifiedName(defs, i)
			fathomKeyBySym[defs[i].scipSymbol] = core.SymbolKey(service, pkg, name)
		}
		states = append(states, docState{doc: doc, defs: defs, pkg: pkg})
	}

	// Pass 2: every non-definition occurrence becomes an edge FROM its
	// innermost enclosing definition (same document) TO the referenced
	// symbol's fathom key (any document) — or core.EdgeUnresolved when no
	// document in this index defines that symbol at all.
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
			fromKey := core.SymbolKey(service, st.pkg, qualifiedName(st.defs, di))

			toSym := occ.GetSymbol()
			var edge core.Edge
			if toKey, ok := fathomKeyBySym[toSym]; ok {
				edge = core.Edge{FromService: service, FromFile: st.doc.GetRelativePath(), FromSymbol: fromKey, ToSymbol: toKey, Kind: core.EdgeDirect}
				rep.Direct++
			} else {
				edge = core.Edge{FromService: service, FromFile: st.doc.GetRelativePath(), FromSymbol: fromKey, ToSymbol: unresolvedMarker(toSym), Kind: core.EdgeUnresolved}
				rep.Unresolved++
			}
			edgesByPkg[st.pkg] = append(edgesByPkg[st.pkg], edge)
		}
	}
	rep.Edges = rep.Direct + rep.Unresolved
	if len(rep.Skipped) == 0 {
		rep.Skipped = nil
	}

	pkgs := make([]string, 0, len(edgesByPkg))
	for p := range edgesByPkg {
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
