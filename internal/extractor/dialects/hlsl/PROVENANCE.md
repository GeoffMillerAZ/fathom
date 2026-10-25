# hlsl — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-hlsl` at `0400dd363f4e8142dcfa7b5a7d2d9db59c129c6e` (MIT, the top-level
  `LICENSE`), ABI 14, with a scanner; built into `internal/extractor/tsengine/grammars/hlsl.wasm.gz` (3,052,819 bytes
  raw, 692,820 bytes gzip embedded). Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json`.
- **The pin is an unmerged pull request head**, upstream PR 82 (`refs/pull/82/head`), not a release (GitHub archive tarball_sha256
  `0a40e0a1573fb4b6e7433f2be53464f20da6740455d6d5f08316cdd67ccd9dbf`); v0.2.0 with the pre-parse step below leaves 112
  of the 644 calibration files (17.4%) in error (one `[attr]` per function, no stacked attributes). The committed blob is
  what the product runs, so an upstream force-push of the PR branch affects only rebuilds. Retry trigger: repin to the
  release that merges PR 82.
- `tags.scm`, `imports.scm`, `references.scm`: written in this repository (LB-Q-hlsl, docs/design/language-breadth.md
  §13) with `indexbench lang-probe`, from the shapes of `dialects/cpp/` (the grammar is a C++ derivative). The grammar
  ships no `queries/` directory, so nothing is drafted from an upstream file.
- Kinds: `HlslFunc` (a function with a body, attributes included), `HlslMethod` (a struct's or class's method with a
  body, in the struct, named `Struct.method`, or defined out of line, named `Struct::method`; its calls are its own),
  `HlslStruct` (a `struct` or `class` with a body),
  `HlslCbuffer` (a `cbuffer` or `tbuffer`, canonical kind `struct`), `HlslTypedef`, `HlslVariable` (file scope only:
  resources, samplers, constants, globals) and `HlslMacro` (emitted, unscored: outside the oracle's kind intersection).
- Grammar gaps the queries work around: a `cbuffer Name { }` with no register binding parses as a `function_definition`
  whose type is the keyword (taken as a cbuffer, excluded from the functions); after an attribute
  (`[[vk::binding(5)]] cbuffer Name { }`) it parses as a `cbuffer_specifier`. Field semantics parse as a
  `bitfield_clause`.
- Pre-parse step `hlsl-macros` (`internal/extractor/tsextract_hlslmacros.go`, the manifest's `preprocess`), tried only
  when a file does not parse as written, each text read only when it parses clean, every rewrite at equal byte length:
  - (a) the register clause of a `cbuffer` or `tbuffer` header (`cbuffer Frame : register(b0) {`), which the grammar's
    `cbuffer_specifier` (a name, an optional base clause, the body) cannot hold: the parser inserts a MISSING `;`.
    Retired by a tree-sitter-hlsl release whose cbuffer and tbuffer headers take a register clause.
  - (b) a function-like all-capitals macro invocation standing where a declaration starts, alone on its line with no
    `;` (`DEFINE_BINDINGS(5, 6, 7)`, a macro that expands to resource declarations), an ERROR node otherwise. Retired
    by a tree-sitter-hlsl release that reads a file-scope macro invocation with no `;` as a declaration or an
    expression statement.
  - (c) a second candidate, tried after the (a)+(b) text still fails: the conditional directive lines blanked with
    every branch kept (`cppBlankConditionals`, as cpp-macros and objc-macros), for a statement head split across
    `#if`/`#else` (an `if` or `for` head per branch reads as two nested heads; `testdata/split.hlsl`). An expression
    split the same way (an argument or operand written once per branch) reads as both branches in a row and stays in
    error. Retired by a tree-sitter-hlsl release that parses a conditional inside a statement or an expression
    (tree-sitter-cpp does not either, so this one is expected to stay).
- `calls`: intrinsics are calls (the compiler has a `CallExpr` for each); vector and matrix constructors and
  all-capitals macro invocations are not, nor is a call-shaped node by where it is written: an attribute
  (`(hlsl_attribute (call_expression))`: `[numthreads(8, 8, 1)]`, `[RootSignature(RS)]`, `[domain("tri")]`,
  `[unroll(4)]` before a loop) or a `register(b0)` or `packoffset(c0)` clause (`(semantics (call_expression))`),
  captured as `@call.none`, which the engine drops. No attribute word is listed by name, so a user function named
  `Domain`, `Instance` or `Shader` called from a body stays a call. On the calibration corpus this exclusion leaves
  the same call pairs as the word list it replaced (3,696 fathom pairs, 3,572 agreeing with dxc-ast); every word of
  that list had a structural home, so none stays.
- `imports.scm`: `#include` only; the manifest's `include-path` strategy resolves the quoted form relative to the
  includer, then the source roots.
- Dispatch: `.hlsl`, `.hlsli` and `.fxh` by extension; `.fx` (also used by other formats, JavaFX script and Flux
  configuration among them) by the manifest's `match.sniff` over the first 16 KiB, which requires shader structure: a
  `cbuffer` or `tbuffer` block header, an `SV_` semantic after a colon, a `register(b0)`-style binding or a
  `technique` block. A bare type word (`float`), an `#include` line or a struct name does not claim a file: each
  appears in a comment or another format as easily. The sniff claims all 12 of DirectXTK12's `.fx` files, as the
  word list it replaced did.
- File scope: a declaration under a preprocessor conditional is a file-scope variable at any depth and in any branch
  (`#if`, `#ifdef`, `#elif`, `#else`), because a function's body is captured `@local.scope` and the engine drops a
  definition inside one: a conditional inside a body holds locals.
