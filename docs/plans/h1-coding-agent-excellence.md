# H1 — coding-agent excellence + expert modules (the dogfood plan)

The first contract-bearing plan (grammar: `docs/design/coding-agent-excellence.md`
§4.2; convention: `docs/plans/README.md`). Checked (`[x]`) items are **claims**
and are verified at pre-push/CI by `fathom verify`; open (`[ ]`) items don't
gate until claimed.

## Shipped this phase

(The adoption-instrument repair — scripts/fathom-adoption.sh aliasing, weekly
rollup, call-log summary — shipped too, but shell-script deliverables have no
index-provable contract, so it is recorded in CHANGELOG.md, not claimed here.)

- [x] Checkbox semantics in plan parsing [ADR-014] {symbol: ParseMarkdownPlan}
- [x] Done-means-proven gate function {symbol: Gate, package: internal/verify}
- [x] fathom verify CLI, HTTP-first with store fallback {symbol: cmdVerify, reachable_from: runWithStdin}
- [x] Repo-relative anchors from the local connector [ADR-017] {symbol: LocalConnector.Discover, skip_test: true, skip: "interface-dispatched in contract tests (SPI with multiple impls) — static resolution honestly refuses multi-impl method calls"}
- [x] Watermark SPI — domain-neutral freshness [ADR-017] {symbol: LocalConnector.Watermark, skip_test: true, skip: "interface-dispatched in contract tests (SPI with multiple impls) — static resolution honestly refuses multi-impl method calls"}
- [x] Legacy-anchor self-heal on ingest [ADR-017] {symbol: SQLiteStore.PurgeService, skip_test: true, skip: "interface-dispatched in contract tests (SPI with multiple impls) — static resolution honestly refuses multi-impl method calls; re-anchored from DuckStore.PurgeService when SS-8/ADR-025 deleted that backend"}
- [x] Store root remap CLI [ADR-017] {symbol: cmdRemap, reachable_from: runWithStdin}
- [x] Portability round-trip proven {symbol: TestStorePortability_RoundTrip, skip_test: true, skip: "the test IS the anchor; nothing reaches a test"}

## Shipped — semantic layers v1 (the obligations tracer bullet)

(historical note: these anchors carried skip_test citing dogfood O6 until
the H1 handoff fix landed; the waivers are gone and the old note below is
retained only for context — the test_reaches
method-edge gap; the tests exist, the static edge doesn't yet.)

- [x] Claims schema: semantic_claims/claim_links/compile_runs/claim_events [ADR-016] {symbol: SQLiteStore.PutClaims, skip_test: true, skip: "interface-dispatched in contract tests (SPI with multiple impls) — static resolution honestly refuses multi-impl method calls; re-anchored from DuckStore.PutClaims when SS-8/ADR-025 deleted that backend"}
- [x] Obligations compiler with compile-time anchor + scope verification [ADR-016] {symbol: Compile}
- [x] Post-ingest citation sweep flips stale claims [ADR-016] {symbol: Sweep}
- [x] Obligations serving, zero-output-on-clean [ADR-016] {symbol: ObligationsFor}
- [x] Obligations + compile_layer MCP tools on their surfaces [ADR-016] {symbol: registerSemanticTools}
- [x] PostToolUse edit-hook injection {symbol: cmdEditHook, reachable_from: runWithStdin}
- [x] Non-git expert module end-to-end (findings corpus) {symbol: TestExpertModule_NonGitCorpus_EndToEnd, skip_test: true, skip: "the test IS the anchor; nothing reaches a test"}
- [x] Precision eval harness with scope traps [ADR-016] {symbol: TestObligationsEval_Precision, skip_test: true, skip: "the test IS the anchor; nothing reaches a test"}

## Open

- [x] Obligations layer compiled on the fleet store + injection coverage ≥90% (operator: migration step 4/4b, then 4-week gate) {id: H1-17, skip_test: true, skip: "operator row; evidence is the instrument: fathom instrument on 2026-08-25 read injection coverage 2042/2067 = 99% over the 168h window (gate ≥90%), obligations compiled and injecting on the fleet store"}
- [x] O6: method-call edge attribution in goextract (H1 handoff branch, merged 518e799: dedup + method edges + ctor-scoped inference + alias canonicalization); skip_test workarounds above removed — claims now prove test-reachability {symbol: TestImpact, package: internal/analysis/callgraph}
