; dialects/javascript/tags.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("javascript"), ADR-021 §4). Also used,
; unmodified, for TypeScript/TSX files (typescript.scm layers TS-only
; patterns on top of this exact set — the underlying class/function/method
; node kinds are shared across the JS and TS grammars).
;
; Predicate-free by design — see python.scm's header comment for why.

(class_declaration
  name: (_) @name) @definition.class

(function_declaration
  name: (identifier) @name
  body: (_) @body) @definition.function

(generator_function_declaration
  name: (identifier) @name
  body: (_) @body) @definition.function

; The name alternation matters (critic B2): #private methods are
; private_property_identifier and computed names ([Symbol.iterator]) are
; computed_property_name — constraining to (property_identifier) alone
; silently drops both, which the incumbent CGO extractor (raw field text,
; kind-agnostic) never did.
(method_definition
  name: [(property_identifier) (private_property_identifier) (computed_property_name)] @name
  body: (_) @body) @definition.method

(variable_declarator
  name: (identifier) @name
  value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function

; Naive same-file call collection — mirrors the incumbent CGO extractor's
; CollectCalls (the "function" field's raw text, e.g. "foo" or "foo.bar"),
; not a typed/resolved edge.
(call_expression
  function: (_) @reference.call)

; @body (LB-F3, design §3.3): a callable's Signature is its definition's text up to
; the start of this capture, whitespace collapsed; without it, the first line.
