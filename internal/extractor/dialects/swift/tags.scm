; source: alex-pinkus/tree-sitter-swift queries/tags.scm @ 35245fbfee2fccf16273c6f4299438fb76875970 (MIT)
; changes: shapes only. The upstream file tags a class, a protocol, a method of a class or protocol, a
; property and a function and nothing else, with no end line, no @body, no callee names and no call
; patterns. Every pattern here is rewritten in this repository against the grammar's own tree
; (dialects/swift/PROVENANCE.md): each kind of declaration has its own definition, a callable carries
; a @body, and a call is named by its callee.
;
; Grammar: alex-pinkus/tree-sitter-swift at commit 35245fb (24 commits past the 0.7.3 tag; the lock's
; generate row). One node, class_declaration, is a class, a struct, an enum, an actor and an extension; which it is is the
; anonymous declaration_kind token. Where a construct lives decides its kind, so a function is a method
; inside a type body (class_body for class, struct, actor and extension, enum_class_body for enum) and a
; function at file level or inside a body of statements. Names are backtick-trimmed by the manifest
; (names.trim_quotes): `default` is default.
;
; Kinds a definition does not have on purpose: an operator declaration (`infix operator <+>`), a
; precedence group, a local variable or constant (a property_declaration inside a function body is no
; definition, only a member of a type body or a file-level one is) and a parameter. A `case a, b` line is
; one enum_entry with two names, so it is two definitions on the same lines.

; --- types -------------------------------------------------------------------------------------
(class_declaration
  declaration_kind: "class"
  name: (type_identifier) @name) @definition.class

(class_declaration
  declaration_kind: "struct"
  name: (type_identifier) @name) @definition.struct

(class_declaration
  declaration_kind: "enum"
  name: (type_identifier) @name) @definition.enum

(class_declaration
  declaration_kind: "actor"
  name: (type_identifier) @name) @definition.actor

