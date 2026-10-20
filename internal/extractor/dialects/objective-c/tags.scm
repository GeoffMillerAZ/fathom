; dialects/objective-c/tags.scm — symbol, call and import queries for the wazero
; tree-sitter engine (internal/extractor.NewTSExtractor("objective-c"), ADR-021 §4,
; LB-Q-objective-c).
;
; source: written in this repository against tree-sitter-grammars/tree-sitter-objc
; (MIT) at 181a81b8f23a2d593e7ab4259981f50122909fda. The grammar's own queries/ has
; no tags.scm; the C shapes (functions, structs, unions, enums, typedefs, macros,
; fields, enumerators, globals) follow dialects/c/tags.scm of this repository, whose
; grammar the Objective-C one extends.
;
; A method is named by its selector (`initWithFrame:style:`): the keyword
; identifiers of a method_declaration / method_definition / message_expression are
; separate children split by their arguments, so a selector of n keywords is one
; pattern of n @name.part captures (the engine joins them, each followed by ':',
; tsNameParts) and a zero-argument selector is a plain @name. One pattern per
; arity, 1..12; the engine keeps only the longest match over one node, so a
; pattern for a shorter selector matching a longer selector's leading keywords is
; harmless. Instance methods ("-") are kind `method`, class methods ("+") kind
; `class_method`. A method with no (return type) is not matched.
;
; Declarations (@interface, @protocol) and definitions (@implementation) are both
; symbols: a declaration has no body, so it carries no calls.

; --- classes, categories, extensions, protocols --------------------------
; @interface Foo : Super   and   @interface Foo ()   -> interface Foo (container)
; @interface Foo (Cat)  -> category Cat (container: its methods qualify as Cat.method)
;   and, nested in it as a member named for its owner (Cat.Foo, so no reference to Foo
;   binds to the category's header: the class Foo is defined elsewhere), the interface
;   head Foo, which ctags tags too.
(class_interface !category . (identifier) @name) @definition.interface

(class_interface . (identifier) category: (identifier) @name) @definition.category

(class_interface . (identifier) @name @definition.interface @member.strict category: (identifier))

(class_implementation !category . (identifier) @name) @definition.implementation

(class_implementation . (identifier) category: (identifier) @name) @definition.category

(class_implementation . (identifier) @name @definition.implementation @member.strict category: (identifier))

(protocol_declaration . (identifier) @name) @definition.protocol

; --- methods ----------------------------------------------------------------
; method_declaration -
(method_declaration "-" (method_type) . (identifier) @name .) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

(method_declaration "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.method

; method_declaration +
(method_declaration "+" (method_type) . (identifier) @name .) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

(method_declaration "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter)) @definition.class_method

; method_definition -
(method_definition "-" (method_type) . (identifier) @name . (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

(method_definition "-" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.method

; method_definition +
(method_definition "+" (method_type) . (identifier) @name . (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

(method_definition "+" (method_type) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) . (identifier) @name.part . (method_parameter) (compound_statement) @body) @definition.class_method

; --- properties and instance variables -----------------------------------
(property_declaration
  (struct_declaration
    (struct_declarator
      [
        (identifier) @name
        (pointer_declarator declarator: (identifier) @name)
        (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
        (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name)))
        (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
      ]))) @definition.property @member.strict

(instance_variable
  (struct_declaration
    (struct_declarator
      [
        (identifier) @name
        (pointer_declarator declarator: (identifier) @name)
        (pointer_declarator declarator: (pointer_declarator declarator: (identifier) @name))
        (array_declarator declarator: (identifier) @name)
        (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name)))
        (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
      ]))) @definition.ivar @member.strict

; --- C: functions ----------------------------------------------------------
; As dialects/c/tags.scm: a function_definition, its declarator wrapped in up
; to two pointer_declarators; a name that is a statement keyword is the
; grammar's recovery from a preprocessor branch, never a function.
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

; --- C: struct / union / enum (only a body counts) ---------------------------
(struct_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition.struct

(union_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition.union

(enum_specifier
  name: (type_identifier) @name
  body: (enumerator_list)) @definition.enum

; --- C: typedef (plain, function pointer, block pointer) ---------------------
(type_definition
  declarator: (type_identifier) @name) @definition.typedef

(type_definition
  declarator: (function_declarator
    declarator: (parenthesized_declarator
      [
        (pointer_declarator declarator: (type_identifier) @name)
        (block_pointer_declarator declarator: (type_identifier) @name)
      ]))) @definition.typedef

; --- C: macros -----------------------------------------------------------------
(preproc_def
  name: (identifier) @name) @definition.macro

(preproc_function_def
  name: (identifier) @name) @definition.macro

; --- C: fields, enumerators, global variables -------------------------------
(field_declaration
  declarator: [
    (field_identifier) @name
    (pointer_declarator declarator: (field_identifier) @name)
    (pointer_declarator declarator: (pointer_declarator declarator: (field_identifier) @name))
    (array_declarator declarator: (field_identifier) @name)
    (array_declarator declarator: (array_declarator declarator: (field_identifier) @name))
    (pointer_declarator declarator: (array_declarator declarator: (field_identifier) @name))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (field_identifier) @name)))
    (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (field_identifier) @name)))
    (pointer_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (field_identifier) @name))))
  ]) @definition.field @member.strict

(enumerator
  name: (identifier) @name) @definition.enumerator

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
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
    (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name))))
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
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
    (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name))))
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
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
    (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name))))
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
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
    (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name))))
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
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
    (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name))))
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
    (pointer_declarator declarator: (array_declarator declarator: (identifier) @name))
    (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (pointer_declarator declarator: (identifier) @name))))
    (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name)))
    (init_declarator declarator: (function_declarator declarator: (parenthesized_declarator (block_pointer_declarator declarator: (identifier) @name))))
  ]) @definition.variable @member)

; --- C: calls ------------------------------------------------------------------
; `@protocol(Name)` is an at_expression around a call_expression of the word `protocol`: no call.
(call_expression
  function: (_) @reference.call
  (#not-match? @reference.call "^protocol$"))

; @body (LB-F3): a function's signature runs to the start of this capture.

; --- message sends -----------------------------------------------------------
; [recv sel] and [recv a:x b:y]: the selector joined as in methods above.
; A selector with no argument is the receiver and one method identifier and nothing else: the grammar reads
; `[A b:(T)c]` as a send of b: followed by a second method identifier c, which is no send of c.
(message_expression receiver: (_) . method: (identifier) @name .) @reference.call

(message_expression method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call

(message_expression method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_) . method: (identifier) @name.part . (_)) @reference.call
