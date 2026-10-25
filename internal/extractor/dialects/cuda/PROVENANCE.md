# cuda — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-cuda v0.21.2` (MIT), commit `d58080a327756e4d1d16ec329ba7cb2048f6c6cd`
  (the tag moved from `cef483d`, upstream issue 123, so the lock's `tag` field carries the commit and the tarball is
  that commit's GitHub archive, the go-template precedent), ABI 15, with its `src/scanner.c` and a
  committed `src/parser.c` (`generate: false`); built into `internal/extractor/tsengine/grammars/cuda.wasm.gz`.
  Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json`. The grammar extends tree-sitter-cpp
  master (`8b5b49e`), not the v0.23.4 cpp pins here, so cpp's query nodes were checked against the goldens.
- `tags.scm`, `references.scm`, `imports.scm`: copied from `dialects/cpp` (written in this repository, LB-Q-cpp) and
  unchanged: the CUDA execution-space qualifiers (`__global__`, `__device__`, `__host__`, `__forceinline__`,
  `__launch_bounds__(...)`) are anonymous children of `function_definition`, so cpp's function patterns match, and a
  `k<<<g, b>>>(args)` launch is a `call_expression` with a `kernel_call_syntax` child, so cpp's call patterns match.
  Storage-space qualifiers (`__constant__`, `__device__`, `__managed__`) on namespace-scope data are type qualifiers.
- Fixtures under `testdata/` were written for this dialect; the goldens were produced by `go test -update` and
  then read line by line against the fixtures and the `lang-probe` captures.
