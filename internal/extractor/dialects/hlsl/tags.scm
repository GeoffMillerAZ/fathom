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
; Not captured: a function prototype (no body), a cbuffer's constants, a
; struct's fields, and a struct's methods (kinds the oracle's intersection
; leaves out).

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
; a CallExpr for each. A vector or matrix constructor (float4(...)), a
; function-like macro invocation (an all-capitals name) and the keyword-like
; calls of a semantic or attribute (register(b0), [numthreads(8, 8, 1)]) are
; not calls.
(call_expression
  function: (identifier) @name
  (#not-match? @name "^(((u?int|float|half|double|bool|min(10float|12int|16(float|int|uint))|dword)[1-4]?(x[1-4])?)|[A-Z][A-Z0-9_]*|if|while|for|switch|return|sizeof|register|packoffset|numthreads|shader|maxvertexcount|domain|partitioning|outputtopology|outputcontrolpoints|patchconstantfunc|instance|WaveSize|earlydepthstencil|clipplanes)$")) @reference.call

; A method call on a resource or struct (tex.Sample(...), buf.Load(...)) keeps
; its receiver (the manifest's calls.qualify is off: the bare name pairs with
; the compiler's member name).
(call_expression
  function: (field_expression
    argument: (_) @call.receiver
    field: (field_identifier) @name)) @reference.call
