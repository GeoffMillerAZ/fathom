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
;                              member a named import of it
;   use a::{b::{c, d}, e};     nested groups: a group's list is a scope record
;                              (@import.scope, its path @import.prefix) and every
;                              member inside takes the prefixes of all the lists
;                              around it, outermost first (a::b for c and d), joined
;                              by the engine (tsimports.go composeImportPrefix, LB-F83)
;   use a::{self};             the module the group names (a), bound to its last segment
;   use a::{b::c};             the two readings of a path member, as for `use a::b::c`
;   use a::{b::*};             a wildcard import of the composed module (a::b)
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

;  Scope records (no spec of their own): a group's list with its path, and the whole
; statement, which gives a member of a nested group the statement's line and span.
(use_declaration) @import.scope @import.stmt

(scoped_use_list
  path: (_) @import.prefix
  list: (use_list) @import.scope)

; use a::{b, c as d, self};   and every nested list, at any depth: the module is the
; composed enclosing prefix
(use_list
  (identifier) @import.name)

(use_list
  (use_as_clause
    path: (identifier) @import.name
    alias: (identifier) @import.alias))

(use_list
  (use_as_clause
    path: (scoped_identifier
      path: (_) @import.module
      name: (identifier) @import.name)
    alias: (identifier) @import.alias))

(use_list
  (self) @import.self)

; use a::{b::c};   (named reading, then whole-module reading bound to the last segment)
(use_list
  (scoped_identifier
    path: (_) @import.module
    name: (identifier) @import.name))

(use_list
  (scoped_identifier
    name: (identifier) @import.alias) @import.module)

; use a::{b::*};
(use_list
  (use_wildcard
    [(scoped_identifier) (identifier) (crate) (self) (super)] @import.module @import.wildcard))

; use {p};   (a top-level list has no prefix: a whole import of the module it names)
(use_declaration
  argument: (use_list
    (identifier) @import.module)) @import.stmt
