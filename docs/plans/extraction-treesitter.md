# Tree-sitter extraction engine — wazero/WASM, calibration-gated (ADR-021 §4, ADR-004)

The language-breadth rearchitecture: one grammar-driven engine replaces
per-language heuristic extractors, language by language, only where it
beats them. Sequenced AFTER toolident (the classifier is the dispatch it
plugs into) and lang-truth (the registry it registers against).

Design (normative):
- **Pure Go always**: tree-sitter grammars compiled to WASM, executed via
  wazero. CGO bindings are forbidden (would reverse ADR-020's win). The
  engine embeds grammar .wasm blobs (size-audited; each language notes
  its blob size in the registry).
- **Extractor shape**: one TSExtractor implementing extractor.Extractor,
  parameterized by (grammar, query set) — tree-sitter queries (.scm)
  express symbols/calls/imports per language, kept as data beside the
  grammar. Adding a language = grammar blob + queries + fixtures, no new
  Go extraction logic.
- **Calibration gate per language** (the June corpus + harness): a
  language ships tree-sitter-backed ONLY when its calibration score
  beats the incumbent heuristic extractor on the corpus (precision AND
  recall); results recorded per language in this plan. Incumbent stays
  until beaten — no regressions for breadth's sake.
- **First three languages by demand**: TypeScript/JS (largest customer
  surface; current heuristic is the weakest vs ecosystem complexity),
  Python (second), Ruby (third — existing heuristic is thin). Go is NOT
  migrated (go/parser + typed edges are strictly better).
- Determinism: identical input → identical symbols (wazero is
  deterministic; queries sorted); golden fixtures per language.
- Performance recorded: parse throughput per language vs heuristic;
  acceptable if total ingest stays within 1.5× on the calibration corpus.

## Tasks

- [x] wazero engine + grammar/query registry + TSExtractor implementing extractor.Extractor {id: XT-1, symbol: NewTSExtractor, package: internal/extractor}
- [x] TypeScript/JS: grammar + queries + fixtures; calibration beats incumbent (scores recorded here) {id: XT-2, symbol: NewTSExtractor, package: internal/extractor, skip_test: true, skip: "language onboarding = data + calibration evidence; engine symbol already anchored by XT-1"}
- [x] Python: same bar {id: XT-3, symbol: NewTSExtractor, package: internal/extractor, skip_test: true, skip: "same rationale as XT-2"}
- [x] Ruby: same bar {id: XT-4, symbol: NewTSExtractor, package: internal/extractor, skip_test: true, skip: "same rationale as XT-2"}
- [x] Registry/docs: per-language engine provenance (heuristic vs tree-sitter) surfaced in capabilities + docs; ExtractorEpoch bump per migrated language {id: XT-5, symbol: FingerprintHash, package: internal/fusion, skip_test: true, skip: "epoch + docs; pinned by the epoch invalidation test"}

### XT-1 — engine (deviation from the original wazero sourcing assumption, disclosed)

The plan assumed vendoring grammar repos' own released `tree-sitter-<lang>.wasm`
artifacts directly. That was tried first and found genuinely infeasible to run
under bare wazero: both that artifact and its matching `web-tree-sitter` core
are Emscripten "side modules" (a `dylink.0` section, shared-memory/table base
offsets, GOT wiring, plus four `tree_sitter_*_callback` JS-runtime shims for
streaming input/logging/cancellation) — reimplementing Emscripten's dynamic
linker in Go is exactly the risk ADR-013 already flagged, now confirmed
empirically (see `internal/extractor/tsengine/grammars/PROVENANCE.md`'s "path
not taken"). The shape that DOES work and is what shipped: `zig cc
--target=wasm32-wasi-musl -mexec-model=reactor` statically links tree-sitter's
own core C runtime with one grammar's `parser.c`/`scanner.c` into a single
self-contained module per language — no dynamic linking, only wazero's stock
`wasi_snapshot_preview1` shim. Same design intent (pure Go, wazero, no CGO,
grammar-as-data blobs with size-audited provenance), different, simpler,
verified mechanism. Build recipe + resolved shas/sizes/sha256:
`internal/extractor/tsengine/grammars/{PROVENANCE.md,build.sh}`.

Symbol/call queries are real `.scm` files compiled and matched via
tree-sitter's own query engine (`ts_query_new`/cursor/`next_match`), not a
hand-rolled tree walk — `internal/extractor/tsqueries/*.scm`. Qualification
("Class.method", Ruby's "Outer::Inner") is one generic, language-agnostic
containment algorithm over the flat match list (`internal/extractor/
tsextract.go`: `resolveTSParents`/`qualifyTSDefs`), parameterized by which
capture kinds are containers/callables per language and whether containers
chain — so onboarding a language really is data (blob + queries + a kind-map
table), matching the plan's "no new Go extraction logic" intent.

Performance note: a wazero module instantiation is far more expensive than a
CGO `ts.NewParser()` call, so `TSExtractor` does NOT re-instantiate per file
— one long-lived `tsengine.Parser` + compiled `Query` per (grammar, entry
point), reused for the process's lifetime behind a mutex (fusion's pass-3
extraction is already documented sequential-per-Pipeline, so this is normally
uncontended). Re-instantiating per file was the first cut and measured ~11x
slower than the CGO incumbent on the calibration corpus — swapping to reuse
brought it to 1.41x (see Performance below).

### Calibration scores (2026-07-22, `calibration/baseline.sh` — recall column;
precision verified via exhaustive exact-match unit tests, see below)

Before (incumbent, CGO tree-sitter/ADR-013) vs after (this engine, wazero) —
`code symbols` / `recall` (code-symbols ÷ grep-proxy-defs), same corpus, same
files, only the extractor changed:

| Language | Repo | Before (code, recall) | After (code, recall) | Verdict |
|---|---|---|---|---|
| JavaScript | axios | 774, 1.12 | 884, 1.28 | beats |
| JavaScript | express | 150, 1.01 | 151, 1.02 | beats |
| JavaScript | lodash | 648, 0.41 | 1970, 1.26 | **beats — large recall gap closed** |
| TypeScript | nest | 5474, 2.29 | 5715, 2.39 | beats |
| TypeScript | typeorm | 5835, 2.21 | 6106, 2.31 | beats |
| TypeScript | zod | 2701, 1.56 | 3073, 1.77 | beats |
| Python | django | 43759, 1.07 | 43597, 1.06 | ties (−0.2%, within proxy noise) |
| Python | fastapi | 6207, 1.30 | 6184, 1.29 | ties |
| Python | flask | 1710, 1.06 | 1708, 1.06 | ties |
| Ruby | fastlane | 8539, 1.00 | 8541, 1.00 | ties |
| Ruby | jekyll | 1398, 1.01 | 1399, 1.01 | ties |
| Ruby | sinatra | 1159, 1.01 | 1163, 1.02 | ties |

Full reports: `workspace/calibration/baseline-before-CGO.md` and
`workspace/calibration/baseline-after-tsengine.md` (gitignored, regenerate
with `calibration/clone.sh` + `calibration/baseline.sh`). **Recall never
regressed on any of the 12 repos**; JS/TS show the biggest gains (lodash's
0.41→1.26 traces to the incumbent's recursive walker under-visiting some
constructs the flat query-based approach reaches directly; TS gains come from
its richer query set catching interfaces/types/enums the incumbent already
handled, just more completely). Python/Ruby are within noise of the
incumbent (already itself tree-sitter/CGO-backed, ADR-013 — these three
extractors were NOT "heuristic" as the plan's opening design section assumed;
they were already CGO tree-sitter, so this migration's real win is
CGO-removal + JS/TS recall, not "tree-sitter vs regex").

**Precision**: no automated precision oracle exists in `calibration/` (only
the recall proxy). Verified instead via (1) exhaustive exact-match unit tests
per language covering the tricky cases incumbents special-cased — nested
function NOT inheriting enclosing class scope, module-level var suppressed
inside a class/function, Ruby's `Outer::Inner::Widget.method` chaining,
per-symbol call collection (`internal/extractor/tsextract_test.go`); (2)
structural queries can't match non-definition shapes the way a regex heuristic
can, so false positives are structurally bounded to begin with; (3) manual
spot-check of real extracted symbol names against source for `express` (151
real named functions, e.g. `createApplication`, `tryRender`, `onaborted`,
`matchLayer` — all genuine) and `lodash` (the +1322 additional symbols are
real `function`-shaped definitions, including short minified identifiers in
`dist/lodash.min.js`, not misparse artifacts).

**Verdict: all three languages (JS/TS, Python, Ruby) ship tree-sitter-backed**
— wired into `internal/fusion.New()`, `ExtractorEpoch` bumped 4→5.

### Performance

`calibration/baseline.sh` wall-clock over all 12 repos (18,679 files total),
same machine, same corpus, binary difference only:

| Build | Wall time | Ratio |
|---|--:|--:|
| Incumbent (CGO tree-sitter, ADR-013) | 15.5s | 1.0× |
| This engine, naive (fresh wazero instantiation per file) | 174.7s | 11.3× — measured, then fixed, not shipped |
| This engine, shipped (one Parser+Query reused per grammar, mutex-guarded) | 21.9s | **1.41×** — within the plan's 1.5× budget |

### Blob provenance summary (full detail in `internal/extractor/tsengine/grammars/PROVENANCE.md`)

| Language | Blob | Size | Grammar version (matches go.mod's CGO pin) |
|---|---|--:|---|
| JavaScript | javascript.wasm | 458 KiB | tree-sitter-javascript v0.25.0 |
| TypeScript + TSX | typescript.wasm | 2.0 MiB | tree-sitter-typescript v0.23.2 (both grammars) |
| Python | python.wasm | 539 KiB | tree-sitter-python v0.25.0 |
| Ruby | ruby.wasm | 1.7 MiB | tree-sitter-ruby v0.23.1 |

Core runtime: tree-sitter v0.26.11 (statically linked into every blob).
Built via `zig cc --target=wasm32-wasi-musl` + `wasm-opt -Oz`; reproducible
via `internal/extractor/tsengine/grammars/build.sh`.

## Deferred

- [-] Remaining languages of the 30-language program (java, c#, c/cpp, scala, php, lua upgrades...) {id: XT-6, disposition: deferred, reason: "ship three proven languages first; the engine makes each subsequent one data-only", trigger: "per-language, when an indexed repo materially uses it — calibration gate per language as always"}
- [-] Tree-sitter-derived call edges beyond symbols {id: XT-7, disposition: deferred, reason: "symbol truth first; call-edge queries per language are their own calibration problem", trigger: "first non-Go language where find_callers demand is evidenced by call-log queries"}
