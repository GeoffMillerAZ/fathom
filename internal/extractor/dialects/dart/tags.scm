; dialects/dart/tags.scm — definitions and calls for the wazero tree-sitter
; engine (internal/extractor.NewTSExtractor("dart"), LB-Q-dart,
; docs/design/language-breadth.md §13).
;
; Grammar: UserNobody14/tree-sitter-dart at commit be07cf71 (MIT, untagged; see
; PROVENANCE.md). Its shape drives every pattern below:
;
;  - A callable's body is a SIBLING of its signature (function_signature,
;    getter_signature, ... then function_body), never a child. A bodied method
;    is a method_signature followed by a function_body in its class body, a
;    bodied top-level function a function_signature followed by a function_body
;    under program. The definition is the signature node and the match also
;    captures the body as @body (the Signature stops where it starts) and @end
;    (the definition reaches its end; design §3.3), so EndLine, the calls the
;    body makes and the definitions nested in it all follow. (A comment between
;    the signature and its body breaks the adjacency: `(comment)*` between the
;    two anchors matched every later body, so one- and two-comment variants of
;    the function and method patterns stand in, LB-F247.) A member without a
;    body (abstract, external, a redirecting or const constructor) is wrapped in
;    a `declaration` instead, which spans it whole; an external top-level
;    function is a function_signature after an anonymous `external` token.
;  - A statement is not one node: an expression is an identifier followed by
;    `selector` siblings (`a.b(1)` is a, selector(.b), selector(argument_part)),
;    a top-level variable is loose children of `program`. Calls are therefore
;    written as sibling sequences, and a call's @name sits on the node that
;    names it.
;
; The starting point was the grammar's own queries/tags.scm (MIT): its class,
; mixin, extension, enum, typedef and method captures are kept as shapes; what
; is new here is written for this repository — sibling bodies, constructors,
; accessors, operators, fields, enum constants, local functions and calls.

; --- types -------------------------------------------------------------------
; abstract/sealed/base/final/interface/mixin class modifiers are one node kind.
(class_definition
  name: (identifier) @name) @definition.class

; class Both = Base with Logger;
(class_definition
  (mixin_application_class
    .
    (identifier) @name)) @definition.class

(mixin_declaration
  (mixin)
  .
  (identifier) @name) @definition.mixin

; An unnamed extension (`extension on int`) has no name and no symbol; its
; members qualify against nothing.
(extension_declaration
  name: (identifier) @name) @definition.extension

(extension_type_declaration
  name: (identifier) @name) @definition.extension_type

(enum_declaration
  name: (identifier) @name) @definition.enum

(enum_constant
  name: (identifier) @name) @definition.enum_constant

; typedef Cb = void Function(int);   and   typedef int Compare(Object a, Object b);
; The alias's name is the first type_identifier of the `=` form and the one
; right before the parameter list (or its type parameters) of the old form.
(type_alias
  .
  (type_identifier) @name
  "=") @definition.typedef

(type_alias
  (type_identifier) @name
  .
  (formal_parameter_list)) @definition.typedef

(type_alias
  (type_identifier) @name
  .
  (type_parameters)
  .
  (formal_parameter_list)) @definition.typedef

; --- callables with a body (signature + sibling function_body) -------------------
; Top-level functions and accessors. The parent is pinned to `program`: a local
; function's lambda_expression also holds a function_signature followed by a
; function_body.
(program
  (function_signature
    name: (identifier) @name) @definition.function
  .
  (function_body) @body @end)

(program
  (getter_signature
    name: (identifier) @name) @definition.accessor
  .
  (function_body) @body @end)

(program
  (setter_signature
    name: (identifier) @name) @definition.accessor
  .
  (function_body) @body @end)

; An external top-level function has no body; the keyword is an anonymous
; sibling right before the signature, which is how it is told from a bodied one.
(program
  "external"
  .
  (function_signature
    name: (identifier) @name) @definition.function)

; A local function is a lambda_expression holding its own signature and body.
(local_function_declaration
  (lambda_expression
    parameters: (function_signature
      name: (identifier) @name)
    body: (function_body) @body) @definition.function)

; Members of a class, mixin, enum, extension or extension type: the wrapper
; (method_signature) is the definition, and it also holds a constructor's
; initializer list.
((method_signature
   (function_signature
     name: (identifier) @name)) @definition.method
 .
 (function_body) @body @end)

((method_signature
   (getter_signature
     name: (identifier) @name)) @definition.accessor
 .
 (function_body) @body @end)

((method_signature
   (setter_signature
     name: (identifier) @name)) @definition.accessor
 .
 (function_body) @body @end)

; operator +, operator ==, operator <, ...
((method_signature
   (operator_signature
     (binary_operator) @name)) @definition.method
 .
 (function_body) @body @end)

