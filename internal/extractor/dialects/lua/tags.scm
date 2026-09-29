; source: tree-sitter-grammars/tree-sitter-lua queries/tags.scm @ 10fe0054734eec83049514ea2e718b2a56acd0c9 (MIT)
; changes: upstream tags a dotted function's last field and a colon method as @definition.function and
; @definition.method, a `x = function` assignment and a table field whose value is a function as
; @definition.function, and every call whose callee is a name, a field or a method. This file keeps the
; incumbent CGO walker's (luaextract.go) one kind and its names: a function is named by the whole name the
; declaration writes (`M.add`, `Class:new`, not the last field), so port parity holds; it adds a nested
; declaration (the walker never entered a body) and the assigned forms, gives the local form its own range
; so a signature starts at `local`, and captures @body as the node after the parameter list (a comment
; that opens the body, the block, or the closing `end` of an empty body) so a signature stops at the
; parameters and a body of nothing still ends. A field keyed by an expression (`[KEY] = function`) names
; nothing (KEY is a variable's value, not the function's name), so it is not a definition; one keyed by a
; string (`["key"] = function`, `t["key"] = function`) is named by the string.

; `function f()`, `local function f()`, `function M.f()`, `function M.a.f()`, `function M:f()`.
(function_declaration
  name: (_) @name
  parameters: (parameters)
  .
  [
    (comment)
    (block)
    "end"
  ] @body) @definition.function

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
        .
        [
          (comment)
          (block)
          "end"
        ] @body)))) @definition.function

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
        .
        [
          (comment)
          (block)
          "end"
        ] @body))) @definition.function)

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
        .
        [
          (comment)
          (block)
          "end"
        ] @body))) @definition.function)

; `{ handler = function() end }`: a table constructor's named field holding a function.
(field
  name: (identifier) @name
  value: (function_definition
    parameters: (parameters)
    .
    [
      (comment)
      (block)
      "end"
    ] @body)
  (#not-match? @definition.function "^\\[")) @definition.function

; `{ ["key"] = function() end }`: a string key names the function by its text, as written.
(field
  name: (string
    content: (string_content) @name)
  value: (function_definition
    parameters: (parameters)
    .
    [
      (comment)
      (block)
      "end"
    ] @body)) @definition.function

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
