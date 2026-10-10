package core

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

// IsModuleSymbol reports whether n is a file's minted module symbol.
func (n SymbolNode) IsModuleSymbol() bool { return n.Name == ModuleSymbolName }
