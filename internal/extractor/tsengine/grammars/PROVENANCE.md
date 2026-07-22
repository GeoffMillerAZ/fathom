# Grammar blob provenance (ADR-021 §4, docs/plans/extraction-treesitter.md)

Every `*.wasm` file in this directory is built by `build.sh` from **official,
upstream tree-sitter sources only** — tree-sitter's own core C runtime plus
each grammar's own C parser — statically linked into one self-contained
`wasm32-wasi` module per language via `zig cc` (musl libc, no Emscripten). No
third-party prebuilt `.wasm` artifact is vendored; no unverifiable blob is
embedded.

## What's linked into each blob

| Blob | Core runtime | Grammar(s) | Entry point(s) exported |
|---|---|---|---|
| `javascript.wasm` | tree-sitter core | tree-sitter-javascript | `tree_sitter_javascript` |
| `typescript.wasm` | tree-sitter core | tree-sitter-typescript (both the `typescript/` and `tsx/` grammars in that repo) | `tree_sitter_typescript`, `tree_sitter_tsx` |
| `python.wasm` | tree-sitter core | tree-sitter-python | `tree_sitter_python` |
| `ruby.wasm` | tree-sitter core | tree-sitter-ruby | `tree_sitter_ruby` |

Grammar versions are pinned to match the CGO grammar versions already in
`go.mod` (`github.com/tree-sitter/tree-sitter-<lang>`) so the calibration
comparison between the incumbent (CGO) and this engine (wazero) is
apples-to-apples — same grammar generation, different execution mechanism.

## Resolved sources (pinned tag → commit SHA, as of this build)

| Repo | Tag | Commit SHA |
|---|---|---|
| tree-sitter/tree-sitter (core runtime) | v0.26.11 | 64402de2857cc197ecc4ca3bc144ea91fda7e72e |
| tree-sitter/tree-sitter-javascript | v0.25.0 | f76aea6aa47322ea5c208c9c2e67f4a350d554f3 |
| tree-sitter/tree-sitter-typescript | v0.23.2 | f975a621f4e7f532fe322e13c4f79495e0a7b2e7 |
| tree-sitter/tree-sitter-python | v0.25.0 | d326e4cad262cf681656e130960e49dfc04c03ea |
| tree-sitter/tree-sitter-ruby | v0.23.1 | 71bd32fb7607035768799732addba884a37a6210 |

The core runtime version does not need to match each grammar's own release
train — tree-sitter's ABI is designed to be backwards-compatible (a newer
runtime accepts older-but-in-range grammar ABI versions; `ts_language_abi_version`
is checked at `ts_parser_set_language` time). Using one recent, well-tested
core across all four languages is intentional, not an oversight.

## Blob sizes and checksums (this build, after `wasm-opt -Oz`)

| Blob | Size | sha256 |
|---|--:|---|
| `javascript.wasm` | 458,000 bytes (458 KiB) | `86d894ae3f1a9aefd7ac0743aefdd85813d5bb7d42e1afadcf0ab9ee13ef8399` |
| `typescript.wasm` | 2,000,000 bytes (2.0 MiB) | `5b124dd43cff8fcdcfc3092f8d95388d1365ffcd702e98e6bd2a3f9a44b2c46` |
| `python.wasm` | 539,000 bytes (539 KiB) | `e9bd8e68f2d18dd12d5a7be8d77e153b1f6c93faed4eae7d60cd0984318b27d` |
| `ruby.wasm` | 1,700,000 bytes (1.7 MiB) | `5750100352c63779282918a1af50537267bc54437182c1ac1a2c47ca04a9ddb` |

(Sizes rounded to 3 significant figures for readability; run
`shasum -a 256 *.wasm` in this directory for exact byte counts/digests —
build.sh prints both at the end of a rebuild.) TypeScript is the largest
blob because it statically links both the `typescript` and `tsx` grammars
(each has its own large generated parse table) alongside the core; Ruby's
grammar tables are large due to the language's syntactic ambiguity.

## Build recipe (reproducible — `./build.sh`)

```
zig cc --target=wasm32-wasi-musl -mexec-model=reactor \
  -I <core>/lib/include -I <core>/lib/src -I <grammar>/src \
  <core>/lib/src/lib.c <grammar>/src/parser.c [<grammar>/src/scanner.c] \
  -o <lang>.wasm -Oz -Wl,--no-entry -Wl,--strip-debug \
  -Wl,--export=<the C API subset internal/extractor/tsengine calls> \
  -Wl,--export=tree_sitter_<lang>
wasm-opt -Oz <lang>.wasm -o <lang>.wasm --enable-bulk-memory   # ~25-30% smaller
```

`zig cc` targets `wasm32-wasi-musl`: Zig bundles a musl-based libc for that
target, so `malloc`/`free`/`memcpy`/etc. are real, working libc calls compiled
in — no hand-written allocator, no CGO. `-mexec-model=reactor` builds a
callable-library module (no `_start`/`main`), matching wazero's instantiate-
then-call-exports usage. The output only imports the three `wasi_snapshot_
preview1` functions (`fd_write`, `fd_seek`, `fd_close`) that libc's startup
code references but never calls in practice — wazero's stock
`imports/wasi_snapshot_preview1` package satisfies them with no custom shim.

Build-time tools (zig, wasm-opt) are **not** a runtime or `go build`
dependency — the committed `.wasm` blobs are embedded via `go:embed` and
consumed purely by wazero (pure Go) at runtime. Re-running `build.sh` requires
network access (to fetch pinned upstream source tarballs) and is meant to be
run on demand to refresh a pin, exactly like `calibration/clone.sh` — never
as part of `task validate`.

## Path not taken: Emscripten/web-tree-sitter-shaped `.wasm`

The grammar repos' own GitHub Releases also publish a `tree-sitter-<lang>.wasm`
asset (built via the tree-sitter CLI's `build --wasm`, i.e. Emscripten). That
artifact — and the matching `web-tree-sitter` core — were downloaded and
inspected before choosing the approach above:

- `tree-sitter-python.wasm` (official v0.25.0 release asset) imports
  `env.malloc/calloc/realloc/free/memcpy`, `env.memory`, `env.
  __indirect_function_table`, and `env.__memory_base`/`__table_base`, and
  carries a `dylink.0` custom section — it is a **side module** that must be
  dynamically linked (Emscripten's PIC relocation scheme) against a **separate
  core module** at a caller-chosen memory/table base offset.
- `web-tree-sitter.wasm` (the matching core, from the `web-tree-sitter` npm
  package) is itself shaped the same way, and additionally imports four
  `tree_sitter_*_callback` functions plus `wasi_snapshot_preview1.fd_write/
  fd_seek/fd_close` and `emscripten_resize_heap`/`_abort_js` — a JS-runtime
  callback shim (for streaming parse input, logging, and cancellation) that
  has to be reimplemented faithfully to get correct results.

Reimplementing Emscripten's dynamic-linking loader (dylink.0 parsing, shared
memory/table base-offset placement, GOT wiring) plus that callback shim in Go
is exactly the risk ADR-013 already flagged ("no production-grade pure-Go
tree-sitter-via-wazero runtime — grammar `.wasm` blobs target the Emscripten/
web-tree-sitter ABI, not bare wazero") — confirmed here empirically, not just
cited. The self-built monolithic `wasm32-wasi` module above sidesteps the
entire problem: one link step, no dynamic linking, no JS-shaped callback ABI,
only wazero's stock WASI shim. It was validated end-to-end (parse + tree walk
+ real `ts_query_*` compile/match/capture) before committing to it as the
production design.
