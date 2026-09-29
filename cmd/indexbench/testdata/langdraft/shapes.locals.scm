; inherits: c
; Written for the lang-draft tests. Not copied from any locals.scm.

; scopes and references are dropped
(module) @local.scope

(identifier) @local.reference

; typed definitions
(function_definition
  name: (identifier) @local.definition.function) @local.scope

(class_definition
  name: (identifier) @local.definition.type)

(namespace_definition
  name: (identifier) @local.definition.namespace)

; grouped, with a supported predicate and an ignored #set!
((function_definition
  name: (identifier) @local.definition.function) @local.scope
  (#eq? @local.definition.function "main")
  (#set! definition.function.scope "parent"))

; a top-level alternation: the branch holding the definition gets the kind
[
  (macro_definition
    name: (identifier) @local.definition.macro)
  (macro_reference)
]

; untyped definitions take their kind from the outer node's type
(declProc
  name: (identifier) @local.definition)

(declEnumValue
  name: (identifier) @local.definition)

(declQuark
  name: (identifier) @local.definition)

; #lua-match? that maps to RE2, and one that does not
((const_declaration
  name: (identifier) @local.definition.constant)
  (#lua-match? @local.definition.constant "^[%u_][%u%d_]*$"))

((method_definition
  name: (identifier) @local.definition.method)
  (#lua-match? @local.definition.method "%f[%a]get%b()"))

; a predicate the engine does not run is flagged
((enum_declaration
  name: (identifier) @local.definition.enum)
  (#has-ancestor? @local.definition.enum module))

; imports go to the imports draft
(import_statement
  name: (identifier) @local.definition.import) @local.scope

; variables, parameters and fields are dropped
(assignment
  left: (identifier) @local.definition.var)

(parameters
  (identifier) @local.definition.parameter)

(field_declaration
  name: (identifier) @local.definition.field)

; two definitions in one pattern: skipped, with a note
(function_definition
  name: (identifier) @local.definition.function
  parameters: (parameters
    (identifier) @local.definition.method))

; the older locals capture names count the same
(interface_declaration
  name: (identifier) @definition.interface)
