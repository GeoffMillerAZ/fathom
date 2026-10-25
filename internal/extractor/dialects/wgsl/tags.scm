; source: include-yy/tree-sitter-wgsl @ b58422ee34029f81ae8b7dd44fc14d043809b321 (MIT) ships no tags query
; (queries/ holds highlights.scm only), so this file is written for this repository from the grammar's node
; names (function_decl, struct_decl, global_variable_decl, global_const_decl, global_override_decl,
; type_alias_decl, call_expression), found with lang-probe.

; `fn f(...) -> T { ... }`, with its attributes (`@vertex`, `@compute @workgroup_size(8)`).
(function_decl
  name: (identifier) @name
  body: (compound_statement) @body) @definition.function

; `struct S { ... }`.
(struct_decl
  .
  (identifier) @name) @definition.struct

; `var<private> v: T`, `@group(0) @binding(0) var<uniform> u: T`. A call in a global's initializer
; (`const c = vec3<f32>(1.0)`) is the global's: @calls.from on the definition's own node gives it the call.
(global_variable_decl
  name: (identifier) @name) @definition.variable @calls.from

; `const c = ...` and `override o: T`.
(global_const_decl
  name: (identifier) @name) @definition.constant @calls.from

(global_override_decl
  name: (identifier) @name) @definition.constant @calls.from

; `alias A = vec4<f32>`.
(type_alias_decl
  .
  (identifier) @name) @definition.type_alias

; `f(x)`, `textureSample(t, s, uv)`, a constructor `vec4<f32>(...)` or `array<f32, 4>(...)`: the callee's name,
; without its template arguments.
(call_expression
  function: (template_ident
    name: (identifier) @name)) @reference.call
