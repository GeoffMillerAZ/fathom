; source: tree-sitter/tree-sitter-rust queries/tags.scm @ 77a3747266f4d621d0757825e6b11edcbf991ca5 (MIT)
; changes: upstream tags struct, enum, union and type alias as @definition.class,
; a trait as @definition.interface, a function inside any declaration_list as
; @definition.method, impl blocks only as @reference.implementation, no consts or
; statics, and calls only for a plain name, a field call and a macro. This file keeps
; its node vocabulary but maps every kind to the incumbent CGO port's own SourceKinds
; (rustextract.go: RustFn, RustStruct, RustEnum, RustTrait, RustImpl, RustMod), makes
; an impl block a definition named by the type it implements (so its methods qualify
; to that type, Type.method), adds unions, module-level type aliases, consts and
; statics, macro_rules! definitions and bodiless trait/extern function signatures,
; captures @body for signatures, and adds the call shapes the upstream query misses
; (a scoped path, a turbofish call, a call written inside a macro's token tree).

; Modules. `mod x { ... }` and the file declaration `mod x;` are both definitions
; (the incumbent emitted both); a module is not a qualifying container, so a function
; in it keeps its bare name.
(mod_item
  name: (identifier) @name) @definition.mod

; Types.
(struct_item
  name: (type_identifier) @name) @definition.struct

(enum_item
  name: (type_identifier) @name) @definition.enum

(union_item
  name: (type_identifier) @name) @definition.union

(trait_item
  name: (type_identifier) @name
  body: (declaration_list) @body) @definition.trait

; Aliases, constants and statics of a module, of the crate root or of a function
; body. An associated type or constant (inside an impl or trait body) is a member
; of that type, not an item: it is not extracted.
(source_file
  (type_item
    name: (type_identifier) @name) @definition.type_alias)

(mod_item
  body: (declaration_list
    (type_item
      name: (type_identifier) @name) @definition.type_alias))

(block
  (type_item
    name: (type_identifier) @name) @definition.type_alias)

(source_file
  (const_item
    name: (identifier) @name) @definition.const)

(mod_item
  body: (declaration_list
    (const_item
      name: (identifier) @name) @definition.const))

(block
  (const_item
    name: (identifier) @name) @definition.const)

(source_file
  (static_item
    name: (identifier) @name) @definition.static)

(mod_item
  body: (declaration_list
    (static_item
      name: (identifier) @name) @definition.static))

(block
  (static_item
    name: (identifier) @name) @definition.static)

(macro_definition
  name: (identifier) @name) @definition.macro

; Callables. A function, a method, a trait's default method and a nested function
; are all function_item; a trait's required method and an extern block's function are
; bodiless (function_signature_item) and end at their ";".
(function_item
  name: (identifier) @name
  body: (block) @body) @definition.function

(function_signature_item
  name: (identifier) @name
  ";" @body) @definition.function

; Impl blocks are named by the type they implement, whether inherent (`impl Foo`)
; or a trait impl (`impl Trait for Foo`); generic arguments, a path prefix and a
; reference are dropped, the head type's own name kept (the incumbent kept the
; source text of the `type:` field whole, "Foo<T>"). A type with no name of its
; own (a slice, a tuple, `dyn Trait`) keeps its source text.
(impl_item
  type: (type_identifier) @name) @definition.impl

(impl_item
  type: (primitive_type) @name) @definition.impl

(impl_item
  type: (generic_type
    type: (type_identifier) @name)) @definition.impl

(impl_item
  type: (generic_type
    type: (scoped_type_identifier
      name: (type_identifier) @name))) @definition.impl

(impl_item
  type: (scoped_type_identifier
    name: (type_identifier) @name)) @definition.impl

(impl_item
  type: (reference_type
    type: (type_identifier) @name)) @definition.impl

(impl_item
  type: (reference_type
    type: (primitive_type) @name)) @definition.impl

(impl_item
  type: (reference_type
    type: (generic_type
      type: (type_identifier) @name))) @definition.impl

(impl_item
  type: (reference_type
    type: (reference_type
      type: (type_identifier) @name))) @definition.impl

(impl_item
  type: (reference_type
    type: [(array_type) (tuple_type) (dynamic_type)] @name)) @definition.impl

(impl_item
  type: [(array_type) (tuple_type) (dynamic_type) (pointer_type) (bounded_type)
         (abstract_type) (function_type) (never_type) (unit_type)] @name) @definition.impl

; Calls. A plain call, a method call and the two generic (turbofish) forms reach
; the callee by its own name; a scoped path (`Foo::new()`, `crate::a::b()`,
; `std::mem::swap()`) is captured whole and the manifest's scope_sep spells it
; Foo.new, so it is the same key as the method it names. A capitalised callee is a
; tuple-struct or enum-variant constructor (`Some(x)`, `Wrapper(y)`), not a function;
; Rust's naming rules make the case reliable. A path with a generic segment
; (`Vec::<u8>::new()`, `<T as Trait>::f()`) keeps its last name alone.
((call_expression
  function: (identifier) @name) @reference.call
  (#not-match? @name "^[A-Z]"))

((call_expression
  function: (generic_function
    function: (identifier) @name)) @reference.call
  (#not-match? @name "^[A-Z]"))

(call_expression
  function: (field_expression
    field: (field_identifier) @name)) @reference.call

(call_expression
  function: (generic_function
    function: (field_expression
      field: (field_identifier) @name))) @reference.call

((call_expression
  function: (scoped_identifier) @name) @reference.call
  (#not-match? @name "<")
  (#not-match? @name "::[A-Z][A-Za-z0-9_]*$"))

((call_expression
  function: (scoped_identifier
    name: (identifier) @name) @path) @reference.call
  (#match? @path "<")
  (#not-match? @name "^[A-Z]"))

((call_expression
  function: (generic_function
    function: (scoped_identifier
      name: (identifier) @name))) @reference.call
  (#not-match? @name "^[A-Z]"))

; A macro invocation is a call by the macro's own name, as the incumbent recorded it.
(macro_invocation
  macro: (identifier) @name) @reference.call

(macro_invocation
  macro: (scoped_identifier
    name: (identifier) @name)) @reference.call

; A macro's arguments are a flat token_tree, not parsed expressions, so `f(x)` and
; `v.m(x)` inside `assert_eq!(...)`, `format!(...)` or `println!(...)` are an
; identifier followed by a parenthesised token_tree. The anchor skips the anonymous
; `,` and operator tokens between them, so `a, (b, c)` reads as a call of `a`; that
; false reading is accepted for the far commoner true one (a test body is mostly
; assertions over calls).
((token_tree
  (identifier) @name
  .
  (token_tree) @args) @reference.call
  (#match? @args "^\\(")
  (#not-match? @name "^[A-Z]")
  (#not-any-of? @name "in" "where" "else" "move" "async" "await" "loop" "unsafe" "dyn" "impl" "mut" "ref" "self" "Self" "crate" "super"))
