# Knowledge retrieval: the plan (vision 38, ADR-049)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/knowledge-retrieval.md` proves it.
Design: `docs/design/knowledge-retrieval.md` (section numbers below are its). Goal G20
(KR-M1, KR-M2). FTS5 is supported by the pure-Go driver in `go.mod` (`modernc.org/sqlite`
v1.52.0, SQLite 3.53.2; probed 2026-09-27). No embeddings (vision 31 stands).

## Wave 2: the index, ranking, links and drift

- [x] `docs_fts` (FTS5, `porter unicode61`), `doc_units` and `doc_links`, additive to store format v0; the store stays pure Go under `CGO_ENABLED=0` (§4) {id: KR-1, symbol: PutDocUnits, package: internal/store, reachable_from: TestDocUnitsFTS5RoundTrip, needs: "responsible-tenant.md#RT-M1"}
- [x] The section splitter: heading path, line range and kind per unit; ADR sections split at bold labels; plan rows as their own units (§3) {id: KR-2, symbol: SplitDocUnits, package: internal/docindex, reachable_from: TestSplitDocUnitsHeadingPaths}
- [x] The corpus: docs, ADRs, intent, plans, design and vision, other Markdown, memories and symbol doc comments; archive and tmp flagged `stale_by_location` (§3) {id: KR-3, symbol: docCorpusFiles, package: internal/docindex, reachable_from: TestDocCorpusIncludesAndFlags, needs: "KR-2"}
- [x] Hash-gated doc ingest: only changed files re-index; deleted files' units removed in the same transaction {id: KR-8, symbol: IngestDocUnits, package: internal/docindex, reachable_from: TestIngestDocUnitsHashGated, needs: "KR-1,KR-3"}
- [x] BM25 (heading weighted 3:1) fused with concern ranking by reciprocal rank fusion (k = 60), `LIMIT 50`, snippets and receipts (§5) {id: KR-4, symbol: RankDocs, package: internal/docindex, reachable_from: TestRankDocsFusion, needs: "KR-8"}
- [x] `search` gains `kind: docs` (no new tool, law 4); `docs/mcp-tools.md` and the use-case `## Maintained` line updated in the same change {id: KR-5, symbol: searchDocs, package: internal/mcp, reachable_from: TestSearchDocsKind, needs: "KR-4"}
- [x] Doc-to-code links: backticked symbols, paths and contract keys resolved exactly, target hash stored at link time (§6) {id: KR-6, symbol: LinkDocSpans, package: internal/docindex, reachable_from: TestLinkDocSpansExactOnly, needs: "KR-8"}
  <!-- library landed 8a89a91e; wired: internal/fusion's indexDocs links every unit IngestDocUnits (re)indexes (docindex.WithOnWrite → LinkUnits with NewStoreResolver, the same resolution StalenessDocDrift re-checks) and syncs doc_links (store.SyncDocLinksForUnit); Report.DocIndex.LinksWritten counts them. docLinkPassVersion is folded into the unit hash gate, so pre-pass units are re-indexed and linked once and unchanged units are never re-linked. Pipeline proof: TestIngestLinksDocSpans. -->
- [x] Doc drift: a changed link target marks the span possibly stale; `staleness_report` gains a `doc_link_drift` section (beside SR-28's `doc_drift`) with the changing commit {id: KR-7, symbol: renderDocLinkDrift, package: internal/mcp, reachable_from: TestStalenessReportDocLinkDrift, needs: "KR-6"}
  <!-- library landed 37e90dc2 (docindex.docDrift, StalenessDocDrift); live since KR-6 writes doc_links at ingest. TestStalenessReportDocLinkDrift drives the real path: ingest a git repo, change the linked symbol in a commit, re-ingest, and doc_link_drift names the span and that commit; the fake-backend shape test is TestStalenessReportDocLinkDriftShape. -->
- [x] Per-service byte budget on the FTS index with a named `elided` past it {id: KR-9, symbol: enforceDocBudget, package: internal/docindex, reachable_from: TestDocIndexBudgetNamesElision, needs: "KR-8"}
- [x] The doc-question bench slice: at least 30 questions whose answers live in docs, criterion registered before the run {id: KR-10, oracle: lexical, path: docs/bench/scenarios/doc-questions.md, pattern: "^Criterion registered", needs: "KR-5"}

## Wave 4: dependency docs

KR-11 reads the cache directory through a `DepCacheLocator` port (`internal/docindex/depdocs.go`);
the dependency plane's cache locator (`system-graph.md#SG-36`) is its production implementation
and the API indexer (SG-37) the caller of `IngestDepDocs`. Both rows are proven at the library
and tool seams against fixture caches laid out like the toolchains' own; until SG-36 and SG-37
land nothing fills the index in a running daemon, and a scoped search answers `not_indexed` by name.

- [ ] Dependency doc units from the pinned version in the local cache (Go doc comments, `.d.ts` JSDoc, docstrings, Rust `///`, the package README), never fetched (I56) {id: KR-11, symbol: depDocUnits, package: internal/docindex, reachable_from: TestDepDocUnitsFromCache, needs: "system-graph.md#SG-37"}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-29): depDocUnits is reached only through docindex.IngestDepDocs, and no product path calls that until the dependency plane's indexer (system-graph.md SG-37) lands; KR-12's search answers from whatever that indexer writes. Re-opened, not waived, for the reason CX-23 gives. -->
- [x] `search kind: docs` scoped to a `pkg:` key answers from that version's docs {id: KR-12, symbol: searchPkgDocs, package: internal/docindex, reachable_from: TestSearchPkgDocsVersionExact, needs: "KR-11"}

## Refused (recorded so they are not re-proposed)

- [-] Embeddings or a vector index for docs {id: KR-X1, disposition: refused, reason: "vision 31's kill criterion stands; KR-M1 is lexical and promotes nothing in F13"}
- [-] Linking bare words outside code spans {id: KR-X2, disposition: refused, reason: "too noisy to link; exact targets only (§6)"}
- [-] A separate search service or index file {id: KR-X3, disposition: refused, reason: "one store (ADR-041); FTS5 lives in it"}

## Measurements (op)

- [ ] (op) KR-M1: the doc-question slice: fathom's pass rate at least grep's at no more than 1.0x grep's bytes {id: KR-M1, oracle: lexical, path: docs/bench/readings/knowledge/doc-questions.md, pattern: "^KR-M1 verdict: PASS", needs: "KR-10"}
- [ ] (op) KR-M2: doc drift on 20 seeded symbol changes: recall at least 0.9, no drift on 20 unchanged symbols {id: KR-M2, oracle: lexical, path: docs/bench/readings/knowledge/doc-drift.md, pattern: "^KR-M2 verdict: PASS", needs: "KR-7"}
- [ ] (op) KR-M3: on this repository the FTS index is below 15% of indexed text and an incremental docs ingest is below 2 s {id: KR-M3, oracle: lexical, path: docs/bench/readings/knowledge/footprint.md, pattern: "^KR-M3 verdict: PASS", needs: "KR-8"}
