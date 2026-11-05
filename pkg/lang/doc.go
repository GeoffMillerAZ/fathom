// Package lang reads source files and configuration into facts: symbols,
// references, refused references and the facts a dialect declares, each with
// the level that readings measured for the dialect that read it.
//
// Bytes and a path go in. Nothing else is needed: no store, no daemon, no
// index, no git and no network. A Reader is safe for concurrent use and
// deterministic: equal inputs give byte-equal JSON.
//
// The package is a door over fathom's own extraction code (ADR-092). Public
// types are defined here, so a change inside fathom does not change this
// API. Symbol.Canonical is a closed vocabulary that only grows; Symbol.Kind
// is each dialect's own name and may change as a language improves.
//
// The wazero runtime that runs the grammars is process-wide, so its on-disk
// compilation cache is a package function, EnableCompilationCache, and is
// off by default. Without it the package writes no file.
package lang

//go:generate go run levels_gen.go
