# lua-require — tier-B Lua import oracle (LB-Q-lua L3, wave lang1)

`lua_require_dump.js` writes `indexbench lang-oracle -tool-oracle`'s neutral `imports` for a Lua subject, scored as L3
`import_resolution` and, with no SCIP index, as the file-edge truth (design section 1.2, LB-F40, LB-F57). Lua has no
SCIP producer in use here, and a `require` names a module, not a file, so the statement and its file are read by two
independent parts:

- **Statements**: luaparse 0.3.1 (fstirlitz/luaparse, MIT, `luaVersion: LuaJIT`; the parser `calibration/oracles/luaparse`
  already reads) gives every call of the global `require` and every `pcall(require, name)`. A name that is one string
  literal (`"a.b"`, `'a.b'`, `[[a.b]]`) with no escape sequence is static. A concatenation, a variable, an escape
  sequence and a table argument are dynamic: counted, never resolved.
- **Lookup**: Lua 5.4.7's own `package.searchpath(name, path)` (`lua_searchpath.lua`, run by a real Lua from the subject
  root), over a `package.path` that is the operator's choice, stated in the dump's `disclose`: `--roots` lists the
  directories in order (default the root, `lua`, `lib`, `src`), each as `?.lua` then `?/init.lua`. Dots in the name are
  directory separators, as `package.path` reads them. A file outside the subject is not an import.

Tier B, not A: the parser is not Lua's, and the path list is the operator's (section 1.3). Tier B caps L3 at good. The
dialect's strategy (`file-symbol`, `source_roots` `["", "lua", "lib", "src"]`, `suffixes` `[".lua"]`, `index_files`
`["init.lua"]`) and this path list are written to agree, so the oracle shows the strategy implements that lookup over
real subjects, not that no other path list is reasonable. A rock, a C module or a runtime-provided module names no
subject file and is outside both (Kong: 1,196 of 3,580 literal names).

    cd <tools> && curl -sLO https://www.lua.org/ftp/lua-5.4.7.tar.gz   # sha256 9fbf5e28ef86c69858f6d3d34eccc32e911c1a28b4120ff3e84aaa70cfbf1e30
    tar xzf lua-5.4.7.tar.gz && make -C lua-5.4.7 macosx && make -C lua-5.4.7 install INSTALL_TOP=<tools>/lua
    npm install --prefix <tools>/luaparse luaparse@0.3.1
    NODE_PATH=<tools>/luaparse/node_modules node lua_require_dump.js --lua <tools>/lua/bin/lua <clone>/kong > out/lua-kong-imports.json
    go run ./cmd/indexbench lang-oracle -dialect lua -subject <clone>/kong -subject-name Kong/kong -only 'kong/**' \
        -tool-oracle out/lua-kong-defs.json -tool-oracle out/lua-kong-calls.json -tool-oracle out/lua-kong-imports.json -out <reading>

`-only 'kong/**'` keeps Kong's measured files to its Lua sources (the corpus shard's `subpath`) while the subject root stays
the repository's, which a module name like `kong.pdk` needs; each oracle file is filtered to paths under `kong/` first.
The calls oracle is `calibration/oracles/ts-tags` run with `--module-caller '<module>'` (the lua dialect mints a module
symbol, K18).

Measured 2026-10-19: Kong/kong 3.9.2 (`kong/`: 587 files): 3,656 statements in 1,257 files, 3,580 static (6 as
`pcall(require, name)`), 2,384 in the subject, 76 dynamic, of which 1,116 import statements in the 587 measured files.
nvim-telescope/telescope.nvim v0.2.2: 290 statements, 286 static, 237 in the subject, 4 dynamic. Bench-only; never a
product dependency.
