; source: tree-sitter-grammars/tree-sitter-glsl @ e47b8b62b59d0e3529f1c31b03e025d6bd475044 (MIT), with declared source patches;
; function and struct patterns adapted from inherited tree-sitter-c queries/tags.scm
; @ 2a265d69a4caf57108a73ad2ed1e6922dd2f998c (MIT).
; changes: GLSL definitions and calls from the grammar's C-derived syntax.

; A prototype is a declaration, so only a function_definition supplies a
; function symbol. @body makes the signature end before its compound body.
(function_definition
  declarator: (function_declarator
    declarator: (identifier) @name)
  body: (compound_statement) @body) @definition.function

; A named struct requires a body; a mention of a struct type does not define it.
(struct_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition.struct

(preproc_def name: (identifier) @name) @definition.macro
(preproc_function_def name: (identifier) @name) @definition.macro

; GLSL interface blocks are declarations whose first identifier precedes a
; field list. The optional instance after the list is not the block name.
(translation_unit
  (declaration
    (identifier) @name
    (field_declaration_list)) @definition.interface_block)

; Direct translation-unit declarations are globals. Do not mark entire function
; bodies as local scopes: macros inside a body remain visible to the
; preprocessor, and a named local struct is still a real definition. Function
; prototypes have a function_declarator and miss the variable shapes below.

(translation_unit
  (declaration
    declarator: [
      (identifier) @name
      (init_declarator declarator: (identifier) @name)
      (array_declarator declarator: (identifier) @name)
      (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    ]) @definition.variable)

; Conditional branches may nest. Interface blocks are valid only at file scope.
; Branch variables require a GLSL global storage qualifier, which function
; locals cannot use. An unqualified or const-only guarded global is currently
; omitted because its syntax alone cannot distinguish it from a guarded local.
(preproc_if
  (declaration
    (identifier) @name
    (field_declaration_list)) @definition.interface_block)
(preproc_ifdef
  (declaration
    (identifier) @name
    (field_declaration_list)) @definition.interface_block)
(preproc_elif
  (declaration
    (identifier) @name
    (field_declaration_list)) @definition.interface_block)
(preproc_else
  (declaration
    (identifier) @name
    (field_declaration_list)) @definition.interface_block)

(preproc_if
  (declaration
    ["uniform" "in" "out" "shared" "attribute" "varying" "buffer" (extension_storage_class)]
    declarator: [
      (identifier) @name
      (init_declarator declarator: (identifier) @name)
      (array_declarator declarator: (identifier) @name)
      (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    ]) @definition.variable)
(preproc_ifdef
  (declaration
    ["uniform" "in" "out" "shared" "attribute" "varying" "buffer" (extension_storage_class)]
    declarator: [
      (identifier) @name
      (init_declarator declarator: (identifier) @name)
      (array_declarator declarator: (identifier) @name)
      (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    ]) @definition.variable)
(preproc_elif
  (declaration
    ["uniform" "in" "out" "shared" "attribute" "varying" "buffer" (extension_storage_class)]
    declarator: [
      (identifier) @name
      (init_declarator declarator: (identifier) @name)
      (array_declarator declarator: (identifier) @name)
      (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    ]) @definition.variable)
(preproc_else
  (declaration
    ["uniform" "in" "out" "shared" "attribute" "varying" "buffer" (extension_storage_class)]
    declarator: [
      (identifier) @name
      (init_declarator declarator: (identifier) @name)
      (array_declarator declarator: (identifier) @name)
      (init_declarator declarator: (array_declarator declarator: (identifier) @name))
    ]) @definition.variable)

; Calls include builtins, primitive constructors and array constructors.
; A method-style array length call uses the field name as its callee.
(call_expression function: (identifier) @name) @reference.call
(call_expression function: (field_expression field: (field_identifier) @name)) @reference.call
(call_expression function: (array_type_specifier (primitive_type) @name)) @reference.call
(call_expression function: (array_type_specifier (type_identifier) @name)) @reference.call
