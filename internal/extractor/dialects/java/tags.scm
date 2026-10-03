; dialects/java/tags.scm — symbol + call queries for the wazero tree-sitter
; engine (internal/extractor.NewTSExtractor("java"), ADR-021 §4, LB-Q-java
; port of the incumbent javaextract.go CGO walker).
;
; source: tree-sitter/tree-sitter-java queries/tags.scm @ v0.23.5 (MIT)
; changes: upstream's five bare patterns tag classes/methods/interfaces and a
; few reference shapes, but have no enum, record, annotation, field or
; import capture, no @body signature capture, and no @name inside
; @reference.call. This file keeps upstream's three reference-shape
; patterns (type_list, object_creation_expression, superclass) close to
; verbatim, then adds every kind the incumbent emitted — class, interface,
; enum, record, method+constructor (one JavaMethod SourceKind, matching the
; incumbent's single qualifyJava(scopeStack, name) treatment of both),
; field, import — with identical SourceKind strings (§11 step 3), plus
; annotation type declarations (new beyond the incumbent, which never
; walked annotation_type_declaration at all — a port may add kinds, never
; drop one, same precedent as LB-Q-c's CUnion/CMacro).
;
; Predicate-free: every pattern relies on structural matching alone.

; --- types ------------------------------------------------------------
(class_declaration
  name: (identifier) @name) @definition.class

(interface_declaration
  name: (identifier) @name) @definition.interface

(enum_declaration
  name: (identifier) @name) @definition.enum

(record_declaration
  name: (identifier) @name) @definition.record

(annotation_type_declaration
  name: (identifier) @name) @definition.annotation

; --- methods and constructors -------------------------------------------
; One SourceKind (JavaMethod) covers both, matching the incumbent's single
; qualifyJava(scopeStack, name) treatment (§11 step 3). An interface or
; abstract method has no body field at all, so @body stays optional (ruby.scm
; precedent, "body: (_)? @body") — its Signature then renders as the
; definition's first line, no truncation point.
(method_declaration
  name: (identifier) @name
  body: (block)? @body) @definition.method

(constructor_declaration
  name: (identifier) @name
  body: (constructor_body) @body) @definition.method

; --- fields ---------------------------------------------------------------
; "int a, b;" is ONE field_declaration with TWO variable_declarator
; "declarator:" fields sharing the same type — this pattern matches once
; per declarator (the query engine's per-field-occurrence semantics), same
; one-symbol-per-declarator output the incumbent got by walking named
; children by kind (ChildByFieldName only ever returns a repeated field's
; first match at the Go API level; the query engine has no such limit).
(field_declaration
  declarator: (variable_declarator
    name: (identifier) @name)) @definition.field

; --- imports ----------------------------------------------------------
; A plain definition listing, matching the incumbent (KindJavaImport, one
; symbol per import statement, static and wildcard included — see
; imports.scm for the generic @import.module/@import.wildcard vocabulary
; internal/fusion's per-file resolver reads separately).
(import_declaration
  (scoped_identifier) @name) @definition.import

; --- package (LB-F5, design §3.5) --------------------------------------
; package_declaration's name child carries no field name and is a bare
; (identifier) for a single segment or a (scoped_identifier) for a dotted
; one — "(_)" matches either. No @scope: a Java package clause is file-wide
; and answers every definition in the file, same precedent design §3.5
; names for Kotlin and PHP.
(package_declaration
  (_) @scope.package)

; --- calls ------------------------------------------------------------
; method_invocation only, matching the incumbent's own CollectCalls(n, src,
; map[string]string{"method_invocation": "name"}): a bare call (max(a, b)),
; a call through a local/field (helper.doThing(x)) and a qualified static
; call (Helper.staticThing()) all reach the callee through the same "name:"
; field regardless of whether "object:" is present.
(method_invocation
  name: (identifier) @name) @reference.call

; A constructor invocation (O268) is a call to its class, by the class's
; simple name: `new Foo(..)`, `new Foo<>(..)`, `new a.b.Foo(..)`,
; `outer.new Inner()` and an anonymous class `new Foo() {..}` all call Foo,
; as scip-java names `Foo#<init>` (internal/langbench javaName) and as
; ctags-style tags name a constructor. The site is the whole expression,
; so an anonymous class's own calls stay with its methods and the
; construction with the method that writes it. Array creation is no call.
(object_creation_expression
  type: (type_identifier) @name) @reference.call

(object_creation_expression
  type: (scoped_type_identifier (type_identifier) @name .)) @reference.call

(object_creation_expression
  type: (generic_type (type_identifier) @name)) @reference.call

(object_creation_expression
  type: (generic_type (scoped_type_identifier (type_identifier) @name .))) @reference.call

; --- reference shapes kept from upstream tags.scm (parsed and counted as
; type_refs, design §3 item 4; not stored until a consumer exists) -------
(type_list
  (type_identifier) @name) @reference.implementation

(object_creation_expression
  type: (type_identifier) @name) @reference.class

(superclass (type_identifier) @name) @reference.class

; @body (LB-F3, design §3.3): a callable's Signature is its definition's
; text up to the start of this capture, whitespace collapsed; without one,
; the definition's first line.
