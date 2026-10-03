; dialects/ruby/locals.scm — Ruby's local-variable scopes for the tags engine's locals rule
; (manifest queries.locals; internal/extractor/tsextract_locals.go, LB-F254). A tags.scm match
; whose @reference.call is also a @local.reference is a call only when no @local.definition of its
; name, in a @local.scope it can see, starts at or before it.
;
; Behaviour reference: tree-sitter/tree-sitter-ruby queries/locals.scm @ 71bd32fb (v0.23.1, MIT),
; its scopes and definitions, re-written here (PROVENANCE.md). Added to it, as Ruby itself reads
; scopes: class, module, singleton class and `def self.x` are scope gates like `def` (a local
; written outside one is not seen inside it; the file is the outermost), and a rescue variable,
; a `for` target and the names a pattern binds (`in [a, *b]`, `in {k:}`, `=> v`) are locals.

; Scope gates: a local defined outside is not seen inside.
([
  (method)
  (singleton_method)
  (class)
  (module)
  (singleton_class)
] @local.scope
 (#set! local.scope-inherits "false"))

; Blocks and lambdas see the locals of the scope around them.
[
  (block)
  (do_block)
  (lambda)
] @local.scope

; Parameters.
(method_parameters (identifier) @local.definition)
(lambda_parameters (identifier) @local.definition)
(block_parameters (identifier) @local.definition)
(destructured_parameter (identifier) @local.definition)
(splat_parameter name: (identifier) @local.definition)
(hash_splat_parameter name: (identifier) @local.definition)
(block_parameter name: (identifier) @local.definition)
(keyword_parameter name: (identifier) @local.definition)
(optional_parameter name: (identifier) @local.definition)

; Assignments.
(assignment left: (identifier) @local.definition)
(operator_assignment left: (identifier) @local.definition)
(left_assignment_list (identifier) @local.definition)
(rest_assignment (identifier) @local.definition)
(destructured_left_assignment (identifier) @local.definition)

; rescue => e, for x in xs.
(exception_variable (identifier) @local.definition)
(for pattern: (identifier) @local.definition)

; Pattern matching: in x, in [a, *b], in [*, a, *], in {k:}, in {k: v}, in X => v.
(in_clause pattern: (identifier) @local.definition)
(array_pattern (identifier) @local.definition)
(find_pattern (identifier) @local.definition)
(alternative_pattern (identifier) @local.definition)
(as_pattern name: (identifier) @local.definition)
(keyword_pattern !value key: (hash_key_symbol) @local.definition)
(keyword_pattern value: (identifier) @local.definition)
