; dialects/dart/references.scm — the file resolver's references and
; re-exports (LB-F74, LB-F75; vocabulary in internal/extractor/tsrefs.go),
; compiled apart from tags.scm and imports.scm (TSExtractor.ExtractFileSpecs).
;
; References: every type name, a prefixed type `p.Baz` (the grammar writes
; the prefix and the name as adjacent type identifiers), a prefixed member
; `p.Thing` and any identifier. The extractor keeps those whose head an import
; binds or that start with an upper case letter (an unprefixed import brings
; every name of its library into scope).
(type_identifier) @ref.name

((type_identifier) @ref.head
  .
  (type_identifier) @ref.member)

((identifier) @ref.head
  .
  (selector
    (unconditional_assignable_selector
      (identifier) @ref.member)))

(identifier) @ref.name

; Re-exports (a barrel: package:bloc/bloc.dart exporting src/*.dart):
; `export 'src/x.dart';` (every name), `... show A, B;` (those names) and
; `... hide C;` (every name but those).
(library_export
  [(configurable_uri
     (uri
       (string_literal) @export.module))
   (uri
     (string_literal) @export.module)]) @export.stmt

(library_export
  [(configurable_uri
     (uri
       (string_literal) @export.module))
   (uri
     (string_literal) @export.module)]
  (combinator
    "show"
    (identifier) @export.name)) @export.stmt

(library_export
  [(configurable_uri
     (uri
       (string_literal) @export.module))
   (uri
     (string_literal) @export.module)]
  (combinator
    "hide"
    (identifier) @export.hide)) @export.stmt

; A library's parts (LB-F246): `part 'emitter.dart';` makes the part's
; declarations the library's own, so the library re-exports every name of the
; part (a name of the part is reached through whoever imports the library).
(part_directive
  (uri
    (string_literal) @export.module)) @export.stmt

; Not references of their own: a selector's member (`x.name`'s name) and a
; named argument's label.
(unconditional_assignable_selector
  (identifier) @ref.not)

(conditional_assignable_selector
  (identifier) @ref.not)

(named_argument
  (label
    (identifier) @ref.not))
