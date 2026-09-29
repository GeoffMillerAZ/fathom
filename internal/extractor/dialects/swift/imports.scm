; dialects/swift/imports.scm — the generic @import.* vocabulary (LB-F7, design §3.7) for
; internal/fusion.ResolveFileEdges's per-file resolver, under the manifest's swift-module strategy
; (design §6): `import M` resolves to every Swift file of the SwiftPM target named M (its `path:` in
; Package.swift, else Sources/M), a whole-module import bringing every top-level name of the target into
; scope. A module with no target in the repository (Foundation, a system module) resolves nowhere.
; Written in this repository: the grammar's own queries carry no import captures.
;
; An import declaration is `import M`, `import M.Sub` (a whole-module import, bound as its text; an
; attribute such as @testable or @_exported is a modifier of the declaration and changes nothing) or
; `import struct M.X` (also class, enum, protocol, typealias, let, var, func: one declaration, a named
; import of its last identifier). The keyword is an anonymous token, so the whole-module pattern
; rules the scoped form out by its text.

(import_declaration
  ["typealias" "struct" "class" "enum" "protocol" "let" "var" "func"]
  (identifier
    (simple_identifier) @import.name .) @import.module) @import.stmt

(import_declaration
  (identifier) @import.module
  (#not-match? @import.stmt "\\bimport\\s+(typealias|struct|class|enum|protocol|let|var|func)\\s")) @import.stmt
