# luaparse_dump.js — tier-B Lua L1 oracle (syntax only)

ADR-048 amendment D2: universal-ctags 6.2.1 is deficient as a Lua L1 oracle. It writes no end line for any Lua
tag (`range_agreement` cannot be measured and L1 caps at good, design 1.2), and it reads Lua by pattern, not by
parse, so it tags text as a function: a variable assigned the result of a call whose argument is a function
(`local ok, err = pcall(function() ... end)`), an immediately invoked function expression
(`local x = (function() ... end)()`), a string or table value on a line that contains the word `function`
(`type = "function"`, `if type(v) ~= "function"` tagged as `~`) — 228 such tags on Kong and telescope.nvim — and code
inside block comments (16). It also
spells a bracketed slot as its key expression (`decoder[TAG.INTEGER] = function` is `INTEGER]`, `["k"] = function`
is `["k"]`). `luaparse` (fstirlitz/luaparse, MIT), a Lua parser library that shares no code with tree-sitter, is the
stronger independent oracle.

The script parses every `*.lua` file under the subject with luaparse (`luaVersion: LuaJIT`, which parses OpenResty
and Neovim code alike) and writes the neutral oracle file (`definitions`) that `indexbench lang-oracle
-tool-oracle` reads, at tier B on the D2 precedent (JavaParser for Java, Roslyn for C#, PSI for Kotlin, php-parser
for PHP): an independent syntactic parser, so L1 can be excellent and L2 at most good. The tier is a claim for
Opus review, not a harness fact. Bench-only: a Node process run beside the harness (npm `--prefix` into the scratch
dir), never a product dependency.

    npm install --prefix <tools>/npm luaparse@0.3.1
    NODE_PATH=<tools>/npm/node_modules node luaparse_dump.js <clone>/kong > out/lua-kong-defs.json
    NODE_PATH=<tools>/npm/node_modules node luaparse_dump.js <clone>/telescope.nvim > out/lua-telescope-defs.json
    # Kong/kong 3.9.2 (kong/): 587 files, 3,324 definitions; nvim-telescope/telescope.nvim v0.2.2: 75 files, 941; 0 parse errors

The dump prints `files`, `parse_errors`, `definitions` and a count per form on stderr. Manifest kind: every
definition is `function` (`dialect.json` `oracle_kinds.tool`).

What it counts, and does not. The parser is independent; the *scope* (which function-valued slots are
definitions) is the dialect author's rule, stated here so a reader can disagree with it:

- A definition is a function stored in a statically named slot. `function f`, `local function f`,
  `function a.b.c`, `function a.b:c` (name: the last identifier; `line` the `function` (or `local`) keyword's line);
  `x = function`, `local x = function`, `a.b = function` (the variable's last identifier); `{ k = function }` (the key);
  `{ ["k"] = function }` and `t["k"] = function` (the string, as written, unquoted). `local a, b = function..., function...`
  pairs each variable with the value at its position. `end` is the function's last line.
- Not a definition: a function in a computed slot (`{ [expr] = function }`, `t[expr] = function`: 27 + 19 on Kong, 4 + 15
  on telescope.nvim), an anonymous function passed as an argument, returned, or invoked in place.
- Names are bare (the last segment, split as the harness's `BareName` splits: after `.`, `:`, `#`, `/`, `\`, space
  or `)`), as the neutral shape asks.
