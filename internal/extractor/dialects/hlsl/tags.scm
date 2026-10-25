; dialects/hlsl/tags.scm — symbol + call queries for the wazero tree-sitter
; engine (internal/extractor.NewTSExtractor("hlsl"), LB-Q-hlsl).
;
; source: written in this repository against tree-sitter-hlsl (the head of
; upstream PR 82, see PROVENANCE.md) with `indexbench lang-probe`; the shapes
; follow dialects/cpp/tags.scm (the grammar is a C++ derivative). The grammar
; ships no queries/ directory.
;
; An HLSL file is flat: definitions sit at file scope (or under a preprocessor
; conditional), so the queries name the definition nodes and not their parents.
; Not captured: a function prototype (no body), a cbuffer's constants and a
; struct's fields.

; --- functions ----------------------------------------------------------------
; A definition needs a body. The grammar reads `cbuffer Name { ... }` (no
; register binding) as a function_definition whose "return type" is the
; keyword, so the type is excluded here and the cbuffer is taken below.
; hlsl_attribute children ([numthreads(8, 8, 1)], [RootSignature(RS)]) are part of
; the definition node.
(function_definition
  type: (_) @_ret
  declarator: (function_declarator
    declarator: (identifier) @name)
  body: (_) @body
  (#not-match? @_ret "^(cbuffer|tbuffer)$")) @definition.function

; --- methods ---------------------------------------------------------------------
; A struct's or class's method with a body, in the struct (its name a
; field_identifier) or defined out of line (Light::luminance); a prototype in
; the struct is a field_declaration and no definition.
(field_declaration_list
  (function_definition
    declarator: (function_declarator
      declarator: (field_identifier) @name)
    body: (_) @body) @definition.method)

(function_definition
  declarator: (function_declarator
    declarator: (qualified_identifier) @name)
  body: (_) @body) @definition.method

; --- structs and classes (only a body counts) ----------------------------------
(struct_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition.struct

(class_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition.struct

; --- cbuffers (and tbuffers) -----------------------------------------------------
; Without a register binding the grammar reads the block as a function_definition
; whose type is the keyword; with `: register(b0)` it reads a declaration of the
; keyword-typed name and the block as the next sibling statement (the range of
; the definition is then the header line).
(function_definition
  type: (type_identifier) @_kw
  declarator: (identifier) @name
  body: (compound_statement)
  (#match? @_kw "^(cbuffer|tbuffer)$")) @definition.cbuffer

(declaration
  type: (type_identifier) @_kw
  declarator: (identifier) @name
  (#match? @_kw "^(cbuffer|tbuffer)$")) @definition.cbuffer

; After an attribute (`[[vk::binding(5)]] cbuffer Name { ... }`) the grammar reads a
; cbuffer_specifier.
(cbuffer_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition.cbuffer

; --- typedefs ---------------------------------------------------------------------
(type_definition
  declarator: (type_identifier) @name) @definition.typedef

(type_definition
  declarator: (array_declarator
    declarator: (type_identifier) @name)) @definition.typedef

; --- macros (emitted, unscored: outside the oracle's kind intersection) ----------
(preproc_def
  name: (identifier) @name) @definition.macro

(preproc_function_def
  name: (identifier) @name) @definition.macro

; --- file-scope variables ----------------------------------------------------------
; Resources (Texture2D<float4> t : register(t0)), samplers, constants and
; groupshared / static globals. A cbuffer header is also a declaration: its
; keyword type is excluded.
(translation_unit
  (declaration
    type: (_) @_ty
    declarator: [
      (identifier) @name
      (init_declarator declarator: (identifier) @name)
      (array_declarator declarator: (identifier) @name)
      (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    ]
    (#not-match? @_ty "^(cbuffer|tbuffer)$")) @definition.variable)

; Under a preprocessor conditional, at any depth and in any branch (#if,
; #ifdef, #elif, #else). A conditional inside a function body holds locals: the
; body is a @local.scope, and the engine drops a definition inside one, so the
; patterns need not be anchored at the translation unit. A cbuffer the grammar
; reads as a function_definition has its constants in that body too.
(function_definition
  body: (compound_statement) @local.scope)

(preproc_if
    (declaration
      type: (_) @_ty
      declarator: [
        (identifier) @name
        (init_declarator declarator: (identifier) @name)
        (array_declarator declarator: (identifier) @name)
        (init_declarator declarator: (array_declarator declarator: (identifier) @name))
      ]
      (#not-match? @_ty "^(cbuffer|tbuffer)$")) @definition.variable)

(preproc_ifdef
    (declaration
      type: (_) @_ty
      declarator: [
        (identifier) @name
        (init_declarator declarator: (identifier) @name)
        (array_declarator declarator: (identifier) @name)
        (init_declarator declarator: (array_declarator declarator: (identifier) @name))
      ]
      (#not-match? @_ty "^(cbuffer|tbuffer)$")) @definition.variable)

(preproc_elif
    (declaration
      type: (_) @_ty
      declarator: [
        (identifier) @name
        (init_declarator declarator: (identifier) @name)
        (array_declarator declarator: (identifier) @name)
        (init_declarator declarator: (array_declarator declarator: (identifier) @name))
      ]
      (#not-match? @_ty "^(cbuffer|tbuffer)$")) @definition.variable)

(preproc_else
    (declaration
      type: (_) @_ty
      declarator: [
        (identifier) @name
        (init_declarator declarator: (identifier) @name)
        (array_declarator declarator: (identifier) @name)
        (init_declarator declarator: (array_declarator declarator: (identifier) @name))
      ]
      (#not-match? @_ty "^(cbuffer|tbuffer)$")) @definition.variable)

; --- calls ---------------------------------------------------------------------------
; A plain call, intrinsics included (mul, saturate), as the compiler's AST has
; a CallExpr for each. A vector or matrix constructor (float4(...)) and a
; function-like macro invocation (an all-capitals name) are not calls, nor is a
; keyword the grammar reads as a callee in a file it parses in error.
(call_expression
  function: (identifier) @name
  (#not-match? @name "^(((u?int|float|half|double|bool|min(10float|12int|16(float|int|uint))|dword)[1-4]?(x[1-4])?)|[A-Z][A-Z0-9_]*|if|while|for|switch|return|sizeof)$")) @reference.call

; Call-shaped but no call, excluded by where they are written (the engine drops
; a @reference.call over a @call.none node): an attribute ([numthreads(8, 8, 1)],
; [RootSignature(RS)], [domain("tri")], [unroll(4)] before a loop) and a
; register(b0) or packoffset(c0) clause, which the grammar reads as a
; semantics node. A user function named Domain or Instance stays a call.
(hlsl_attribute (call_expression) @call.none)
(semantics (call_expression) @call.none)

; A static member call (Ray::create(o)) is the member's bare name, as the
; compiler's DeclRefExpr names it.
(call_expression
  function: (qualified_identifier
    name: (identifier) @name)) @reference.call

; A method call on a resource or struct (tex.Sample(...), buf.Load(...)) keeps
; its receiver (the manifest's calls.qualify is off: the bare name pairs with
; the compiler's member name).
(call_expression
  function: (field_expression
    argument: (_) @call.receiver
    field: (field_identifier) @name)) @reference.call
