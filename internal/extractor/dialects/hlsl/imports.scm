; dialects/hlsl/imports.scm — #include, the only import form (LB-Q-hlsl). The
; manifest's include-path strategy resolves the quoted form relative to the
; includer, then the source roots.
(preproc_include path: (string_literal) @import.module) @import.stmt
(preproc_include path: (system_lib_string) @import.module) @import.stmt
