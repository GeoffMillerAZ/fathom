# Lua oracle candidates — 2026-10-25

**Measured:** the [current reading](../bench/readings/lang/lua.json) remains unchanged:
L2 call precision .9701 / recall .9834 at tier C; L3 resolution and file-edge
precision/recall 1 at tier B, over 1,351 in-repo import statements. The search was
incomplete before review. LuaLS and EmmyLua are viable semantic candidates whose
resolved references remain tier C under owner OL-R2 (ADR-048 unchanged). Native Telescope tracing is feasible but the measured
route covers 81.01%, below the 90% tier-A requirement. Kong tracing is deferred by owner OL-R3 until an
environment installs entirely inside lane roots; Docker is forbidden on this shared machine. These are distinct limits, not proof that a
stronger free oracle is impossible. ADR-048 and LB-Q-lua-rest govern the next work.

## Named candidates checked

| Candidate / route | Version | Concrete result / why it does not lift the current reading |
|---|---|---|
| Universal-ctags | 6.2.1 | Existing unscored cross-check has no Lua end lines; the reading names its unmatched-definition classes. It does not resolve calls. |
| luaparse | 0.3.1, LuaJIT mode | Existing independent definition parser; a syntax call dump is at most tier B and cannot lift L2. Retained as a literal-import scanner. |
| Upstream tree-sitter query | tree-sitter-lua v0.5.0, `10fe0054` (grammars.lock.json) | Existing L2 truth is tier C; names by syntax do not become native semantic truth. |
| lua-require static lookup | Lua 5.4.7 + luaparse 0.3.1 | Operator-selected package.path agrees with fathom's configured lookup; it is not the subjects' actual runtime. Remains tier B. |
| LuaLS doc/check/LSP | 3.19.2, source tag commit `b144c872` | `--doc` exports globals/documentation; module-local probe emits one config record and no definitions/fields/calls. `--check` emits diagnostics. LSP definition/reference is a viable resolver, but a third-party resolver is not automatically tier A under ADR-048. OL-R2 classifies its references C; no L2 lift is allowed. |
| EmmyLua analyzer / check / doc CLI / LSP | 0.25.1, release commit `6f8b424` | MIT analyzer provides another LSP resolver. `emmylua_check --output-format json` emits file diagnostics (unused local in the committed probe), not call pairs; doc CLI exports documentation/AST, not a measured complete call graph. OL-R2 keeps its references C, so it cannot lift L2; it remains a viable cross-check. |
| luacheck | 1.2.0 | MIT linter's pinned CLI contract emits warning reports and formatter output; no resolved-call/reference or native import-trace export. It cannot attest the target population. |
| Native Neovim + Telescope tests | Neovim 0.12.6; Telescope v0.2.2 at `5255aa27`; Plenary `74b06c6c` | Original Plenary test entrypoint exits 0. Its executed literal in-repo imports are 85/237 (35.86%); adding a separate native process loading every original Lua file raises this to 192/237 (81.01%). Below 90%, so no tier-A file is emitted. |
| Kong resty/busted native trace | Kong 3.9.2 at `3666d50b`; Docker server 27.4.0 | Pinned developer guide requires Kong's patched OpenResty, dependencies and native test setup. Local full Xcode is absent (CLT-only, explicitly unsupported by that guide). Docker is reachable, but image/container artifacts need an exception to the lane's workspace/cache-only rule. OL-R3 denies Docker and keeps artifacts inside lane roots; native execution and coverage remain unmeasured until an allowed environment is available. No impossibility claimed. |
| Maintained SCIP/LSIF producer search | SCIP producer registry `d556dd3c` inspected 2026-10-25; LuaLS 3.19.2 and EmmyLua 0.25.1 release interfaces | None of these checked interfaces names a Lua SCIP producer. This scoped negative search does not establish global absence. |

