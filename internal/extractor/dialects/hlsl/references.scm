; dialects/hlsl/references.scm — the file resolver's references (vocabulary in
; internal/extractor/tsrefs.go), compiled apart from tags.scm and imports.scm.
; Written in this repository against tree-sitter-hlsl; no upstream query.
;
; What an HLSL file names without calling it, and a header can define: every
; type name (a struct, typedef or template argument, wherever it is written)
; and an upper-case word (a macro or an all-capitals constant), read or
; invoked. The resolver binds a name only to a definition in a header the file
; reaches through its #include graph, and never to a name the file defines
; itself.
(type_identifier) @ref.name

((identifier) @ref.name
  (#match? @ref.name "^[A-Z][A-Z0-9_]*$"))
