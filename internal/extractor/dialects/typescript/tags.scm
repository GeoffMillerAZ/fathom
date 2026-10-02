; dialects/typescript/tags.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("javascript") dispatches .ts/.tsx files
; to this query set, ADR-021 §4). TypeScript's grammar is a superset of
; JavaScript's, so javascript.scm's patterns still apply (class
; declarations, functions, methods, arrow-function const bindings) — this
; file repeats them plus the TS-only constructs (interfaces, type aliases,
; enums). Used for both the "typescript" and "tsx" grammar entry points.
;
; Predicate-free by design — see python.scm's header comment for why.
;
; Scope (LB-Q-typescript-rest, measured against scip-typescript 0.4.0 on zod
; 1fb56a5c and nest 9ff83d54), the javascript dialect's rule: a function
; declaration is a definition at any depth (scip-typescript reports a nested
; one as a local symbol whose documentation says function); a binding to an
; arrow function or function expression only as a direct child of the
; program or of an export statement (so an exported namespace member is
; one), never inside a function, block or test callback, where the oracle
; records a const and never a definition; a method only in a class body.
; No member of an object literal is a definition, a shorthand method or a
; pair alike, `export default { ... }` included: scip-typescript defines
; one only as a local symbol, and only where the object's contextual type
; declares no such member (zod packages/bench/array.ts:12, `zod3() {…}` passed
; to `metabench`: "(method) zod3"); otherwise the name is only a reference to
; the declared member (zod packages/zod/src/v4/classic/schemas.ts:256 `check`,
; to ZodType#check(); packages/bench/property-access.ts:22 `get`, to
; ProxyHandler#get()). That is the type checker's rule, not a module or
; member rule the D3 rule (docs/bench/readings/lang/d3-audit.md) could
; admit. On the subjects this scope rule moves sym_precision .9381 -> .986
; and sym_recall .9937 -> .9729, all of that loss the 185 object-literal
; methods and getters the oracle had reported (zod's benchmark objects and
; recursive-schema getters most). A call written inside a binding that is
; no longer a definition belongs to the definition around it.

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

; export const f = () => … (at module scope, or in a namespace).
(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)])

(interface_declaration
  name: (type_identifier) @name) @definition.interface

(type_alias_declaration
  name: (type_identifier) @name) @definition.type

(enum_declaration
  name: (identifier) @name) @definition.enum

(call_expression
  function: (_) @reference.call)

; `new X(...)` is a call to X (LB-F171, O440): scip-typescript references
; X's constructor there, a class with none by its own name, and the text
; reads as X's (calls.qualify compacts `new ns.X` to ns.X).
(new_expression
  constructor: (_) @reference.call)

; A decorator's call belongs to the definition it decorates, and a class
; field's (its initializer's or decorator's) to the class (LB-F173, O440):
; scip-typescript's enclosing range of a class or method starts at its
; first decorator, and a field is a property with no range of its own. In
; this grammar a class's decorators are its children, a method's are the
; siblings before it in the class body.
(class_declaration
  decorator: (decorator) @calls.from) @calls.to

; `@Dec() export class X` — the decorator is the export statement's.
(export_statement
  decorator: (decorator) @calls.from
  declaration: [(class_declaration) (abstract_class_declaration)] @calls.to)

(abstract_class_declaration
  decorator: (decorator) @calls.from) @calls.to

(class_body
  (decorator) @calls.from
  .
  (decorator)*
  .
  (method_definition) @calls.to)

(class_declaration
  body: (class_body
    (public_field_definition) @calls.from)) @calls.to

(abstract_class_declaration
  body: (class_body
    (public_field_definition) @calls.from)) @calls.to

; @body (LB-F3, design §3.3): a callable's Signature is its definition's text up to
; the start of this capture, whitespace collapsed; without it, the first line.
