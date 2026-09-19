// Package scip imports a SCIP (github.com/sourcegraph/scip) index into
// core.Edge values fathom's typed_edges table already knows how to store
// (ADR-041 decision 5, docs/plans/substrate.md SU-9): the mechanism that
// carries typed edges beyond Go, without fathom running or vendoring any
// per-language type-checker of its own. Where internal/fusion.
// EnrichTypedEdges derives edges by type-checking Go source directly, a
// language without a native pass gains typed edges by IMPORTING an
// external indexer's SCIP output instead — scip-typescript, scip-python,
// and (per docs/languages.md's tracked order) the delegate's own list
// after that. This package never runs an external indexer itself; it only
// decodes bytes one already produced (or, offline, a hand-built fixture —
// see fixture.go and testdata/tsfixture).
//
// Wire types come from third_party/scip (vendored generated code only —
// see that directory's README for why the upstream module is not a direct
// dependency), the same vendor internal/analysis/scipexport already
// depends on for `fathom export scip`. This package is scipexport's
// inverse: scipexport renders fathom's OWN symbol/edge tables as SCIP;
// this package reads someone ELSE's SCIP index back into fathom's tables.
//
// # Occurrence -> edge mapping
//
// A SCIP Document pairs Occurrences (a symbol mentioned at a Range) with
// SymbolInformation (DisplayName, for a symbol the document itself
// DEFINES). An occurrence whose SymbolRoles carries the Definition bit is
// a DEFINITION site; every other occurrence in that document is a
// REFERENCE.
//
// Definitions nest: a method's definition occurrence sits inside its
// class's. ImportIndex recovers that nesting from EnclosingRange
// containment alone — the same structural signal internal/extractor's
// tsextract.go uses (resolveTSParents/qualifyTSDefs) to build "Owner.name"
// qualified names — rather than trusting DisplayName to already carry it:
// a real external indexer's DisplayName is typically the BARE name only
// ("greet", not "Greeter.greet"), so nesting has to come from geometry,
// not text. A definition's fathom name is therefore
// innermostEnclosingDefinition.Name + "." + DisplayName, recursively, and
// its fathom package is core.QualifiedPackage(doc.RelativePath, pkg),
// where pkg mirrors tsextract/pyextract's own pkgFromPath (the file's
// basename, minus extension) — see pkgFromPath below. Two definitions this
// package derives names for from the SAME document therefore land on
// EXACTLY the SymbolKey the language's own extractor would have minted
// for them, which is what lets a reference's edge join fathom's existing
// symbol table at all.
//
// A reference's edge FROM is the innermost definition (in the SAME
// document) whose span (EnclosingRange, falling back to Range) contains
// the reference's own Range start; a reference outside every definition's
// span (a top-level import statement, most commonly) contributes no edge
// and is counted in Report.Skipped rather than dropped silently. A
// reference's edge TO is the referenced symbol's fathom key when some
// document in the index DEFINES that exact SCIP symbol string (kind
// core.EdgeDirect, mirroring EnrichTypedEdges' own EdgeDirect for a
// statically resolved Go call); otherwise the referenced symbol is
// EXTERNAL to this index (a stdlib call, a dependency this index didn't
// cover) and the edge is recorded core.EdgeUnresolved — never silently
// dropped, matching EnrichTypedEdges' own "record it explicitly" rule for
// a call site it could not resolve.
//
// Edges are grouped per fathom package (the FROM symbol's package) because
// store.PutTypedEdges (internal/store) replaces typed edges at exactly
// that granularity — the same package-granular replace unit the Go typed
// pass already uses, so re-importing one changed file's worth of SCIP
// output never disturbs another package's rows.
package scip
