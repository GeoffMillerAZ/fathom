# luau — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-luau` 1.2.0 (MIT), commit `a8914d6c1fc5131f8e1c13f769fa704c9f5eb02f`, ABI 14,
  external scanner, grammar at the repository root (`tree_sitter_luau`), built into
  `internal/extractor/tsengine/grammars/luau.wasm.gz` (213,773 bytes raw, 72,266 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`. Licence read at the pinned commit: the top-level `LICENSE` (MIT).
- `tags.scm`: the Lua dialect's (`dialects/lua/tags.scm`, itself tree-sitter-lua's `queries/tags.scm`, MIT), because the
  Luau grammar descends from tree-sitter-lua and keeps its node names (the repository ships no `queries/tags.scm`). Two
  changes: the body is anchored by field (`body: (block)`; `!body "end"` for an empty one), since a Luau parameter list
  can be followed by a return type; and a type alias (`type T = ...`, `export type T<U> = ...`) is a `LuauType`.
- Naming and calls are the Lua dialect's (see its PROVENANCE.md): a function is named by the whole name the declaration
  writes, a call by its last identifier, a call belongs to the innermost definition around it.
- Not extracted, on purpose: variables, table fields that do not hold a function, parameters and locals, the `require`
  graph (an instance path or an alias is no strategy of the closed set; the target is L2). A comment that opens a
  function's body is part of its signature (the Lua dialect trims it).
- Known grammar gap: `export type X = <A, B>(...) -> ...` (an alias of a generic function type) is not parsed into a
  `type_definition` by 1.2.0, so it is not extracted (20 of 452 oracle definitions on Fusion).
- Oracles (calibration/oracles/README.md): L1 against `luau-ast` (Luau's own parser, tier B), L2 against tree-sitter-lua's
  `tags.scm` run on the Luau grammar by py-tree-sitter (tier C, D5).
