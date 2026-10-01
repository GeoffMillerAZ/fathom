; dialects/php/references.scm — the file resolver's references (LB-F74;
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm
; and imports.scm (TSExtractor.ExtractFileSpecs) once the manifest names it
; (queries.references): the class names a file writes outside a call — a
; type hint, `new X`, extends, implements, a trait use, `X::CONST`,
; `X::$prop`, `X::class`, instanceof, a catch — as written (unqualified,
; qualified or fully qualified); the resolver reads them as nikic/PHP-Parser's
; NameResolver does (a use alias, else the file's own namespace).
(named_type [(name) (qualified_name)] @ref.name)

(base_clause [(name) (qualified_name)] @ref.name)

(class_interface_clause [(name) (qualified_name)] @ref.name)

(object_creation_expression [(name) (qualified_name)] @ref.name)

(class_constant_access_expression
  .
  [(name) (qualified_name)] @ref.name)

(scoped_call_expression
  scope: [(name) (qualified_name)] @ref.name)

(scoped_property_access_expression
  scope: [(name) (qualified_name)] @ref.name)

(binary_expression
  right: [(name) (qualified_name)] @ref.name)

(use_declaration [(name) (qualified_name)] @ref.name)
