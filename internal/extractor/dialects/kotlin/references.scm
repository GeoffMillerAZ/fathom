; dialects/kotlin/references.scm — the file resolver's references (LB-F74;
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm
; and imports.scm (TSExtractor.ExtractFileSpecs).
;
; A type (a property's, a parameter's, a return type, a supertype, a type
; argument, an annotation, `p.Outer.Inner`), the object or class a navigation
; names (`Other.CONST`, `Util.call()`), and any other name outside a call's
; own text (a top-level property or function referenced by value). The
; extractor keeps those whose head an import binds or that start with an
; upper case letter (Kotlin sees its own package's names unqualified).
(user_type) @ref.name

(navigation_expression
  .
  (simple_identifier) @ref.head
  .
  (navigation_suffix
    (simple_identifier) @ref.member))

(simple_identifier) @ref.name

; Not references of their own: a navigation's member (`Kinds.DEFAULT`'s
; DEFAULT), a named argument's label.
(navigation_suffix
  (simple_identifier) @ref.not)

(value_argument
  (simple_identifier) @ref.not
  .
  "=")
