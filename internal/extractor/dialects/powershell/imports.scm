; dialects/powershell/imports.scm — dot-sourcing, Import-Module, `using module` and a module
; manifest's RootModule and NestedModules, with a path (LB-Q-powershell L3, wave lf6).
; source: written for this dialect over airbus-cert/tree-sitter-powershell v0.26.5's tree.
;
; - `. path` (dot-sourcing, the "." invocation operator; `&` runs a script in its own scope and
;   imports nothing): the command name is the path.
; - `Import-Module path` / `ipmo path`: each argument that is no parameter and reads as a path (a
;   `/` or `\`, or a .ps1, .psm1, .psd1 or .dll name); a bare module name is PSModulePath's and is
;   left out.
; - `using module path`: the grammar reads it as a command named `using`.
; - RootModule, ModuleToProcess and NestedModules in a manifest's hashtable: each string literal
;   value, alone or in an array (`@('a.psm1', 'b.psm1')` or `'a.psm1', 'b.psm1'`).
;
; A path opening with $PSScriptRoot (bare, `"$PSScriptRoot\x.ps1"` or `${PSScriptRoot}`) names the
; script's own directory: the manifest's dir_vars resolve it there; any other variable, a
; parenthesized expression (Join-Path) or a subexpression names no file statically, and the
; resolver leaves it unresolved. `\` is a path separator (the manifest's separator). The whole
; target file is brought into scope: dot-sourcing runs it in the caller's scope, and a module
; exports its functions.

((command
  (command_invokation_operator) @_op
  command_name: (command_name_expr) @import.module)
 (#eq? @_op ".")
 (#match? @import.module "(?i)([/\\\\]|\\.(ps1|psm1|psd1|dll)[\"']?$)")
 (#not-match? @import.module "^[(@]"))

((command
  command_name: (command_name) @_cmd
  command_elements: (command_elements
    [(generic_token) (array_literal_expression)] @import.module))
 (#match? @_cmd "(?i)^(import-module|ipmo)$")
 (#match? @import.module "(?i)([/\\\\]|\\.(ps1|psm1|psd1|dll)[\"']?$)")
 (#not-match? @import.module "^[(@]"))

((command
  command_name: (command_name) @_cmd
  command_elements: (command_elements
    (generic_token) @_kw
    .
    (command_argument_sep)
    .
    [(generic_token) (array_literal_expression)] @import.module))
 (#match? @_cmd "(?i)^using$")
 (#match? @_kw "(?i)^module$")
 (#match? @import.module "(?i)([/\\\\]|\\.(ps1|psm1|psd1|dll)[\"']?$)"))

; A manifest entry's value is the expression chain the grammar writes for any pipeline: a string
; literal alone, or each string literal of an array, `@('a.psm1', 'b.psm1')` or `'a.psm1', 'b.psm1'`.
((hash_entry
  (key_expression) @_key
    (pipeline (pipeline_chain (logical_expression (bitwise_expression (comparison_expression
    (additive_expression (multiplicative_expression (format_expression (range_expression
    (array_literal_expression (unary_expression (string_literal) @import.module))))))))))))
 (#match? @_key "(?i)^(RootModule|ModuleToProcess|NestedModules)$"))

((hash_entry
  (key_expression) @_key
    (pipeline (pipeline_chain (logical_expression (bitwise_expression (comparison_expression
    (additive_expression (multiplicative_expression (format_expression (range_expression
    (array_literal_expression (unary_expression (array_expression (statement_list (pipeline
    (pipeline_chain (logical_expression (bitwise_expression (comparison_expression
    (additive_expression (multiplicative_expression (format_expression (range_expression
    (array_literal_expression (unary_expression (string_literal)
    @import.module)))))))))))))))))))))))))
 (#match? @_key "(?i)^NestedModules$"))
