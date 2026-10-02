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
