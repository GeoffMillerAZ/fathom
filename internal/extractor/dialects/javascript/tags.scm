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

; React (LB-F377, wave c3, item react): a class whose heritage, before its
; body, extends Component or PureComponent is a class component; every other
; class is a class (one node, one definition: the two predicates split it).
(class_declaration
  name: (_) @name
  (class_heritage) @_heritage
  (#match? @_heritage "^extends\\s+(React\\.)?(Pure)?Component\\b")) @definition.class_component

(class_declaration
  name: (_) @name) @definition.class

; React (LB-F377): a PascalCase function (capitalized, with a lower-case
; letter) whose body writes JSX (`</` or `/>`) is a component, a use[A-Z0-9]
; function that calls a hook is a hook, and every other combination stays a
; function (the typescript dialect's rule, read from the text so the two
; dialects agree). One node and one name make one definition, the earlier
; pattern's (oneDefPerNode, O732): the component and hook patterns come
; first, and the function pattern after them needs no predicate.
(function_declaration
  name: (identifier) @name
  body: (_) @body
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @body "</|/>")) @definition.component

(function_declaration
  name: (identifier) @name
  body: (_) @body
  (#match? @name "^use[A-Z0-9]")
  (#match? @body "\\buse([A-Z0-9][A-Za-z0-9_$]*)?\\s*\\(")) @definition.hook

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

; const f = () => … / var g = function () {…} at module scope, and
; export const f = () => … : a function, a component or a hook by the
; function declaration's rules above.
(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.component)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.component)]
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @body "</|/>"))

(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.hook)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.hook)]
  (#match? @name "^use[A-Z0-9]")
  (#match? @body "\\buse([A-Z0-9][A-Za-z0-9_$]*)?\\s*\\("))

(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)])

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.component)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.component)]
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @body "</|/>"))

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.hook)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.hook)]
  (#match? @name "^use[A-Z0-9]")
  (#match? @body "\\buse([A-Z0-9][A-Za-z0-9_$]*)?\\s*\\("))

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)])

; A PascalCase module-scope binding to memo, forwardRef, lazy or a styled
; template, or to a PascalCase member (a library component under a local
; name), is a component (LB-F377; the typescript dialect's rule).
(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: (call_expression function: (_) @_wrap)) @definition.component)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: (call_expression function: (_) @_wrap)) @definition.component)]
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @_wrap "^((React\\.)?(memo|forwardRef|lazy)$|styled\\b)"))

(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: (member_expression) @_alias) @definition.component)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: (member_expression) @_alias) @definition.component)]
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @_alias "^[A-Z][A-Za-z0-9_$]*\\.[A-Z][A-Za-z0-9_$]*[a-z][A-Za-z0-9_$]*$"))

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: (call_expression function: (_) @_wrap)) @definition.component)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: (call_expression function: (_) @_wrap)) @definition.component)]
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @_wrap "^((React\\.)?(memo|forwardRef|lazy)$|styled\\b)"))

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: (member_expression) @_alias) @definition.component)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: (member_expression) @_alias) @definition.component)]
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @_alias "^[A-Z][A-Za-z0-9_$]*\\.[A-Z][A-Za-z0-9_$]*[a-z][A-Za-z0-9_$]*$"))

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
;
;
; Module-level constants (LB-F382, wave lf3, item tsjs): a `const` binding at
; module scope, or exported (in a namespace too), is a definition, JSConst
; (canonical constant): scip-typescript defines every such binding (O746: 1,102
; of TypeScript's and 2,660 + 1,212 of webpack's unmapped definitions, the
; targets of 1,500 and 10,295 unmapped references). A const bound to a function
; or a component is the function or component the patterns above name: one
; node and one name make one definition, the earlier pattern's
; (oneDefPerNode), so these patterns need no predicate. Each name a
; destructuring const binds is one, its own node. A constant is no callable:
; the calls its initializer makes are attached to it (@calls.from on its own
; match), as scip-typescript's const, which has no enclosing range, is read over
; its mapped definition's lines (D5).
;
; Module-level `let` and `var` (wave lf5, item tsjs, O749): the same patterns
; read every lexical declaration and every `var` statement at module scope.
; scip-typescript defines a module-level let or var as it defines a const (a
; global term of the file, `(var)` or `(let)` in its documentation), and a
; var-style CommonJS codebase binds its modules, its app and its tables that way
; (express's 468 and webpack's 294 such bindings were unmapped definitions, the
; targets of their references). A binding inside a function, a block or a loop
; head (`for (var i ...)`) is none: only a direct child of the program or of an
; export statement is read. The kind stays JSConst (canonical constant) so the
; scip oracle's kind intersection, and L1, are unchanged.
(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: (_)? @calls.from) @definition.constant)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: (_)? @calls.from) @definition.constant)])

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: (_)? @calls.from) @definition.constant)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: (_)? @calls.from) @definition.constant)])