; Foo(...), Foo.named(...): the name is the identifier right before the
; parameter list (a named constructor has two). An unnamed one is Foo.Foo.
((method_signature
   [(constructor_signature
      name: (identifier) @name
      .
      parameters: (formal_parameter_list))
    (factory_constructor_signature
      (identifier) @name
      .
      (formal_parameter_list))]) @definition.constructor
 .
 (function_body) @body @end)

;
; A comment (or two) between the signature and its `=>` (web_socket_channel's
; `static String signKey(String key) // note ... => ...`, LB-F247) is a sibling
; of its own: one and two comment variants of the function and method patterns.
(program
  (function_signature
    name: (identifier) @name) @definition.function
  .
  (comment)
  .
  (function_body) @body @end)

(program
  (function_signature
    name: (identifier) @name) @definition.function
  .
  (comment)
  .
  (comment)
  .
  (function_body) @body @end)

((method_signature
   (function_signature
     name: (identifier) @name)) @definition.method
 .
 (comment)
 .
 (function_body) @body @end)

((method_signature
   (function_signature
     name: (identifier) @name)) @definition.method
 .
 (comment)
 .
 (comment)
 .
 (function_body) @body @end)

; --- callables without a body ----------------------------------------------------
; Abstract and external members, and constructors that only initialise or
; redirect, are `declaration` nodes that span them whole. There is no body to
; end the Signature at, so it is the declaration's first line.
(declaration
  (function_signature
    name: (identifier) @name)) @definition.method

(declaration
  [(getter_signature
     name: (identifier) @name)
   (setter_signature
     name: (identifier) @name)]) @definition.accessor

(declaration
  (operator_signature
    (binary_operator) @name)) @definition.method

(declaration
  [(constructor_signature
     name: (identifier) @name
     .
     parameters: (formal_parameter_list))
   (constant_constructor_signature
     (identifier) @name
     .
     (formal_parameter_list))
   (redirecting_factory_constructor_signature
     (identifier) @name
     .
     (formal_parameter_list))
   (factory_constructor_signature
     (identifier) @name
     .
     (formal_parameter_list))]) @definition.constructor

; --- fields and variables -------------------------------------------------------
; One definition per declared name (`int a = 1, b;` is two); the `.` pins the
; name to the first child, since the initialiser may itself be an identifier.
(declaration
  (initialized_identifier_list
    (initialized_identifier
      .
      (identifier) @name) @definition.field))

(declaration
  (static_final_declaration_list
    (static_final_declaration
      .
      (identifier) @name) @definition.field))

; abstract final Uri url;  (the names are an identifier_list)
(declaration
  (abstract)
  (identifier_list
    (identifier) @name)) @definition.field

(program
  (initialized_identifier_list
    (initialized_identifier
      .
      (identifier) @name) @definition.variable))

(program
  (static_final_declaration_list
    (static_final_declaration
      .
      (identifier) @name) @definition.variable))

; --- calls ------------------------------------------------------------------------
; foo(1) and Foo(1) (a constructor call reads the same): an identifier followed
; by an argument selector.
((identifier) @name @reference.call
 .
 (selector
   (argument_part)))

; a.b(1), a?.b(1), Foo.named(1), a.b(1).c(2): the member's selector followed
; by an argument selector; the callee is the member's name.
((selector
   [(unconditional_assignable_selector
      (identifier) @name)
    (conditional_assignable_selector
      (identifier) @name)]) @reference.call
 .
 (selector
   (argument_part)))

; super.f(1) and cascades: the member selector is a direct sibling.
((unconditional_assignable_selector
   (identifier) @name) @reference.call
 .
 (selector
   (argument_part)))

(cascade_section
  (cascade_selector
    (identifier) @name) @reference.call
  .
  (argument_part))

; A member chain inside a cascade, x..a.b(1): the selectors and the arguments
; are direct children of the cascade_section.
(cascade_section
  [(unconditional_assignable_selector
     (identifier) @name)
   (conditional_assignable_selector
     (identifier) @name)] @reference.call
  .
  (argument_part))

; Foo<T>.named(1) is read by the grammar as `Foo < T > .named(1)`: the member
; is a dot_shorthand (which is also how a Dart 3.10 `.named(1)` is written).
((dot_shorthand
   (identifier) @name) @reference.call
 .
 (selector
   (argument_part)))

; new Foo(1) / const Foo(1); with a named constructor the member is the callee.
(new_expression
  (type_identifier) @name @reference.call
  .
  [(type_arguments) (arguments)])

(new_expression
  (type_identifier)
  .
  (identifier) @name @reference.call
  .
  (arguments))

(const_object_expression
  (type_identifier) @name @reference.call
  .
  [(type_arguments) (arguments)])

(const_object_expression
  (type_identifier)
  .
  (identifier) @name @reference.call
  .
  (arguments))
