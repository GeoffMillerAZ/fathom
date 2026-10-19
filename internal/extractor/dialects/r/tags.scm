; source: r-lib/tree-sitter-r queries/tags.scm @ 346d3707b8c9301f1051e8f6e32666e67529f7d2 (v1.3.0, MIT)
; changes: upstream's four assignment patterns (`<-` or `=` with an identifier or a string on the left of a
; function_definition) are one pattern that also takes `<<-`, and each definition captures @body, the
; function's body, so a signature stops at the parameter list (`f <- function(x, y = 1)`), as ctags' does
; not but a reader wants. The call patterns are upstream's unchanged: a plain `f(x)` and a namespaced
; `pkg::f(x)` or `pkg:::f(x)`, recorded by the callee's last name. Not extracted, on purpose: a function
; held in a list slot or an environment (`obj$method <- function`), `->` assignments, S4 and R6 classes
; (`setClass`, `R6Class`) and `setMethod` definitions (a later row), variables that hold a non-function.

; `f <- function(x) ...`, `f = function(x) ...`, `f <<- function(x) ...`, `"f" <- function(x) ...`,
; and a nested one inside another function's body.
(binary_operator
  lhs: [
    (identifier) @name
    (string
      content: (string_content) @name)
  ]
  operator: [
    "<-"
    "<<-"
    "="
  ]
  rhs: (function_definition
    parameters: (parameters)
    body: (_) @body)) @definition.function

; Calls: `f(x)`, `pkg::f(x)`.
(call
  function: (identifier) @name) @reference.call

(call
  function: (namespace_operator
    rhs: (identifier) @name)) @reference.call
