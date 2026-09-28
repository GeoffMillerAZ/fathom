; dialects/python/imports.scm — imports (SU-11, docs/plans/substrate.md §B; ADR-041 decision 5) ───────
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
