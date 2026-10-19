# luau-ast — tier-B Luau L1 oracle (LB-Q-luau, wave lang1)

`luau_ast_dump.py` parses every `*.luau` file under a subject with `luau-ast`, the AST printer of Luau's own toolchain
(luau-lang/luau, MIT; the parser Roblox runs), and writes the neutral oracle file (`definitions`) that
`indexbench lang-oracle -tool-oracle` reads. The queue's oracle for Luau is a tree-sitter-tags query (tier C), but the
grammar ships no `tags.scm` and its `locals.scm` defines `function` statements only (sym_precision .86 against it on the
two subjects, every assigned and table-field function dialect-only), so the stronger independent parser is used: Luau's
own, which shares no code with tree-sitter. Tier B, the D2 precedent (luaparse for Lua, JavaParser for Java). The tier is a
claim for Opus review, not a harness fact. Bench-only, never a product dependency.

    cmake -S <luau checkout at tag 0.743> -B <tools>/luau-ast/build -G Ninja -DCMAKE_BUILD_TYPE=Release \
        -DLUAU_BUILD_TESTS=OFF -DLUAU_BUILD_WEB=OFF && cmake --build <tools>/luau-ast/build --target Luau.Ast.CLI -j 4
    python3 -I luau_ast_dump.py --luau-ast <tools>/luau-ast/build/luau-ast <clone> > out/luau-<subject>-defs.json
    # luau-lang/luau 0.743: 440 files, 4 rejected, 1,873 definitions; dphfox/Fusion v0.3-beta: 113 files, 0 rejected, 452

A definition is a function stored in a statically named slot or a type alias; the scope is the dialect author's rule,
stated here so a reader can disagree with it:

- `function f`, `local function f`, `function a.b.c`, `function a.b:c` (name: the last identifier; `line` the statement's
  first line, `local` included), `x = function`, `local x = function`, `a.b = function`, `t["k"] = function` (the
  variable's last identifier, the string unquoted), `{ k = function }` and `{ ["k"] = function }` (the key), and
  `local a, b = function..., function...` (each variable with the value at its position). `end` is the function's last line.
- `type T = ...`, `export type T<U> = ...`: kind `type` (canonical `type_alias`), `line` the statement's first line.
- Not a definition: a function in a computed slot (`{ [expr] = function }`, `t[expr] = function`), an anonymous function
  passed as an argument, returned or invoked in place.
- A file luau-ast rejects is counted on stderr and skipped (invalid UTF-8 in a string constant is replaced, not rejected).

The L2 calls oracle is `calibration/oracles/ts-tags` with `--lang luau --query lua-tags.scm --ext .luau`.