; An extension is named by the type it extends (a dotted path for a nested one), so its members
; qualify under that type, the way a caller writes them (Box.Inner.encoded). The name is the type
; without its generic arguments (`extension UnsafeMutablePointer<CInterop.FTSEnt>` is
; UnsafeMutablePointer, `extension Foo.Bar<Int>` is Bar), and an array or optional sugar type
; (`extension [HTTPHeader]`) is its own spelling; a dictionary type is left out.
(class_declaration
  declaration_kind: "extension"
  name: (user_type) @name
  (#not-match? @name "<")) @definition.extension

(class_declaration
  declaration_kind: "extension"
  name: (user_type
    (type_identifier) @name
    .
    (type_arguments) .)) @definition.extension

(class_declaration
  declaration_kind: "extension"
  name: [(array_type) (optional_type)] @name) @definition.extension

(protocol_declaration
  name: (type_identifier) @name) @definition.protocol

; The alias is the first name: the aliased type is a second `name:` (a user_type, a function_type, ...).
(typealias_declaration
  name: (type_identifier) @name) @definition.typealias

(associatedtype_declaration
  name: (type_identifier) @name) @definition.associatedtype

; A `case a, b(Int)` line is one enum_entry with several names, and the engine takes a definition that
; shares its range with another as that one's child (its name would then stay unqualified), so each
; case is the definition at its own name, which is also where the compiler's parser ranges a plain
; case.
(enum_entry
  name: (simple_identifier) @name @definition.enum_case)

; --- callables ---------------------------------------------------------------------------------
; A function's body is the function_body child (it starts at "{"), so a Signature ends before it and
; carries the attributes, modifiers, generics, effects, return type and where clause before it.
[
  (class_body
    (function_declaration
      name: (simple_identifier) @name
      body: (function_body)? @body) @definition.method)
  (enum_class_body
    (function_declaration
      name: (simple_identifier) @name
      body: (function_body)? @body) @definition.method)
]

; An operator function (`static func == (l: V, r: V)`) has no simple_identifier: its name is the
; operator token, an anonymous `name:` child. The text test keeps out the return type, which is the
; other `name:` child of a function_declaration (a type never reads as a run of operator characters).
[
  (class_body
    (function_declaration
      name: _ @name
      body: (function_body)? @body
      (#match? @name "^[-+*/%<>=!&|^~?.]+$")) @definition.method)
  (enum_class_body
    (function_declaration
      name: _ @name
      body: (function_body)? @body
      (#match? @name "^[-+*/%<>=!&|^~?.]+$")) @definition.method)
]

(protocol_function_declaration
  name: (simple_identifier) @name) @definition.method

; A global operator function (`func == (l: T, r: T) -> Bool` at file level) is named the same way.
[
  (source_file
    (function_declaration
      name: _ @name
      body: (function_body)? @body
      (#match? @name "^[-+*/%<>=!&|^~?.]+$")) @definition.function)
  (statements
    (function_declaration
      name: _ @name
      body: (function_body)? @body
      (#match? @name "^[-+*/%<>=!&|^~?.]+$")) @definition.function)
]

[
  (source_file
    (function_declaration
      name: (simple_identifier) @name
      body: (function_body)? @body) @definition.function)
  (statements
    (function_declaration
      name: (simple_identifier) @name
      body: (function_body)? @body) @definition.function)
]

; init, deinit and subscript occur only inside a type body, so they need no parent pattern; the
; keyword is the name (Box.init, Box.subscript).
(init_declaration
  "init" @name
  body: (function_body)? @body) @definition.init

(deinit_declaration
  "deinit" @name
  body: (function_body)? @body) @definition.deinit

(subscript_declaration
  "subscript" @name
  (computed_property)? @body) @definition.subscript

(macro_declaration
  (simple_identifier) @name) @definition.macro

; --- properties --------------------------------------------------------------------------------
; A stored or computed property of a type body, one definition per bound name (`var x, y: Int` is
; two). The @body is a computed property's accessor block or an observer block, so the Signature reads
; `var length: Double`; a stored property's Signature is its declaration's first line. A tuple pattern
; (`let (p, q) = pair`) binds each of its names.
[
  (class_body
    (property_declaration
      name: (pattern bound_identifier: (simple_identifier) @name)
      [(computed_property) (willset_didset_block)]? @body) @definition.property)
  (enum_class_body
    (property_declaration
      name: (pattern bound_identifier: (simple_identifier) @name)
      [(computed_property) (willset_didset_block)]? @body) @definition.property)
]

[
  (class_body
    (property_declaration
      name: (pattern (pattern (simple_identifier) @name))
      [(computed_property) (willset_didset_block)]? @body) @definition.property)
  (enum_class_body
    (property_declaration
      name: (pattern (pattern (simple_identifier) @name))
      [(computed_property) (willset_didset_block)]? @body) @definition.property)
]

(protocol_property_declaration
  name: (pattern bound_identifier: (simple_identifier) @name)
  (protocol_property_requirements)? @body) @definition.property

; A file-level `let` or `var` (and, in a script, a top-level one).
(source_file
  (property_declaration
    name: (pattern bound_identifier: (simple_identifier) @name)
    [(computed_property) (willset_didset_block)]? @body) @definition.variable)

(source_file
  (property_declaration
    name: (pattern (pattern (simple_identifier) @name))
    [(computed_property) (willset_didset_block)]? @body) @definition.variable)

; --- calls -------------------------------------------------------------------------------------
; The grammar reads a subscript `a[0]` as a call_expression whose value_arguments open with "[", so a
; call is a call_suffix whose arguments open with "(" or a trailing closure. The callee is the name at
; the end of the callee expression: a bare name, the member after a dot (a.b?.c(x), a!.c(x), a[0].c(x),
; Type.init(x), self.init(x), super.init(x)) or an implicit member (`.make(x)`, which the grammar
; writes as a prefix_expression). A callee that is itself a call or a parenthesised expression
; (f()(x), (g)(x)) has no name and is not captured; the inner call is.
;
; Not calls, though the grammar writes them as one: `defer { ... }`, `nonisolated(unsafe) var x`, a
; closure's own argument called as `$0(x)`, and a function reference that names its argument labels
; (`register(handler: handle(_:))`, `#selector(run(a:b:))`).
((call_expression
  .
  (simple_identifier) @name
  (call_suffix [(value_arguments "(") (lambda_literal)])
  (#not-match? @name "^(defer|nonisolated|\\$[0-9]+)$")) @reference.call
  (#not-match? @reference.call "^[^()]*\\((?:[A-Za-z_][A-Za-z0-9_]*:)+\\)$"))

((call_expression
  .
  (navigation_expression
    suffix: (navigation_suffix
      suffix: (simple_identifier) @name))
  (call_suffix [(value_arguments "(") (lambda_literal)])) @reference.call
  (#not-match? @reference.call "^[^()]*\\((?:[A-Za-z_][A-Za-z0-9_]*:)+\\)$"))

((call_expression
  .
  (prefix_expression
    target: (simple_identifier) @name)
  (call_suffix [(value_arguments "(") (lambda_literal)])) @reference.call
  (#not-match? @reference.call "^[^()]*\\((?:[A-Za-z_][A-Za-z0-9_]*:)+\\)$"))

; A prefix operator binds to the callee's navigation (`!items.contains(x)`, `-a.b(c)`): the callee is
; then the prefix_expression around the member access.
(call_expression
  .
  (prefix_expression
    target: (navigation_expression
      suffix: (navigation_suffix
        suffix: (simple_identifier) @name)))
  (call_suffix [(value_arguments "(") (lambda_literal)])) @reference.call

; A call as the right operand of `+ - * / %` (`now() + .hours(1)`, `i % Int(x)`, `a() - B.length(n: 0)`,
; `xs + ys.flatMap { $0 }`): the grammar hangs the call's arguments on the whole additive or
; multiplicative expression, whose right operand is the callee (a prefix_expression for an implicit
; member, a bare name, or a member access). Every other infix operator reads correctly.
(call_expression
  .
  (_
    rhs: (prefix_expression
      target: (simple_identifier) @name))
  (call_suffix [(value_arguments "(") (lambda_literal)])) @reference.call

(call_expression
  .
  (_
    rhs: (simple_identifier) @name)
  (call_suffix [(value_arguments "(") (lambda_literal)])) @reference.call

(call_expression
  .
  (_
    rhs: (navigation_expression
      suffix: (navigation_suffix
        suffix: (simple_identifier) @name)))
  (call_suffix [(value_arguments "(") (lambda_literal)])) @reference.call

; Where the grammar cannot tell a specialised initialiser's angle brackets from comparisons it reads
; `Foo<Bar> { ... }` and `Types.Foo<Void>(flag: x)` as a chain of two comparison_expressions ending in
; the arguments or the closure; two comparisons in a row are not valid Swift, so that shape is the
; initialiser call, named by the type (a member access's last name).
(comparison_expression
  lhs: (simple_identifier) @name
  rhs: (comparison_expression
    rhs: [(tuple_expression) (lambda_literal)])) @reference.call

(comparison_expression
  lhs: (navigation_expression
    suffix: (navigation_suffix
      suffix: (simple_identifier) @name))
  rhs: (comparison_expression
    rhs: [(tuple_expression) (lambda_literal)])) @reference.call

; A specialised type's initialiser call (`Foo<Int>(x)`, `Foo.Bar<T>(x)`) is a constructor_expression,
; named by the last type identifier before its type arguments.
(constructor_expression
  constructed_type: (user_type
    (type_identifier) @name .)) @reference.call

(constructor_expression
  constructed_type: (user_type
    (type_identifier) @name
    .
    (type_arguments) .)) @reference.call
