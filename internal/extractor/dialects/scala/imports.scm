; dialects/scala/imports.scm — the generic @import.* vocabulary (LB-F7, design
; §3.7) for internal/fusion.ResolveFileEdges under the manifest's namespace-dir
; strategy, "." between segments (LB-F438): a class-qualified name maps to its
; file ("cats.data.NonEmptyList" -> ".../cats/data/NonEmptyList.scala" under any
; source root, else a file of package cats.data declaring the name), and a
; wildcard to the files of the package directory or the object's file.
;
; tree-sitter-scala writes an import's path as sibling identifiers (the
; import's own `path` fields), so @import.module is captured on each of them
; and on a selector's name: several @import.module captures in one match read
; as their texts joined by "." (internal/extractor/tsimports.go,
; joinedModule). The engine's query runner keeps one node per quantified
; capture, so each shape is written out for a path of 1 to 9 segments, the
; anchors keeping the path whole and the shapes apart:
;
;   import cats.data.NonEmptyList        named NonEmptyList, module cats.data.NonEmptyList
;   import cats.data.{Validated, Valid => V}
;                                         named Validated; named Valid bound as V
;   import cats.data.NonEmptyList as Nel  (Scala 3) named NonEmptyList bound as Nel
;   import cats.syntax.all._             wildcard, module cats.syntax.all (also `*`, `given`)
;   import cats.{Functor => F, _}         named Functor bound as F; wildcard of cats
;
; An unimport (`{Foo => _}`: its alias is a wildcard node, not an identifier)
; binds nothing, and `{given T}` names no member. A path is resolved as
; written; one relative to an enclosing package (`package cats; import
; data.X`) still finds cats/data/X.scala through the strategy's suffix match.
; Generated: one block per shape, the path's segments repeated (a path longer
; than 9 segments binds nothing; neither subject has one longer than 6).



; --- named: the path's last segment is the name ---

(import_declaration
  .
  (identifier) @import.name @import.module
  .) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.name @import.module
  .) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.name @import.module
  .) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.name @import.module
  .) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.name @import.module
  .) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.name @import.module
  .) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.name @import.module
  .) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.name @import.module
  .) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.name @import.module
  .) @import.stmt


; --- a selector's name, plain ---

(import_declaration
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (identifier) @import.name @import.module)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (identifier) @import.name @import.module)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (identifier) @import.name @import.module)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (identifier) @import.name @import.module)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (identifier) @import.name @import.module)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (identifier) @import.name @import.module)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (identifier) @import.name @import.module)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (identifier) @import.name @import.module)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (identifier) @import.name @import.module)) @import.stmt


; --- a selector renamed (`A => B`, Scala 3's `A as B`) ---

(import_declaration
  .
  (identifier) @import.module
  .
  (namespace_selectors
    [(arrow_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)
     (as_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)])) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    [(arrow_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)
     (as_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)])) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    [(arrow_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)
     (as_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)])) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    [(arrow_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)
     (as_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)])) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    [(arrow_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)
     (as_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)])) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    [(arrow_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)
     (as_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)])) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    [(arrow_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)
     (as_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)])) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    [(arrow_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)
     (as_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)])) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    [(arrow_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)
     (as_renamed_identifier
       name: (_) @import.name @import.module
       alias: (identifier) @import.alias)])) @import.stmt


; --- Scala 3's `import a.B as C` ---

(import_declaration
  .
  (identifier) @import.module
  .
  (as_renamed_identifier
    name: (_) @import.name @import.module
    alias: (identifier) @import.alias)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (as_renamed_identifier
    name: (_) @import.name @import.module
    alias: (identifier) @import.alias)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (as_renamed_identifier
    name: (_) @import.name @import.module
    alias: (identifier) @import.alias)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (as_renamed_identifier
    name: (_) @import.name @import.module
    alias: (identifier) @import.alias)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (as_renamed_identifier
    name: (_) @import.name @import.module
    alias: (identifier) @import.alias)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (as_renamed_identifier
    name: (_) @import.name @import.module
    alias: (identifier) @import.alias)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (as_renamed_identifier
    name: (_) @import.name @import.module
    alias: (identifier) @import.alias)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (as_renamed_identifier
    name: (_) @import.name @import.module
    alias: (identifier) @import.alias)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (as_renamed_identifier
    name: (_) @import.name @import.module
    alias: (identifier) @import.alias)) @import.stmt


; --- a wildcard (`_`, `*`, `given`) after the path or among the selectors ---

(import_declaration
  .
  (identifier) @import.module
  .
  (namespace_wildcard) @import.wildcard) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_wildcard) @import.wildcard) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_wildcard) @import.wildcard) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_wildcard) @import.wildcard) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_wildcard) @import.wildcard) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_wildcard) @import.wildcard) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_wildcard) @import.wildcard) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_wildcard) @import.wildcard) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_wildcard) @import.wildcard) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (namespace_wildcard) @import.wildcard)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (namespace_wildcard) @import.wildcard)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (namespace_wildcard) @import.wildcard)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (namespace_wildcard) @import.wildcard)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (namespace_wildcard) @import.wildcard)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (namespace_wildcard) @import.wildcard)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (namespace_wildcard) @import.wildcard)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (namespace_wildcard) @import.wildcard)) @import.stmt

(import_declaration
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (identifier) @import.module
  .
  (namespace_selectors
    (namespace_wildcard) @import.wildcard)) @import.stmt
