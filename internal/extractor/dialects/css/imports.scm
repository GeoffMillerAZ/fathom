; dialects/css/imports.scm — @import edges (LB-Q-css, design §3.7/§6).
; Generic vocabulary only (@import.stmt/@import.module — LB-F7): CSS's
; @import has no local binding to track, only a target file (`relative-path`
; strategy, same as JS/TS and HTML src/href — design §6 table).
;
; Two shapes (indexbench lang-probe, testdata/shapes.css):
;   @import "base.css";                    — a bare string_value
;   @import url("theme.css") screen;       — the url() function form
(import_statement
  (string_value) @import.module) @import.stmt

(import_statement
  (call_expression
    (function_name) @_fn
    (#eq? @_fn "url")
    (arguments
      (string_value) @import.module))) @import.stmt
