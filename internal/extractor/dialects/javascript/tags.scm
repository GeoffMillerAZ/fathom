; dialects/javascript/tags.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("javascript"), ADR-021 §4). TypeScript
; (dialects/typescript/tags.scm) repeats these patterns and adds its own:
; the class/function/method node kinds are shared across the JS and TS grammars.
;
; Mostly predicate-free — see python.scm's header comment for why — except
; the `#eq?` guard below that tells a prototype method assignment apart from
; any other member assignment (host-evaluated since LB-F2,
; tsengine/predicates.go; §3.2 supports it).
;
; Scope (LB-Q-javascript-rest, measured against scip-typescript 0.4.0 on
; axios cdcfd214 and express df0abc93): a definition is a module or member
; definition, never a local. A function or generator DECLARATION is one at
; any depth (scip-typescript reports a nested one too, as a local symbol whose
; documentation says function). A binding to an arrow function or function
; expression is one only at module scope: a direct child of the program or
; of an export statement (an exported TypeScript namespace member is one as
; well); inside a function, a block, a test callback or an IIFE it is a
; local variable, which the oracle records as a const/let/var and never as a
; definition. A method is one only in a class body. An assignment
; (`Foo.prototype.bar = function`, `app.foo = function`, `exports.foo =
; function`) is one only as a module-scope statement: the same assignment
; inside a function is a handler attached at run time.
;
; Object literals: no member of an object literal is a definition, a
; shorthand method (`{ m() {} }`) or a pair (`{ p: () => 1 }`) alike, in
; `module.exports = { ... }` and `export default { ... }` too. Evidence:
; scip-typescript defines an object-literal method only as a local symbol,
; and only where the object's contextual type declares no such member
; (axios lib/helpers/cookies.js:8, `export default cond ? { write() {…} } :
; {…}`: `local 8`, "(method) write"); otherwise the name is only a reference
; to the declared member (axios lib/helpers/trackStream.js:59 `pull`, to
; UnderlyingByteSource#pull). Its reports follow the type checker, not a
; module or member rule, and under the D3 rule (docs/bench/readings/lang/
; d3-audit.md, which withdrew the object-literal-method class) only a real
; module or member definition the oracle missed is admissible. On the
; subjects this scope rule moves sym_precision .8368 -> .9905 and
; sym_recall .9908 -> .9632, all of that loss the 9 object-literal methods
; the oracle had reported (cookies.js's six `export default` members among
; them).
;
; A call written inside a binding that is no longer a definition belongs to
; the definition around it (the @reference.call pattern is unchanged).

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
(class_body
  (method_definition
    name: [(property_identifier) (private_property_identifier) (computed_property_name)] @name
    body: (_) @body) @definition.method)

; const f = () => … / var g = function () {…} at module scope.
(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)])

; export const f = () => … (at module scope, or in a TypeScript namespace).
(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)])

; Assignment-based definitions (LB-Q-javascript L4 measurement, design §13
; step 21): express/axios lean on these idioms (calibration/corpus.yaml
; "prototype-based method assignment", "dynamically attached methods on
; prototypes") and universal-ctags classifies both shapes below as real
; definitions. Not added: a bare identifier assignment (`foo =
; function(){}`) and a non-prototype multi-level member chain (`module.
; exports.foo = function(){}`) — real ctags-recognized shapes too, left as a
; known gap in the reading's notes.

; Foo.prototype.bar = function(){} / => {} — a prototype method (ctags:
; kind "method", scoped under the constructor's name).
(program
  (expression_statement
    (assignment_expression
      left: (member_expression
        object: (member_expression
          property: (property_identifier) @_proto)
        property: (property_identifier) @name)
      right: [(function_expression body: (_) @body) (arrow_function body: (_) @body)]) @definition.method)
  (#eq? @_proto "prototype"))

; app.foo = / exports.foo = ... (a single-level member assignment: the LHS
; object is a bare identifier, so this can never also match the prototype
; pattern above, which requires the object to itself be a member_expression).
(program
  (expression_statement
    (assignment_expression
      left: (member_expression
        object: (identifier)
        property: (property_identifier) @name)
      right: [(function_expression body: (_) @body) (arrow_function body: (_) @body)]) @definition.function))

; app.get = app.head = function head(){} — the function's own assignment is
; the inner one of a module-scope chain.
(program
  (expression_statement
    (assignment_expression
      right: (assignment_expression
        left: (member_expression
          object: (identifier)
          property: (property_identifier) @name)
        right: [(function_expression body: (_) @body) (arrow_function body: (_) @body)]) @definition.function)))

; Naive same-file call collection — mirrors the incumbent CGO extractor's
; CollectCalls (the "function" field's raw text, e.g. "foo" or "foo.bar"),
; not a typed/resolved edge.
(call_expression
  function: (_) @reference.call)

; `new X(...)` is a call to X, and a class field's initializer belongs to the
; class (LB-F171, LB-F173, O440): the typescript dialect's rules, read by
; the same oracle (scip-typescript).
(new_expression
  constructor: (_) @reference.call)

(class_declaration
  decorator: (decorator) @calls.from) @calls.to

; `@Dec() export class X` — the decorator is the export statement's.
(export_statement
  decorator: (decorator) @calls.from
  declaration: (class_declaration) @calls.to)

(class_declaration
  body: (class_body
    (field_definition) @calls.from)) @calls.to

; @body (LB-F3, design §3.3): a callable's Signature is its definition's text up to
; the start of this capture, whitespace collapsed; without it, the first line.
