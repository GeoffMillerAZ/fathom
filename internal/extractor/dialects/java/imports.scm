; dialects/java/imports.scm — the generic @import.module/@import.wildcard
; vocabulary (LB-F7, design §3.7) for internal/fusion.ResolveFileEdges's
; per-file resolver. No upstream or incumbent equivalent (javaextract.go
; never resolved imports — its KindJavaImport is a plain definition
; listing, tags.scm's own @definition.import). imports.strategy:
; namespace-dir (design §6) maps a class-qualified name straight to its
; file ("com.acme.Foo" -> ".../com/acme/Foo.java" under any source root)
; and a package-qualified name to every file directly in its directory —
; internal/fusion/resolvestrategy.go's resolveNamespaceDir, already
; generically tested (TestResolveStrategies, LB-F11).
;
; import com.example.util.Helper;  -> Module = the whole FQN (ImportWhole,
;   LocalName the full dotted text — same precedent as Python's plain
;   "import pkg.mod", design §3 item 4's ImportWhole doc comment).
; import com.example.other.*;      -> the optional trailing "asterisk"
;   sibling (the ".*"'s "*", not part of the scoped_identifier's own text —
;   mirrors the incumbent's own wildcard handling, which likewise drops the
;   "*") marks the SAME match ImportWildcard instead, one pattern covering
;   both shapes (kotlin.scm's "(wildcard_import)? @import.wildcard"
;   precedent) so a wildcard import is never double-captured.
(import_declaration
  (scoped_identifier) @import.module
  (asterisk)? @import.wildcard) @import.stmt
