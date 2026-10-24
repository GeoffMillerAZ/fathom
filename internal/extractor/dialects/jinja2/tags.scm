; dialects/jinja2/tags.scm — Jinja2 templates (LB-Q-jinja2, design §13.2).
; Source: cathaysia/tree-sitter-jinja v0.5.0 (Apache-2.0) ships only
; queries/highlights.scm and injections.scm, no tags query; neither aider nor
; nvim-treesitter carries a jinja tags.scm/locals.scm. Written in this
; repository against `indexbench lang-probe -dialect jinja2` output over
; testdata/, never from grammar.js/node-types.json.
;
; The grammar is FLAT: a `{% block %}`..`{% endblock %}` pair is two sibling
; `control` nodes with the body as sibling `definition`s, never a nested
; node. A definition's range is therefore its opening tag, and no definition
; contains another.

; macro: {% macro name(args) %}
(control
  (statement
    (macro_statement
      (function_call
        (identifier) @name))) @body) @definition.macro

; block: {% block name %}
(control
  (statement
    (block_statement
      (identifier) @name))) @definition.block

; top-level-or-nested assignment: {% set name = value %}
(control
  (statement
    (set_statement
      . (expression
          (binary_expression
            (unary_expression
              (primary_expression
                (identifier) @name))))))) @definition.variable

; macro and function calls: {{ name(args) }}, {% call name() %}. A call
; expression sits under a primary_expression; the function_call inside a
; `{% macro name(args) %}` header does not, so a definition is no call of
; itself. A filter written with arguments (`x | join(",")`) is the right
; operand of a `|`: no call of a definition, left out by its operator.
; Template-engine builtins are no reference to a definition.
(binary_expression
  . (unary_expression
      (primary_expression
        (function_call
          (identifier) @name) @reference.call))
  (#not-any-of? @name "super" "caller" "range" "dict" "lipsum" "cycler" "joiner" "namespace" "varargs" "kwargs"))

(binary_expression
  (binary_operator) @_op
  . (unary_expression
      (primary_expression
        (function_call
          (identifier) @name) @reference.call))
  (#not-eq? @_op "|")
  (#not-any-of? @name "super" "caller" "range" "dict" "lipsum" "cycler" "joiner" "namespace" "varargs" "kwargs"))

; {% call name(args) %}: the callee is the statement's own function_call.
(call_statement
  (function_call
    (identifier) @name) @reference.call)
