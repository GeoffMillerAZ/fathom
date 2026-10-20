; source: written for this dialect (victorhqc/tree-sitter-prisma ships queries/highlights.scm only, no tags.scm;
; neither aider nor nvim-treesitter carries Prisma tags). Node shapes read with `indexbench lang-probe`, never
; grammar.js.
; Kinds: a model, a view, a composite type (`type`), an enum, a datasource and a generator block, each named by its
; identifier; a field of a model, view or type (Model.field); an enum value (Enum.VALUE).
; References: every non-scalar type a field writes (a model, view, composite type or enum, owned by the field), and
; every name in an @relation's fields: and references: lists. The nine built-in scalars are no reference.

(model_declaration (identifier) @name (statement_block)? @body) @definition.model
(view_declaration (identifier) @name (statement_block)? @body) @definition.view
(type_declaration (identifier) @name (statement_block)? @body) @definition.type
(enum_declaration (identifier) @name (enum_block)? @body) @definition.enum
(datasource_declaration (identifier) @name (statement_block)? @body) @definition.datasource
(generator_declaration (identifier) @name (statement_block)? @body) @definition.generator
(column_declaration (identifier) @name) @definition.field
(enumeral (identifier) @name) @definition.enum_value

((column_type (identifier) @name) @reference.call
  (#not-any-of? @name "String" "Boolean" "Int" "BigInt" "Float" "Decimal" "DateTime" "Json" "Bytes"))

((column_declaration
   (attribute
     (call_expression
       (identifier) @_fn
       (arguments
         (type_expression
           (identifier) @_key
           (array (identifier) @name @reference.call))))))
  (#eq? @_fn "relation")
  (#any-of? @_key "fields" "references"))
