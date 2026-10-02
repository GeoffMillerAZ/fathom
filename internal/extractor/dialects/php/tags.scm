; source: tree-sitter/tree-sitter-php queries/tags.scm @ 5b5627faaa290d89eb3d01b9bf47c3bb9e797dea (MIT)
; changes: upstream tags a namespace as @definition.module, a trait as
; @definition.interface, a method as @definition.function, has @definition.field
; for properties and @reference.class/@reference.implementation shapes, and puts
; @name inside @reference.call. This file keeps its node vocabulary but maps
; every kind to the incumbent CGO port's own SourceKinds (phpextract.go:
; PhpClass, PhpInterface, PhpTrait, PhpEnum, PhpFunction, PhpMethod; LB-F81 adds
; PhpProperty, PhpConst and PhpEnumCase), adds a
; PhpNamespace definition whose name is also the file's @scope.package
; (design §3.5), captures @body for signatures, and recurses into enum bodies
; (the incumbent did not; an enum's methods are real definitions).

; Namespaces. `namespace A\B;` has no body: its name is a file-wide package
; clause. `namespace A\B { ... }` has a body and its own range is the package
; scope of everything declared inside it. A bare `namespace { ... }` has no
; name and matches neither.
(namespace_definition
  name: (namespace_name) @name @scope.package
  body: (compound_statement)) @definition.namespace @scope

(namespace_definition
  name: (namespace_name) @name @scope.package
  !body) @definition.namespace

; Types.
(class_declaration
  name: (name) @name
  body: (_) @body) @definition.class

(interface_declaration
  name: (name) @name
  body: (_) @body) @definition.interface

(trait_declaration
  name: (name) @name
  body: (_) @body) @definition.trait

(enum_declaration
  name: (name) @name
  body: (_) @body) @definition.enum

; Callables. A function declared inside an `if (!function_exists(...))` guard is
; a definition too (the incumbent's generic descent found it); a bodiless
; interface or abstract method ends at its ";".
(function_definition
  name: (name) @name
  body: (_) @body) @definition.function

(method_declaration
  name: (name) @name
  body: (compound_statement) @body) @definition.method

(method_declaration
  name: (name) @name
  ";" @body) @definition.method

; Members (LB-F81): the definitions scip-php writes beyond the callables. A
; property is named with its `$` (as scip-php writes it), once per element of a
; grouped declaration (`public $a, $b;`) and once per promoted constructor
; parameter; a constant is named per const_element (a `const A = 1, B = 2;`
; group is two); an enum case is one definition. With dialect.json's
; qualify.chain_containers each is "Owner.name", as a method is.
(property_element
  name: (variable_name) @name) @definition.property

(property_promotion_parameter
  name: (variable_name) @name) @definition.property

(const_element
  (name) @name) @definition.const

(enum_case
  name: (name) @name) @definition.enum_case

; Calls: the callee is the called name alone — a plain function, an instance
; method, a nullsafe method and a static/parent:: call all reach it through the
; same name: field. `new X(...)` is a type reference, not a call. isset, empty,
; die and eval are language constructs this grammar parses as function calls
; (keywords are case-insensitive); no index counts them as callables.
((function_call_expression
  function: (name) @name) @reference.call
  (#not-match? @name "^(?i:isset|empty|die|eval|exit)$"))

((function_call_expression
  function: [(qualified_name (name) @name .) (relative_name (name) @name .)]) @reference.call
  (#not-match? @name "^(?i:isset|empty|die|eval|exit)$"))

; With dialect.json's calls.qualify (LB-F65) a method call's receiver and a
; scoped call's scope qualify the name: `$this->client->send()` is stored as
; "this.client.send", `Helper::assist()` as "Helper::assist", `self::x()` as
; "self::x", so the file resolver can bind the qualifier to a `use` or to the
; enclosing class. The last segment is the called name, as before.
(member_call_expression
  object: (_) @call.receiver
  name: (name) @name) @reference.call

(nullsafe_member_call_expression
  object: (_) @call.receiver
  name: (name) @name) @reference.call

(scoped_call_expression
  scope: (_) @call.scope
  name: (name) @name) @reference.call
