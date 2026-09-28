# Languages and dialects — what fathom extracts, and how far

A dialect is an extractor: the knowledge of how to read a language or tool format
(`docs/design/pack-taxonomy.md`). This table is the tracker ADR-041 names: extraction
approach, whether a tree-sitter grammar is in the tree, where typed edges come from, and
what is left. Typed-edge order is the owner's (2026-09-19): Go, CUE, TypeScript,
Python, then the delegate's. `TestLanguagesTrackerMatchesRegistry` (SU-13) keeps the
rows honest against the extractor registry.

**The breadth program (ADR-048, vision 37).** Support levels are L1 symbols, L2 syntax call
edges, L3 imports and file-resolved edges, L4 typed edges, each claimed only when a committed
reading under `docs/bench/readings/lang/` proves it against an independent oracle (ctags or
SCIP; design `docs/design/language-breadth.md` §1). The "Support levels" table at the end of
this file is **generated** (LB-F17, `go run ./cmd/indexbench lang-tracker > docs/languages.md`)
from the queue (`docs/plans/language-breadth.queue.json`: 77 languages and 65 tool formats),
the registered dialects and the readings; `TestLanguagesTrackerIsGenerated` fails when this
file differs from the rendering and when a row claims a level above its reading (I54). The
typed-edge table below is the hand-kept template part, held true against the registry by
`TestLanguagesTrackerMatchesRegistry`; the "heuristic" label on Java, Scala, Rust, C, C++, C#,
PHP and Lua means hand-written Go walkers over CGO tree-sitter — real ASTs, never measured
against an oracle until their queue items land.

Typed-edge levels: **native** (fathom resolves types itself), **scip** (imported from a
language-native SCIP indexer), **file** (per-file resolver, tier pre-scan, labelled
`file-resolved`), **syntax** (call names by syntax only).

