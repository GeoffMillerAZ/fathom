; dialects/kotlin/tags.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("kotlin"), ADR-021 §4, wave P2).
;
; Predicate-free by design — see python.scm's header comment for why. Unlike
; the JS/Python/Ruby grammars, fwcd/tree-sitter-kotlin's grammar.js uses no
; field() names at all (verified against its own grammar.js), so every
; pattern below locates the name/callee by NODE KIND and structural position
; rather than a field: constraint — including anonymous keyword tokens
; ("class"/"interface"/"object") and the "." anchor to pin a child to a
; specific position (both native tree-sitter query mechanics, not
; application-level predicates).

; class_declaration covers plain classes, "enum class", AND interfaces (one
; grammar rule, distinguished only by which anonymous keyword token is
; present). Two patterns below match the "interface"-keyword and
; "class"-keyword variants separately (mutually exclusive — a given
; class_declaration node contains exactly one of the two tokens); "enum
; class" contains BOTH an "enum" and a "class" token, so it matches the
; "class" pattern (KindKotlinClass) too — enum classes are not distinguished
; from plain classes, matching the task scope (classes/objects/interfaces).
(class_declaration
  "interface"
  (type_identifier) @name) @definition.interface

(class_declaration
  "class"
  (type_identifier) @name) @definition.class

; object_declaration ("object Foo { ... }") and companion_object ("companion
; object Foo { ... }") both introduce a named singleton — same
; KindKotlinObject. An anonymous companion object (no name) has no
; type_identifier child and simply does not match (no symbol emitted, same
; behavior as an unnamed def in any other language here).
(object_declaration
  (type_identifier) @name) @definition.object

(companion_object
  (type_identifier) @name) @definition.object

; function_declaration is the ONE node kind for both top-level functions and
; class/object member methods (fwcd/tree-sitter-kotlin has no separate
; method_declaration) — same precedent as KindPyFunc covering both.
(function_declaration
  (simple_identifier) @name
  (function_body)? @body) @definition.function

; Only a top-level (direct child of source_file) property counts as a
; KotlinProperty, mirroring python.scm's module-level-only KindPyVar
; constraint — a member property inside a class_body is NOT captured here.
; property_declaration wraps the actual name one level deeper inside
; variable_declaration (binding_pattern_kind "val"/"var" is an earlier
; sibling, not captured).
(source_file
  (property_declaration
    (variable_declaration
      (simple_identifier) @name)) @definition.property)

; import_header's dotted path is aliased to a single "identifier" node
; (alias($._import_identifier, $.identifier) in grammar.js) — its own text
; span already renders as "pkg.sub.Name", so no further join is needed.
(import_header
  (identifier) @name) @definition.import

; Naive same-file call collection: the callee expression is call_expression's
; FIRST child (no field name in this grammar — "." anchors the pattern to
; that position so the second child, call_suffix, is never captured). Mirrors
; every other language's naive CollectCalls-equivalent, not a typed/resolved
; edge.
(call_expression
  .
  (_) @reference.call)

; @body (LB-F3, design §3.3): a callable's Signature is its definition's text up to
; the start of this capture, whitespace collapsed; without it, the first line.
