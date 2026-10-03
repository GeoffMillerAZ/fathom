; dialects/cpp/references.scm — the file resolver's references (LB-F74, LB-F196;
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm
; and imports.scm once the manifest names it (queries.references). Written in
; this repository against tree-sitter-cpp's grammar; no upstream query. C++
; reaches ingest through the cppextract incumbent, which runs this query
; over its own tree (nativespecs.go, EW-53).
;
; What a C++ file names without calling it, and a header can define:
;   - every type name (type_identifier): a class, struct, enum, alias or
;     template argument, wherever it is written;
;   - a qualified name `ns::Name`, `ns::inner::Name` (qualified_identifier),
;     including the one a using-declaration (`using ns::Name;`) names; its last
;     segment is read with its path, never as a bare name of its own;
;   - an upper-case word (a macro or an all-capitals constant), read or invoked.
; The resolver binds a name only to a definition in a header the file reaches
; through its #include graph, and never to a name the file defines itself.
(type_identifier) @ref.name

(qualified_identifier) @ref.name

(qualified_identifier
  name: (type_identifier) @ref.not)

(qualified_identifier
  name: (identifier) @ref.not)

(qualified_identifier
  name: (template_type) @ref.not)

((identifier) @ref.name
  (#match? @ref.name "^[A-Z][A-Z0-9_]*$"))

; A test macro invoked as a definition, `TEST(suite, name) { ... }`
; (googletest, Catch's TEST_CASE style), reads as a function whose parameters
; are types. Its LAST argument names the test (`TEST(xchar_test, ostream)`),
; never a type, even where a reached header defines a class of that name; an
; earlier argument may be a type (TEST_F(Fixture, name)) and stays a reference.
(function_definition
  declarator: (function_declarator
    declarator: (identifier) @_macro
    parameters: (parameter_list
      (parameter_declaration
        type: (type_identifier) @ref.not) .))
  (#match? @_macro "^[A-Z][A-Z0-9_]*$"))
