; javascript.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("javascript"), ADR-021 §4). Also used,
; unmodified, for TypeScript/TSX files (typescript.scm layers TS-only
; patterns on top of this exact set — the underlying class/function/method
; node kinds are shared across the JS and TS grammars).
;
; Predicate-free by design — see python.scm's header comment for why.

(class_declaration
  name: (_) @name) @definition.class

(function_declaration
  name: (identifier) @name) @definition.function

(generator_function_declaration
  name: (identifier) @name) @definition.function

(method_definition
  name: (property_identifier) @name) @definition.method

(variable_declarator
  name: (identifier) @name
  value: [(arrow_function) (function_expression)]) @definition.function

; Naive same-file call collection — mirrors the incumbent CGO extractor's
; CollectCalls (the "function" field's raw text, e.g. "foo" or "foo.bar"),
; not a typed/resolved edge.
(call_expression
  function: (_) @reference.call)
