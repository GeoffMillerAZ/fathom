; Source: bennypowers/tree-sitter-jinja-dialects 4f832fe (MIT).
; Queries authored from owned fixture probes; no upstream query copied.
; Keyword captures anchor the import's line. The grammar's parent statement
; and opening delimiter can include preceding template whitespace.
(extends_statement "extends" @import.bare_stmt (string) @import.bare_source)
(include_statement "include" @import.bare_stmt (string) @import.bare_source)
(import_statement "import" @import.stmt (string) @import.module alias: (identifier) @import.alias)
(from_statement
  "from" @import.stmt
  (string) @import.module
  (import_list (import_name . (identifier) @import.name alias: (identifier)? @import.alias)))
