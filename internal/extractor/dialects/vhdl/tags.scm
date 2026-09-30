; source: jpt13653903/tree-sitter-vhdl queries/Neovim/{folds,context,textobjects}.scm @ a7cf493d07dbcf52ca287596451ae3fb8b12099f (MIT)
; changes: the grammar ships no tags.scm (its queries live under queries/Neovim and queries/Helix, and are
; highlight, fold, indent and text-object queries), so this file is drafted from the node shapes those queries
; name (the `entity:`, `architecture:`, `package:`, `function:`, `procedure:`, `component:` and `type:`
; identifier fields, `label_declaration`, `subprogram_head`) and grown against the grammar's tree. It follows
; universal-ctags' VHDL parser: an entity, an architecture (named by its own identifier, not the entity's),
; a package and a package body, a labelled process, a function or procedure and its prototype, a type, a
; subtype, a record element and a component. `@body` is the node that ends a signature: `subprogram_head`
; (`is`) for a subprogram, `;` for a prototype, the process' declarative part or its `begin` block.

; ---- design units ---------------------------------------------------------------------------------

(entity_declaration
  entity: (_) @name) @definition.entity

(architecture_definition
  architecture: (_) @name) @definition.architecture

(package_declaration
  package: (_) @name) @definition.package

(package_definition
  package: (_) @name) @definition.package_body

; ---- subprograms ----------------------------------------------------------------------------------

; A body: `function f (x : t) return t is ... end function f;`
(subprogram_definition
  (function_specification
    function: (_) @name)
  (subprogram_head) @body) @definition.function

(subprogram_definition
  (procedure_specification
    procedure: (_) @name)
  (subprogram_head) @body) @definition.procedure

; A prototype: `function f (x : t) return t;`
(subprogram_declaration
  (function_specification
    function: (_) @name)
  ";" @body) @definition.function_spec

(subprogram_declaration
  (procedure_specification
    procedure: (_) @name)
  ";" @body) @definition.procedure_spec

; ---- processes ------------------------------------------------------------------------------------

; `label : process (clk) begin ... end process label;`, no declarative part.
(process_statement
  (label_declaration
    (label) @name)
  .
  (sequential_block) @body) @definition.process

(process_statement
  (label_declaration
    (label) @name)
  .
  (sensitivity_specification)
  .
  (sequential_block) @body) @definition.process

; `label : process variable v : t; begin ... end process label;`, a declarative part.
(process_statement
  (label_declaration
    (label) @name)
  (process_head) @body) @definition.process

; ---- types ----------------------------------------------------------------------------------------

(type_declaration
  type: (_) @name) @definition.type

(subtype_declaration
  type: (_) @name) @definition.subtype

(element_declaration
  (identifier_list
    (identifier) @name)) @definition.record_field

(component_declaration
  component: (_) @name) @definition.component

; ---- calls ----------------------------------------------------------------------------------------

; `pulse(rst);` and `pulse;`
(procedure_call_statement
  (name
    [
      (identifier)
      (library_function)
    ] @name
    .
    (parenthesis_group))) @reference.call

(procedure_call_statement
  (name
    .
    [
      (identifier)
      (library_function)
    ] @name
    .)) @reference.call

; `work.util.report_done;` and `work.util.report_done(x);`
(procedure_call_statement
  (name
    (selection
      (identifier) @name)
    .
    (parenthesis_group)?
    .)) @reference.call

; `next_count(value, LIMIT)` in an expression: a function call or an index, which VHDL's syntax cannot tell
; apart, so both are recorded and resolved by name later.
(simple_expression
  (name
    [
      (identifier)
      (library_function)
    ] @name
    .
    (parenthesis_group))) @reference.call

(simple_expression
  (name
    (selection
      (identifier) @name)
    .
    (parenthesis_group))) @reference.call
