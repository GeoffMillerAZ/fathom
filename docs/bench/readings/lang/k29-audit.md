# K29 audit: the readings that predate K25 ruling 3 (2026-10-10, wave lf5 item cfam)

**Verdict: eight committed readings were read before K25 ruling 3 on SCIP file-edge truth. Six are
re-read under the current harness (c, cpp, dart, java, php, python), two are unaffected by
construction (go, csharp), none is left stale. One level fell: C's L3, excellent to unmet, so
LB-Q-c reopens (O863). C++'s L3 rose from good to excellent under owner ruling L2. Java's
file_edge_recall moved by .0004. Dart, PHP and Python read exactly as before.**

K29 (owner, 2026-10-10, design §1.8) says a harness rule change re-reads every committed reading it
touches in the same wave, or marks each one stale with a row naming the rule. K25 ruling 3
(f274aa29, LB-F387, on main with fb7d9f3f) resolves a SCIP symbol several documents define to every
defining document in the file-edge truth. Before it, the harness took the first definer alone. The
ruling was measured on rust-analyzer and then applied to every producer, and several readings taken
before it were never re-read (O790, O806). This file is LB-F479's record.

Method:

- Every committed reading under `docs/bench/readings/lang/`, `tuned/` and `holdout/` was listed
  where a SCIP producer is among the oracles and the metrics include the file-edge pair.
- Each one's `fathom_commit` was tested for ancestry against f274aa29 (`git merge-base
  --is-ancestor`).
- For each reading, `ScoreSCIPWith`'s definer switch (`internal/langbench/scip.go`) shows whether
  ruling 3 can reach its truth at all: a producer's own rule takes a multiply-defined symbol before
  ruling 3's default case.
- Each reading that ruling 3 can reach was re-read: java and php at d6b18b93, the others at
  f2b852ab. The harness code is the same at both, because the commits between them touch readings
  and the plan only. The oracle outputs were rebuilt from the pinned subjects, or reused where they
  still existed and matched every count (java, php).

| Reading | fathom_commit | Producer | Can ruling 3 reach it? | Verdict | file_edge_recall / precision, before -> after |
|---|---|---|---|---|---|
| c.json | 085a1b86 | scip-clang | yes (the default case) | re-read: L3 excellent -> unmet, level_met L4 -> L2; LB-Q-c reopened, LB-Q-c-rest deferred (O863) | .5326 / .9945 -> .2455 / .9948; 8,789 pairs to a later definer (libuv 8,613: file-local test callbacks named alike in every translation unit) |
| cpp.json | 89b21684 | scip-clang | yes | re-read under owner ruling L2 (LB-F473): L3 good -> excellent, level_met L4 | .4355 / .9754 -> .5354 / .9656; 14,694 namespace pairs left out, 3,029 to a later definer |
| csharp.json | 90870a7f | scip-dotnet | no: lang-oracle always passes a source, so `SCIPOptionsFor("csharp")` sets CSharpCalls, and a symbol two documents define takes LB-F287's case (`opts.CSharpCalls && defDoc[sym] == ""`, no edge) before the default; the first-definer case drew nothing before the ruling and draws nothing after | unaffected | .5332 / .9814, unchanged by construction |
| dart.json | cd4c1e9e | scip-dart | yes | re-read: unchanged (no pair to a later definer) | .7844 / .9918 -> the same |
| go.json | a4ea15ec | scip-go | no: a symbol two documents define takes LB-F298's case (`multiDef.holds`, no edge; a package is defined at every file's package clause) before the default, so the default sees one definer, as before | unaffected | .7034 / .9972, unchanged by construction |
| java.json | a291c100 | scip-java | yes | re-read: every grade unchanged | .829 / 1 -> .8286 / 1; 2 pairs to a later definer (retrofit) |
| php.json | 1f8b8faa | scip-php | yes | re-read: unchanged (no pair to a later definer) | .9193 / .9989 -> the same |
| python.json | 7767696c | scip-python | yes | re-read: unchanged (no pair to a later definer) | .9191 / .9875 -> the same |

The oracles each re-read used:

- **c:** libuv v1.53.0 and curl 8.22.0, their cmake compile databases (477 and 495 entries),
  scip-clang 0.4.0, the preprocessor oracle and clang-ast-defs, rebuilt.
- **cpp:** fmt, json and spdlog as the corpus shard says, with clang-calls and universal-ctags 6.2.1
  too, rebuilt.
- **dart:** scip_dart 1.7.0 over the eight pinned packages after `dart pub get`, rebuilt.
- **java:** the scip-java 0.12.3 indexes and JavaParser 3.28.2 definition dumps of 2026-10-02.
  Every count the reading holds but the file-edge truth read identically, so they are the
  reading's own.
- **php:** the scip-php 71a5b117 indexes and php-parser 5.7.0 dumps of wave lh item pkc. Every
  count read identically.
- **python:** scip-python 0.6.6 run.sh in fresh scratch environments, rebuilt.

C's residual: scip-clang names a file-local function or variable alike in every translation unit
that defines it. For example, libuv's 73 test files each write `static void close_cb(uv_handle_t*)`,
and all of them are the one symbol `close_cb(74ef0d3a3c5968a2).`, so every test file reaches every
other. This is O806's second shape. A truth rule for it is the owner's (K19), and none is committed
here. A prototype harness switch, never committed, reads a reference inside a document defining the
symbol as that document's own copy (O751 step 2, applied to scip-clang). Under it c.json reads
file_edge_recall .5667 and precision .9948, every level excellent (O863).

Not in scope, and why:

- `tuned/javascript.json`, `tuned/typescript.json` (7a4d7cf2, scip-typescript, before ruling 3):
  these belong to wave lf5 item tsjs and are re-read there.
- react.json (7a4d7cf2), rego.json (0f82424c) and ruby.json (cfa58b63) were read before ruling 3,
  but their file-edge truth comes from no SCIP index (react-docgen-typescript with the TypeScript
  checker, opa parse, the ruby-require dump). Ruling 3 lives in `ScoreSCIPWith` alone, so it cannot
  reach them.
- kotlin.json, scala.json, rust.json, typescript.json, javascript.json and `holdout/` were read
  after ruling 3. rust.json is re-read in this wave for owner ruling L1.
- `parity/` and `baseline/` hold no level the queue reads, because the tracker grades
  `docs/bench/readings/lang/*.json` only (LB-F437 states this for `holdout/`). A parity reading
  decided a dispatch that has already shipped, and a baseline records a state from before the
  dialects.

K29 scopes the two rules this wave adds as well:

- **Owner ruling L2** (a scip-clang namespace draws no file edge, LB-F473) touches the scip-clang
  readings only. cpp.json is re-read. c.json is re-read above, and the rule leaves nothing out
  there because C has no namespace. `holdout/cpp.json` was read with the rule in place.
- **Owner ruling L1** restores O751's steps 2 and 4 (LB-F477). It touches three readings:
  - rust.json is re-read (LB-Q-rust-rest).
  - `holdout/rust.json` (crossbeam) was read at 58eba2bb with steps 1 to 4 in place, so the
    restored harness is the one it was read with.
  - `holdout/rust-regex.json` was read with the steps restored.
- No reading is left stale, so no stale row (LB-F480) is written.
