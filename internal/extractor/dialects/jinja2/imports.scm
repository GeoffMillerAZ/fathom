; dialects/jinja2/imports.scm — extends / include / import / from-import
; edges (LB-Q-jinja2). A template name is a loader-relative path string.
(extends_statement (string_literal) @import.module) @import.stmt
(include_statement (string_literal) @import.module) @import.stmt
(import_statement
  (string_literal) @import.module
  (import_as (identifier) @import.alias)) @import.stmt
(import_statement
  (import_from (string_literal) @import.module)
  (identifier) @import.name
  . (import_as (identifier) @import.alias)?) @import.stmt
