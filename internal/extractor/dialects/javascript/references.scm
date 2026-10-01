; dialects/javascript/references.scm — the file resolver's references and
; re-exports (LB-F74, LB-F75; vocabulary in internal/extractor/tsrefs.go),
; compiled apart from tags.scm and imports.scm (TSExtractor.ExtractFileSpecs).
;
; References: every identifier or dotted member access outside a call's own
; text (a value passed or assigned, `new X`, `extends X`, a JSX element). The
; extractor keeps only those whose head an import binds (an ES module or a
; require binds every name it brings), so a local, a parameter or a property
; never reaches the store.
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

; A default export (LB-F75): the name `export default X;`,
; `export default function X`, `export default class X` or CommonJS's
; `module.exports = X;` makes the module's default, which a default import
; (or a require bound whole) binds.
(export_statement
  "default"
  value: (identifier) @export.default)

(export_statement
  "default"
  declaration: [(function_declaration name: (_) @export.default) (generator_function_declaration name: (_) @export.default) (class_declaration name: (_) @export.default)])

((assignment_expression
  left: (member_expression
    object: (identifier) @_module
    property: (property_identifier) @_exports)
  right: (identifier) @export.default)
 (#eq? @_module "module")
 (#eq? @_exports "exports"))
