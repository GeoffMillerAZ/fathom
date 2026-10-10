package core

import "strings"

// ModuleSymbolName is the name of a file's module symbol (K18, ADR-048
// amendment 2026-10-10): the one definition the tags engine mints per file of
// a dialect whose top level executes code (its manifest's module_symbol,
// internal/extractor/tsextract_module.go), from line 1 to the file's last line.
// It is what a call or an import made at the file's top level is made from,
// and what an import of the module targets.
//
// The name is Python's own for a module's top-level code (a traceback's
// `in <module>`, a code object's co_name). No language spells an identifier
// with angle brackets, so it never collides with, or is mistaken for, a
// definition of the file (scip-python names a module `<path>/__init__:`, and a
// leaf `__init__` would join a class's own `X.__init__`, O527). The file is the
// symbol's identity: its key carries the file's directory and package
// (SymbolKey), and a dialect minting it takes its package from the file's own
// name (package strategy "dir", checked at load), so each file's module symbol
// has a key of its own while the dialect claims one extension per name.
const ModuleSymbolName = "<module>"

// IsModuleSymbol reports whether n is a file's minted module symbol. The
// name alone decides, never the kind, so every dialect whose manifest turns
// module_symbol on inherits the lookup contract below with no query edit.
//
// The lookup contract (wave h2, .fathom/observations/O741.md): a module
// symbol answers an exact `<module>` query and the file-scoped lookups
// (outline, the symbol enclosing a line), and keeps every edge, so
// blast_radius, find_callers and symbol_context walk through it. It is no
// answer to a query that did not name it: find_symbol's case-insensitive,
// substring and typo tiers skip it, keyword-ranked queries (query_symbols,
// pack_context, find_symbol's last did_you_mean) rank it only when the
// query names `<module>` exactly, and risk ranking reads past its stored
// centrality. Each reader excludes it on read; nothing stored changes.
func (n SymbolNode) IsModuleSymbol() bool { return n.Name == ModuleSymbolName }

// IsModuleSymbolKey is IsModuleSymbol for a canonical key (SymbolKey's
// "service::package.Name"), for rows that carry only the key: the
// symbol_centrality rows risk ranking reads. A key names a module symbol
// when its name, after the package's "." or the bare "::", is
// ModuleSymbolName; no package qualifier ends in angle brackets.
func IsModuleSymbolKey(key string) bool {
	return strings.HasSuffix(key, "."+ModuleSymbolName) || strings.HasSuffix(key, "::"+ModuleSymbolName)
}
