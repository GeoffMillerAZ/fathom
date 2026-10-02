; dialects/rust/references.scm — the file resolver's references (LB-F74;
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm
; and imports.scm (TSExtractor.ExtractFileSpecs) once the manifest names it
; (queries.references): every type name (a field's, a parameter's, a return
; type, a type argument, an impl's trait and self type), a scoped type
; `m::Bar` and a scoped path `Foo::new`, `X::CONST`. The extractor keeps those
; whose head a `use` binds or, in a file with a glob `use`, that start with an
; upper case letter. A `pub use` re-export (LB-F75's channel) is not captured
; here yet.
(type_identifier) @ref.name

(scoped_type_identifier) @ref.name

(scoped_identifier) @ref.name

; Not references of their own: a scoped path's last segment (`m::Bar`'s
; Bar, read with its path).
(scoped_type_identifier
  name: (type_identifier) @ref.not)

(scoped_identifier
  name: (identifier) @ref.not)
