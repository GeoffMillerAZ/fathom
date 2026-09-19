; python.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("python"), ADR-021 §4).
;
; Deliberately predicate-free (no #eq?/#strip!/#select-adjacent! etc. — those
; tree-sitter query predicates are evaluated by the calling application, not
; the C query engine itself, and this engine's query runner does not
; implement predicate evaluation; every pattern here relies on structural
; matching alone, which the query engine evaluates natively). Adapted from
; tree-sitter-python's own queries/tags.scm (upstream, same repo as the
; grammar) with the doc-comment association predicates dropped, since fathom
; extracts symbols, not documentation.

(class_definition
  name: (identifier) @name) @definition.class

(function_definition
  name: (identifier) @name) @definition.function

; Only a DIRECT top-level assignment counts as a module-level variable — the
; nesting in this pattern (module > expression_statement > assignment) is a
; structural constraint the query engine enforces, so assignments inside a
; function or class body never match this pattern at all.
(module
  (expression_statement
    (assignment
      left: (identifier) @name)) @definition.variable)

; Naive same-file call collection (whatever expression is being called, as
; source text) — mirrors the incumbent CGO extractor's CollectCalls, not a
; typed/resolved edge (ADR-021 §3 typed edges are a Go-only enrichment pass;
; tree-sitter-derived call edges beyond this are deferred, see XT-7).
(call
  function: (_) @reference.call)

; ─── Imports (SU-11, docs/plans/substrate.md §B; ADR-041 decision 5) ───────
; Captures for internal/fusion.ResolveFileEdges's per-file resolver — see
; javascript.scm's matching header for the "additive, never consumed by
; Extract's own switch" contract.

; import pkg.mod
(import_statement
  name: (dotted_name) @import.module) @import.stmt

; import pkg.mod as alias
(import_statement
  name: (aliased_import
    name: (dotted_name) @import.module
    alias: (identifier) @import.alias)) @import.stmt

; from pkg.mod import name [, name2, ...]
(import_from_statement
  module_name: (dotted_name) @import.frommodule
  name: (dotted_name) @import.fromname) @import.fromstmt

; from pkg.mod import name as alias
(import_from_statement
  module_name: (dotted_name) @import.frommodule
  name: (aliased_import
    name: (dotted_name) @import.fromname
    alias: (identifier) @import.fromalias)) @import.fromstmt

; from . import name / from .relmod import name — relative imports; the
; module_name field is a relative_import node (one or more leading dots
; plus an optional dotted_name), rendered as source text ("." / ".relmod").
(import_from_statement
  module_name: (relative_import) @import.relmodule
  name: (dotted_name) @import.relname) @import.relstmt

(import_from_statement
  module_name: (relative_import) @import.relmodule
  name: (aliased_import
    name: (dotted_name) @import.relname
    alias: (identifier) @import.relalias)) @import.relstmt
