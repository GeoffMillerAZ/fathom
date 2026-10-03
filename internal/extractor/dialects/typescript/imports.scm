; dialects/typescript/imports.scm — imports (SU-11, docs/plans/substrate.md §B; ADR-041 decision 5) ───────
; Captures for internal/fusion.ResolveFileEdges's per-file resolver — never
; consumed by TSExtractor.Extract's own splitTSMatches (its capture switch
; only recognizes "name"/"reference.call"/"definition.*"; every "import.*"
; capture here is a no-op for the existing symbol extraction, additive and
; safe). Predicate-free (see python.scm's header) — the require() forms
; below capture ANY single-string-argument call's callee identifier, and
; internal/extractor's Go code filters the callee text to "require" itself
; since this engine's query runner cannot evaluate #eq?.

(import_statement
  (import_clause
    (identifier) @import.default)
  source: (string) @import.source) @import.stmt

(import_statement
  (import_clause
    (namespace_import
      (identifier) @import.namespace))
  source: (string) @import.source) @import.stmt

(import_statement
  (import_clause
    (named_imports
      (import_specifier
        name: (identifier) @import.named
        alias: (identifier)? @import.alias)))
  source: (string) @import.source) @import.stmt

; A side-effect-only import (`import './x'`) has no clause at all — this
; pattern also matches every import_statement the three above already
; matched (Go groups by the enclosing statement's byte range and keeps only
; the more specific capture when both fire).
(import_statement
  source: (string) @import.bare_source) @import.bare_stmt

; const mod = require('./x') — CommonJS whole-module require.
(variable_declarator
  name: (identifier) @import.requirename
  value: (call_expression
    function: (identifier) @import.requirecallee
    arguments: (arguments (string) @import.requiresource))) @import.requirestmt

; const {a, b} = require('./x') — CommonJS destructured require.
(variable_declarator
  name: (object_pattern
    (shorthand_property_identifier_pattern) @import.requirenamed)
  value: (call_expression
    function: (identifier) @import.requirecallee2
    arguments: (arguments (string) @import.requiresource2))) @import.requirestmt2

; import x = require('./y') — TypeScript's import-equals declaration binds the
; whole module, as `import * as x` does (wave lh). Its source sits on the
; import_require_clause, so the side-effect pattern above never matched it and
; the statement bound nothing.
(import_statement
  (import_require_clause
    (identifier) @import.namespace
    source: (string) @import.source)) @import.stmt
