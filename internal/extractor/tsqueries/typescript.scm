; typescript.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("javascript") dispatches .ts/.tsx files
; to this query set, ADR-021 §4). TypeScript's grammar is a superset of
; JavaScript's, so every javascript.scm pattern still applies (class
; declarations, functions, methods, arrow-function const bindings) — this
; file repeats them plus the TS-only constructs (interfaces, type aliases,
; enums). Used for both the "typescript" and "tsx" grammar entry points.
;
; Predicate-free by design — see python.scm's header comment for why.

(class_declaration
  name: (_) @name) @definition.class

(function_declaration
  name: (identifier) @name) @definition.function

(generator_function_declaration
  name: (identifier) @name) @definition.function

; Name alternation per critic B2 — see javascript.scm's comment.
(method_definition
  name: [(property_identifier) (private_property_identifier) (computed_property_name)] @name) @definition.method

(variable_declarator
  name: (identifier) @name
  value: [(arrow_function) (function_expression)]) @definition.function

(interface_declaration
  name: (type_identifier) @name) @definition.interface

(type_alias_declaration
  name: (type_identifier) @name) @definition.type

(enum_declaration
  name: (identifier) @name) @definition.enum

(call_expression
  function: (_) @reference.call)

; ─── Imports (SU-11, docs/plans/substrate.md §B; ADR-041 decision 5) ───────
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
