; Locals-style patterns on tree-sitter-python nodes, written for the lang-draft
; compile check. Not copied from any locals.scm.
(module) @local.scope

((function_definition
  name: (identifier) @local.definition.function) @local.scope
  (#lua-match? @local.definition.function "^[%a_][%w_]*$")
  (#set! definition.function.scope "parent"))

(class_definition
  name: (identifier) @local.definition.type) @local.scope

(import_statement
  name: (dotted_name
    (identifier) @local.definition.import)) @local.scope

(assignment
  left: (identifier) @local.definition.var)
