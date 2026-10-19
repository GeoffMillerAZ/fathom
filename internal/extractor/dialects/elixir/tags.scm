; dialects/elixir/tags.scm — definitions and calls for the wazero tree-sitter engine
; (internal.extractor.NewTSExtractor("elixir"), ADR-021 §4; LB-Q-elixir).
; source: elixir-lang/tree-sitter-elixir v0.3.5 queries/tags.scm @ e2d9e6e0e76b0c436fa48a0b8c32a031d0cbdf49
; (Apache-2.0), extended: the grammar parses every definer as an ordinary call, so each pattern keys on the
; call's target name.

; defmodule Foo.Bar do ... end, defprotocol P do ... end, defimpl P, for: T do ... end
((call
   target: (identifier) @_k
   (arguments . (alias) @name @scope.package)
   (do_block)? @body) @definition.module @scope
 (#eq? @_k "defmodule"))

((call
   target: (identifier) @_k
   (arguments . (alias) @name @scope.package)
   (do_block)? @body) @definition.protocol @scope
 (#eq? @_k "defprotocol"))

((call
   target: (identifier) @_k
   (arguments . (alias) @name)
   (do_block)? @body) @definition.implementation
 (#eq? @_k "defimpl"))

; Functions and macros: def f, def f(a), def f(a) when g(a), with or without a do block. The name is the
; first argument's: a bare identifier, a call, or either of them on the left of `when`.
((call
   target: (identifier) @_k
   (arguments
     .
     [(identifier) @name
      (call target: (identifier) @name)
      (binary_operator left: (call target: (identifier) @name) operator: "when")
      (binary_operator left: (identifier) @name operator: "when")])
   (do_block)? @body) @definition.function
 (#any-of? @_k "def" "defp" "defn" "defnp"))

((call
   target: (identifier) @_k
   (arguments
     .
     [(identifier) @name
      (call target: (identifier) @name)
      (binary_operator left: (call target: (identifier) @name) operator: "when")
      (binary_operator left: (identifier) @name operator: "when")])
   (do_block)? @body) @definition.macro
 (#any-of? @_k "defmacro" "defmacrop"))

((call
   target: (identifier) @_k
   (arguments
     .
     [(identifier) @name
      (call target: (identifier) @name)
      (binary_operator left: (call target: (identifier) @name) operator: "when")
      (binary_operator left: (identifier) @name operator: "when")])
   (do_block)? @body) @definition.guard
 (#any-of? @_k "defguard" "defguardp"))

((call
   target: (identifier) @_k
   (arguments
     .
     [(identifier) @name
      (call target: (identifier) @name)])) @definition.delegate
 (#eq? @_k "defdelegate"))

; @type t :: ..., @callback f(a) :: b
(unary_operator
  operator: "@"
  operand: (call
    target: (identifier) @_k
    (arguments
      .
      (binary_operator
        left: [(identifier) @name (call target: (identifier) @name)]
        operator: "::")))
  (#any-of? @_k "type" "typep" "opaque")) @definition.type

(unary_operator
  operator: "@"
  operand: (call
    target: (identifier) @_k
    (arguments
      .
      (binary_operator
        left: [(identifier) @name (call target: (identifier) @name)]
        operator: "::")))
  (#any-of? @_k "callback" "macrocallback")) @definition.callback

; ExUnit: test "name" do ... end
((call
   target: (identifier) @_k
   (arguments . (string (quoted_content) @name))
   (do_block)? @body) @definition.test
 (#eq? @_k "test"))

; Calls: a local call and a remote call (`Mod.fun(x)`, `var.fun`), whole receiver kept for a name or an
; alias so an alias binds it; a pipe into a bare name. Special forms and the definers above are not calls.
((call
   target: (identifier) @name) @reference.call
 (#not-any-of? @name
   "def" "defp" "defdelegate" "defguard" "defguardp" "defmacro" "defmacrop" "defn" "defnp" "defmodule"
   "defprotocol" "defimpl" "defstruct" "defexception" "defoverridable" "alias" "case" "cond" "else" "for"
   "if" "import" "quote" "raise" "receive" "require" "reraise" "super" "throw" "try" "unless" "unquote"
   "unquote_splicing" "use" "with" "test"
   "moduledoc" "doc" "typedoc" "spec" "type" "typep" "opaque" "callback" "macrocallback" "behaviour" "impl"
   "derive" "enforce_keys" "compile" "dialyzer" "deprecated" "since" "optional_callbacks" "before_compile"
   "after_compile" "on_definition" "on_load" "external_resource"
   "term" "integer" "non_neg_integer" "pos_integer" "neg_integer" "binary" "boolean" "charlist" "iodata"
   "iolist" "no_return" "timeout" "mfa" "nonempty_list" "maybe_improper_list" "keyword" "arity" "number"
   "float" "byte" "char" "as_boolean" "atom" "any" "none" "pid" "port" "reference" "tuple"))

(call
  target: (dot
    left: [(alias) (identifier) (atom)]
    right: (identifier)) @name) @reference.call

(call
  target: (dot
    left: [(call) (dot)]
    right: (identifier) @name)) @reference.call

(binary_operator
  operator: "|>"
  right: (identifier) @name) @reference.call
