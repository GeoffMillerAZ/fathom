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
- [x] Done-means-proven gate function {symbol: Gate}
- [x] fathom verify CLI, HTTP-first with store fallback {symbol: cmdVerify, reachable_from: runWithStdin, skip_test: true}
- [x] Repo-relative anchors from the local connector [ADR-017] {symbol: Discover, skip_test: true}
- [x] Watermark SPI — domain-neutral freshness [ADR-017] {symbol: manifestWatermark, skip_test: true}
- [x] Legacy-anchor self-heal on ingest [ADR-017] {symbol: PurgeService, skip_test: true}
- [x] Store root remap CLI [ADR-017] {symbol: cmdRemap, reachable_from: runWithStdin, skip_test: true}
- [x] Portability round-trip proven {symbol: TestStorePortability_RoundTrip, skip_test: true}

## Shipped — semantic layers v1 (the obligations tracer bullet)

(skip_test on method-reached symbols cites dogfood O6 — the test_reaches
method-edge gap; the tests exist, the static edge doesn't yet.)

- [x] Claims schema: semantic_claims/claim_links/compile_runs/claim_events [ADR-016] {symbol: PutClaims, skip_test: true}
- [x] Obligations compiler with compile-time anchor + scope verification [ADR-016] {symbol: Compile, skip_test: true}
- [x] Post-ingest citation sweep flips stale claims [ADR-016] {symbol: Sweep}
- [x] Obligations serving, zero-output-on-clean [ADR-016] {symbol: ObligationsFor}
- [x] Obligations + compile_layer MCP tools on their surfaces [ADR-016] {symbol: registerSemanticTools, skip_test: true}
- [x] PostToolUse edit-hook injection {symbol: cmdEditHook, reachable_from: runWithStdin, skip_test: true}
- [x] Non-git expert module end-to-end (findings corpus) {symbol: TestExpertModule_NonGitCorpus_EndToEnd, skip_test: true}
- [x] Precision eval harness with scope traps [ADR-016] {symbol: TestObligationsEval_Precision, skip_test: true}

## Open

- [ ] Obligations layer compiled on the fleet store + injection coverage ≥90% (operator: migration step 4/4b, then 4-week gate)
- [ ] O6: method-call edge attribution in goextract, then remove skip_test workarounds above
