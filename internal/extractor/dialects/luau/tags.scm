; source: tree-sitter-grammars/tree-sitter-lua queries/tags.scm @ 10fe0054734eec83049514ea2e718b2a56acd0c9 (MIT),
; through this repository's dialects/lua/tags.scm; the Luau grammar (tree-sitter-grammars/tree-sitter-luau
; @ a8914d6c1fc5131f8e1c13f769fa704c9f5eb02f, MIT) ships no tags query (queries/ holds folds, highlights,
; indents, injections and locals).
; changes: the Lua dialect's patterns, which the Luau grammar's node names match, with the body anchored by
; field instead of by position: a Luau parameter list can be followed by a return type (`): number`), so
; `body: (block)` names the body and `!body "end"` an empty one (a comment that opens a body is part of the
; signature, which the Lua dialect trims). Adds the type alias (`type T = ...`, `export type T = ...`) as
; a LuauType, named by its identifier (a generic's name without its parameters).

; `function f()`, `local function f()`, `function M.f()`, `function M.a.f()`, `function M:f()`.
(function_declaration
  name: (_) @name
  parameters: (parameters)
  body: (block) @body) @definition.function

; `function f()`, `local function f()`, `function M.f()`, `function M.a.f()`, `function M:f()`.
(function_declaration
  name: (_) @name
  parameters: (parameters)
  !body
"end" @body) @definition.function

; `local f = function() end`.
(variable_declaration
  (assignment_statement
    (variable_list
      .
      name: (identifier) @name)
    (expression_list
      .
      value: (function_definition
        parameters: (parameters)
        body: (block) @body)))) @definition.function

; `local f = function() end`.
(variable_declaration
  (assignment_statement
    (variable_list
      .
      name: (identifier) @name)
    (expression_list
      .
      value: (function_definition
        parameters: (parameters)
        !body
"end" @body)))) @definition.function

; `f = function() end` and `M.f = function() end`, at the top of a file or in a block (a
; `local` declaration wraps its assignment in a variable_declaration, matched above).
(chunk
  (assignment_statement
    (variable_list
      .
      name: [
        (identifier) @name
        (dot_index_expression) @name
        (bracket_index_expression
          field: (string
            content: (string_content) @name))
      ])
    (expression_list
      .
      value: (function_definition
        parameters: (parameters)
        body: (block) @body))) @definition.function)

; `f = function() end` and `M.f = function() end`, at the top of a file or in a block (a
; `local` declaration wraps its assignment in a variable_declaration, matched above).
(chunk
  (assignment_statement
    (variable_list
      .
      name: [
        (identifier) @name
        (dot_index_expression) @name
        (bracket_index_expression
          field: (string
            content: (string_content) @name))
      ])
    (expression_list
      .
      value: (function_definition
        parameters: (parameters)
        !body
"end" @body))) @definition.function)

(block
  (assignment_statement
    (variable_list
      .
      name: [
        (identifier) @name
        (dot_index_expression) @name
        (bracket_index_expression
          field: (string
            content: (string_content) @name))
      ])
    (expression_list
      .
      value: (function_definition
        parameters: (parameters)
        body: (block) @body))) @definition.function)

(block
  (assignment_statement
    (variable_list
      .
      name: [
        (identifier) @name
        (dot_index_expression) @name
        (bracket_index_expression
          field: (string
            content: (string_content) @name))
      ])
    (expression_list
      .
      value: (function_definition
        parameters: (parameters)
        !body
"end" @body))) @definition.function)

; `{ handler = function() end }`: a table constructor's named field holding a function.
(field
  name: (identifier) @name
  value: (function_definition
    parameters: (parameters)
    body: (block) @body)
  (#not-match? @definition.function "^\\[")) @definition.function

; `{ handler = function() end }`: a table constructor's named field holding a function.
(field
  name: (identifier) @name
  value: (function_definition
    parameters: (parameters)
    !body
"end" @body)
  (#not-match? @definition.function "^\\[")) @definition.function

; `{ ["key"] = function() end }`: a string key names the function by its text, as written.
(field
  name: (string
    content: (string_content) @name)
  value: (function_definition
    parameters: (parameters)
    body: (block) @body)) @definition.function

; `{ ["key"] = function() end }`: a string key names the function by its text, as written.
(field
  name: (string
    content: (string_content) @name)
  value: (function_definition
    parameters: (parameters)
    !body
"end" @body)) @definition.function

; Calls: `f()`, `f "s"`, `f {}`, `a.b.f()`, `obj:f()`. A callee that is not a name (`t[i]()`,
; `f()()`, `(g or h)()`) has no name to record and is left out.
(function_call
  name: [
    (identifier) @name
    (dot_index_expression
      field: (identifier) @name)
    (method_index_expression
      method: (identifier) @name)
  ]) @reference.call


; `type Point = { x: number }`, `export type Callback<T> = (T) -> ()`: a type alias.
(type_definition
  name: [
    (identifier) @name
    (generic_type
      .
      (identifier) @name)
  ]) @definition.type
