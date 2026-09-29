; source: tree-sitter-grammars/tree-sitter-lua queries/tags.scm
; @ 10fe0054734eec83049514ea2e718b2a56acd0c9 (v0.5.0, MIT), verbatim below this
; header. It is the grammar's own upstream tags query, run here by py-tree-sitter
; (the C runtime) as the tier-C L2 call oracle (design §1.5 D5).

(function_declaration
  name: [
    (identifier) @name
    (dot_index_expression
      field: (identifier) @name)
  ]) @definition.function

(function_declaration
  name: (method_index_expression
    method: (identifier) @name)) @definition.method

(assignment_statement
  (variable_list
    .
    name: [
      (identifier) @name
      (dot_index_expression
        field: (identifier) @name)
    ])
  (expression_list
    .
    value: (function_definition))) @definition.function

(table_constructor
  (field
    name: (identifier) @name
    value: (function_definition))) @definition.function

(function_call
  name: [
    (identifier) @name
    (dot_index_expression
      field: (identifier) @name)
    (method_index_expression
      method: (identifier) @name)
  ]) @reference.call
