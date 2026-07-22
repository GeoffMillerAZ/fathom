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

## Blob sizes and checksums (the committed blobs, after `wasm-opt -Oz`)

Exact values — verify with `shasum -a 256 *.wasm` and `stat -f%z *.wasm` in
this directory; build.sh prints both at the end of a rebuild:

| Blob | Size (bytes) | sha256 (64 hex chars) |
|---|--:|---|
| `javascript.wasm` | 458,741 | `7245ee58564f991827498e36b5ec14420b99759596490e3a74ed0993b24ed904` |
| `typescript.wasm` | 2,037,040 | `30080208c90581bf3b29b412e9ee2e9a11c89b36153b6a94cc07da7b5000afcb` |
| `python.wasm` | 539,848 | `ef1216ec19691690d3d237759d5ab8c3521e362423eef44f5e5234f7cb507ffc` |
| `ruby.wasm` | 1,710,411 | `3299b506f05b73cb341de2cf65f1f327e9427e0149ce47d21f65af3d89b8924f` |

(458,741 B ≈ 448 KiB; 2,037,040 B ≈ 1.94 MiB; 539,848 B ≈ 527 KiB;
1,710,411 B ≈ 1.63 MiB.) TypeScript is the largest blob because it
statically links both the `typescript` and `tsx` grammars (each has its own
large generated parse table) alongside the core; Ruby's grammar tables are
large due to the language's syntactic ambiguity.

Each blob also compiles in `budget.c` (this directory, checked in — the only
non-upstream C in the build): the deadline-bounded parse/query entry points
(`fathom_parse_bounded`, `fathom_query_cursor_exec_bounded`,
`fathom_take_cancelled`) and the scalar-argument node accessor
(`fathom_node_end_byte`). See that file's header comments for why each
exists.

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
then-call-exports usage. The output only imports three `wasi_snapshot_preview1`
functions — `fd_write`/`fd_close` (libc startup references, never called in
practice) and `clock_time_get` (budget.c's CLOCK_MONOTONIC deadline checks)
— wazero's stock `imports/wasi_snapshot_preview1` package satisfies all of
them with no custom shim.

Build-time tools (zig, wasm-opt) are **not** a runtime or `go build`
dependency — the committed `.wasm` blobs are embedded via `go:embed` and
consumed purely by wazero (pure Go) at runtime. Re-running `build.sh` requires
network access (to fetch pinned upstream source tarballs) and is meant to be
run on demand to refresh a pin, exactly like `calibration/clone.sh` — never
as part of `task validate`.

**Toolchain that produced the committed blobs**: `zig 0.16.0` (Homebrew
bottle, arm64 macOS) and `wasm-opt version 129` (binaryen). Byte-identical
third-party reproduction is NOT achievable across toolchain versions — a
different zig (different bundled clang/wasm-ld) or binaryen emits different
(functionally equivalent) code, so a rebuild verifies provenance by matching
the *source* pins above, not by matching these sha256s. The digests bind the
committed bytes; the pins + recipe bind what those bytes were built from.

## Runtime behavior note: the pathological-input bound (critic B1)

On pathological input — `({[` repeated 50,000x (150KB, the shape of a
truncated minified bundle) — the original unbounded engine ran **52.9s**
(200,000x: 11+ minutes at 100% CPU) where the CGO incumbent finishes in
~49ms. Root-cause attribution, established by direct measurement during the
fix: the **parse** of this input completes in ~70ms — the time goes to the
**query cursor's scan** of the resulting error tree (45s+ measured for the
150KB input). The wedge risk is real either way: uncancellable wasm
execution holding the extractor's mutex.

Three mechanisms bound it now (each chosen against a measured alternative):

1. **Guest-side budgets** (`fathom_parse_bounded` /
   `fathom_query_cursor_exec_bounded` in `budget.c`): tree-sitter's own
   progress-callback cancellation, one CLOCK_MONOTONIC read (WASI
   clock_time_get) per ~100 operations. Cost ~2% corpus-wide. The rejected
   alternative — wazero's `WithCloseOnContextDone` — compiles termination
   checks into every loop back-edge and measured **~3x slower corpus-wide**.
2. **Host-side hard backstop** (`tsengine.ParseHardBackstop`, default 2x the
   10s budget): a goroutine+reaper harness abandons any guest operation the
   budgets fail to stop (progress checks arrive at a slow real-time cadence
   on exactly the pathological inputs — measured: a 16x-amortized clock
   check let the full 45s scan run to completion), so the ingest thread is
   ALWAYS released within a fixed bound; the runaway call burns out in the
   background and its module is then closed, freeing everything.
3. **Parser re-creation on any engine fault** (`TSExtractor.noteFault`):
   faults are counted per-ingest on `fusion.Report.TSEngineFaults` with one
   stderr line each; the affected file is fingerprinted (not retried until
   its bytes or the ExtractorEpoch change); the next file of that language
   gets a fresh parser. Recovery is never a fallback to the CGO incumbent
   extractors (fallback doctrine — plan M2 ruling).

The v0.26-vs-v0.25 core delta remains the suspected variable for WHY the
error-tree scan explodes at all (unconfirmed upstream); the bounds above are
version-independent by design.

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
