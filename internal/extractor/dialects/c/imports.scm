; dialects/c/imports.scm — #include captures for internal/fusion's
; per-file resolver (design §3.7, §6 include-path). Written in this
; repository: tree-sitter-c's own queries/tags.scm carries no import
; captures at all.
;
; #include "local.h"   -> a quoted (relative-first) include
; #include <system.h>  -> an angle-bracket (source_roots / -I) include
; Both shapes are the same preproc_include grammar node; the string vs the
; system_lib_string child tells them apart, so one capture per path suffices
; and the whole token's text (quotes/angles included) is unquoted downstream
; by resolveIncludePath (internal/fusion/resolvestrategy.go).
(preproc_include
  path: (string_literal) @import.module) @import.stmt

(preproc_include
  path: (system_lib_string) @import.module) @import.stmt
