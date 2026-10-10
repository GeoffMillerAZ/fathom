; dialects/scss/imports.scm — Sass's three loads (LB-F465). Shapes from
; `indexbench lang-probe -dialect scss`. Each resolves by the relative-path
; strategy with Sass's partial rule (manifest imports: partial_prefix "_",
; the .scss and .css suffixes, the _index and index files), the rule Dart
; Sass's own loader follows; each is also a file edge between the module
; symbols (K18).

; @use "url" [as alias] [with (...)]: a module bound to its namespace. A
; built-in module (`sass:math`) names no file.
(use_statement
  (string (string_content) @import.module)
  alias: (module_name)? @import.alias
  (#not-match? @import.module "^sass:")) @import.stmt

; @forward "url" [show ...|hide ...]: the module's members re-exported.
(forward_statement
  (string (string_content) @import.module)
  (#not-match? @import.module "^sass:")) @import.stmt

; @import "a", "b": the legacy load (the one query_statement whose prelude is a
; string: @media, @supports and @container write none), every member global, so each target's
; names are in scope (a wildcard). A plain CSS import (a .css URL, an
; absolute or protocol URL, url()) is the browser's, not a Sass load.
(query_statement
  (string (string_content) @import.module)
  (#not-match? @import.module "[.]css$|^([a-z]+:)?//")) @import.stmt @import.wildcard
