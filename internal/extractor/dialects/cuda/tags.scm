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
; A definition named by a statement keyword is the grammar's recovery from an
; `else if (...) {` that opens a preprocessor branch, or a `switch` after a
; macro written where an access specifier goes (LB-F117), never a function: its
; calls belong to the function around it.
(function_definition
  declarator: (function_declarator
    declarator: [(identifier) (field_identifier) (qualified_identifier)
                 (destructor_name) (operator_name) (template_function)] @name)
  body: (_) @body
  (#not-match? @name "^(if|else|switch|while|for|do|return|case|default|goto|sizeof|try|catch|throw|new|delete)$")) @definition.function

(function_definition
  declarator: (_
    (function_declarator
      declarator: [(identifier) (field_identifier) (qualified_identifier)
                   (destructor_name) (operator_name) (template_function)] @name))
  body: (_) @body
  (#not-match? @name "^(if|else|switch|while|for|do|return|case|default|goto|sizeof|try|catch|throw|new|delete)$")) @definition.function

(function_definition
  declarator: (_
    (_
      (function_declarator
        declarator: [(identifier) (field_identifier) (qualified_identifier)
                     (destructor_name) (operator_name) (template_function)] @name)))
  body: (_) @body
  (#not-match? @name "^(if|else|switch|while|for|do|return|case|default|goto|sizeof|try|catch|throw|new|delete)$")) @definition.function

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

; --- data members, enumerators, namespace-scope variables (LB-F295) --------
; The definitions scip-clang names that L4's def_mapping looks for (the C
; dialect's O367 shapes). A data member is a member of the nearest class,
; struct or union around it (@member.strict, tsextract_members.go), so an
; anonymous union's members are the named class's (scip-clang's
; point#$anonymous_type_N#as_int); a member function's declaration (a
; function_declarator) is none. The declarator shapes are written out (the
; query engine matches fixed depths): a plain, pointer, reference, array,
; bit-field or function-pointer member.
(field_declaration
  declarator: [
    (field_identifier) @name
    (pointer_declarator declarator: (field_identifier) @name)
    (pointer_declarator declarator: (pointer_declarator declarator: (field_identifier) @name))
    (reference_declarator (field_identifier) @name)
    (array_declarator declarator: (field_identifier) @name)
    (array_declarator declarator: (array_declarator declarator: (field_identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (field_identifier) @name))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (field_identifier) @name)))
    (pointer_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (field_identifier) @name))))
  ]) @definition.field @member.strict

; An enumerator is a member of its enum (`shape::kind::circle`, the spelling
; C++11 allows for every enum), or, in an anonymous enum, of the class or
; namespace around it; at file scope it keeps its own name (@member).
(enumerator
  name: (identifier) @name) @definition.enumerator @member

; A namespace-scope variable: a declaration in a namespace body, at file
; scope, in a template or an #if group, that declares no function (a
; prototype's declarator is a function_declarator; a function pointer's is
; parenthesized), extern ones included (scip-clang gives them the Definition
; role); a member of its namespace (@member), dropped inside a function (a
; local). The parent is named so a declaration the grammar recovered inside an
; ERROR node is none; locals and parameters are never captured (LB-F239).
(translation_unit
  (declaration
    declarator: [
    (identifier) @name
    (init_declarator declarator: (identifier) @name)
    (pointer_declarator declarator: (identifier) @name)
    (init_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (reference_declarator (identifier) @name)
    (init_declarator declarator: (reference_declarator (identifier) @name))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

(declaration_list
  (declaration
    declarator: [
    (identifier) @name
    (init_declarator declarator: (identifier) @name)
    (pointer_declarator declarator: (identifier) @name)
    (init_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (reference_declarator (identifier) @name)
    (init_declarator declarator: (reference_declarator (identifier) @name))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

(template_declaration
  (declaration
    declarator: [
    (identifier) @name
    (init_declarator declarator: (identifier) @name)
    (pointer_declarator declarator: (identifier) @name)
    (init_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (reference_declarator (identifier) @name)
    (init_declarator declarator: (reference_declarator (identifier) @name))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

(preproc_if
  (declaration
    declarator: [
    (identifier) @name
    (init_declarator declarator: (identifier) @name)
    (pointer_declarator declarator: (identifier) @name)
    (init_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (reference_declarator (identifier) @name)
    (init_declarator declarator: (reference_declarator (identifier) @name))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

(preproc_ifdef
  (declaration
    declarator: [
    (identifier) @name
    (init_declarator declarator: (identifier) @name)
    (pointer_declarator declarator: (identifier) @name)
    (init_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (reference_declarator (identifier) @name)
    (init_declarator declarator: (reference_declarator (identifier) @name))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

(preproc_else
  (declaration
    declarator: [
    (identifier) @name
    (init_declarator declarator: (identifier) @name)
    (pointer_declarator declarator: (identifier) @name)
    (init_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (reference_declarator (identifier) @name)
    (init_declarator declarator: (reference_declarator (identifier) @name))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

(preproc_elif
  (declaration
    declarator: [
    (identifier) @name
    (init_declarator declarator: (identifier) @name)
    (pointer_declarator declarator: (identifier) @name)
    (init_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (reference_declarator (identifier) @name)
    (init_declarator declarator: (reference_declarator (identifier) @name))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

(preproc_elifdef
  (declaration
    declarator: [
    (identifier) @name
    (init_declarator declarator: (identifier) @name)
    (pointer_declarator declarator: (identifier) @name)
    (init_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (reference_declarator (identifier) @name)
    (init_declarator declarator: (reference_declarator (identifier) @name))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

; --- calls ------------------------------------------------------------------
; The callee is the @name inside the call: a plain name, a scoped name
; (kept qualified, "ns.util"), the member of `obj.m()` / `p->m()`, or the name
; of an explicitly instantiated template call. Three shapes that look like a
; call are not one: a C++ cast, which is spelled like a template call; a
; qualified type used as a function (ns::T(x)); and a template parameter used
; as one (T(0), a functional cast), spelled as one capital letter and an
; optional digit. Unlike C++, a function-like
; macro invocation is a call, in any spelling (checkCudaErrors(...),
; CUDA_CHECK(...)): CUDA takes C's convention (owner ruling C-R3, 2026-10-25),
; the one the oracle scores with ClangMacroCalls on.
; A functional cast to a builtin type or a standard integer typedef
; (unsigned(t), short(4), std::size_t(n), LB-F117) names no function either:
; the build's compiler writes a CXXFunctionalCastExpr there, never a call. Nor
; does an operator or a statement keyword the grammar reads as a callee
; (noexcept(...), sizeof(...), decltype(...), an `if (` after a macro the
; grammar cannot place), or a standard function-like macro (assert, offsetof,
; va_arg); the calls written inside them stay calls.
(call_expression
  function: (identifier) @name
  (#not-match? @name "^([A-Z][0-9]?|unsigned|signed|short|long|char|int|bool|float|double|wchar_t|char(8|16|32)_t|size_t|ssize_t|ptrdiff_t|u?intptr_t|u?intmax_t|u?int(8|16|32|64)_t|u?int_(least|fast)(8|16|32|64)_t|__u?int128_t|__int128|__float128|nullptr_t|max_align_t|if|while|for|switch|return|catch|throw|sizeof|alignof|alignas|decltype|noexcept|typeid|static_assert|assert|offsetof|va_arg|va_start|va_end|va_copy)$")) @reference.call

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

; A member call keeps its receiver (`d.day`, `p->count` as `p.count`, this->m
; as `this.m`; the manifest's calls.qualify, O523): the resolver never binds
; it to a top-level definition by its last segment, and `this.m` binds the
; caller's own class member or nothing.
(call_expression
  function: (field_expression
    argument: (_) @call.receiver
    field: (dependent_name
      (template_method
        name: (field_identifier) @name)))) @reference.call

(call_expression
  function: (field_expression
    argument: (_) @call.receiver
    field: (field_identifier) @name)) @reference.call

(call_expression
  function: (field_expression
    argument: (_) @call.receiver
    field: (template_method
      name: (field_identifier) @name))) @reference.call

(call_expression
  function: (template_function
    name: (identifier) @name)
  (#not-match? @name "^(static_cast|dynamic_cast|reinterpret_cast|const_cast)$")) @reference.call
