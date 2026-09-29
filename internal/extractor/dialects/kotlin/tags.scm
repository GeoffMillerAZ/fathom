; dialects/kotlin/tags.scm — definitions, calls and packages for the wazero
; tree-sitter engine (internal/extractor.NewTSExtractor("kotlin"), ADR-021 §4,
; LB-Q-kotlin, docs/design/language-breadth.md §13).
;
; Grammar: fwcd/tree-sitter-kotlin at commit 1852ea17 (MIT, untagged since
; 0.3.8; see PROVENANCE.md for why 0.3.8 was not kept). The grammar declares no
; field() names for these nodes, so every pattern locates a name by node kind
; and position (an anonymous keyword token, or the "." anchor for "first
; child") — native tree-sitter query mechanics, no predicates.
;
; The definition shapes follow the grammar's own queries/tags.scm (MIT) for
; classes, objects, functions, properties, enum entries and type aliases, and
; the previous version of this file for the KotlinClass/KotlinInterface/
; KotlinObject/KotlinFunction/KotlinProperty/KotlinImport SourceKinds; what is
; new here is written for this repository: @body signatures, member and
; primary-constructor properties, enum entries, type aliases, secondary
; constructors, the package clause, and @name inside every call.

; --- types ------------------------------------------------------------------
; One class_declaration rule covers a class, an "enum class" and an interface
; (also "fun interface", "sealed interface"); which one it is is only the
; anonymous keyword present ("enum class" holds both "enum" and "class", so it
; is a KotlinClass, as before). Modifiers (data, sealed, value, annotation,
; inner, abstract) are not separate kinds.
(class_declaration
  "interface"
  (type_identifier) @name) @definition.interface

(class_declaration
  "class"
  (type_identifier) @name) @definition.class

; A named object and a NAMED companion object are singletons. An unnamed
; companion has no type_identifier and matches nothing: its members qualify
; against the enclosing class ("Base.create"), the way callers write them.
(object_declaration
  (type_identifier) @name) @definition.object

(companion_object
  (type_identifier) @name) @definition.object

(enum_entry
  (simple_identifier) @name) @definition.enum_entry

(type_alias
  (type_identifier) @name) @definition.typealias

; --- callables ---------------------------------------------------------------
; function_declaration is the one node for top-level, member, extension and
; local functions. The name is the direct simple_identifier child (a receiver
; type and the parameters sit in nodes of their own). function_body starts at
; "{" or "=", so the Signature ends before it; an abstract or interface
; function has none and its Signature is its first line.
(function_declaration
  (simple_identifier) @name
  (function_body)? @body) @definition.function

; A secondary constructor has no name token: the "constructor" keyword is its
; name ("User.constructor"). Its Signature ends before the block's "{".
(secondary_constructor
  "constructor" @name
  "{"? @body) @definition.constructor

; --- properties ----------------------------------------------------------------
; Module-level val/var (a script's too), member val/var of a class, object,
; companion, interface or enum body, and a primary-constructor val/var
; parameter (the one form where a constructor parameter is a property). A local
; variable inside a function body is none of these and is not a definition.
(source_file
  (property_declaration
    (variable_declaration
      .
      (simple_identifier) @name)) @definition.property)

(class_body
  (property_declaration
    (variable_declaration
      .
      (simple_identifier) @name)) @definition.property)

(enum_class_body
  (property_declaration
    (variable_declaration
      .
      (simple_identifier) @name)) @definition.property)

; The "." anchor pins the name to the child right after val/var: a parameter's
; default value can itself be a bare identifier (`val b: Int = a`), which
; would otherwise match as a second property named `a`.
(class_parameter
  (binding_pattern_kind)
  .
  (simple_identifier) @name) @definition.property

; --- package (LB-F5, design §3.5) ------------------------------------------------
; The package clause is file-wide (no @scope range) and answers every
; definition in the file; a file without one (a .kts script) keeps the
; directory rule.
(package_header
  (identifier) @scope.package)

; --- imports -------------------------------------------------------------------
; import_header's dotted path is one "identifier" node whose text already reads
; "pkg.sub.Name"; a wildcard's ".*" is a sibling, so the name is the package.
; See imports.scm for the resolver's vocabulary.
(import_header
  (identifier) @name) @definition.import

; --- calls ---------------------------------------------------------------------
; A plain call f(x), and a call through any receiver a.b.f(x), a?.f(x),
; a!!.f(x), a[0].f(x) or f<T>(x): the callee is the call_expression's first
; child and its name the last identifier of it. A callee that is itself a call
; or a parenthesised expression (f()(x), (g)(x)) has no name and is not
; captured; the inner call is.
(call_expression
  .
  (simple_identifier) @name) @reference.call

(call_expression
  .
  (navigation_expression
    (navigation_suffix
      (simple_identifier) @name))) @reference.call

; @body (LB-F3, design §3.3): a callable's Signature is its definition's text up to
; the start of this capture, whitespace collapsed; without it, the first line.
