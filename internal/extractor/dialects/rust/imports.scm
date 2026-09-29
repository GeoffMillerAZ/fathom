; dialects/rust/imports.scm — `use` declarations and file modules for internal/fusion's
; per-file resolver (design §3.7, §6 crate-mod). Written in this repository:
; tree-sitter-rust's own queries/tags.scm carries no import captures at all, and the
; incumbent CGO port (rustextract.go) never resolved an import.
;
;   mod x;                     a child module file: module `x`, resolved to x.rs or
;                              x/mod.rs beside the declaring file (an inline
;                              `mod x { }` declares nothing to resolve)
;   use a::b::c;               two specs, because a path's last segment may be an
;                              item (a named import of `c` from module `a::b`) or a
;                              module (a whole import of `a::b::c` bound to `c`); the
;                              strategy resolves the deepest module file the path
;                              names, so the wrong reading resolves nothing extra
;   use a::b::c as d;          the same two readings bound to the alias `d`
;   use a::b::*;               a wildcard import of module `a::b`
;   use a::{b, c as d, self};  a group: the shared prefix is the module and each
;                              identifier member a named import of it; a nested
;                              path or group inside the braces carries a prefix the
;                              query cannot compose with its parent, so it is left
;                              unresolved (disclosed in PROVENANCE.md)
;
; Module specs are kept as written (`crate::util`, `super::x`, `self::m`, `::std`):
; internal/fusion's resolveCrateMod reads those roots itself.

; mod x;
(mod_item
  name: (identifier) @import.module
  !body) @import.stmt

; use a;
(use_declaration
  argument: (identifier) @import.module) @import.stmt

; use a::b::c;   (named reading)
(use_declaration
  argument: (scoped_identifier
    path: (_) @import.module
    name: (identifier) @import.name)) @import.stmt

; use a::b::c;   (whole-module reading, bound to the last segment)
(use_declaration
  argument: (scoped_identifier
    name: (identifier) @import.alias) @import.module) @import.stmt

; use a::b::c as d;
(use_declaration
  argument: (use_as_clause
    path: (scoped_identifier
      path: (_) @import.module
      name: (identifier) @import.name)
    alias: (identifier) @import.alias)) @import.stmt

(use_declaration
  argument: (use_as_clause
    path: [(scoped_identifier) (identifier)] @import.module
    alias: (identifier) @import.alias)) @import.stmt

; use a::b::*;
(use_declaration
  argument: (use_wildcard
    [(scoped_identifier) (identifier) (crate) (self) (super)] @import.module @import.wildcard)) @import.stmt

; use a::{b, c as d, self};
(use_declaration
  argument: (scoped_use_list
    path: (_) @import.module
    list: (use_list
      (identifier) @import.name))) @import.stmt

(use_declaration
  argument: (scoped_use_list
    path: (_) @import.module
    list: (use_list
      (use_as_clause
        path: (identifier) @import.name
        alias: (identifier) @import.alias)))) @import.stmt

(use_declaration
  argument: (scoped_use_list
    path: (identifier) @import.module
    list: (use_list
      (self)))) @import.stmt

(use_declaration
  argument: (scoped_use_list
    path: (scoped_identifier
      name: (identifier) @import.alias) @import.module
    list: (use_list
      (self)))) @import.stmt

; use {p::q, r};
(use_declaration
  argument: (use_list
    (scoped_identifier
      path: (_) @import.module
      name: (identifier) @import.name))) @import.stmt

(use_declaration
  argument: (use_list
    (identifier) @import.module)) @import.stmt
