# Lua oracle ceiling — 2026-10-25

The present Lua recipe has a documented oracle ceiling. LuaLS is a credible next
instrument, but its documentation export is not a call-graph oracle. **Measured:**
the committed [reading](../bench/readings/lang/lua.json) reports call precision .9701
and recall .9834 against the upstream query, tier C; its 1,351 in-repo import statements
have resolution, file-edge precision and file-edge recall all 1 at tier B. These are
existing numbers, not fresh measurements. [LB-Q-lua-rest](../plans/language-breadth.md)
records the limit under ADR-048 and design section 1.3. No reading or grade changed.

## Existing work checked

- **Measured, source inspection:** the [SCIP producer registry](https://github.com/scip-code/scip#tools-using-scip)
  inspected on this date lists no Lua indexer. That registry is not an exhaustive proof
  that no Lua indexer exists.
- **Measured, source inspection:** [luaparse](../../calibration/oracles/luaparse/README.md)
  already supplies the independent L1 parser. Reading its syntax for calls would at
  most supply tier B; it cannot lift L2 to excellent. The upstream tree-sitter query
  already supplies the committed tier-C calls population.
- **Measured, source inspection:** [lua-require](../../calibration/oracles/lua-require/README.md)
  invokes Lua 5.4.7's own package.searchpath, but supplies the path list. It does not
  observe the subjects' LuaJIT/OpenResty or Neovim execution. Relabelling this as native
  tier A would conceal the operator-selected lookup roots.
- **Measured, source inspection:** [LuaLS](https://github.com/LuaLS/lua-language-server)
  is MIT-licensed. At source revision `a58fc7b01e1746c46275c71b0e61bfed40a6296d`,
  `script/provider/provider.lua` exposes definition and reference LSP requests;
  `script/core/reference.lua` uses `vm.getRefs`. Its `--check` path exports diagnostics;
  `--doc` starts from exportable globals (`script/cli/doc/export.lua`). These are
  distinct from a complete, source-attributed inventory of written calls.

## Offline export probe

**Measured:** the standalone official LuaLS 3.19.2 distribution was installed only
under gitignored workspace. Archive SHA256:
`79149584320a23bd5fcd5bc31f47a1975acd8a4560a0e89036bb1016fa61687f`.
Its `--doc` command ran at niceness 10 with one-minute load below 6, with logs,
metadata and export paths under workspace. The probe module declared a local helper,
a returned table's `run` method calling it, and an unused local function also calling
it. The export exited 0; doc.json contained one configuration record, with empty
`defines` and `fields`, and no calls or references. This is an instrument-contract
probe, not a real-subject calibration or a test of LuaLS's LSP resolver accuracy.

## Next trigger

**Hypothesis:** a bounded offline LSP client can obtain resolved references from
LuaLS and convert them to the neutral oracle format, independently of fathom. That
requires a new adapter and a real-corpus reading, with external and unresolved sites
counted rather than discarded. The current export probe does not establish its tier,
coverage, call precision or call recall. A runtime require trace must additionally use
the subjects' own environment and meet design section 1.3's coverage rule before it
can lift the imports tier. LB-Q-lua-rest records those retry conditions without
claiming that a stronger free oracle is impossible.
