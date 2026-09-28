; dialects/csharp/imports.scm — using-directive captures for internal/fusion's
; per-file resolver (design §3.7, §6 namespace-dir). Written in this
; repository: tree-sitter-c-sharp's own queries/tags.scm carries no import
; captures at all.
;
; using System;                 -> bare module, no field name on the child
; using System.Collections.Generic; -> same shape, a qualified_name child
; using Sys = System;           -> name: field is the ALIAS, the module is
;                                   the remaining (unnamed) child
; using static System.Math;     -> "static" is an anonymous token (not a
;                                   named field), so this reads identically
;                                   to a plain dotted using — still a real
;                                   dependency edge, just without the
;                                   whole-type-members-in-scope distinction
; global using System.Linq;     -> "global" is likewise anonymous; same shape
;
; Two patterns, anchored ("."): a single optional name: field ahead of an
; unqualified (_) double-matched every statement (verified directly,
; docs/design/language-breadth.md §13 step 12 — tree-sitter's query engine
; explores both "alias consumed the first child" and "alias skipped, (_)
; consumed it instead" as independent matches once a later unqualified
; capture could re-bind it). "." anchors pin exact child-count and order
; instead: the bare shape requires its captured child to be BOTH the first
; AND the last (i.e. the only) child, which an aliased using_directive's two
; children can never satisfy, and vice versa — one match per statement,
; either shape, confirmed by lang-probe on all five shapes in the header.
(using_directive
  .
  name: (_) @import.alias
  .
  (_) @import.module
  .) @import.stmt

(using_directive
  .
  (_) @import.module
  .) @import.stmt
