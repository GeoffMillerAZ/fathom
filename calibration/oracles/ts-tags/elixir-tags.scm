; source: elixir-lang/tree-sitter-elixir queries/tags.scm
; @ e2d9e6e0e76b0c436fa48a0b8c32a031d0cbdf49 (v0.3.5, Apache-2.0), verbatim below this
; header. It is the grammar's own upstream tags query, run here by py-tree-sitter
; (the C runtime) as the tier-C L2 call oracle (design §1.5 D5). The elixir dialect's
; tags.scm started from the same file, so the oracle is independent in runtime, not in
; query shape.

; Definitions

; * modules and protocols
(call
  target: (identifier) @ignore
  (arguments (alias) @name)
  (#any-of? @ignore "defmodule" "defprotocol")) @definition.module

; * functions/macros
(call
  target: (identifier) @ignore
  (arguments
    [
      ; zero-arity functions with no parentheses
      (identifier) @name
      ; regular function clause
      (call target: (identifier) @name)
      ; function clause with a guard clause
      (binary_operator
        left: (call target: (identifier) @name)
        operator: "when")
    ])
  (#any-of? @ignore "def" "defp" "defdelegate" "defguard" "defguardp" "defmacro" "defmacrop" "defn" "defnp")) @definition.function

; References

; ignore calls to kernel/special-forms keywords
(call
  target: (identifier) @ignore
  (#any-of? @ignore "def" "defp" "defdelegate" "defguard" "defguardp" "defmacro" "defmacrop" "defn" "defnp" "defmodule" "defprotocol" "defimpl" "defstruct" "defexception" "defoverridable" "alias" "case" "cond" "else" "for" "if" "import" "quote" "raise" "receive" "require" "reraise" "super" "throw" "try" "unless" "unquote" "unquote_splicing" "use" "with"))

; ignore module attributes
(unary_operator
  operator: "@"
  operand: (call
    target: (identifier) @ignore))

; * function call
(call
  target: [
   ; local
   (identifier) @name
   ; remote
   (dot
     right: (identifier) @name)
  ]) @reference.call

; * pipe into function call
(binary_operator
  operator: "|>"
  right: (identifier) @name) @reference.call

; * modules
(alias) @name @reference.module