**Measured, source inspection:** LuaLS's
[provider](https://github.com/LuaLS/lua-language-server/blob/b144c872/script/provider/provider.lua),
[reference resolver](https://github.com/LuaLS/lua-language-server/blob/b144c872/script/core/reference.lua)
and [doc exporter](https://github.com/LuaLS/lua-language-server/blob/b144c872/script/cli/doc/export.lua)
are cited at the probed release commit, not the subsequent master commit.
**Claimed by a vendor:** EmmyLua's [release](https://github.com/EmmyLuaLs/emmylua-analyzer-rust/releases/tag/0.25.1)
and [crate interfaces](https://github.com/EmmyLuaLs/emmylua-analyzer-rust/tree/0.25.1/crates)
name the analyzer, checker and documentation commands;
[luacheck's pinned CLI](https://github.com/lunarmodules/luacheck/blob/v1.2.0/docsrc/cli.rst)
describes diagnostic reports, not semantic reference export.

## Reproducible contract probes

**Measured:** official `lua-language-server-3.19.2-darwin-arm64.tar.gz` has SHA256
`79149584320a23bd5fcd5bc31f47a1975acd8a4560a0e89036bb1016fa61687f`.
`emmylua_check-darwin-arm64.tar.gz` 0.25.1 has SHA256
`eeaad59d173cb8d7cb0fe778936d7b584fe114a3eb46587ef3e4aec24b605af4`.
These platform-specific assets were installed under workspace, not globally.
The [probe module](data/lua-oracle-2026-10-25/probe.lua),
[runner](data/lua-oracle-2026-10-25/probe.py),
[LuaLS output](data/lua-oracle-2026-10-25/luals-doc.json),
[EmmyLua output](data/lua-oracle-2026-10-25/emmy-diagnostics.json) and
[summary](data/lua-oracle-2026-10-25/probe-summary.json) are committed.
Only absolute probe paths are replaced with `<probe>` in these published outputs.
The probe tests command contracts, not corpus accuracy or LSP coverage.

```sh
nice -n 10 python3 docs/assessments/data/lua-oracle-2026-10-25/probe.py \
  --luals workspace/tools/luals-3.19.2/bin/lua-language-server \
  --emmy workspace/tools/emmylua-0.25.1/emmylua_check \
  --out workspace/tmp/lua-reviewed-probe-replay
```

## Native trace feasibility

**Measured:** [the committed runner](data/lua-oracle-2026-10-25/trace_telescope.py)
wraps native `require` and loader functions, preserving their lookup and values.
It sources Telescope's unchanged `scripts/minimal_init.vim`, runs its Plenary
entrypoint sequentially, then loads all 75 original Lua files in separate native
processes. No globals or APIs are stubbed; failing loads are counted. XDG editor
state and logs go under the requested workspace output directory. Neovim's native
loaders resolve the independently scanned full literal population, including
statements never executed. The [native population](data/lua-oracle-2026-10-25/telescope-native-population.json)
and [trace summary](data/lua-oracle-2026-10-25/telescope-trace-summary.json) preserve
237 denominator sites, all observed sites and every file-load exit. The existing
reading's 235 import statements reflects its own extracted/matched population;
this feasibility denominator does not drop the other two native sites.

```sh
npm install --prefix workspace/tools/luaparse --ignore-scripts --no-audit --no-fund luaparse@0.3.1
# Telescope at 5255aa27 and sibling plenary.nvim at 74b06c6c; local Neovim 0.12.6:
nice -n 10 python3 docs/assessments/data/lua-oracle-2026-10-25/trace_telescope.py \
  --nvim workspace/tools/nvim-macos-arm64/bin/nvim \
  --subject workspace/calibration/telescope.nvim \
  --luaparse workspace/tools/luaparse/node_modules \
  --out workspace/tmp/lua-telescope-trace-replay
```

**Measured, source inspection:** 45 literal in-repo sites remain unexecuted;
several are inside UI/action functions. The subject itself skips its two picker
integration suites on macOS; no platform guard was removed. A successful process
exit does not imply full import coverage. The measured recipe is insufficient for
tier A under design section 1.3; the 90% bar remains unchanged. No holdout enters
the canonical corpus shard. Kong's separate native experiment is deferred by OL-R3,
under its [pinned setup](https://github.com/Kong/kong/blob/3666d50b/DEVELOPER.md).

## Independent retry conditions

**Measured, owner rulings:** OL-R2 retains tier C for LuaLS 3.19.2 and EmmyLua
0.25.1 under ADR-048, without amendment. Their resolving-reference adapters cannot
lift L2. A future maintained Lua SCIP producer would be a separate candidate,
requiring a complete corpus comparison; none is claimed available here.

**Measured, owner ruling:** OL-R3 keeps every runtime artifact inside lane roots
and forbids Docker on this shared machine. Telescope's 81.01% trace stands; Kong
can join later in an environment that installs inside those roots. No Docker
image was pulled and no container was created.

**Hypothesis:** independently, an actual subject-runtime require trace reaching
90% of in-repo literal statements can lift that subject's L3 oracle tier, after a
new reading. Telescope's current 81.01% does not meet it; Kong is unmeasured. The
L2 and L3 retry conditions are disjoint. No existing numbers or grades were raised.
