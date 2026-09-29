; dialects/kotlin/imports.scm — the generic @import.* vocabulary (LB-F7, design
; §3.7) for internal/fusion.ResolveFileEdges's per-file resolver, under the
; manifest's namespace-dir strategy (design §6): a class-qualified name maps to
; its file ("com.acme.Foo" -> ".../com/acme/Foo.kt" under any source root) and a
; package-qualified wildcard to every file that declares the package or sits in
; its directory.
;
; import_header holds the whole dotted path as one "identifier" node and an
; optional alias / wildcard sibling. The trailing "." anchors keep the three
; shapes apart, since a query cannot say "no wildcard sibling":
;
;   import com.acme.Foo         named Foo,  module com.acme.Foo
;   import com.acme.Foo as F    named Foo bound as F
;   import com.acme.*           wildcard,   module com.acme
;
; A name that is not a class file (a top-level function or property imported by
; name: `import com.acme.helper`) resolves nowhere under namespace-dir, which
; looks for com/acme/helper.kt; that needs the package/member split the
; strategy does not make (LB-Q-kotlin-rest).
(import_header
  (identifier
    (simple_identifier) @import.name .) @import.module .) @import.stmt

(import_header
  (identifier
    (simple_identifier) @import.name .) @import.module
  (import_alias
    (type_identifier) @import.alias)) @import.stmt

(import_header
  (identifier) @import.module
  (wildcard_import) @import.wildcard) @import.stmt
