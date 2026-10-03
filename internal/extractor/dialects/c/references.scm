; dialects/c/references.scm — the file resolver's references (LB-F74, LB-F196;
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm
; and imports.scm once the manifest names it (queries.references).
; Written in this repository against tree-sitter-c's grammar; no upstream query.
;
; What a C file names without calling it, and a header can define:
;   - every type name: a typedef name, a `struct x` / `union x` / `enum x`
;     tag (type_identifier), wherever it is written (a declaration, a cast, a
;     sizeof, a field, a parameter);
;   - an upper-case word (an object-like or function-like macro, an enumerator
;     by the all-capitals convention: UV_EOF, CURLE_OK), where it is read or
;     invoked.
; The resolver binds a name only to a definition in a header the file reaches
; through its #include graph (include-path strategy, compile-database -I
; order), and never to a name the file defines itself. A lower-case enumerator
; or global variable is no definition the dialect's tags query yields (variables
; and enumerators are not symbols), so it is not captured: that gap is
; disclosed in the reading, not guessed.
(type_identifier) @ref.name

((identifier) @ref.name
  (#match? @ref.name "^[A-Z][A-Z0-9_]*$"))
