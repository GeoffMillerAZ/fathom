# hlsl — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-hlsl` at `0400dd363f4e8142dcfa7b5a7d2d9db59c129c6e` (MIT, the top-level
  `LICENSE`), ABI 14, with a scanner; built into `internal/extractor/tsengine/grammars/hlsl.wasm.gz` (3,052,819 bytes
  raw, 692,820 bytes gzip embedded). Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json`.
- **The pin is an unmerged pull request head**, upstream PR 82 (`refs/pull/82/head`), not a release; v0.2.0 leaves
  11.5% of the calibration files in error (one `[attr]` per function, no stacked attributes). The committed blob is
  what the product runs, so an upstream force-push of the PR branch affects only rebuilds. Retry trigger: repin to the
  release that merges PR 82.
- `tags.scm`, `imports.scm`, `references.scm`: written in this repository (LB-Q-hlsl, docs/design/language-breadth.md
  §13) with `indexbench lang-probe`, from the shapes of `dialects/cpp/` (the grammar is a C++ derivative). The grammar
  ships no `queries/` directory, so nothing is drafted from an upstream file.
- Kinds: `HlslFunc` (a function with a body, attributes included), `HlslStruct` (a `struct` or `class` with a body),
  `HlslCbuffer` (a `cbuffer` or `tbuffer`, canonical kind `struct`), `HlslTypedef`, `HlslVariable` (file scope only:
  resources, samplers, constants, globals) and `HlslMacro` (emitted, unscored: outside the oracle's kind intersection).
- Grammar gaps the queries work around: a `cbuffer Name { }` with no register binding parses as a `function_definition`
  whose type is the keyword (taken as a cbuffer, excluded from the functions); `cbuffer Name : register(b0) { }` parses
  as a `declaration` plus the block as the next sibling statement, so that cbuffer's range is its header line.
  Field semantics parse as a `bitfield_clause`. A file-scope binding macro written with no semicolon
  (`DEFINE_BINDINGS(5, 6, 7)`) is an ERROR node: kajiya has twelve such files.
- `calls`: intrinsics are calls (the compiler has a `CallExpr` for each); vector and matrix constructors, all-capitals
  macro invocations and the call-shaped words of a semantic or attribute (`register`, `numthreads`) are not.
- `imports.scm`: `#include` only; the manifest's `include-path` strategy resolves the quoted form relative to the
  includer, then the source roots.
- Dispatch: `.hlsl`, `.hlsli` and `.fxh` by extension; `.fx` (also a Flux config extension) by the manifest's
  `match.sniff` over the first 16 KiB, which requires a shader token.
