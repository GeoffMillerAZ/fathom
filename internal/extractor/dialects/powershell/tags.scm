; dialects/powershell/tags.scm - PowerShell (LB-Q-powershell, design 13.2)
;
; Written in this repository. airbus-cert/tree-sitter-powershell v0.26.5 ships
; no queries/tags.scm of its own (only highlights.scm), and Aider-AI/aider
; carries no powershell-tags.scm (checked 2026-09-28). nvim-treesitter's
; runtime/queries/powershell/locals.scm @ 728e031f (Apache-2.0) was read for
; node shapes only and nothing was copied: its class_statement,
; class_method_definition and function_statement patterns name the same nodes
; this file does, which is the grammar's own vocabulary. Every shape below was
; confirmed with `indexbench lang-probe -dialect powershell <fixture>` (13
; step 12), never from grammar.js/node-types.json. See PROVENANCE.md.
;
; source: (none borrowed)
; changes: n/a - authored here
;
; Signatures. `@body` is the function's (or method's) script_block_body, which
; starts after any param() block, so the signature runs from the definition's
; first token through the whole param block, e.g.
;   function Get-Greeting { [CmdletBinding()] param( [Parameter(Mandatory)] [string]$Name )
; A function with no statements has no script_block_body, so `@body` is
; optional and its signature is the definition's first line.
;
; Names. `@name` is the node's text verbatim. A function named with a scope
; prefix (`function Global:Prompt`) keeps it: the parser hands the whole token
; back and there is no name-transform predicate (design 3.2). For the same reason
; class properties and script/global variables are NOT emitted: a PowerShell
; variable is one token including its `$` sigil, so a property would be named
; `$Name` and never match an oracle's `Name` (PROVENANCE.md, Not emitted).
; Qualification is the engine's (qualifyTSDefs): classes and callables qualify
; (`Animal.Speak`), an enum member keeps its bare name.
;
; Predicates are limited to #not-match? (LB-F2), host-side, on call names.

; --- functions, filters and workflows ---------------------------------------
(function_statement
  (function_name) @name
  (script_block
    (script_block_body) @body)?) @definition.function

; --- classes and enums --------------------------------------------------------
(class_statement
  . (simple_name) @name) @definition.class

(enum_statement
  . (simple_name) @name) @definition.enum

(enum_member
  . (simple_name) @name) @definition.enum_member

; --- class methods and constructors -------------------------------------------
(class_method_definition
  (simple_name) @name
  (script_block
    (script_block_body) @body)?) @definition.method

; --- calls -----------------------------------------------------------------
; A command invocation: `Get-Item -Path x`. The grammar reads a few tokens as
; commands that are not calls, and the callee filter drops exactly those:
; a name that starts with `$` (a variable), an integer range `1..$n` or
; `1..$x.Count` (the whole range comes out as one command name), and the
; statement keywords else/elseif/catch/finally when error recovery or an
; `$x = if (...) { } <newline> else { }` expression leaves them as commands.
(command
  command_name: (command_name) @name
  (#not-match? @name "^(\\$|[0-9]+\\.\\.\\$|(?i:else|elseif|catch|finally)$)")) @reference.call

; `& Get-Item` and `. Import-Helper`: the call or dot-source operator wraps
; the name in a command_name_expr, as a path token (a bare word) or, for a
; path such as `. .\other.ps1`, a command_name. `& $dynamic` names no callee,
; and neither does `& $context.Runner` (a variable before the token: the
; anchors keep the bare-word case only).
(command
  command_name: (command_name_expr
    [(command_name) @name
     (path_command_name
       . (path_command_name_token) @name .)])) @reference.call

; A member invocation in statement or expression position: `$obj.Method($a)`
; and `[Type]::Method($a)`.
(invokation_expression
  (member_name
    (simple_name) @name)) @reference.call

; A member invocation as a command argument (`Format-Type $Value.GetType()`):
; the grammar leaves the argument_list as the member_access expression's
; sibling inside command_elements instead of wrapping both in an
; invokation_expression.
(command_elements
  (array_literal_expression
    (unary_expression
      (member_access
        (member_name
          (simple_name) @name))))
  .
  (argument_list)) @reference.call
