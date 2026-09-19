# The substrate round — plan (vision 31, ADR-041)

Design: `docs/vision/31-substrate.md`. Decision: ADR-041 (the owner's answers of
2026-09-19). Tracker for languages: `docs/languages.md`. Grammar: `docs/plans/README.md`.
Rows are proven by `fathom verify`; every round below ends with a free-tier measurement
(ADR-032) and no benchmark is run by this plan.

## A. Hygiene and freshness (ADR-041 decisions 3, 4, 2)

- [ ] Indexes on every column a serve path filters: semantic_claims(service_id, layer, status), claim_links(service_id, claim_id, rel), typed_edges by (service_id, to_symbol) and (service_id, from_symbol), observations(service_id, source_ref), external_findings(service_id, claim_id) — created idempotently at open; an EXPLAIN QUERY PLAN test pins index use on the obligations and find_callers queries {id: SU-1, symbol: EnsureIndexes, package: internal/store}
- [ ] Claims are prefiltered in SQL (layer, served statuses, scope prefix) before Go scoring; `obligationsCore` consumes the prefilter and its results are byte-identical to the full-scan path on the existing goldens {id: SU-2, symbol: ClaimsForServe, package: internal/store}
- [ ] A dispatch-level response budget wraps every tools/call so no handler can return more than the byte budget without the elision block; a synthetic oversized handler proves it {id: SU-3, symbol: budgetedHandler, package: internal/mcp}
- [ ] The call-graph cache invalidates per package (the granularity typed edges already use) instead of per watermark, and a single-file re-ingest touches only its package's entry; a test proves a second query after a one-file change does not rebuild the whole graph {id: SU-4, symbol: InvalidatePackage, package: internal/analysis/callgraph}
- [ ] The daemon refreshes one file on request: re-extracts its symbols and syntax edges in place, keeps typed edges package-granular, records the refreshed fingerprint, and reports what it refreshed {id: SU-5, symbol: ReingestFile, package: internal/fusion, reachable_from: TestReingestFile_ReplacesOneFile}
- [ ] The pre-edit hook asks for that refresh when the edited file's content hash differs from the index, answers from the refreshed index, and names what was refreshed in the staleness line; measured p50 under 150 ms on a synthetic file in the hook harness {id: SU-6, symbol: ensureFileFresh, package: cmd/fathom}
- [ ] The prompt hook ignores system notifications and only resolves path-shaped tokens that exist in the index or the tree; the notification text that produced `publication/observation/__prompt__.go` is a regression fixture {id: SU-7, symbol: isSystemNotification, package: cmd/fathom}
- [ ] Embeddings are removed: `internal/vector`, the `WithVector` option and the hybrid-rank vector branch are deleted with the agentic-kit embedding-client import; `docs/architecture.md` describes the lexical and graph paths that exist; a test refuses any reintroduction of a vector option on the query engine {id: SU-8, symbol: TestQueryEngineHasNoVectorOption, package: internal/query, skip_test: true, skip: "the row's symbol IS the test"}

## B. Typed edges beyond Go (decision 5)

