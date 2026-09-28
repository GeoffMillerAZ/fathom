; dialects/typescript/tags.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("javascript") dispatches .ts/.tsx files
; to this query set, ADR-021 §4). TypeScript's grammar is a superset of
; JavaScript's, so every javascript.scm pattern still applies (class
; declarations, functions, methods, arrow-function const bindings) — this
; file repeats them plus the TS-only constructs (interfaces, type aliases,
; enums). Used for both the "typescript" and "tsx" grammar entry points.
;
; Predicate-free by design — see python.scm's header comment for why.

(class_declaration
  name: (_) @name) @definition.class

(function_declaration
  name: (identifier) @name) @definition.function

(generator_function_declaration
  name: (identifier) @name) @definition.function

; Name alternation per critic B2 — see javascript.scm's comment.
(method_definition
  name: [(property_identifier) (private_property_identifier) (computed_property_name)] @name) @definition.method

(variable_declarator
  name: (identifier) @name
  value: [(arrow_function) (function_expression)]) @definition.function

(interface_declaration
  name: (type_identifier) @name) @definition.interface

(type_alias_declaration
  name: (type_identifier) @name) @definition.type

(enum_declaration
  name: (identifier) @name) @definition.enum

(call_expression
  function: (_) @reference.call)
