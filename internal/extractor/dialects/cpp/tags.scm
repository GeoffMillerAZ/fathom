; dialects/cpp/tags.scm — symbol + call queries for the wazero tree-sitter
; engine (internal/extractor.NewTSExtractor("cpp"), ADR-021 §4, LB-Q-cpp port
; of the incumbent cppextract.go CGO walker).
;
; source: tree-sitter/tree-sitter-cpp queries/tags.scm @ f41e1a044c8a84ea9fa8577fdd2eab92ec96de02 (MIT)
; changes: the upstream file tags every function_declarator (prototypes and
; call-site-shaped declarators included), has no body check on a class or
; enum (a forward declaration would tag), files unions under "class" and
; enums/typedefs under "type", and has no namespace, macro, alias or call
; capture. This file instead mirrors the incumbent extractor's semantics —
; a definition needs a body, a struct/class/enum/union keeps its own kind,
; out-of-class members carry their qualified name — and adds namespaces,
; #define macros, `using` aliases and @reference.call.
;
; Names with a scope path ("Widget::render", "Box<T>::make", "a::b" in
; `namespace a::b`) are captured whole; the manifest's qualify.scope_sep
; rewrites them to "Widget.render" / "Box.make" / "a.b" (the CT-1 contract).
;
; Predicate use: only #not-match? on a cast keyword (C++ spells casts as
; template calls).

; --- functions ----------------------------------------------------------
; A function_definition's declarator is a function_declarator, optionally
; wrapped in up to two pointer/reference declarators for the return type
; (`T* f()`, `T& operator=()`); its name is a plain identifier (a free
; function, or a constructor inside its class), a field_identifier (an
; inline method), a qualified_identifier (an out-of-class definition), a
; destructor_name, an operator_name or a template_function (an explicit
; specialisation). A `= default` / `= delete` definition has no body and no
; body: field, and is not a definition of code (ctags does not tag it either).
(function_definition
  declarator: (function_declarator
    declarator: [(identifier) (field_identifier) (qualified_identifier)
                 (destructor_name) (operator_name) (template_function)] @name)
  body: (_) @body) @definition.function

(function_definition
  declarator: (_
    (function_declarator
      declarator: [(identifier) (field_identifier) (qualified_identifier)
                   (destructor_name) (operator_name) (template_function)] @name))
  body: (_) @body) @definition.function

(function_definition
  declarator: (_
    (_
      (function_declarator
        declarator: [(identifier) (field_identifier) (qualified_identifier)
                     (destructor_name) (operator_name) (template_function)] @name)))
  body: (_) @body) @definition.function

; --- classes, structs, unions, enums (only a body counts) ----------------
; A specialisation `template <> struct Foo<int> {}` names a template_type;
; a nested definition `struct A::B {}` a qualified_identifier.
(class_specifier
  name: [(type_identifier) (qualified_identifier) (template_type)] @name
  body: (field_declaration_list)) @definition.class

(struct_specifier
  name: [(type_identifier) (qualified_identifier) (template_type)] @name
  body: (field_declaration_list)) @definition.struct

(union_specifier
  name: [(type_identifier) (qualified_identifier) (template_type)] @name
  body: (field_declaration_list)) @definition.union

(enum_specifier
  name: [(type_identifier) (qualified_identifier)] @name
  body: (enumerator_list)) @definition.enum

; --- namespaces -------------------------------------------------------------
; `namespace a::b {}` (C++17) is one definition named "a::b" -> "a.b"; an
; anonymous namespace has no name and defines nothing.
(namespace_definition
  name: [(namespace_identifier) (nested_namespace_specifier)] @name) @definition.namespace

; --- typedefs and aliases ---------------------------------------------------
; typedef T Name;   typedef T *Name;   typedef T Name[N];
; typedef R (*Name)(A);   using Name = T;
(type_definition
  declarator: (type_identifier) @name) @definition.typedef

(type_definition
  declarator: (pointer_declarator
    declarator: (type_identifier) @name)) @definition.typedef

(type_definition
  declarator: (pointer_declarator
    declarator: (pointer_declarator
      declarator: (type_identifier) @name))) @definition.typedef

(type_definition
  declarator: (array_declarator
    declarator: (type_identifier) @name)) @definition.typedef

(type_definition
  declarator: (function_declarator
    declarator: (parenthesized_declarator
      (pointer_declarator
        declarator: (type_identifier) @name)))) @definition.typedef

(alias_declaration
  name: (type_identifier) @name) @definition.typedef

; --- macros -----------------------------------------------------------------
; #define NAME ...   (object-like)         #define NAME(args) ...  (function-like)
(preproc_def
  name: (identifier) @name) @definition.macro

(preproc_function_def
  name: (identifier) @name) @definition.macro

; --- calls ------------------------------------------------------------------
; The callee is the @name inside the call: a plain name, a scoped name
; (kept qualified, "ns.util"), the member of `obj.m()` / `p->m()`, or the name
; of an explicitly instantiated template call. Three shapes that look like a
; call are not one: a C++ cast, which is spelled like a template call; a
; function-like macro invocation (FMT_ASSERT(...), an all-capitals name), which
; the preprocessor expands before any call exists; and a type used as a
; function (T(x), Char(0) — a template parameter or class named in capitals).
; A functional cast to a builtin type or a standard integer typedef
; (unsigned(t), short(4), std::size_t(n), LB-F117) names no function either:
; the build's compiler writes a CXXFunctionalCastExpr there, never a call. Nor
; does an operator or a statement keyword the grammar reads as a callee
; (noexcept(...), sizeof(...), decltype(...), an `if (` after a macro the
; grammar cannot place), or a standard function-like macro (assert, offsetof,
; va_arg); the calls written inside them stay calls.
(call_expression
  function: (identifier) @name
  (#not-match? @name "^([A-Z][A-Z0-9_]*|unsigned|signed|short|long|char|int|bool|float|double|wchar_t|char(8|16|32)_t|size_t|ssize_t|ptrdiff_t|u?intptr_t|u?intmax_t|u?int(8|16|32|64)_t|u?int_(least|fast)(8|16|32|64)_t|__u?int128_t|__int128|__float128|nullptr_t|max_align_t|if|while|for|switch|return|catch|throw|sizeof|alignof|alignas|decltype|noexcept|typeid|static_assert|assert|offsetof|va_arg|va_start|va_end|va_copy)$")) @reference.call

; A qualified callee is kept whole ("ns.util"); one written through the
; `template` disambiguator (Factory::template create<T>(x)) is the name after
; it, below.
(call_expression
  function: (qualified_identifier) @name
  (#not-match? @name "(^|::)([A-Z][A-Z0-9_]*|size_t|ptrdiff_t|u?intptr_t|u?intmax_t|u?int(8|16|32|64)_t|u?int_(least|fast)(8|16|32|64)_t|nullptr_t|max_align_t)$")
  (#not-match? @name "::[ \t]*template[ \t]")) @reference.call

(call_expression
  function: (qualified_identifier
    name: (dependent_name
      (template_function
        name: (identifier) @name)))) @reference.call

(call_expression
  function: (field_expression
    field: (dependent_name
      (template_method
        name: (field_identifier) @name)))) @reference.call

(call_expression
  function: (field_expression
    field: (field_identifier) @name)) @reference.call

(call_expression
  function: (field_expression
    field: (template_method
      name: (field_identifier) @name))) @reference.call

(call_expression
  function: (template_function
    name: (identifier) @name)
  (#not-match? @name "^(static_cast|dynamic_cast|reinterpret_cast|const_cast)$")) @reference.call