- [ ] SCIP import: a SCIP index (`--scip PATH`, or auto-detected `index.scip` / `.fathom/scip/*.scip`) maps definition and reference occurrences to `typed_edges` (kind direct; unresolved symbols reported), replaced per package, with the producing indexer's identity and version recorded and reported by ingest; a committed fixture index built by a Go helper from a tiny TypeScript project proves the mapping offline {id: SU-9, symbol: ImportIndex, package: internal/scip, reachable_from: TestImportIndex_MapsOccurrencesToTypedEdges}
- [ ] CUE typed edges natively via `cue/load`: imports, definition references and field references become typed edges for `.cue` corpora (this repo's guards, layers and schemas are the first subject); fixture-pinned {id: SU-10, symbol: EnrichCUEEdges, package: internal/fusion, reachable_from: TestEnrichCUEEdges_Fixture}
- [ ] Per-file resolver for TypeScript/JavaScript and Python where no SCIP index runs: import graph plus module-scope name resolution from the tree-sitter queries, edges labelled `file-resolved` at tier pre-scan and never presented as typed; fixture-pinned {id: SU-11, symbol: ResolveFileEdges, package: internal/fusion, reachable_from: TestResolveFileEdges_TypeScriptImports}
- [ ] Free-tier L0 oracle for typed edges per language: `indexbench scip-oracle` compares fathom's edges for a calibration subject against a SCIP index (precision, recall, unresolved) and `task bench-l0-scip -- <language>` runs it; an offline fixture run is a test {id: SU-12, symbol: ScipOracle, package: internal/indexbench}
- [ ] `docs/languages.md` is kept true by a test: every extractor in the registry has a row, and every row's typed-edge status matches what the code provides; `fathom capabilities` reports typed-edge producers per language {id: SU-13, symbol: TestLanguagesTrackerMatchesRegistry, package: internal/connector, skip_test: true, skip: "the row's symbol IS the test"}

## C. Localization (decision 6)

- [ ] `localize` expands seeds through the typed neighbourhood to depth 2, ranks by graph distance then evidence, reformulates when seeds are empty (identifier splitting and synonyms from the concern), and caps its payload at the tool budget; the LCA corpus's `max_turns` and Kotlin index notes are re-checked so the corpus is runnable on the new instrument {id: SU-14, symbol: expandSeedsByGraph, package: internal/mcp}

## D. Findings learning telemetry (decision 8)

- [ ] Rule candidates decay on dispute, weight members by CODEOWNERS ownership, and cap their cadence; the promoted-rule draft carries a `promoted_from` provenance tag {id: SU-15, symbol: CandidatePolicy, package: internal/analysis/findings}
- [ ] Post-promotion rule telemetry from hook beats and the call log: served, touched, disputed, last fired per claim; `findings status` shows rules that have not fired in 30 days {id: SU-16, symbol: RuleTelemetry, package: internal/analysis/findings}
- [ ] `fathom claim add --file --scope --rule` records one human-authored scoped claim (producer `human.review`, tier capped at sme-stated, content-addressed like every other claim) {id: SU-17, symbol: cmdClaimAdd, package: cmd/fathom}
- [ ] `fathom claim refute --dry-run` previews the disposition: which served rows it would withhold, for which files, before anything is recorded {id: SU-18, symbol: previewRefute, package: cmd/fathom}

## E. Claims gain time and identity (decision 7)

- [ ] Bi-temporal validity on claims: additive `valid_from` / `valid_to` beside compiled-at; supersede and external closure set `valid_to`; `obligations` accepts `as_of <commit>` and serves what was valid then {id: SU-19, symbol: ObligationsAsOf, package: internal/semantic}
- [ ] Every supersede (native review, external closure, sweep) writes a `supersedes` claim link from the successor to the predecessor, and the sweep can walk the chain {id: SU-20, symbol: WriteSupersedes, package: internal/semantic}
- [ ] Claims anchored to a resolved symbol carry an `anchors` link to its symbol key beside the line span; a rename re-anchors the span through `rename_impact` instead of marking the claim drifted {id: SU-21, symbol: AnchorSymbol, package: internal/semantic}
- [ ] Every claim and event records the activity that asserted it (compile run id, review idempotency key, import cursor, or hook session), served as `asserted_by` {id: SU-22, symbol: AssertedBy, package: internal/core}

## F. Benchmarks runnable and unrun (decision 9); gate discipline (decision 10)

- [x] TypeScript and Python lookups corpora on the calibration subjects (nest, fastapi; pinned shas), ≥ 6 tasks each with oracles that do not name the gold file, arms fathom vs grep, committed and NOT run {id: SU-23, symbol: TestLanguageLookupsCorporaParse, package: internal/scenariobench, skip_test: true, skip: "the row's symbol IS the test"}
- [ ] Every corpus passes `scenariobench plan` in a test, and `docs/bench/README.md` gains a "Next round" section naming the order (guards, findings-loop, lookups Go, lookups TypeScript, lookups Python, LCA) with cost estimates from the ledger {id: SU-24, symbol: TestAllCorporaPlan, package: internal/scenariobench, skip_test: true, skip: "the row's symbol IS the test"}
- [ ] agentic-kit's `evidence-producer` is vendored into the mirror and `internal/semantic/producers.go` delegates cap, admit and independence to it; a test cross-checks the two CUE catalogs {id: SU-25, symbol: CatalogFromKit, package: internal/semantic}
- [ ] Pushes are gated by construction: `scripts/push-check.sh` refuses when `.git/hooks/pre-push` is not the lefthook hook, `task push` runs it, and `scripts/push-check_test.sh` is in scripts-check (invariant I40) {id: SU-26, symbol: TestPushCheckScript, package: cmd/fathom, skip_test: true, skip: "shell script; guarded by scripts-check"}
