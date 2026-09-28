package connector

// typededges.go — the per-language typed-edge PRODUCER table (ADR-041
// decision 5, SU-13, docs/plans/substrate.md): the code-side fact
// `fathom capabilities`'s typed_edge_producers field reports, and the
// table TestLanguagesTrackerMatchesRegistry (internal/connector/
// languages_test.go) cross-checks against docs/languages.md's own bolded
// level word so the doc and the code can never drift silently.
//
// This reports what MECHANISM currently exists in code for a language —
// never an aspirational target. CUE is the case that matters: docs/
// languages.md names "native" as CUE's TARGET level (tracked as SU-10,
// still open), but internal/fusion.EnrichCUEEdges does not exist yet, so
// the honest fact here is "syntax" — ADR-027's rule that an instrument
// may not degrade toward healthy applies to what fathom claims about
// itself as much as to what it claims about a service it indexes.
var typedEdgeProducers = map[string]string{
	// native: fathom resolves types itself.
	"go": "native",

	// scip: fathom's generic `fathom ingest --scip` import mechanism
	// (internal/scip.ImportIndex, SU-9) exists and works for any SCIP
	// index regardless of which external indexer produced it — the tool
	// named after the colon is the delegate docs/languages.md tracks for
	// this language, per ADR-041 decision 5's order, not a claim that
	// mechanism has actually been run for any particular service.
	"ts":     "scip:scip-typescript",
	"js":     "scip:scip-typescript",
	"py":     "scip:scip-python",
	"java":   "scip:scip-java",
	"kotlin": "scip:scip-java",
	"scala":  "scip:scip-java",
	"rust":   "scip:rust-analyzer",
	"c":      "scip:scip-clang",
	"cpp":    "scip:scip-clang",
	"ruby":   "scip:scip-ruby",
	"cs":     "scip:scip-dotnet",

	// syntax: call names by syntax only — every other registered
	// language, CUE included (see the doc comment above).
	"cue":        "syntax",
	"php":        "syntax",
	"sh":         "syntax",
	"lua":        "syntax",
	"sql":        "syntax",
	"terraform":  "syntax",
	"yaml":       "syntax",
	"md":         "syntax",
	"dockerfile": "syntax",
	"cmake":      "syntax",
	"proto":      "syntax",
	"graphql":    "syntax",
	"toml":       "syntax",
	"json":       "syntax",
	"templ":      "syntax",

	// file: no maintained SCIP producer for CSS (design §7's producer
	// table has no CSS row), but its manifest names an imports query and
	// a relative-path strategy (LB-F11), so @import edges DO reach the
	// file-resolved pass (internal/fusion.ResolveFileEdges) — a stronger,
	// honest claim than the syntax-only floor, though still heuristic
	// tier (core.EdgeConfidenceOf never presents it as typed).
	"css": "file",
}

// TypedEdgeProducer returns lang's typed-edge producer string — "native",
// "scip:<tool>", "file", or "syntax" — or "" when lang is not a
// registered language tag (RegisteredExtensions/LanguageForPath).
func TypedEdgeProducer(lang string) string { return typedEdgeProducers[lang] }

// TypedEdgeProducers returns the full lang -> producer map, one entry per
// distinct Lang tag RegisteredExtensions() classifies — `fathom
// capabilities`'s typed_edge_producers field (SU-13).
func TypedEdgeProducers() map[string]string {
	out := map[string]string{}
	for _, ext := range RegisteredExtensions() {
		lang := LanguageForPath(ext)
		if lang == "" {
			continue
		}
		if p, ok := typedEdgeProducers[lang]; ok {
			out[lang] = p
		}
	}
	return out
}
