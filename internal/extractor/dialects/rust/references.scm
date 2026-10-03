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

; A constant or static read by its bare name (`SHERLOCK`, `FULL_TEMPLATE`): an
; identifier in the Rust convention for one (all upper case), where the engine keeps it
; only if a `use` binds it or the file globs. It resolves to a RustConst or RustStatic
; the dialect extracts, so the edge to its file is the dialect's alone.
((identifier) @ref.name
  (#match? @ref.name "^[A-Z][A-Z0-9_]+$"))

; A name written inside a macro's arguments (`assert_eq!(e.kind(), ErrorKind::Bad)`,
; `rgtest!(t, |dir: Dir| ..)`): the arguments are a flat token_tree of identifiers, never
; a type_identifier or a scoped path, so a type, an enum or a constant named there is
; seen only here. Upper-case-first names only (a type, a variant, a constant): a
; lower-case token is as likely a local as an import.
(token_tree
  (identifier) @ref.name
  (#match? @ref.name "^[A-Z]"))

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
; group nested more than one level deep is not captured.

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

; pub use crate::{a::B, c::{D, E}};   (one group inside a group: the outer path
; is @export.prefix, the inner path the module)
(use_declaration
  (visibility_modifier)
  argument: (scoped_use_list
    path: (_) @export.prefix
    list: (use_list
      (scoped_use_list
        path: (_) @export.module
        list: (use_list
          (identifier) @export.name))))) @export.stmt

(use_declaration
  (visibility_modifier)
  argument: (scoped_use_list
    path: (_) @export.prefix
    list: (use_list
      (scoped_identifier
        path: (_) @export.module
        name: (identifier) @export.name)))) @export.stmt

; --- #[macro_use] mod x;  (O471): a file module whose macros the crate root
; shares. A `#[macro_export] macro_rules!` item lives at the crate root, and
; `#[macro_use] mod macros;` is how a crate declares the file defining them, so the
; declaring file re-exports every name of x (a glob). It over-exports x's other
; top-level items, which a named import only finds when it asks for one by name. A
; plain `mod y;` and an inline `mod y { }` export nothing. Other attributes may follow
; #[macro_use] (clap's `#[macro_use] #[allow(missing_docs)] mod macros;`).
((attribute_item
  (attribute
    (identifier) @_macro_use
    (#eq? @_macro_use "macro_use")))
.
(attribute_item)*
.
(mod_item
  name: (identifier) @export.module
  !body) @export.stmt)
