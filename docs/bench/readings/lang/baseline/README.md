Baseline taken at 56c18502
# LB-M1 — the extraction baseline before the tags engine changes

Plan row LB-M1 (`docs/plans/language-breadth.md`), taken 2026-09-27 with
`indexbench lang-oracle` (LB-F23) at fathom 56c18502, before LB-F1 moves any language onto
dialect manifests — so every later port is measured for parity against these numbers, same
harness, same mapping (`internal/langbench/incumbents.json`), same subjects.

**Verdict, in plain words.** Only Go (L1 good) and Protocol Buffers (L1 good) meet a level
today. Symbol recall is high where a real AST walker exists (Java 0.98, PHP 0.99, Bash 0.99,
Rust 0.96, C# 0.96) and low for Lua (0.26), Kotlin (0.34) and JavaScript (0.37). Every
language except Go misses L1 for two reasons the queue items exist to fix: no extractor but
Go fills `Signature` (signature_coverage 0.00), and the wasm-engine languages (Python, Ruby,
TypeScript, JavaScript, Kotlin) emit no `EndLine` (range_agreement 0.00 where measurable).
Tenancy holds everywhere: no faults, the slowest file 64 ms, throughput ≥ 2.8 MB/s, the
largest grammar module 602 pages (Kotlin) of the 1024 cap.

## How it was taken (reduced, and why)

- One subject per dialect — the first of the queue entry's `calibration_subjects`, or its
  smaller one — shallow-cloned at its default-branch HEAD (the SHA is in each reading), then
  deleted. The queue items pin tagged subjects in `calibration/corpus.d/` and add the second.
- Oracle: universal-ctags 6.2.1 (tier B) only. No SCIP indexer was installed on the measuring
  machine, so L2–L4 are `not-attempted`; each queue item runs its SCIP producer (§13.4).
- Measured: the 18 registered dialects universal-ctags can read. **Not measured**: Scala
  (universal-ctags 6.2.1 has no Scala parser) and the registered tool formats with no tier-B
  oracle (Dockerfile, GitHub Actions, Kubernetes, Compose, Helm, Kustomize, OpenAPI, go.mod,
  CUE, Markdown, templ, GraphQL, JSON/TOML) — their queue items measure them against the
  tool-native oracles the queue names. Recorded as `LB-M1-rest` in the plan.

## Readings

| Dialect | Subject | Files | sym_recall | sym_precision | kind | range | signature | level_met (L1) | MB/s | max file ms | peak pages |
|---|---|---|---|---|---|---|---|---|---|---|---|
| bash | nvm-sh/nvm@a885b885 | 6 | 0.99 | 1.00 | 1.00 | — | 0.00 | none (unmet) | 10.77 | 15 | 0 |
| c | libuv/libuv@abe835d4 | 369 | 0.74 | 0.81 | 1.00 | 1.00 | 0.00 | none (unmet) | 8.31 | 22 | 0 |
| cmake | fmtlib/fmt@5da4e9a3 | 11 | 0.98 | 0.89 | — | — | — | none (unmet) | 129.23 | 0 | 0 |
| cpp | fmtlib/fmt@5da4e9a3 | 47 | 0.87 | 0.93 | 1.00 | 1.00 | 0.00 | none (unmet) | 5.68 | 64 | 0 |
| csharp | App-vNext/Polly@dc933fd1 | 801 | 0.96 | 0.80 | 0.99 | 1.00 | 0.00 | none (unmet) | 5.95 | 31 | 0 |
| go | spf13/cobra@adbc8813 | 36 | 0.93 | 1.00 | 1.00 | 1.00 | 1.00 | L1 (good) | 22.19 | 8 | 0 |
| java | square/retrofit@e27d855b | 306 | 0.98 | 1.00 | 1.00 | 1.00 | 0.00 | none (unmet) | 9.21 | 14 | 0 |
| javascript | expressjs/express@9a34acf0 | 141 | 0.37 | 0.99 | 0.94 | — | 0.00 | none (unmet) | 3.42 | 9 | 311 |
| kotlin | square/okhttp@40a3b874 | 617 | 0.34 | 0.95 | 1.00 | — | 0.00 | none (unmet) | 3.36 | 57 | 602 |
| lua | nvim-telescope/telescope.nvim@40aedd8a | 77 | 0.26 | 1.00 | 1.00 | — | 0.00 | none (unmet) | 8.65 | 6 | 0 |
| php | guzzle/guzzle@93939470 | 137 | 0.99 | 1.00 | 1.00 | — | 0.00 | none (unmet) | 6.96 | 60 | 0 |
| protobuf | bufbuild/protovalidate@d06a71c9 | 156 | 1.00 | 0.97 | — | — | — | L1 (good) | 79.82 | 1 | 0 |
| python | pallets/flask@d73fa1cd | 83 | 0.94 | 1.00 | 1.00 | 0.00 | 0.00 | none (unmet) | 4.53 | 19 | 350 |
| ruby | sinatra/sinatra@cb22afd7 | 147 | 0.91 | 0.99 | 1.00 | 0.00 | 0.00 | none (unmet) | 2.89 | 21 | 428 |
| rust | BurntSushi/ripgrep@3fce3b5b | 110 | 0.96 | 0.97 | 1.00 | — | 0.00 | none (unmet) | 8.03 | 28 | 0 |
| sql | lerocha/chinook-database@7f677725 | 11 | 1.00 | 0.82 | 1.00 | — | — | none (unmet) | 272.16 | 2 | 0 |
| terraform | terraform-aws-modules/terraform-aws-vpc@b3abd6df | 77 | 0.89 | 1.00 | — | — | — | none (unmet) | 17.31 | 4 | 0 |
| typescript | colinhacks/zod@2bf7b063 | 515 | 0.89 | 0.70 | 1.00 | — | 0.00 | none (unmet) | 3.02 | 50 | 468 |

`—` is unmeasured: the oracle gave no end lines (universal-ctags emits `end` for almost no
Rust, JavaScript, Kotlin, Lua, PHP or TypeScript tags — those need a tier-A or tier-C range
oracle to prove L1), no callables were in scope, or the metric is not a tool-format metric.

## Oracle caveats that move numbers (read before comparing a port)

- TypeScript precision 0.70: universal-ctags tags `const f = () => …` as `constant`, outside
  the kind intersection, while fathom emits a function — an oracle-vocabulary artefact the
  TypeScript item should settle with a SCIP definition oracle, not a mapping change.
- Kotlin recall 0.34: universal-ctags tags local `val`/`var` as `variable`, mapped to
  `property`; fathom extracts class-level properties only.
- Python maps ctags `member` to `function` (fathom's Python taxonomy has no method kind);
  Rust maps ctags `method` to `function` (fathom emits methods as `RustFn`).
- C reads every `.h` as C; fathom's header sniff routes some to C++.

Regenerate a reading: `go run ./cmd/indexbench lang-oracle -dialect <id> -subject <clone>
-subject-name <owner/repo> -out docs/bench/readings/lang/baseline/<id>.json`.

## LB-F1 parity (the move onto dialect manifests)

TSExtractor's registry moved from Go tables and `internal/extractor/tsqueries/` to
`internal/extractor/dialects/{javascript,typescript,python,ruby,kotlin}/`. Extraction over the
five subjects above, symbols and import specs serialised file by file (EndLine and Signature
excluded, both empty before LB-F3), is byte-identical before (fdbc2ee4) and after:

| Subject | Files | Symbols | sha256 of the dump |
|---|---|---|---|
| pallets/flask@d73fa1cd | 83 | 1756 | cf52e8f03ca333bd2e6f9756985799855898b995fe037c8a8e653cf785d79de8 |
| expressjs/express@9a34acf0 | 141 | 127 | cec1e913c1df65b874834b38fc57cd829706407c3bf0674dd08454a63e2a75f9 |
| colinhacks/zod@2bf7b063 | 523 | 4058 | 3ecff8d8b08cc204fb08713d80a44aa5ca841d90cca93494a6c39ec83033a7af |
| sinatra/sinatra@cb22afd7 | 147 | 1165 | 9db26e55f9ae013056603ef607fc792ee1e336e3c72fd7f26f688146fcf0b284 |
| square/okhttp@40a3b874 | 617 | 15390 | ab7322e31be0f2b3a425b60cc13c6f9621dabc454a3ed2609aeea9cb04def747 |

The fixtures' goldens (`dialects/<id>/testdata/*.golden.json`, written by the pre-manifest
engine) hold the same parity in the gate: `TestTagsDialectParity`.
