; dialects/java/references.scm — the file resolver's references (LB-F74;
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm
; and imports.scm (TSExtractor.ExtractFileSpecs).
;
; Type references (a field's, a parameter's, a return type, a type argument,
; `new X`, extends and implements, a cast, a class literal, a throws clause),
; a qualified type `Outer.Inner` or `a.b.C`, an annotation, and the class a
; static member access names (`Util.call()`, `Other.CONST`, `Foo::bar`). The
; extractor keeps those whose head an import binds or that start with an
; upper case letter (Java sees its own package's classes unqualified).
(type_identifier) @ref.name
(scoped_type_identifier) @ref.name

(marker_annotation name: [(identifier) (scoped_identifier)] @ref.name)
(annotation name: [(identifier) (scoped_identifier)] @ref.name)

(field_access
  object: (identifier) @ref.head
  field: (identifier) @ref.member)

(method_invocation
  object: (identifier) @ref.head
  name: (identifier) @ref.member)

(method_reference
  .
  (identifier) @ref.name)

; Not a reference of its own: a qualified type's last segment
; (`Outer.Inner`'s Inner, read with its qualifier).
(scoped_type_identifier
  (type_identifier) @ref.not
  .)
