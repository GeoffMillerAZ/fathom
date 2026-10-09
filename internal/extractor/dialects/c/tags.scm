; dialects/c/tags.scm — symbol + call queries for the wazero tree-sitter
; engine (internal/extractor.NewTSExtractor("c"), ADR-021 §4, LB-Q-c port of
; the incumbent cextract.go CGO walker).
;
; source: tree-sitter/tree-sitter-c queries/tags.scm @ v0.24.2 (MIT)
; changes: the upstream file has only five bare patterns (struct/union as
; "class", enum/typedef as "type", one function_declarator shape with no
; body check) and no macro or call captures at all. This file instead
; mirrors the incumbent extractor's own semantics (only a struct/union/enum
; WITH a body counts, keeping forward declarations out) with dedicated
; canonical kinds for struct/union/enum/typedef, adds #define macro
; definitions (object-like and function-like), and adds @reference.call for
; the same-file naive call collection the incumbent's CollectCalls did.
;
; One predicate (LB-F117): a function definition named by a statement keyword
; is the grammar's recovery from a preprocessor branch that opens with `else
; if (...) {` (curl's `#ifdef USE_CURL_ASYNC` chains), never a function.

; --- functions --------------------------------------------------------
; function_definition's declarator is a function_declarator, optionally
; wrapped in 1-2 pointer_declarators for a pointer return type (cDeclName's
; incumbent walk handled arbitrary depth in Go; the query engine only
; matches fixed depths, so up to two pointers are covered here — a third
; level is vanishingly rare in real C and left unmatched, same tradeoff
; LB-F1's migrated dialects already make for other unusual shapes).
(function_definition
  declarator: (function_declarator
    declarator: (identifier) @name)
  body: (compound_statement) @body
  (#not-match? @name "^(if|else|switch|while|for|do|return|case|default|goto|sizeof)$")) @definition.function

(function_definition
  declarator: (pointer_declarator
    declarator: (function_declarator
      declarator: (identifier) @name))
  body: (compound_statement) @body
  (#not-match? @name "^(if|else|switch|while|for|do|return|case|default|goto|sizeof)$")) @definition.function

(function_definition
  declarator: (pointer_declarator
    declarator: (pointer_declarator
      declarator: (function_declarator
        declarator: (identifier) @name)))
  body: (compound_statement) @body
  (#not-match? @name "^(if|else|switch|while|for|do|return|case|default|goto|sizeof)$")) @definition.function

; --- struct / union / enum (only a body counts — no forward declarations,
; matching the incumbent's ChildByFieldName("body") == nil skip) ----------
(struct_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition.struct

(union_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition.union

(enum_specifier
  name: (type_identifier) @name
  body: (enumerator_list)) @definition.enum

; --- typedef ------------------------------------------------------------
; The simple shape: typedef <type> Name;  (declarator is a bare
; type_identifier — "typedef struct Point PointAlias;", "typedef int
; MyInt;"). Function-pointer typedefs ("typedef int (*Callback)(int);") put
; the whole thing as a function_declarator (the return type is the
; type_definition's own "type" field, not part of the declarator chain);
; the alias sits inside its parenthesized "(*Callback)" declarator.
(type_definition
  declarator: (type_identifier) @name) @definition.typedef

(type_definition
  declarator: (function_declarator
    declarator: (parenthesized_declarator
      (pointer_declarator
        declarator: (type_identifier) @name)))) @definition.typedef

; --- macros ---------------------------------------------------------------
; #define NAME ...   (object-like)         #define NAME(args) ...  (function-like)
(preproc_def
  name: (identifier) @name) @definition.macro

(preproc_function_def
  name: (identifier) @name) @definition.macro

;
; --- fields, enumerators, global variables (O367, LB-F291) ----------------
; The definitions scip-clang names that L4's def_mapping looks for. A field
; is a member of the nearest struct, union or typedef around it (@member.strict,
; tsextract_members.go: those kinds are containers), so `struct node { int
; len; }` defines node.len, a field of an anonymous member union is the named
; struct's (scip-clang's node#$anonymous_type_N#as_int), and one of an
; anonymous typedef'd struct is the typedef's (span_t.tv_sec); a field with
; no named owner (`static struct { int a; } cfg;`) is dropped. The declarator
; shapes are written out (the query engine matches fixed depths): a plain,
; pointer, array, bit-field or function-pointer field.
(field_declaration
  declarator: [
    (field_identifier) @name
    (pointer_declarator declarator: (field_identifier) @name)
    (pointer_declarator declarator: (pointer_declarator declarator: (field_identifier) @name))
    (array_declarator declarator: (field_identifier) @name)
    (array_declarator declarator: (array_declarator declarator: (field_identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (field_identifier) @name))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (field_identifier) @name)))
    (pointer_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (field_identifier) @name))))
  ]) @definition.field @member.strict

; An enumerator is a name of the scope around its enum (C has no enum scope):
; bare, as scip-clang writes it (CURLMSG_NONE, never CURLMSG.CURLMSG_NONE).
(enumerator
  name: (identifier) @name) @definition.enumerator

; A global variable: a declaration at file scope, also inside an #if group,
; that declares no function (a prototype's declarator is a function_declarator;
; a function pointer's is parenthesized). An extern declaration counts:
; scip-clang gives it the Definition role. The parent is named so a
; declaration the grammar recovered from (inside an ERROR node, a function
; whose signature a macro hid) is no global; @member drops a declaration in
; an #if group inside a function (a local), and locals and parameters are
; never captured (LB-F239).
(translation_unit
  (declaration
    declarator: [
    (identifier) @name
    (init_declarator declarator: (identifier) @name)
    (pointer_declarator declarator: (identifier) @name)
    (init_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name)))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (array_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (array_declarator declarator: (array_declarator declarator: (identifier) @name)))
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
    (init_declarator declarator: (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name)))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (array_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (array_declarator declarator: (array_declarator declarator: (identifier) @name)))
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
    (init_declarator declarator: (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name)))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (array_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (array_declarator declarator: (array_declarator declarator: (identifier) @name)))
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
    (init_declarator declarator: (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name)))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (array_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (array_declarator declarator: (array_declarator declarator: (identifier) @name)))
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
    (init_declarator declarator: (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name)))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (array_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (array_declarator declarator: (array_declarator declarator: (identifier) @name)))
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
    (init_declarator declarator: (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name)))
    (array_declarator declarator: (identifier) @name)
    (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    (array_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (array_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (init_declarator declarator: (pointer_declarator declarator: (array_declarator declarator: (identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

; --- calls ---------------------------------------------------------------
; Naive same-file call collection (whatever expression is being called, as
; source text) — mirrors the incumbent's treesitter.CollectCalls convention
; used by every other tags dialect (javascript.scm, python.scm): no @name
; sub-capture, so the engine falls back to the callee expression's whole
; text (LB-F4). Covers a plain call (helper(a)), a function-pointer call
; through a local (cb(v)) and a call through a struct pointer field
; (ctx->handler(ctx)), all via the same call_expression shape.
(call_expression
  function: (_) @reference.call)

; @body (LB-F3, design §3.3): a function's Signature is its definition's
; text up to the start of this capture, whitespace collapsed.
