; dialects/javascript/tags.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("javascript"), ADR-021 §4). Also used,
; unmodified, for TypeScript/TSX files (typescript.scm layers TS-only
; patterns on top of this exact set — the underlying class/function/method
; node kinds are shared across the JS and TS grammars).
;
; Mostly predicate-free — see python.scm's header comment for why — except
; the two `#eq?`/`#not-eq?` guards below that tell a prototype method
; assignment apart from any other member assignment (both host-evaluated
; since LB-F2, tsengine/predicates.go; §3.2 supports them).

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

; Assignment-based definitions (LB-Q-javascript L4 measurement, design §13
; step 21): express/axios lean on these idioms (calibration/corpus.yaml
; "prototype-based method assignment", "dynamically attached methods on
; prototypes") and universal-ctags already classifies both shapes below as
; real definitions — dropping them silently undercounted sym_recall against
; ctags. Deliberately NOT added here: a bare identifier assignment (`foo =
; function(){}`) and a non-prototype multi-level member chain (`module.
; exports.foo = function(){}`) — both are real ctags-recognized shapes too,
; but TestTagsDialectParity (LB-F1) freezes the five migrated dialects'
; existing fixtures' output byte-for-byte, and shapes.js/common.cjs already
; contain exactly those two shapes; widening past what's below regresses
; that frozen parity and is not a worker's call to make (needs-strategy,
; design §13.2 step 13's spirit — an Opus row would have to re-baseline
; those goldens first). Recorded as a known remaining gap in the reading's
; notes, not fixed here.

; Foo.prototype.bar = function(){} / => {} — a prototype method (ctags:
; kind "method", scoped under the constructor's name).
(assignment_expression
  left: (member_expression
    object: (member_expression
      property: (property_identifier) @_proto)
    property: (property_identifier) @name)
  right: [(function_expression body: (_) @body) (arrow_function body: (_) @body)]
  (#eq? @_proto "prototype")) @definition.method

; app.foo = / exports.foo = ... (a single-level member assignment: the LHS
; object is a bare identifier, so this can never also match the prototype
; pattern above, which requires the object to itself be a member_expression).
(assignment_expression
  left: (member_expression
    object: (identifier)
    property: (property_identifier) @name)
  right: [(function_expression body: (_) @body) (arrow_function body: (_) @body)]) @definition.function

; Naive same-file call collection — mirrors the incumbent CGO extractor's
; CollectCalls (the "function" field's raw text, e.g. "foo" or "foo.bar"),
; not a typed/resolved edge.
(call_expression
  function: (_) @reference.call)

; @body (LB-F3, design §3.3): a callable's Signature is its definition's text up to
; the start of this capture, whitespace collapsed; without it, the first line.
