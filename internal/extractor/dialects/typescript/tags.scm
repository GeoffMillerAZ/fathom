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

; Declarations without a body (owner decision D2's re-measurement against
; scip-typescript, 2026-09-28: every one is a definition the tier-A oracle
; — and universal-ctags — reports, and fathom missed): an abstract class,
; an interface's method signature, an abstract method, and a function or
; method overload signature / `declare function` (no @body: the signature
; is the declaration's first line, tsSignature's rule).
(abstract_class_declaration
  name: (_) @name) @definition.class

(method_signature
  name: [(property_identifier) (private_property_identifier) (computed_property_name)] @name) @definition.method

(abstract_method_signature
  name: [(property_identifier) (private_property_identifier) (computed_property_name)] @name) @definition.method

(function_signature
  name: (identifier) @name) @definition.function

(function_declaration
  name: (identifier) @name
  body: (_) @body) @definition.function

(generator_function_declaration
  name: (identifier) @name
  body: (_) @body) @definition.function

; Name alternation per critic B2 — see javascript.scm's comment.
(method_definition
  name: [(property_identifier) (private_property_identifier) (computed_property_name)] @name
  body: (_) @body) @definition.method

(variable_declarator
  name: (identifier) @name
  value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function

(interface_declaration
  name: (type_identifier) @name) @definition.interface

(type_alias_declaration
  name: (type_identifier) @name) @definition.type

(enum_declaration
  name: (identifier) @name) @definition.enum

(call_expression
  function: (_) @reference.call)

; @body (LB-F3, design §3.3): a callable's Signature is its definition's text up to
; the start of this capture, whitespace collapsed; without it, the first line.
