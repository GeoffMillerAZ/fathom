# solc-ast -- tier-A Solidity oracle (LB-Q-solidity, wave lang6)

`solc_ast_dump.js` compiles every `.sol` file under a subject's scope one at a time, with its import closure, through
`solc --standard-json` (the compiler's own AST, `ast` output only, no code generation), and writes the neutral oracle file
(`definitions`, `calls`) that `indexbench lang-oracle -tool-oracle` reads. solc (ethereum/solidity, GPL-3.0) is a bench-only
tool run as a separate process, never a product dependency. Tier A: the compiler's `referencedDeclaration` is the truth.

    # install (toolcache only): the 0.8.37 universal macOS binary, sha256 a27396e7732aa52e80ff89ad7bd8a2e46fec2a6dcc4ef20cd16e5e0c502d6821
    curl -Lo <tools>/solc/solc-0.8.37 https://binaries.soliditylang.org/macosx-amd64/solc-macosx-amd64-v0.8.37+commit.f401782d
    node solc_ast_dump.js --solc <tools>/solc/solc-0.8.37 --root <clone> --scope contracts [--remap prefix=target] [--allow dir] \
        --excluded-out out/oz-ex.json > out/oz.json
    # OpenZeppelin v5.6.1 contracts: 338 files, 3,827 definitions, 2,820 call pairs, 0 excluded
    # foundry-rs/forge-std v1.17.0 src: 32 / 2,288 / 577 / 0; solmate a9e3ea26 src: 41 files, 24 compiled, 17 excluded (O1417)
    # Vectorized/solady v0.1.26 src (holdout): 107 / 3,763 / 1,734 / 0

A definition is a declaration the dialect names, `line` its first line and `end` its last (from the AST's byte range):

- `contract` (also abstract), `interface`, `library`, `struct`, `enum`, `type` (user-defined value type), `event`, `error`,
  `modifier`, `function` (a free function), `method` (a function of a contract, interface or library), `constructor` (the
  `constructor`, `receive` and `fallback` functions, named by their keyword), `variable` (a state variable), `constant` (a
  file-level constant).
- Not a definition: a struct member, an enum value, a parameter, a local, a function-typed variable.

A call pair is `{path, from, to}`, once per (file, from, to):

- a `FunctionCall` of kind `functionCall` (an internal or external call, a library call through `using for`, `super.f()`, an
  `emit` of an event, a `revert` of a custom error, an error raised in `require`) or a `ModifierInvocation`, whose callee
  (`Identifier` or `MemberAccess`, through call options) has a positive `referencedDeclaration` naming a function, modifier,
  event or error declared in the file's closure; `to` is that declaration's name;
- `from` is the nearest enclosing function, modifier or constructor-like definition of the file; a call in a state variable's
  initialiser or outside any of them is skipped.
- Not a call: a type conversion, a struct constructor, a builtin (negative id), a base-constructor specifier, a call through
  a variable (a function pointer, a public state variable's getter).

A file solc rejects (an error-severity diagnostic: a syntax error, an unreadable import, a pragma the one binary does not
satisfy) has no AST: it is counted on stderr, listed in `--excluded-out`, and the harness is run with `-only` over the files
that compiled so the reading names the restriction. Only solc 0.8.37 runs here (O1417).
