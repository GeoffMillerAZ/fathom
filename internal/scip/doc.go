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
// # Occurrence -> edge mapping, and the O115 fix
//
// A SCIP Document pairs Occurrences (a symbol mentioned at a Range) with
// SymbolInformation (DisplayName, for a symbol the document itself
// DEFINES). An occurrence whose SymbolRoles carries the Definition bit is
// a DEFINITION site; every other occurrence in that document is a
// REFERENCE.
//
// ImportIndex used to derive a definition's fathom-facing name from its
// SymbolInformation.DisplayName plus containment geometry (an "Owner.name"
// convention mirroring tsextract.go's own qualifyTSDefs), falling back to
// the raw SCIP symbol string when a producer omitted DisplayName. That
// fallback turned out to be the ONLY path real producers ever take: a
// protobuf scan of scip-typescript 0.4.0's and scip-python 0.6.6's own
// output (the current, only published releases of each) found ZERO
// SymbolInformation entries with a non-empty DisplayName across 23,583 and
// 5,551 definitions respectively (.fathom/observations.md O115). Every
// imported edge's endpoint was therefore keyed by the raw, opaque SCIP
// symbol string, which never joins fathom's own core.SymbolKey namespace —
// `fathom ingest -scip` reported edges written, but find_callers/
// references/blast_radius resolved none of them, silently.
//
// The fix (O115) never trusts DisplayName or the index's own geometry to
// reconstruct a name at all. Instead, ImportIndex is handed the SAME
// service's own already-extracted core.SymbolNode table (the native or
// file-resolved pass's output for the files this SCIP index covers —
// always available in practice, since `fathom ingest -scip` only imports
// after a syntax ingest has written the service) and maps each definition
// occurrence to the ONE SymbolNode that shares its document path and its
// Range's start line (1-based, matching core.SymbolNode.Line's own
// convention — see resolveDefinition). When that position match is
// AMBIGUOUS (no SymbolNode at that exact file+line, or more than one — a
// decorator, export keyword, or JSDoc block can shift a producer's own
// idea of a definition's line relative to the extractor's), ImportIndex
// falls back to parsing the SCIP symbol string's own descriptor suffix
// (descriptorLeafName, per SCIP's published grammar) for its last
// namespace/type/term/method name, and resolves that leaf name against a
// SymbolNode in the SAME file. A definition resolved by NEITHER path is
// reported unmapped (Report.Unmapped) and contributes NO edge in either
// direction — never a guess, and never silently dropped either: every
// reference an unmapped definition would have produced, and every
// reference TO a definition this index defines but could not map, is
// counted under Report.Skipped["unmapped-endpoint"], kept distinct from a
// genuinely external reference (core.EdgeUnresolved, a symbol no document
// in this index defines at all) so the two conditions — "not ours" versus
// "ours, but we couldn't name it" — are never conflated.
//
// A reference's edge FROM is the innermost mapped definition (in the SAME
// document) whose span contains the reference's own Range start; a
// reference outside every definition's span (a top-level import statement,
// most commonly) contributes no edge and is counted in Report.Skipped
// rather than dropped silently. A definition's span is its EnclosingRange
// when the producer set one; otherwise (LB-F54) a MAPPED definition whose
// fathom node knows its EndLine takes that node's own whole lines — the
// harness's D5 fallback (internal/langbench) — and every other definition
// its Range (the name token). scip-php, scip-clang and scip-dotnet set no
// enclosing_range on any definition, so before the fallback their imports
// stored no typed edge; Report.EnclosingFallback discloses how many
// definitions took it.
//
// Local symbols (LB-F55): a reference to a "local N" the index defines but
// no extractor maps is counted under Report.Skipped["local-target"], never
// "unmapped-endpoint" — a local variable has no fathom identity, so it is
// not a typed edge the import failed to name. A reference whose innermost
// definition is an unmapped local (inside the local's initializer, which
// scip-dart's enclosing_range covers) is attributed to the nearest mapped
// definition enclosing that local; the climb passes only through unmapped
// locals, and Parameter/TypeParameter definitions are handled as below,
// unchanged.
//
// Two further correctness guards the O115 fix required, both found by
// hand-verifying real scip-typescript/scip-python output rather than by
// inspection alone:
//
//   - A Parameter or TypeParameter descriptor (SCIP's own grammar: a bare
//     "(name)" or "[name]") is refused BEFORE position matching is even
//     attempted (descriptorIsParameterLike). A parameter's own Range
//     normally sits on the EXACT SAME LINE as its enclosing function's
//     signature, and no fathom extractor ever mints a SymbolNode for a
//     parameter — so an ungated position join would silently collapse
//     "this parameter" onto "the function that declares it", and every
//     later reference to that parameter inside the function's own body
//     (its `self` in Python alone is nearly every line of nearly every
//     method) would mint a spurious edge from the function to ITSELF.
//   - A "local <id>" symbol (SCIP's reserved form for a value with no
//     cross-reference identity, unique only within its enclosing
//     document — freely reused by every other document) is looked up
//     document-scoped (scopedSymbolKey), never in the same flat table
//     ordinary globally-unique symbols share. Without this, an unrelated
//     document's "local 5" could silently satisfy a reference to a
//     completely different document's own "local 5".
//
// Edges are grouped per fathom package (the FROM symbol's OWN
// SymbolNode.PackagePath(), never a geometry-derived guess) because
// store.PutTypedEdges (internal/store) replaces typed edges at exactly
// that granularity — the same package-granular replace unit the Go typed
// pass already uses, so re-importing one changed file's worth of SCIP
// output never disturbs another package's rows.
package scip
