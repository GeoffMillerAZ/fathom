# elixir — provenance

- Grammar: `elixir-lang/tree-sitter-elixir v0.3.5` (Apache-2.0), commit `e2d9e6e0e76b0c436fa48a0b8c32a031d0cbdf49`,
  ABI 14, external scanner, grammar at the repository root (`tree_sitter_elixir`), built into
  `internal/extractor/tsengine/grammars/elixir.wasm.gz` (1,365,700 bytes raw, 229,991 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`. Licence read at the pinned commit: the top-level `LICENSE`
  (Apache-2.0) and `NOTICE`; the repository ships no licence file under `queries/` or `src/`.
- `tags.scm`: started from the grammar's own `queries/tags.scm` (Apache-2.0). The grammar parses every definer as an
  ordinary `call`, so each pattern keys on the call's target (`defmodule`, `def`, ...). Upstream tags modules and
  functions; this file adds protocols, implementations, macros, guards, delegates, `@type`/`@callback` declarations
  and ExUnit `test` names, gives every definition a `@body` (a signature runs from the definer to its `do`) and an
  end line, and sets `@scope.package` on `defmodule`/`defprotocol` so a symbol's package is its enclosing module
  (nested modules joined with `.`), which namespace-dir reads as "the file that defines module M".
- Calls: a local call, a remote call (`Mod.fun(x)`, `var.field`, the receiver kept when it is an alias, a name or an
  atom so an alias can bind it) and a pipe into a bare name. Not calls: the definers, the special forms upstream
  lists, module attributes (`@doc`, `@spec`, ...) and the built-in type names (`term`, `integer`, ...), which a
  `@spec` writes as calls.
- `imports.scm`: written for this dialect (Elixir reference, `Kernel.SpecialForms`). `alias`/`require`/`use` are whole
  imports, `import` a wildcard, `alias M, as: N` binds `N`, `alias M.{A, B.C}` is one import per member (two
  `@import.module` captures join with `.`).
- Not extracted, on purpose: `defexception` and `defstruct` (no name of their own; ctags tags the former under the
  enclosing module's name), operator definitions (`def a + b`), `unquote`d names, `@spec`, `defoverridable`,
  `describe`/`setup`/`on_exit`, and any definition the language builds at compile time. An ExUnit `test` is a
  definition (kind `ElixirTest`, canonical `test`) but not a callable: its calls belong to the module around it, and
  its name (a sentence) is outside the harness's last-segment name rule, so it is not scored at L1.
- Known limits: `alias Foo.Bar` binds the full name `Foo.Bar`, not `Bar` (the grammar's alias is one token and a query
  cannot cut it), so only `Foo.Bar.fun()`, `as:` and group members bind a qualified call; `use` is read as an import
  of the module, not of the names its `__using__` macro injects; a call written in a type expression or in a
  definition's own head (`def f(x)` lists `f`) is also a call; module-level `<module>` symbols are not minted
  because `module_symbol` needs a directory or file package and this dialect's package is the module.
- Oracles: `calibration/oracles/elixir-ast` (Elixir's own parser; definitions, tier B; imports, tier B),
  `calibration/oracles/ts-tags` with `elixir-tags.scm` (calls, tier C), universal-ctags 6.2.1 (disclosed deficiency).
