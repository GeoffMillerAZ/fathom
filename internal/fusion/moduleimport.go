package fusion

import (
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// moduleImportEdges (K18, ADR-048 amendment 2026-10-10): every import of path
// that names a module of the universe is an edge to that module's symbol
// (core.ModuleSymbolName, minted by a dialect whose manifest names
// module_symbol), made from the definition around the import's line — the
// file's own module symbol at its top level, a function for an import inside
// it (enclosingAt, the reference rule). The module an import names is the one
// its module path resolves to (`import a.b`, `from a.b import c`: a.b) and,
// for a named import whose name is itself a submodule (`from a import b`),
// that submodule too. A target with no module symbol (a dialect that mints
// none) gives no edge, so every other dialect's edges are unchanged.
func (idx *resolveIndex) moduleImportEdges(r *importResolver, path string, syms []core.SymbolNode, imports []extractor.ImportSpec) []core.Edge {
	if len(imports) == 0 || len(syms) == 0 {
		return nil
	}
	at := enclosingAt(syms)
	var edges []core.Edge
	for _, spec := range imports {
		from := at(spec.Line)
		if from == nil {
			continue
		}
		targets := r.resolveImport(path, spec, idx)
		if spec.Kind == extractor.ImportNamed {
			if sep := r.submoduleSep(); sep != "" {
				targets = append(targets, r.resolveSub(path, spec, idx, sep)...)
			}
		}
		for _, file := range targets {
			if file == path {
				continue
			}
			if to, ok := idx.moduleSymbol(file); ok && to != from.Key() {
				edges = append(edges, core.Edge{FromFile: path, FromSymbol: from.Key(), ToSymbol: to, Kind: core.EdgeFileResolved})
			}
		}
	}
	return edges
}

// moduleSymbol is file's module symbol's key, when its dialect mints one.
func (idx *resolveIndex) moduleSymbol(file string) (string, bool) {
	for _, s := range idx.byFile[file] {
		if isModuleSymbol(s) {
			return s.Key(), true
		}
	}
	return "", false
}

// isModuleSymbol reports whether s is a file's minted module symbol: its name
// and its kind's canonical "module" both say so.
func isModuleSymbol(s core.SymbolNode) bool {
	if !s.IsModuleSymbol() {
		return false
	}
	c, ok := extractor.CanonicalKind(string(s.Kind))
	return ok && c == "module"
}
