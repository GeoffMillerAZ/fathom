package extractor

// cuedoc.go — the CUE dialect's cross-file references (LB-Q-cue, design §4):
// what CUEExtractor.ExtractDoc gives the document-path engine's cross-file
// pass (fusion.ResolveDocXrefs), which draws them as file-tier edges at ingest
// (the "doc-xref" typed edges) and the harness scores them as cross-file
// references.
//
// A CUE file names other files three ways, and the extractor, seeing one
// file, records each as written for fusion's cue-import and cue-package
// strategies (internal/fusion/cuexref.go) to resolve over the whole set:
//
//   - an import (`import "example.com/m/schema"`), from the import entity: the
//     files of the package it loads (strategy cue-import, Raw the import path,
//     no Target), each file's package entity the target;
//   - a selector through an import (`schema.#Service`, `schema.#Service.port`),
//     from the referring field: the declaration of that name in the package
//     (cue-import, Raw the name below the package, Target the import path);
//   - a name the file does not declare (`#Kit`, `shared.x`), from the
//     referring field: the declaration of that name in another file of the
//     same package (cue-package, Raw the name).
//
// The standard library resolves to no file and is not recorded.

import "sort"

// The CUE dialect's cross-file strategies and edges.
const (
	CUEImportStrategy  = "cue-import"
	CUEPackageStrategy = "cue-package"
	CUEImportsEdge     = "imports"
	CUEReferencesEdge  = "references"
)

// sortCUECross orders a file's cross-file references deterministically: by
// line, then strategy, origin and value.
func sortCUECross(xs []DocCrossFileRef) {
	sort.SliceStable(xs, func(i, j int) bool {
		a, b := xs[i], xs[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		for _, p := range [][2]string{{a.Strategy, b.Strategy}, {a.From, b.From}, {a.Raw, b.Raw}, {a.Target, b.Target}, {a.Edge, b.Edge}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
}
