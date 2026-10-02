; dialects/rust/references.scm — the file resolver's references (LB-F74;
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm
; and imports.scm (TSExtractor.ExtractFileSpecs) once the manifest names it
; (queries.references): every type name (a field's, a parameter's, a return
; type, a type argument, an impl's trait and self type), a scoped type
; `m::Bar` and a scoped path `Foo::new`, `X::CONST`. The extractor keeps those
; whose head a `use` binds or, in a file with a glob `use`, that start with an
; upper case letter. A `pub use` re-export (LB-F75's channel, LB-F197) is
; captured below: the names its module makes visible through this file.
(type_identifier) @ref.name

(scoped_type_identifier) @ref.name

(scoped_identifier) @ref.name

; Not references of their own: a scoped path's last segment (`m::Bar`'s
; Bar, read with its path).
(scoped_type_identifier
  name: (type_identifier) @ref.not)

(scoped_identifier
  name: (identifier) @ref.not)

; --- pub use re-exports (LB-F197, O289): the resolver follows a named import
; of a re-exported name to the file that defines it. @export.stmt is the whole
; declaration; a statement with no @export.name re-exports every name of its
; module (a glob); a group's members each match once and are joined by statement.
; Written in this repository against tree-sitter-rust's use_declaration; a
; nested group (`pub use a::{b::C}`) is not captured.

; pub use a::b::C;
(use_declaration
  (visibility_modifier)
  argument: (scoped_identifier
    path: (_) @export.module
    name: (identifier) @export.name)) @export.stmt

; pub use a::b::C as D;
(use_declaration
  (visibility_modifier)
  argument: (use_as_clause
    path: (scoped_identifier
      path: (_) @export.module
      name: (identifier) @export.name)
    alias: (identifier) @export.alias)) @export.stmt

; pub use a::b::*;
(use_declaration
  (visibility_modifier)
  argument: (use_wildcard
    [(scoped_identifier) (identifier) (crate) (self) (super)] @export.module)) @export.stmt

; pub use a::{B, C as D};
(use_declaration
  (visibility_modifier)
  argument: (scoped_use_list
    path: (_) @export.module
    list: (use_list
      (identifier) @export.name))) @export.stmt

(use_declaration
  (visibility_modifier)
  argument: (scoped_use_list
    path: (_) @export.module
    list: (use_list
      (use_as_clause
        path: (identifier) @export.name
        alias: (identifier) @export.alias)))) @export.stmt
