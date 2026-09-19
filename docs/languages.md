# Languages and dialects — what fathom extracts, and how far

A dialect is an extractor: the knowledge of how to read a language or tool format
(`docs/design/pack-taxonomy.md`). This table is the tracker ADR-041 names: extraction
approach, whether a tree-sitter grammar is in the tree, where typed edges come from, and
what is left. Typed-edge order is the owner's (2026-09-19): Go, CUE, TypeScript,
Python, then the delegate's. `TestLanguagesTrackerMatchesRegistry` (SU-13) keeps the
rows honest against the extractor registry.

Typed-edge levels: **native** (fathom resolves types itself), **scip** (imported from a
language-native SCIP indexer), **file** (per-file resolver, tier pre-scan, labelled
`file-resolved`), **syntax** (call names by syntax only).

| Language / dialect | Extraction | Grammar in tree | Calibration subjects | Typed edges | Next |
|---|---|---|---|---|---|
| Go | `go/parser` + `go/types` | n/a | cobra, cli/cli, prometheus | **native** (done; selector-method and interface gaps O39/O40 open) | close O39/O40 |
| CUE | `cueextract.go` + `cue/load` | n/a | this repo's `.fathom/`, `schema/` | **native** (SU-10) | — |
| TypeScript | tree-sitter (wasm) | yes | nest, zod, typeorm | **scip** — `fathom ingest --scip` imports scip-typescript's own output (SU-9, `internal/scip`; the L0 oracle SU-12 is also done); **file** fallback pending (SU-11) | run scip-typescript on a calibration clone, then `task bench-l0-scip`; lookups corpus (SU-23) |
| JavaScript (`.js .jsx .mjs .cjs`) | tree-sitter (wasm) | yes | express, axios, lodash | **scip** — same `--scip` import mechanism as TypeScript (SU-9, SU-12 done); **file** fallback pending (SU-11) | run scip-typescript on a calibration clone, then `task bench-l0-scip` |
| Python | tree-sitter (wasm) | yes | fastapi, flask, django | **scip** — `fathom ingest --scip` imports scip-python's own output (SU-9, SU-12 done); **file** fallback pending (SU-11) | run scip-python on a calibration clone, then `task bench-l0-scip`; lookups corpus (SU-23) |
| TypeScript | tree-sitter (wasm) | yes | nest, zod, typeorm | **scip** via scip-typescript (SU-9); **file** fallback (SU-11) | L0 oracle (SU-12) |
| JavaScript (`.js .jsx .mjs .cjs`) | tree-sitter (wasm) | yes | express, axios, lodash | **scip** via scip-typescript (SU-9); **file** fallback (SU-11) | L0 oracle |
| Python | tree-sitter (wasm) | yes | fastapi, flask, django | **scip** via scip-python (SU-9); **file** fallback (SU-11) | L0 oracle (SU-12) |
| Java | heuristic (`javaextract.go`) | no | guava, retrofit | **scip** via scip-java (SU-9) | grammar; oracle |
| Kotlin | tree-sitter (wasm, fwcd 0.3.8) | yes | okhttp, kotlinx.coroutines, Exposed | **scip** via scip-java (SU-9) | oracle (LCA corpus) |
| Scala | heuristic (`scalaextract.go`) | no | cats, zio, http4s | **scip** via scip-java | grammar |
| Rust | heuristic (`rustextract.go`) | no | (corpus.yaml) | **scip** via rust-analyzer | grammar; oracle |
| C | heuristic (`cextract.go`, `cheaderextract.go`) | no | curl, libuv, sqlite | **scip** via scip-clang | grammar |
| C++ | heuristic (`cppextract.go`) | no | (corpus.yaml) | **scip** via scip-clang | grammar |
| Ruby | tree-sitter (wasm) | yes | sinatra, jekyll, fastlane | **scip** via scip-ruby | oracle |
| C# | heuristic (`csharpextract.go`) | no | (corpus.yaml) | **scip** via scip-dotnet | grammar |
| PHP | heuristic (`phpextract.go`) | no | laravel, guzzle, monolog | **syntax** (no mature exporter) | grammar; file resolver |
| Swift | heuristic | no | (corpus.yaml) | **syntax** | grammar |
| Lua | heuristic (`luaextract.go`) | no | (corpus.yaml) | **syntax** | grammar |
| Bash | heuristic (`bashextract.go`) | no | (corpus.yaml) | **syntax** | — |
| SQL | heuristic (`sqlextract.go`) | no | — | **syntax** | — |
| Terraform/HCL | pure Go (`hclextract.go`) | n/a | (corpus.yaml) | **syntax** (resource references) | — |
| Kubernetes YAML, Helm, Kustomize, docker-compose | pure Go | n/a | (corpus.yaml) | **syntax** (`service_ref` cross-repo edges) | — |
| Dockerfile, CMake, GitHub Actions, proto, GraphQL, OpenAPI, JSON, TOML, templ, Markdown, go.mod | pure Go | n/a | (corpus.yaml where listed) | **syntax** | — |

Bench coverage today: every scenario corpus has a Go subject; the Long Code Arena corpus
covers Python, Java and Kotlin bug localization. SU-23 (done) adds TypeScript
(`testdata/scenariobench/lookups-ts.json`, `nest`) and Python
(`testdata/scenariobench/lookups-py.json`, `fastapi`) lookups corpora, committed and
**not run** (ADR-041 decision 9) — both corpora's own `notes.typed_edges` records that
neither SU-9 nor SU-11 is wired yet, so their `full`-arm predictions are written against
today's heuristic (syntax) call edges only, never a typed tier. SU-12 adds the
free-tier typed-edge oracle per language.
