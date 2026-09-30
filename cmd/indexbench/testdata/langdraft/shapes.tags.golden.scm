; source: nvim-treesitter/nvim-treesitter runtime/queries/fixture/locals.scm @ 0123456789abcdef0123456789abcdef01234567 (Apache-2.0)
; changes: drafted by indexbench lang-draft (locals2tags): @local.definition -> @name, outer node -> @definition.<kind>; scopes, references and parameters dropped; a draft to tune, never an L1 oracle (ADR-048 amendment)
; inherits: c (not followed; run lang-draft on that locals.scm and merge by hand)

; from locals L10: @local.definition.function
(function_definition
  name: (identifier) @name) @definition.function

; from locals L13: @local.definition.type
(class_definition
  name: (identifier) @name) @definition.type

; from locals L16: @local.definition.namespace
(namespace_definition
  name: (identifier) @name) @definition.module

; from locals L20: @local.definition.function
((function_definition
  name: (identifier) @name) @definition.function
  (#eq? @name "main")
  (#set! definition.function.scope "parent"))

; from locals L26: @local.definition.macro
[
  (macro_definition
    name: (identifier) @name) @definition.macro
  (macro_reference)
]

; from locals L33: @local.definition (untyped; kind function from declProc)
(declProc
  name: (identifier) @name) @definition.function

; from locals L36: @local.definition (untyped; kind enum_member from declEnumValue)
(declEnumValue
  name: (identifier) @name) @definition.enum_member

; SKIPPED L39: untyped definition: no kind for node type declQuark

; from locals L43: @local.definition.constant
((const_declaration
  name: (identifier) @name) @definition.constant
  (#match? @name "^[A-Z_][A-Z0-9_]*$"))

; from locals L47: @local.definition.method
; UNSUPPORTED predicate #lua-match?: rewrite before use
((method_definition
  name: (identifier) @name) @definition.method
  (#lua-match? @name "%f[%a]get%b()"))

; from locals L52: @local.definition.enum
; UNSUPPORTED predicate #has-ancestor?: rewrite before use
((enum_declaration
  name: (identifier) @name) @definition.enum
  (#has-ancestor? @name module))

; SKIPPED L71: several definitions in one pattern

; from locals L77: @definition.interface
(interface_declaration
  name: (identifier) @name) @definition.interface
