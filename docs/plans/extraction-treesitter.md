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

- [ ] wazero engine + grammar/query registry + TSExtractor implementing extractor.Extractor {id: XT-1, symbol: NewTSExtractor, package: internal/extractor}
- [ ] TypeScript/JS: grammar + queries + fixtures; calibration beats incumbent (scores recorded here) {id: XT-2, symbol: NewTSExtractor, package: internal/extractor, skip_test: true, skip: "language onboarding = data + calibration evidence; engine symbol already anchored by XT-1"}
- [ ] Python: same bar {id: XT-3, symbol: NewTSExtractor, package: internal/extractor, skip_test: true, skip: "same rationale as XT-2"}
- [ ] Ruby: same bar {id: XT-4, symbol: NewTSExtractor, package: internal/extractor, skip_test: true, skip: "same rationale as XT-2"}
- [ ] Registry/docs: per-language engine provenance (heuristic vs tree-sitter) surfaced in capabilities + docs; ExtractorEpoch bump per migrated language {id: XT-5, symbol: FingerprintHash, package: internal/fusion, skip_test: true, skip: "epoch + docs; pinned by the epoch invalidation test"}

## Deferred

- [-] Remaining languages of the 30-language program (java, c#, c/cpp, scala, php, lua upgrades...) {id: XT-6, disposition: deferred, reason: "ship three proven languages first; the engine makes each subsequent one data-only", trigger: "per-language, when an indexed repo materially uses it — calibration gate per language as always"}
- [-] Tree-sitter-derived call edges beyond symbols {id: XT-7, disposition: deferred, reason: "symbol truth first; call-edge queries per language are their own calibration problem", trigger: "first non-Go language where find_callers demand is evidenced by call-log queries"}
