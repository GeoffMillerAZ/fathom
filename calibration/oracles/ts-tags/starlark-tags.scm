; source: tree-sitter/tree-sitter-python queries/tags.scm @ 293fdc02038ee2bf0e2e206711b69c90ac0d413f (v0.25.0, MIT,
; Copyright (c) 2016 Max Brunsfeld), fetched verbatim and cut by one pattern: the class_definition pattern, because
; tree-sitter-starlark (tree-sitter-grammars, v1.3.0) has no class_definition node and the query would not compile.
; The grammar is a descendant of tree-sitter-python and keeps the node names the other patterns use (module,
; assignment, function_definition, call, attribute). The LB-Q-tiltfile dialect's tags.scm was written for the
; Tiltfile (the Tilt API entities are its own); the reference.call pattern has the same shape as this query's, so the
; oracle is independent in runtime and in query, not in the shape of a call.
(module (expression_statement (assignment left: (identifier) @name) @definition.constant))

(function_definition
  name: (identifier) @name) @definition.function

(call
  function: [
      (identifier) @name
      (attribute
        attribute: (identifier) @name)
  ]) @reference.call