| Language / dialect | Extraction | Grammar in tree | Calibration subjects | Typed edges | Next |
|---|---|---|---|---|---|
| Go | `go/parser` + `go/types` | n/a | cobra, cli/cli, prometheus | **native** (done; selector-method and interface gaps O39/O40 open) | close O39/O40 |
| CUE | `cueextract.go` + `cue/load` | n/a | this repo's `.fathom/`, `schema/` | **native** (SU-10) | — |
| TypeScript | tree-sitter (wasm) | yes | nest, zod, typeorm | **scip** — `fathom ingest --scip` imports scip-typescript's own output (SU-9, `internal/scip`; the L0 oracle SU-12 is also done); against the real, only-published scip-typescript@0.4.0 it emits zero `DisplayName`s, which broke the import until O115's fix (2026-09-24): `ImportIndex` now maps each definition to fathom's own symbol table by position instead, measured on zod at 82.0% of raw SCIP definitions unmapped (no fathom-level counterpart — SCIP's finer granularity, expected) and the rest correctly keyed; **the file-resolved tier shipped (SU-11, `internal/fusion/fileresolve.go`) remains the fallback** when no scip index is imported, now with a real precision/recall read against the fixed scip truth: 0.87 precision / 0.03 recall (CV-M7, `docs/bench/readings/l0-ts-py-2026-09-24.md`) | CV-M7 done; a `PutTypedEdges` replace-granularity gap the re-run surfaced (a package with zero NEW edges keeps its stale old rows) is unfixed, its own thread; lookups corpus (SU-23) |
| JavaScript (`.js .jsx .mjs .cjs`) | tree-sitter (wasm) | yes | express, axios, lodash | **scip** — same `--scip` import mechanism as TypeScript (SU-9, SU-12 done), same O115 fix (2026-09-24: position-based mapping, fixed); **the file-resolved tier shipped (SU-11) remains the fallback** when no scip index is imported | — |
| Python | tree-sitter (wasm) | yes | fastapi, flask, django | **scip** — `fathom ingest --scip` imports scip-python's own output (SU-9, SU-12 done); against the real, only-published scip-python@0.6.6 it emits zero `DisplayName`s, which broke the import until O115's fix (2026-09-24: position-based mapping), measured on flask at 65.5% of raw SCIP definitions unmapped (expected — SCIP's finer granularity) and the rest correctly keyed; **the file-resolved tier shipped (SU-11) remains the fallback** when no scip index is imported, now with a real precision/recall read against the fixed scip truth: 0.95 precision / 0.12 recall (CV-M7, `docs/bench/readings/l0-ts-py-2026-09-24.md`) | CV-M7 done; lookups corpus (SU-23) |
| Java | heuristic (`javaextract.go`) | no | guava, retrofit | **scip** via scip-java (SU-9) | grammar; oracle |
| Kotlin | tree-sitter (wasm, fwcd 0.3.8) | yes | okhttp, kotlinx.coroutines, Exposed | **scip** via scip-java (SU-9) | oracle (LCA corpus) |
| Scala | heuristic (`scalaextract.go`) | no | cats, zio, http4s | **scip** via scip-java | grammar |
| Rust | heuristic (`rustextract.go`) | no | (corpus.yaml) | **scip** via rust-analyzer | grammar; oracle |
| C | heuristic (`cextract.go`, `cheaderextract.go`) | no | curl, libuv, sqlite | **scip** via scip-clang | grammar |
| C++ | heuristic (`cppextract.go`) | no | (corpus.yaml) | **scip** via scip-clang | grammar |
| Ruby | tree-sitter (wasm) | yes | sinatra, jekyll, fastlane | **scip** via scip-ruby | oracle |
| HTML | tree-sitter (wasm) | yes | html5-boilerplate, bootstrap (visual tests) | **file** — `src`/`href` resolve via the relative-path strategy (LB-F11); no SCIP producer exists for HTML, so this is the ceiling until one does | — |
| CSS | tree-sitter (wasm) | yes | normalize.css, pico | **file** — `@import` resolves via the relative-path strategy (LB-F11); no SCIP producer exists for CSS, so this is the ceiling until one does | BareName's separator set mismatches CSS's own `.`/`#`/`:` selector-marker punctuation (internal/langbench/score.go, Opus); tree-sitter-css > v0.25.0 for `:not(A, B)` |
| C# | heuristic (`csharpextract.go`) | no | (corpus.yaml) | **scip** via scip-dotnet | grammar |
| PHP | heuristic (`phpextract.go`) | no | laravel, guzzle, monolog | **syntax** (no mature exporter) | grammar; file resolver |
| Swift | **none** — disclosed as unsupported, reported per service by `list_services` | no | (corpus.yaml) | **none** | extractor (fleet demand: app-kit) |
| Lua | heuristic (`luaextract.go`) | no | (corpus.yaml) | **syntax** | grammar |
| Bash | heuristic (`bashextract.go`) | no | (corpus.yaml) | **syntax** | — |
| SQL | heuristic (`sqlextract.go`) | no | — | **syntax** | — |
| Terraform/HCL | pure Go (`hclextract.go`) | n/a | (corpus.yaml) | **syntax** (resource references) | — |
| Kubernetes YAML, Helm, Kustomize, docker-compose | pure Go | n/a | (corpus.yaml) | **syntax** (`service_ref` cross-repo edges) | — |
| Dockerfile, CMake, GitHub Actions, proto, GraphQL, OpenAPI, JSON, TOML, templ, Markdown, go.mod, package.json, pyproject.toml | pure Go | n/a | (corpus.yaml where listed) | **syntax** | — |

Disclosed as unsupported (named per service in `list_services`, never silently skipped; `internal/connector/unsupported_language.go`): Clojure, Crystal, Dart, Elixir, Elm, Erlang, F#, Groovy, Haskell, Julia, Nim, OCaml, Swift, Zig.

Bench coverage today: every scenario corpus has a Go subject; the Long Code Arena corpus
covers Python, Java and Kotlin bug localization. SU-23 (done) adds TypeScript
(`testdata/scenariobench/lookups-ts.json`, `nest`) and Python
(`testdata/scenariobench/lookups-py.json`, `fastapi`) lookups corpora, committed and
**not run** (ADR-041 decision 9) — both corpora's own `notes.typed_edges` records that
neither SU-9 nor SU-11 is wired yet, so their `full`-arm predictions are written against
today's heuristic (syntax) call edges only, never a typed tier. SU-12 adds the
free-tier typed-edge oracle per language.

{{LANGBENCH_GENERATED}}
