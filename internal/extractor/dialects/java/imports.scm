; dialects/java/imports.scm — the generic @import.module/@import.name/
; @import.wildcard vocabulary (LB-F7, design §3.7) for
; internal/fusion.ResolveFileEdges's per-file resolver. No upstream or
; incumbent equivalent (javaextract.go never resolved imports — its
; KindJavaImport is a plain definition listing, tags.scm's own
; @definition.import). imports.strategy: namespace-dir (design §6) maps a
; class-qualified name straight to its file ("com.acme.Foo" ->
; ".../com/acme/Foo.java" under any source root) and a package-qualified
; name to every file directly in its directory — internal/fusion/
; resolvestrategy.go's resolveNamespaceDir (TestResolveStrategies, LB-F11).
;
; import com.example.util.Helper;  -> ImportNamed: Helper (the last
;   segment, the name the import binds in the file's scope) from Module =
;   the whole FQN, Kotlin's shape (LB-F74: before it the FQN was bound as a
;   whole-module prefix, so neither `Helper` nor `Helper.x()` resolved).
;   The FQN still names the class's file; a static import
;   (`import static a.b.Foo.bar;`) names a member, which the resolver finds
;   in the file of the class its module less the last segment names (the
;   namespace-dir package/member split, LB-F78).
; import com.example.other.*;      -> ImportWildcard over the package (or, for
;   `import static a.b.Foo.*;`, the class: manifest type_wildcard is not set,
;   so its file's top-level names). The ".*"'s "*" is an "asterisk" sibling,
;   not part of the scoped_identifier's own text; the named pattern's anchor
;   (scoped_identifier last) keeps a wildcard from also matching it.
(import_declaration
  (scoped_identifier
    name: (identifier) @import.name) @import.module
  .) @import.stmt

(import_declaration
  (scoped_identifier) @import.module
  (asterisk) @import.wildcard) @import.stmt
