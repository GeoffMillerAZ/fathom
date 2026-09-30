; source: briot/tree-sitter-ada queries/locals.scm @ dd5fa4cdb3aba91abc687aa68fb1431396fce6a6 (MIT)
; changes: the grammar ships no tags.scm, so this file is drafted from locals.scm's node shapes (the
; `name:` field of a package and of a procedure or function specification, the leading identifier of a
; type, subtype, exception and task or protected declaration) and grown against the grammar's tree. It
; follows universal-ctags' Ada parser: a specification and a body are two definitions of one entity, so a
; package spec, a subprogram declaration, a task or protected spec and an entry declaration each have a
; `*_spec` kind beside the body's kind; a generic instantiation and a renaming define what they name;
; `@body` is the token that ends a signature (`is` for a body, `;` for a declaration).

; ---- packages -------------------------------------------------------------------------------------

; `package P is ... end P;` (also the package inside a `generic` part).
(package_declaration
  name: (_) @name) @definition.package_spec

; `package body P is ... end P;`
(package_body
  name: (_) @name) @definition.package

; `package IO is new Ada.Text_IO.Integer_IO (Integer);`
(generic_instantiation
  "package"
  name: (_) @name) @definition.package_spec

; `package Short renames Ada.Text_IO;`
(package_renaming_declaration
  name: (_) @name) @definition.package_spec

; ---- subprograms ----------------------------------------------------------------------------------

; A body: `procedure P (X : T) is ... end P;`, `function F return T is ... end F;`.
(subprogram_body
  (procedure_specification
    name: (_) @name)
  "is" @body) @definition.procedure

(subprogram_body
  (function_specification
    name: (_) @name)
  "is" @body) @definition.function

; An expression function: `function F (X : T) return T is (X + 1);`
(expression_function_declaration
  (function_specification
    name: (_) @name)
  "is" @body) @definition.function

; `procedure P (X : T) is null;`
(null_procedure_declaration
  (procedure_specification
    name: (_) @name)
  "is" @body) @definition.procedure

; A declaration: `procedure P (X : T);` (also `is abstract`).
(subprogram_declaration
  (procedure_specification
    name: (_) @name)
  ";" @body) @definition.procedure_spec

(subprogram_declaration
  (function_specification
    name: (_) @name)
  ";" @body) @definition.function_spec

; A generic declaration names its specification: `generic type T is private; procedure P (X : T);`
(generic_subprogram_declaration
  (procedure_specification
    name: (_) @name) @definition.procedure_spec)

(generic_subprogram_declaration
  (function_specification
    name: (_) @name) @definition.function_spec)

; `procedure Sort is new Generic_Sort (Integer);` and `function Sum is new Generic_Sum (Integer);`
(generic_instantiation
  "procedure"
  name: (_) @name
  "is" @body) @definition.procedure

(generic_instantiation
  "function"
  name: (_) @name
  "is" @body) @definition.function

; `procedure Print (X : Integer) renames Put;`
(subprogram_renaming_declaration
  (procedure_specification
    name: (_) @name)
  "renames" @body) @definition.procedure

(subprogram_renaming_declaration
  (function_specification
    name: (_) @name)
  "renames" @body) @definition.function

; ---- types ----------------------------------------------------------------------------------------

(full_type_declaration
  .
  (identifier) @name) @definition.type

(private_type_declaration
  .
  (identifier) @name) @definition.type

(private_extension_declaration
  .
  (identifier) @name) @definition.type

(incomplete_type_declaration
  .
  (identifier) @name) @definition.type

(subtype_declaration
  .
  (identifier) @name) @definition.subtype

; ---- tasks and protected objects ---------------------------------------------------------------

(task_body
  .
  (identifier) @name
  "is" @body) @definition.task

(task_type_declaration
  .
  (identifier) @name) @definition.task_spec

(single_task_declaration
  .
  (identifier) @name) @definition.task_spec

(protected_body
  .
  (identifier) @name) @definition.protected

(protected_type_declaration
  .
  (identifier) @name) @definition.protected_spec

(single_protected_declaration
  .
  (identifier) @name) @definition.protected_spec

; `entry E (X : T);` in a task or protected specification.
(entry_declaration
  entry_name: (_) @name
  ";" @body) @definition.entry_spec

; `entry E (X : T) when Cond is ... end E;` in a protected body.
(entry_body
  .
  (identifier) @name
  "is" @body) @definition.entry

; `accept E (X : T) do ... end E;` in a task body.
(accept_statement
  entry_direct_name: (_) @name
  "do" @body) @definition.entry

; `accept E;`: no body, so the statement's own `;` ends the signature.
(accept_statement
  entry_direct_name: (_) @name
  .
  ";" @body) @definition.entry

(accept_statement
  entry_direct_name: (_) @name
  parameter_profile: (_)
  .
  ";" @body) @definition.entry

; ---- exceptions and constants --------------------------------------------------------------------

(exception_declaration
  (identifier) @name) @definition.exception

(exception_renaming_declaration
  .
  (identifier) @name) @definition.exception

(number_declaration
  (identifier) @name) @definition.constant

(object_declaration
  name: (identifier) @name
  "constant") @definition.constant

; ---- calls ----------------------------------------------------------------------------------------

; `Check (S);`, `Shapes.Check (S);`, `W.Stop;`
(procedure_call_statement
  name: [
    (identifier) @name
    (selected_component
      selector_name: (identifier) @name)
  ]) @reference.call

; `Area (S)` in an expression.
(function_call
  name: [
    (identifier) @name
    (selected_component
      selector_name: (identifier) @name)
  ]) @reference.call