(program
  [(lexical_declaration
     (variable_declarator
       name: (object_pattern
         [(shorthand_property_identifier_pattern) @name @definition.constant
          (pair_pattern value: (identifier) @name @definition.constant)
          (rest_pattern (identifier) @name @definition.constant)])))
   (variable_declaration
     (variable_declarator
       name: (object_pattern
         [(shorthand_property_identifier_pattern) @name @definition.constant
          (pair_pattern value: (identifier) @name @definition.constant)
          (rest_pattern (identifier) @name @definition.constant)])))])

(program
  [(lexical_declaration
     (variable_declarator
       name: (array_pattern (identifier) @name @definition.constant)))
   (variable_declaration
     (variable_declarator
       name: (array_pattern (identifier) @name @definition.constant)))])

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
;
; Members (LB-F383): a class field is a definition, JSField (canonical field),
; named Owner.name by the member pass, its initializer's calls its own (the
; typescript dialect's rule).
(class_body
  (field_definition
    property: [(property_identifier) (private_property_identifier)] @name
    value: (_)? @calls.from) @definition.field @member)

;
; CommonJS exports a Lambda handler is bound by (O721, LB-F384): an export made
; by member assignment to the module's exports is the export's function when
; its value is a function (`module.exports.x = async () => {}`, `= async
; function () {}`) or a call wrapping one (`exports.handler = metricScope(m =>
; async (e) => …)`, `module.exports.x = middy(async (event) => …)`): the export
; is what the runtime calls, and the calls its value makes are the function's.
; A wrapper assigned to anything but the module's exports is none.
(program
  (expression_statement
    (assignment_expression
      left: (member_expression
        object: (member_expression
          object: (identifier) @_m
          property: (property_identifier) @_e)
        property: (property_identifier) @name)
      right: [(function_expression body: (_) @body) (arrow_function body: (_) @body)]) @definition.function)
  (#eq? @_m "module")
  (#eq? @_e "exports"))

(program
  (expression_statement
    (assignment_expression
      left: (member_expression
        object: (identifier) @_x
        property: (property_identifier) @name)
      right: (call_expression
        arguments: (arguments [(arrow_function) (function_expression)]))) @definition.function)
  (#eq? @_x "exports"))

(program
  (expression_statement
    (assignment_expression
      left: (member_expression
        object: (member_expression
          object: (identifier) @_m
          property: (property_identifier) @_e)
        property: (property_identifier) @name)
      right: (call_expression
        arguments: (arguments [(arrow_function) (function_expression)]))) @definition.function)
  (#eq? @_m "module")
  (#eq? @_e "exports"))

;
; A CommonJS export assigned a value at module scope (wave lf5, item tsjs,
; O749): `exports.x = value` and `module.exports.x = value` bind the module's
; export x, which scip-typescript defines as a property of the file's module
; and a reference to it resolves to; a JSConst, its value's calls its own. An
; export assigned a function, or a call wrapping one, is the JSFunc the patterns
; above name (the earlier pattern's, oneDefPerNode); a member assigned on any
; other object (`app.settings = {}`) and `module.exports = value` are none.
(program
  (expression_statement
    (assignment_expression
      left: (member_expression
        object: (identifier) @_x
        property: (property_identifier) @name)
      right: (_) @calls.from) @definition.constant)
  (#eq? @_x "exports"))

(program
  (expression_statement
    (assignment_expression
      left: (member_expression
        object: (member_expression
          object: (identifier) @_m
          property: (property_identifier) @_e)
        property: (property_identifier) @name)
      right: (_) @calls.from) @definition.constant)
  (#eq? @_m "module")
  (#eq? @_e "exports"))

(call_expression
  function: (_) @reference.call)

; `new X(...)` is a call to X, and a class field's initializer belongs to the
; class (LB-F171, LB-F173, O440): the typescript dialect's rules, read by
; the same oracle (scip-typescript).
(new_expression
  constructor: (_) @reference.call)

; A JSX element is a call to its tag (LB-F377): a capitalized or member tag
; (<Card/>, <Icon.Glyph/>) is a call from the enclosing definition; an
; intrinsic lower-case tag (<div>) is none. A closing tag repeats its opening
; tag's name and is not read.
([(jsx_opening_element
    name: (identifier) @reference.call)
  (jsx_self_closing_element
    name: (identifier) @reference.call)]
  (#match? @reference.call "^[A-Z]"))

([(jsx_opening_element
    name: (member_expression) @reference.call)
  (jsx_self_closing_element
    name: (member_expression) @reference.call)])

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
