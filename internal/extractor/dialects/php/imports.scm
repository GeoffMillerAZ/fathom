; dialects/php/imports.scm — `use` declarations for internal/fusion's per-file
; resolver (design §3.7, §6 namespace-dir with composer.json's PSR-4 prefixes).
; Written in this repository: tree-sitter-php's own queries/tags.scm carries
; no import captures at all, and the incumbent CGO port (phpextract.go) never
; resolved an import.
;
;   use A\B\C;                 a class-like name brought into scope: module
;                              A\B\C, a wildcard (PHP puts the name in scope
;                              unqualified, exactly the shape of C#'s `using X;`,
;                              owner decision D7), resolved to C.php by PSR-4
;   use function A\f; use const A\X;   the same clause shape under the
;                              anonymous "function"/"const" keyword; captured
;                              identically
;   use A\B\C as D;            an alias: module A\B\C bound to the local name D
;   use A\B\{C, D as E};       a group: the shared prefix is the module and each
;                              clause a named import of it
;
; The grouped form carries its prefix in a sibling node of the clause, so its
; named imports (module A\B, name C) resolve through the strategy's
; submodule retry (module + separator + name); the ungrouped forms carry the
; whole name in one node.

; use A\B\C;   (single child: a clause with an alias has two)
(namespace_use_declaration
  (namespace_use_clause
    .
    [(name) (qualified_name)] @import.module @import.wildcard
    .)) @import.stmt

; use A\B\C as D;
(namespace_use_declaration
  (namespace_use_clause
    .
    [(name) (qualified_name)] @import.module
    alias: (name) @import.alias)) @import.stmt

; use A\B\{C, D\E};
(namespace_use_declaration
  (namespace_name) @import.module
  body: (namespace_use_group
    (namespace_use_clause
      .
      [(name) (qualified_name)] @import.name
      .))) @import.stmt

; use A\B\{C as D};
(namespace_use_declaration
  (namespace_name) @import.module
  body: (namespace_use_group
    (namespace_use_clause
      .
      [(name) (qualified_name)] @import.name
      alias: (name) @import.alias))) @import.stmt
