# lua — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-lua v0.5.0` (MIT), commit `10fe0054734eec83049514ea2e718b2a56acd0c9`
  (the tag the incumbent CGO binding, `go.mod`'s `tree-sitter-lua v0.5.0`, pinned), ABI 15, external scanner,
  grammar at the repository root (`tree_sitter_lua`), built into
  `internal/extractor/tsengine/grammars/lua.wasm.gz` (173,721 bytes raw, 64,669 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json` (that directory's `PROVENANCE.md` regenerates its own
  tables from it). Licence read at the pinned commit: the top-level `LICENSE.md` (MIT, Munif Tanjim 2021); the
  repository ships no licence file under `queries/` or `src/`.
- `tags.scm`: started from tree-sitter-lua's own `queries/tags.scm` (MIT). Upstream tags `function f()` and
  `function M.f()` (by the last field) as `@definition.function`, a colon method as `@definition.method`, a
  `x = function` assignment and a table constructor holding a function-valued field as `@definition.function`
  (the constructor, not the field), and any call whose callee is a name, a field or a method. This file keeps the
  incumbent `luaextract.go`'s one kind (`LuaFunction`) and its names, and adds what the walker did not do.
- Naming, kept from the incumbent (§11 step 3, port parity): a function declaration is named by the whole name
  the declaration writes, `function M.add()` is `M.add` and `function Class:new()` is `Class:new` (the walker read
  the `name` field's text); a function in a function body keeps its bare name (Lua has no qualifying container).
  Different from it: a declaration nested in a body is a definition (the walker stopped at the first
  `function_declaration` and never entered it); a function assigned to a name is a definition named as written
  (`local f = function`, `f = function`, `M.f = function`), so is a function stored in a table constructor's field
  (`{ handler = function() end }`, named by the key) and in a string-keyed field or slot
  (`{ ["on-close"] = function() end }`, `t["key"] = function() end`, named by the string as written). A field keyed
  by an expression (`[KEY] = function`, `t[i] = function`) has no static name and is not a definition (the grammar
  puts the key expression in the same `name:` field as an identifier key, so the pattern rejects a field whose text
  starts with `[`). Every callable carries a `Signature` (its source from the start of the definition, `local`
  included, up to its body or to the comment that opens it, whitespace collapsed; a body of nothing ends at its
  `end`) and an `EndLine`.
- Not extracted, on purpose: module tables (`local M = {}`), variables and constants, table fields that do not hold a
  function, `local a, b = function..., function...` past the first variable (rare; the anchored pattern reads the
  first pair), parameters and locals. ctags 6.2.1 defines none of the tables and
  variables either.
- Calls: the callee is the called name's last identifier, whatever the call shape: `f()`, `f "s"`, `f {}`,
  `a.b.f()` (`f`), `obj:f()` (`f`), `builder():name("x"):build()` (`builder`, `name`, `build`). A callee that is not a
  name (`t[i]()`, `f()()`'s outer call, `(g or h)()`) has none to record. A call belongs to the innermost definition
  around it: a call inside an anonymous function passed as an argument belongs to the named definition it sits in,
  a call inside a nested named definition to that one, a call at the top of a file to the file's `<module>` symbol (K18, below). The walker recorded the
  callee's whole text (`M.foo`, `self:bar`); the last identifier is the one the definitions are keyed by.
- Oracles (calibration/oracles/README.md): L1 against `luaparse` 0.3.1 (MIT, an independent Lua parser; a syntax
  dump, tier B on the D2 precedent of the JavaParser, Roslyn and PSI dumps), L2 against the grammar's own
  `queries/tags.scm` run by py-tree-sitter (tier C, D5). universal-ctags 6.2.1 is a cross-check, not the oracle:
  it writes no end line for Lua and tags text inside comments and strings (see the reading's notes).
- L3 (wave lang1): `imports.scm` is written for this dialect (Lua 5.4 manual 6.3, `require`). It reads a call of
  `require` with one string literal (`require("a.b")`, `require "a.b"`, `require 'a.b'`, `require [[a.b]]`) and
  `pcall(require, "a.b")`; a concatenation, a variable, an escape sequence and a call by another name name no module
  statically. No alias is bound. The manifest's `file-symbol` strategy reads the dots as directory separators and
  tries `a/b.lua`, then `a/b/init.lua`, under the repository root, `lua/`, `lib/` and `src/`, the layouts of a plain
  Lua project, a LuaRocks or OpenResty library and a Neovim plugin. `module_symbol` mints a `LuaModule` symbol
  (`<module>`, line 1 to the last line) per file (K18), so a `require` at the top of a file, and any call there, is
  made from it. Oracle: `calibration/oracles/lua-require` (tier B, so L3 is at most good): luaparse for the
  statements, Lua 5.4.7's own `package.searchpath` over the same path list for the lookup.
