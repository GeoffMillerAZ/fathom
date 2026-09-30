; source: none upstream. tree-sitter-grammars/tree-sitter-vim (v0.8.1, 3092fcd99eb87bbd0fc434aa03650ba58bd5b43b, MIT) ships no
; queries/tags.scm (only folds, highlights and injections), and neither aider's nor nvim-treesitter's collections carry a Vim
; script tags file, so these patterns are written for this repository against `indexbench lang-probe` trees.
; changes: not applicable (an original file). It tags `function`, `command` and `augroup` and every call whose callee is a
; name. A function is named as written (`s:Foo`, `foo#bar#Baz`, `Dict.method`), a callee the same way, so a `call s:Foo()`
; and its definition share one key; `g:` and `<SID>` are stripped by the manifest's `names.strip_prefix`.

; `function! Name(...) ... endfunction`, also `func`/`endfunc` and `fu`. @body is the first node of the body when there is one, so a
; signature ends at the parameters and attributes; a function with no body ends at its first line.
(function_definition
  (function_declaration
    name: [
      (identifier)
      (scoped_identifier)
      (field_expression)
    ] @name)
  (body)? @body) @definition.function

; `command! -attr... Name replacement`: the name is a `command_name`.
(command_statement
  name: (command_name) @name) @definition.command

; `augroup Name`. The first named child must be the name (`augroup! Name` deletes a group and has a bang first), and
; `augroup END` closes one.
(augroup_statement
  .
  (augroup_name) @name
  (#not-match? @name "^[Ee][Nn][Dd]$")) @definition.augroup

; Calls: `call f()`, `f()` in an expression, `s:f()`, `a#b#f()`, `dict.method()` and `a:dict.child.method()`. A callee that is
; a call itself (`Foo().Bar()`, which the grammar reads for `Foo() . Bar()`), a lambda or a subscript names nothing.
(call_expression
  function: [
    (identifier)
    (scoped_identifier)
  ] @name) @reference.call

(call_expression
  function: (field_expression
    value: [
      (identifier)
      (argument)
      (scoped_identifier)
    ]) @name) @reference.call

(call_expression
  function: (field_expression
    value: (field_expression
      value: [
        (identifier)
        (argument)
        (scoped_identifier)
      ])) @name) @reference.call
