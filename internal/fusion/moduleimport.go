package fusion

import (
	"slices"

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
// that submodule too. A re-export of a module (a TypeScript barrel's
// `export * from`) reads it the same way. A target with no module symbol (a
// dialect that mints none) gives no edge, so every other dialect's edges are
// unchanged.
//
// drawn is the symbol edges path's own calls and references already drew
// (LB-F411, owner ruling D1, O748): a binding ES import (a named or default
// one, ImportSpec.Binding) of a file one of them reaches into adds nothing
// a file-level question needs, so it stores no module edge; a side-effect
// import, `import * as`, a CommonJS require and a re-export keep theirs.
func (idx *resolveIndex) moduleImportEdges(r *importResolver, path string, syms []core.SymbolNode, imports []extractor.ImportSpec, drawn []core.Edge) []core.Edge {
	if (len(imports) == 0 && len(idx.specs[path].Exports) == 0) || len(syms) == 0 {
		return nil
	}
	at := enclosingAt(importers(syms))
	var reached map[string]bool // the files drawn reaches into, built on a binding import's first use
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
		if spec.Binding {
			if reached == nil {
				reached = idx.filesReached(drawn)
			}
			targets = slices.DeleteFunc(targets, func(f string) bool { return reached[f] })
		}
		edges = idx.appendModuleEdges(edges, path, from, targets)
	}
	// A re-export of a module (`export * from "./a"`, `export { B } from
	// "./b"`, LB-F381) reads that module as an import does: an edge from the
	// definition around it, the barrel's module symbol at its top level.
	for _, spec := range idx.specs[path].Exports {
		if spec.Module == "" {
			continue
		}
		if from := at(spec.Line); from != nil {
			edges = idx.appendModuleEdges(edges, path, from, r.resolveUse(path, spec.Module))
		}
	}
	return edges
}

// filesReached is every file declaring a symbol one of edges points to.
func (idx *resolveIndex) filesReached(edges []core.Edge) map[string]bool {
	out := make(map[string]bool, len(edges))
	for _, e := range edges {
		for _, f := range idx.keyFiles[e.ToSymbol] {
			out[f] = true
		}
	}
	return out
}

// appendModuleEdges appends an edge from from to each target file's module
// symbol, the importing file itself and a file with none left out.
func (idx *resolveIndex) appendModuleEdges(edges []core.Edge, path string, from *core.SymbolNode, targets []string) []core.Edge {
	for _, file := range targets {
		if file == path {
			continue
		}
		if to, ok := idx.moduleSymbol(file); ok && to != from.Key() {
			edges = append(edges, core.Edge{FromFile: path, FromSymbol: from.Key(), ToSymbol: to, Kind: core.EdgeFileResolved})
		}
	}
	return edges
}

// valueKinds are the canonical kinds that name a value, never code that
// imports: a module-level `const h = require("./h")` (LB-F382) is the module's
// import, written in the constant's initializer.
var valueKinds = map[string]bool{"constant": true, "variable": true, "field": true, "property": true}

// importers is syms less the definitions that name a value (valueKinds): an
// import is made from the callable or container around it, or the module.
func importers(syms []core.SymbolNode) []core.SymbolNode {
	out := make([]core.SymbolNode, 0, len(syms))
	for _, s := range syms {
		if c, _ := extractor.CanonicalKind(string(s.Kind)); !valueKinds[c] {
			out = append(out, s)
		}
	}
	return out
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
