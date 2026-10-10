; dialects/scala/tags.scm — definitions and calls for the wazero tree-sitter
; engine (internal/extractor.NewTSExtractor("scala"), LB-Q-scala, design §13).
;
; Grammar: tree-sitter/tree-sitter-scala v0.26.2 (b931fcc3, MIT). The
; definition shapes start from the grammar's own queries/tags.scm at that tag
; (MIT): classes, objects, traits, enums, enum cases, functions, vals, vars,
; givens and type definitions. Written here for LB-Q-scala: the SourceKinds
; scalaextract.go emitted (ScalaClass, ScalaObject, ScalaTrait, ScalaDef, kept
; byte-identical), an operator or backticked name (`def +`, `def `a b``),
; abstract members (function_declaration, val_declaration, var_declaration),
; package objects, secondary constructors, the scope a val or var counts in,
; class parameters that are fields, @body signatures, and @name inside every
; call. See PROVENANCE.md for what upstream's file does differently.

; --- types and singletons -----------------------------------------------------
; A case class, an abstract, sealed, final or implicit class are one node; the
; modifiers are no kind of their own. A case object is an object_definition,
; a package object (`package object cats`) its own node, named like an object.
(class_definition
  name: (_) @name) @definition.class

(object_definition
  name: (_) @name) @definition.object

(package_object
  name: (_) @name) @definition.object

(trait_definition
  name: (_) @name) @definition.trait

; Scala 3 enums and their cases (`case Red`, `case Green, Blue`, `case
; Leaf(value: A)`), each case a member of its enum (`Color.Red`).
(enum_definition
  name: (_) @name) @definition.enum

(simple_enum_case
  name: (identifier) @name) @definition.enum_case

(full_enum_case
  name: (identifier) @name) @definition.enum_case

; A named given (`given intShow: Show[Int] with ...`, `given ord[T](using
; ...): Ordering[T] = ...`) is an instance its members qualify against; an
; anonymous given names nothing, so its members qualify against the enclosing
; template, the way an extension's do.
(given_definition
  name: (identifier) @name) @definition.given

; A type alias, an abstract type member, an opaque type, anywhere.
(type_definition
  name: (_) @name) @definition.type

; --- callables -----------------------------------------------------------------
; Every def: top-level (Scala 3), a member, an extension method, a local def in
; a body, a def inside an anonymous class. The name is any node (`def +`,
; `def ::`, a backticked name: names.trim_quotes drops the backticks). The
; Signature ends before `=`; a procedure (`def f() { }`) and an abstract def
; have no `=` and sign with their first line.
(function_definition
  name: (_) @name
  "="? @body
  (#not-eq? @name "this")) @definition.function

(function_declaration
  name: (_) @name
  (#not-eq? @name "this")) @definition.function

; A secondary constructor, `def this(...) = this(...)`, named `this`
; (`Square.this`).
(function_definition
  name: (_) @name
  "="? @body
  (#eq? @name "this")) @definition.constructor

; --- vals and vars ---------------------------------------------------------------
; A val or var is a definition where it is a member: in a template body (a
; class, object, trait, package object, anonymous class or `new X { }`), a
; given's `with` body, an enum body, or at the top level of a file (Scala 3, a
; script). A local val in a block or a function body is not. Each name of
; `val a, b = 1` is one; a pattern (`val (a, b) = ...`) names none.
(template_body
  (val_definition
    pattern: [(identifier) (operator_identifier)] @name) @definition.val)
(template_body
  (val_definition
    pattern: (identifiers [(identifier) (operator_identifier)] @name)) @definition.val)
(template_body
  (val_declaration
    name: (_) @name) @definition.val)
(template_body
  (var_definition
    pattern: [(identifier) (operator_identifier)] @name) @definition.var)
(template_body
  (var_definition
    pattern: (identifiers [(identifier) (operator_identifier)] @name)) @definition.var)
(template_body
  (var_declaration
    name: (_) @name) @definition.var)

(with_template_body
  (val_definition
    pattern: [(identifier) (operator_identifier)] @name) @definition.val)
(with_template_body
  (val_definition
    pattern: (identifiers [(identifier) (operator_identifier)] @name)) @definition.val)
(with_template_body
  (val_declaration
    name: (_) @name) @definition.val)
(with_template_body
  (var_definition
    pattern: [(identifier) (operator_identifier)] @name) @definition.var)
(with_template_body
  (var_declaration
    name: (_) @name) @definition.var)

(enum_body
  (val_definition
    pattern: [(identifier) (operator_identifier)] @name) @definition.val)
(enum_body
  (var_definition
    pattern: [(identifier) (operator_identifier)] @name) @definition.var)

(compilation_unit
  (val_definition
    pattern: [(identifier) (operator_identifier)] @name) @definition.val)
(compilation_unit
  (val_definition
    pattern: (identifiers [(identifier) (operator_identifier)] @name)) @definition.val)
(compilation_unit
  (var_definition
    pattern: [(identifier) (operator_identifier)] @name) @definition.var)

; A class parameter is a field when it is written `val` or `var`, and every
; parameter of a case class or of an enum case with parameters is a val
; (`case class Circle(radius: Double)`), in every parameter list. The lists are
; matched without their field name: with `class_parameters:` a query reads only
; the first of several (`(a: A)(implicit b: B)` lost `b`).
(class_parameter
  "val"
  name: (identifier) @name) @definition.val

(class_parameter
  "var"
  name: (identifier) @name) @definition.var

(class_definition
  "case"
  (class_parameters
    (class_parameter
      name: (identifier) @name) @definition.val
    (#not-match? @definition.val "^[^:=]*\\b(val|var)\\b")))

(full_enum_case
  (class_parameters
    (class_parameter
      name: (identifier) @name) @definition.val
    (#not-match? @definition.val "^[^:=]*\\b(val|var)\\b")))

; --- calls -----------------------------------------------------------------------
; f(x), a.b.f(x), f[T](x), a.f[T](x), f { ... }: the callee is the last
; identifier of the call's function. A call whose function is itself a call
; (`f(a)(b)`) names nothing of its own; the inner call does. `this(...)` in a
; secondary constructor is no call (it is the constructor's own invocation). An
; infix (`xs map f`), `new X(...)`, a pattern and a parameterless member are
; not calls here.
(call_expression
  function: [(identifier) (operator_identifier)] @name
  (#not-eq? @name "this")) @reference.call

(call_expression
  function: (field_expression
    field: [(identifier) (operator_identifier)] @name)) @reference.call

(call_expression
  function: (generic_function
    function: (identifier) @name)) @reference.call

(call_expression
  function: (generic_function
    function: (field_expression
      field: [(identifier) (operator_identifier)] @name))) @reference.call

; The grammar reads `a <-> F.f[T](x)` and `k -> V[T](x)` with the type
; arguments applied to the whole infix expression (generic_function over an
; infix_expression); the call is still to the infix's right operand, the way
; scalameta reads it (ApplyInfix(a, <->, Apply(ApplyType(F.f, T), x))).
(call_expression
  function: (generic_function
    function: (infix_expression
      right: (identifier) @name))) @reference.call

(call_expression
  function: (generic_function
    function: (infix_expression
      right: (field_expression
        field: [(identifier) (operator_identifier)] @name)))) @reference.call
