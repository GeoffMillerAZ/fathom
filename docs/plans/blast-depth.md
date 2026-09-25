# Blast depth — depth buckets, edge confidence, and test-gap risk

Design: `docs/vision/26-proof-credibility.md` §3. No new tools (law 4):
additive response fields on `find_callers`, `references`, `blast_radius`,
`review_diff`; the CLI mirrors follow (law 10).

Verify: `fathom verify --service fathom --plan docs/plans/blast-depth.md`.

- [x] `EdgeConfidenceOf(edge)` classifies an edge `resolved` (typed pass) or `heuristic` (syntax-only pass) from the fusion provenance fathom already records {id: BD-1, symbol: EdgeConfidenceOf, package: internal/core}
- [x] `find_callers` and `references` results carry `depth` and `edge_confidence` per row and a `by_depth` summary (`withDepthBuckets`) {id: BD-2, symbol: withDepthBuckets, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] `blast_radius` groups its result by depth with per-bucket confidence counts (`blastRadiusByDepth`) {id: BD-3, symbol: blastRadiusByDepth, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] `review_diff` composes `test_impact`: `affected_tests` (tests that statically reach a changed symbol) and `test_gap` (changed symbols no test reaches) with file:line evidence (`reviewTestGap`) {id: BD-4, symbol: reviewTestGap, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] `fathom review` prints affected tests and the test gap (`renderTestGap`) {id: BD-5, symbol: renderTestGap, package: cmd/fathom}
- [x] `docs/mcp-tools.md`, the blast-radius and change-review use-case `## Maintained` lines updated; `TestMcpToolsDocMatchesRegistry` green {id: BD-6, oracle: lexical, path: docs/use-cases/change-review.md, pattern: "`affected_tests` and `test_gap`, BD-4", skip_test: true, skip: "docs; evidence is the named test"}
