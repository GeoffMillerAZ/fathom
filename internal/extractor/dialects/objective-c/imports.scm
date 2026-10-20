; dialects/objective-c/imports.scm — #import, #include and @import captures for
; internal/fusion's per-file resolver (design §3.7, §6 include-path). Written in
; this repository: tree-sitter-objc ships no import queries.
;
; #import "Local.h"        -> a quoted (relative-first) import
; #import <Framework/X.h>  -> an angle-bracket (source_roots / -I) import
; #include "x.h"           -> the same preproc_include node as #import
; The whole token's text (quotes or angles included) is unquoted downstream by
; resolveIncludePath (internal/fusion/resolvestrategy.go).
(preproc_include
  path: (string_literal) @import.module) @import.stmt

(preproc_include
  path: (system_lib_string) @import.module) @import.stmt
