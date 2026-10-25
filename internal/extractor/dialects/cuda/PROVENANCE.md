# cuda — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-cuda v0.21.2` (MIT), commit `d58080a327756e4d1d16ec329ba7cb2048f6c6cd`
  (the tag moved from `cef483d`, upstream issue 123, so the lock's `tag` field carries the commit and the tarball is
  that commit's GitHub archive, the go-template precedent), ABI 15, with its `src/scanner.c` and a
  committed `src/parser.c` (`generate: false`); built into `internal/extractor/tsengine/grammars/cuda.wasm.gz`.
  Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json`. The grammar extends tree-sitter-cpp
  master (`8b5b49e`), not the v0.23.4 cpp pins here, so cpp's query nodes were checked against the goldens.
- `tags.scm`, `references.scm`, `imports.scm`: copied from `dialects/cpp` (written in this repository, LB-Q-cpp) and
  unchanged but for one line: under owner ruling C-R3 (2026-10-25) CUDA takes C's convention for a function-like macro
  invocation, a call on both sides (the oracle's `ClangMacroCalls`, `internal/langbench/scip.go`), not C++'s lexical
  proxy, so the plain-callee filter no longer drops an all-capitals name (`checkCudaErrors(...)` and `CUDA_CHECK(...)`
  are calls); it still drops a one-capital-letter name, a template parameter's functional cast (`T(0)`). The CUDA execution-space qualifiers (`__global__`, `__device__`, `__host__`, `__forceinline__`,
  `__launch_bounds__(...)`) are anonymous children of `function_definition`, so cpp's function patterns match, and a
  `k<<<g, b>>>(args)` launch is a `call_expression` with a `kernel_call_syntax` child, so cpp's call patterns match.
  Storage-space qualifiers (`__constant__`, `__device__`, `__managed__`) on namespace-scope data are type qualifiers.
- Fixtures under `testdata/` were written for this dialect; the goldens were produced by `go test -update` and
  then read line by line against the fixtures and the `lang-probe` captures.
