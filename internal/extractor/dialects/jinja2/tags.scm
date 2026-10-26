; Source: bennypowers/tree-sitter-jinja-dialects 4f832fe (MIT).
; Queries authored from owned fixture probes; no upstream query copied.
; Header captures over owned fixture probe output. Header-only ranges match
; the independent Jinja AST/lexer oracle; @end extends the keyword token.
; Opening delimiters may own leading whitespace, so only signatures use them.
(macro_statement
  ["{%" "{%-" "{%~"] @signature.lead
  "macro" @definition.macro @signature.of
  name: (identifier) @name
  (argument_list) .
  ["%}" "-%}" "~%}"] @end
  "endmacro")

(block_statement
  "block" @definition.block
  name: (identifier) @name
  "scoped"? .
  ["%}" "-%}" "~%}"] @end
  "endblock")

(set_statement
  "set" @definition.variable
  variable: (identifier) @name
  "=" . (_) .
  ["%}" "-%}" "~%}"] @end)

(call_expression
  function: (identifier) @name
  (#not-any-of? @name "super" "caller" "range" "dict" "lipsum" "cycler" "joiner" "namespace" "varargs" "kwargs")) @reference.call

(call_expression
  function: (member_expression object: (_) @call.receiver property: (identifier) @name)) @reference.call
