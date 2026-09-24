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
| TypeScript | tree-sitter (wasm) | yes | nest, zod, typeorm | **scip** — `fathom ingest --scip` imports scip-typescript's own output (SU-9, `internal/scip`; the L0 oracle SU-12 is also done), but is **silently non-functional against the real, only-published scip-typescript@0.4.0 today**: it emits zero `DisplayName`s, so every imported edge keys off the raw SCIP symbol string instead of fathom's `SymbolKey` and joins nothing (O115); **file-resolved (SU-11, `internal/fusion/fileresolve.go`) is the tier that actually works** and is used as the fallback when no scip index is imported (also the tier CV-M7's L0 reading hand-verified 26/26 on this same corpus) | fix `internal/scip`'s empty-`DisplayName` fallback against the real producer (O115), then re-run scip-typescript on a calibration clone and `task bench-l0-scip`; lookups corpus (SU-23) |
| JavaScript (`.js .jsx .mjs .cjs`) | tree-sitter (wasm) | yes | express, axios, lodash | **scip** — same `--scip` import mechanism as TypeScript (SU-9, SU-12 done), same O115 non-functional-against-the-real-producer caveat; **file-resolved (SU-11) is the tier that actually works** and is used as the fallback when no scip index is imported | fix `internal/scip`'s empty-`DisplayName` fallback (O115), then re-run scip-typescript on a calibration clone and `task bench-l0-scip` |
| Python | tree-sitter (wasm) | yes | fastapi, flask, django | **scip** — `fathom ingest --scip` imports scip-python's own output (SU-9, SU-12 done), but is **silently non-functional against the real, only-published scip-python@0.6.6 today** for the same reason (zero `DisplayName`s, O115); **file-resolved (SU-11) is the tier that actually works** and is used as the fallback when no scip index is imported (hand-verified 26/26 on this same corpus, CV-M7) | fix `internal/scip`'s empty-`DisplayName` fallback against the real producer (O115), then re-run scip-python on a calibration clone and `task bench-l0-scip`; lookups corpus (SU-23) |
| Java | heuristic (`javaextract.go`) | no | guava, retrofit | **scip** via scip-java (SU-9) | grammar; oracle |
| Kotlin | tree-sitter (wasm, fwcd 0.3.8) | yes | okhttp, kotlinx.coroutines, Exposed | **scip** via scip-java (SU-9) | oracle (LCA corpus) |
| Scala | heuristic (`scalaextract.go`) | no | cats, zio, http4s | **scip** via scip-java | grammar |
| Rust | heuristic (`rustextract.go`) | no | (corpus.yaml) | **scip** via rust-analyzer | grammar; oracle |
| C | heuristic (`cextract.go`, `cheaderextract.go`) | no | curl, libuv, sqlite | **scip** via scip-clang | grammar |
| C++ | heuristic (`cppextract.go`) | no | (corpus.yaml) | **scip** via scip-clang | grammar |
| Ruby | tree-sitter (wasm) | yes | sinatra, jekyll, fastlane | **scip** via scip-ruby | oracle |
| C# | heuristic (`csharpextract.go`) | no | (corpus.yaml) | **scip** via scip-dotnet | grammar |
| PHP | heuristic (`phpextract.go`) | no | laravel, guzzle, monolog | **syntax** (no mature exporter) | grammar; file resolver |
| Swift | **none** — disclosed as unsupported, reported per service by `list_services` | no | (corpus.yaml) | **none** | extractor (fleet demand: app-kit) |
| Lua | heuristic (`luaextract.go`) | no | (corpus.yaml) | **syntax** | grammar |
| Bash | heuristic (`bashextract.go`) | no | (corpus.yaml) | **syntax** | — |
| SQL | heuristic (`sqlextract.go`) | no | — | **syntax** | — |
| Terraform/HCL | pure Go (`hclextract.go`) | n/a | (corpus.yaml) | **syntax** (resource references) | — |
| Kubernetes YAML, Helm, Kustomize, docker-compose | pure Go | n/a | (corpus.yaml) | **syntax** (`service_ref` cross-repo edges) | — |
| Dockerfile, CMake, GitHub Actions, proto, GraphQL, OpenAPI, JSON, TOML, templ, Markdown, go.mod | pure Go | n/a | (corpus.yaml where listed) | **syntax** | — |

Disclosed as unsupported (named per service in `list_services`, never silently skipped; `internal/connector/unsupported_language.go`): Clojure, Crystal, Dart, Elixir, Elm, Erlang, F#, Groovy, Haskell, Julia, Nim, OCaml, Swift, Zig.

Bench coverage today: every scenario corpus has a Go subject; the Long Code Arena corpus
covers Python, Java and Kotlin bug localization. SU-23 (done) adds TypeScript
(`testdata/scenariobench/lookups-ts.json`, `nest`) and Python
(`testdata/scenariobench/lookups-py.json`, `fastapi`) lookups corpora, committed and
**not run** (ADR-041 decision 9) — both corpora's own `notes.typed_edges` records that
neither SU-9 nor SU-11 is wired yet, so their `full`-arm predictions are written against
today's heuristic (syntax) call edges only, never a typed tier. SU-12 adds the
free-tier typed-edge oracle per language.
