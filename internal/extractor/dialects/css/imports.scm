; dialects/css/imports.scm — @import edges (LB-Q-css, design §3.7/§6).
; Generic vocabulary only (@import.stmt/@import.module — LB-F7): CSS's
; @import has no local binding to track, only a target file (`relative-path`
; strategy, same as JS/TS and HTML src/href — design §6 table).
;
;
; An imported stylesheet joins the importing one: its rules enter the same
; cascade, so its custom properties and @keyframes are names a var() or an
; animation of the importer reads (design 1.8 W2, LB-F459). Each @import is
; therefore also an @import.wildcard, every top-level name of the target in
; scope (the CMake include precedent), and with the manifest's module_symbol
; (K18) each one is a file edge from the importing sheet's module symbol to
; the imported sheet's.
;
; Two shapes (indexbench lang-probe, testdata/shapes.css):
;   @import "base.css";                    — a bare string_value
;   @import url("theme.css") screen;       — the url() function form
(import_statement
  (string_value) @import.module) @import.stmt @import.wildcard

(import_statement
  (call_expression
    (function_name) @_fn
    (#eq? @_fn "url")
    (arguments
      (string_value) @import.module))) @import.stmt @import.wildcard
