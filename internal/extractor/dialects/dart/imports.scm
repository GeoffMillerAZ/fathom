; dialects/dart/imports.scm — the generic @import.* vocabulary (LB-F7, design
; §3.7) for internal/fusion.ResolveFileEdges's per-file resolver, under the
; manifest's namespace-dir strategy (design §6): `package:<name>/<path>` through
; the pubspec.yaml whose name is <name> (lib/<path>), and a bare path relative
; to the importing file. `dart:` libraries resolve nowhere (not in the repo).
; Written in this repository: the grammar's own queries carry no import
; captures.
;
; An import statement is an import_specification: the uri, then an optional
; alias identifier and optional show/hide combinators. The trailing "."
; anchors keep the shapes apart, since a query cannot say "no alias":
;
;   import 'a.dart';                 wildcard: every public name of the library
;                                    is in scope unqualified (the shape of C#'s
;                                    `using X;`, owner decision D7)
;   import 'a.dart' hide X;          the same
;   import 'a.dart' as p;            the whole library bound to the prefix p
;   import 'a.dart' show X, Y;       named X and named Y
;   import 'a.dart' as p show X;     the prefix binding (X is written p.X)
;
; A conditional import (`if (dart.library.io) 'a_io.dart'`) resolves its
; default uri, the first. export and part directives are not imports.

(import_specification
  [(configurable_uri
     (uri
       (string_literal) @import.module @import.wildcard))
   (uri
     (string_literal) @import.module @import.wildcard)]
  .) @import.stmt

(import_specification
  [(configurable_uri
     (uri
       (string_literal) @import.module @import.wildcard))
   (uri
     (string_literal) @import.module @import.wildcard)]
  .
  (combinator
    "hide")) @import.stmt

(import_specification
  [(configurable_uri
     (uri
       (string_literal) @import.module))
   (uri
     (string_literal) @import.module)]
  .
  (identifier) @import.alias) @import.stmt

(import_specification
  [(configurable_uri
     (uri
       (string_literal) @import.module))
   (uri
     (string_literal) @import.module)]
  .
  (combinator
    "show"
    (identifier) @import.name)) @import.stmt
