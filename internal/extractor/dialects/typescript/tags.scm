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

; React (LB-F377, wave c3, item react): a class whose heritage, before its
; body, extends Component or PureComponent (React's own, or imported bare) is
; a class component; every other class is a class. The two patterns split one
; node by a predicate over its heritage (extends …), so it is never both.
(class_declaration
  name: (_) @name
  (class_heritage)? @_heritage
  (#not-match? @_heritage "^extends\\s+(React\\.)?(Pure)?Component\\b")) @definition.class

(class_declaration
  name: (_) @name
  (class_heritage) @_heritage
  (#match? @_heritage "^extends\\s+(React\\.)?(Pure)?Component\\b")) @definition.class_component

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

; React (LB-F377): a function declaration (at any depth, as before) is a
; component when its name is PascalCase (capitalized, with a lower-case
; letter: GET or MAX_SIZE is a constant's or an HTTP verb's) and its body
; writes JSX (a closing
; `</` or a self-closing `/>`: every JSX expression has one), and a hook when
; its name is use[A-Z0-9] and its body calls a hook (React's rule: a function
; that calls no hook is not one, whatever its name). Each other combination
; stays a function, so one node is one definition. The JSX test reads the
; body's text, since the typescript grammar (shared with tsx) has no JSX node
; to name. Canonical kinds are unchanged: a component and a hook are
; functions to every oracle.
(function_declaration
  name: (identifier) @name
  body: (_) @body
  (#not-match? @name "^([A-Z]|use[A-Z0-9])")) @definition.function

(function_declaration
  name: (identifier) @name
  body: (_) @body
  (#match? @name "^[A-Z]")
  (#not-match? @body "</|/>")) @definition.function

(function_declaration
  name: (identifier) @name
  body: (_) @body
  (#match? @name "^[A-Z][^a-z]*$")
  (#match? @body "</|/>")) @definition.function

(function_declaration
  name: (identifier) @name
  body: (_) @body
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @body "</|/>")) @definition.component

(function_declaration
  name: (identifier) @name
  body: (_) @body
  (#match? @name "^use[A-Z0-9]")
  (#not-match? @body "\\buse([A-Z0-9][A-Za-z0-9_$]*)?\\s*(<[^()]*>)?\\(")) @definition.function

(function_declaration
  name: (identifier) @name
  body: (_) @body
  (#match? @name "^use[A-Z0-9]")
  (#match? @body "\\buse([A-Z0-9][A-Za-z0-9_$]*)?\\s*(<[^()]*>)?\\(")) @definition.hook

(generator_function_declaration
  name: (identifier) @name
  body: (_) @body) @definition.function

; Name alternation per critic B2 — see javascript.scm's comment.
(class_body
  (method_definition
    name: [(property_identifier) (private_property_identifier) (computed_property_name)] @name
    body: (_) @body) @definition.method)

; const f = () => … / var g = function () {…} at module scope, and
; export const f = () => … (at module scope, or in a namespace): a function,
; a component or a hook by the function declaration's rules above.
(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)]
  (#not-match? @name "^([A-Z]|use[A-Z0-9])"))

(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)]
  (#match? @name "^[A-Z]")
  (#not-match? @body "</|/>"))

(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)]
  (#match? @name "^[A-Z][^a-z]*$")
  (#match? @body "</|/>"))

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
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)]
  (#match? @name "^use[A-Z0-9]")
  (#not-match? @body "\\buse([A-Z0-9][A-Za-z0-9_$]*)?\\s*(<[^()]*>)?\\("))

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
  (#match? @body "\\buse([A-Z0-9][A-Za-z0-9_$]*)?\\s*(<[^()]*>)?\\("))

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)]
  (#not-match? @name "^([A-Z]|use[A-Z0-9])"))

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)]
  (#match? @name "^[A-Z]")
  (#not-match? @body "</|/>"))

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)]
  (#match? @name "^[A-Z][^a-z]*$")
  (#match? @body "</|/>"))

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
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.function)]
  (#match? @name "^use[A-Z0-9]")
  (#not-match? @body "\\buse([A-Z0-9][A-Za-z0-9_$]*)?\\s*(<[^()]*>)?\\("))

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
  (#match? @body "\\buse([A-Z0-9][A-Za-z0-9_$]*)?\\s*(<[^()]*>)?\\("))

; A PascalCase binding at module scope to a component-making call is a
; component (LB-F377): memo, forwardRef or lazy (React's own, or bare), a
; styled-components or emotion template (styled.div`…`, styled(X)`…`,
; styled.div.attrs(…)`…`), whose value is no function the patterns above
; read; so is a capitalized alias of a capitalized member (`const Dialog =
; DialogPrimitive.Root`, a library component re-exported under a local name;
; never a CONSTANT_CASE member such as `X.FUNCTION_PROPERTY_DROP_HEAD`).
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

(interface_declaration
  name: (type_identifier) @name) @definition.interface

(type_alias_declaration
  name: (type_identifier) @name) @definition.type

(enum_declaration
  name: (identifier) @name) @definition.enum

; React props (LB-F377): a property of an interface or type alias named
; *Props (Props, IProps, ButtonProps) is a prop, named Owner.prop by the member
; pass (@member: the interface or type alias is its container). A type
; alias's object type is read directly or as a member of an intersection one
; or two deep (`A & B & { … }`, `{ … } & A & B`). A method signature stays a
; method (the method pattern above names it Owner.method); a props type
; written inline on a parameter or as a type argument has no owner and no
; prop (written ceilings, docs/plans/language-breadth.md LB-Q-react-rest).
(interface_declaration
  name: (type_identifier) @_owner
  body: (interface_body
    (property_signature
      name: (property_identifier) @name) @definition.prop @member)
  (#match? @_owner "Props$"))

(type_alias_declaration
  name: (type_identifier) @_owner
  value: (object_type
    (property_signature
      name: (property_identifier) @name) @definition.prop @member)
  (#match? @_owner "Props$"))

(type_alias_declaration
  name: (type_identifier) @_owner
  value: (intersection_type
    (object_type
      (property_signature
        name: (property_identifier) @name) @definition.prop @member))
  (#match? @_owner "Props$"))

(type_alias_declaration
  name: (type_identifier) @_owner
  value: (intersection_type
    (intersection_type
      (object_type
        (property_signature
          name: (property_identifier) @name) @definition.prop @member)))
  (#match? @_owner "Props$"))

(call_expression
  function: (_) @reference.call)

; `new X(...)` is a call to X (LB-F171, O440): scip-typescript references
; X's constructor there, a class with none by its own name, and the text
; reads as X's (calls.qualify compacts `new ns.X` to ns.X).
(new_expression
  constructor: (_) @reference.call)

; A JSX element is a call to its tag (LB-F377): React calls the component the
; tag names, so a capitalized tag or a member tag (<Card/>, <Icon.Glyph/>) is
; a call from the enclosing definition, same-file or imported alike; an
; intrinsic lower-case tag (<div>) is none. The typescript grammar (shared
; with tsx) has no JSX node to name, so the pattern names what a JSX opening
; or self-closing element is in tsx, a node with a `<` token and a name field,
; which no node of the typescript grammar is (a type argument, a type
; parameter list and a `<T>x` assertion have no name field).
(_
  "<"
  name: (identifier) @reference.call
  (#match? @reference.call "^[A-Z]"))

(_
  "<"
  name: (member_expression) @reference.call)

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
