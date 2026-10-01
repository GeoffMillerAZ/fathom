; dialects/typescript/references.scm — the file resolver's references and
; re-exports (LB-F74, LB-F75; vocabulary in internal/extractor/tsrefs.go),
; compiled apart from tags.scm and imports.scm (TSExtractor.ExtractFileSpecs),
; for both the typescript and tsx entry points (no JSX-only node is named:
; a JSX element's name is an identifier or a member expression).
;
; References: every type name and every identifier or dotted member access
; outside a call's own text. The extractor keeps only those whose head an
; import binds (TypeScript has no wildcard import), so a local, a parameter
; or a property never reaches the store.
(type_identifier) @ref.name
(nested_type_identifier) @ref.name
(identifier) @ref.name
(member_expression) @ref.name
(shorthand_property_identifier) @ref.name

; Re-exports: `export { A, B as C } from './m'`, `export * as ns from './m'`
; (the namespace form is not followed) and `export * from './m'`.
(export_statement
  (export_clause
    (export_specifier
      name: (identifier) @export.name
      alias: (identifier)? @export.alias))
  source: (string) @export.module) @export.stmt

(export_statement
  (namespace_export) @export.namespace
  source: (string) @export.module) @export.stmt

(export_statement
  source: (string) @export.module) @export.stmt

; Not references of their own: a qualified type's last segment (`ns.Baz`'s
; Baz) and a member's name are read with their qualifier.
(nested_type_identifier
  name: (type_identifier) @ref.not)

; A default export (LB-F75): the name `export default X;`,
; `export default function X` or `export default class X` makes the module's
; default, which a default import binds.
(export_statement
  "default"
  value: (identifier) @export.default)

(export_statement
  "default"
  declaration: [(function_declaration name: (_) @export.default) (generator_function_declaration name: (_) @export.default) (class_declaration name: (_) @export.default) (abstract_class_declaration name: (_) @export.default)])
