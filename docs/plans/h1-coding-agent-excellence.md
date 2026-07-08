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

## Open (claims land as the work does)

- [ ] Claims schema: semantic_claims/claim_links/compile_runs/claim_events [ADR-016] {symbol: PutClaims}
- [ ] Obligations compile with anchor verification [ADR-016] {symbol: CompileObligations}
- [ ] Post-ingest citation sweep flips stale claims [ADR-016] {symbol: SweepClaims}
- [ ] Obligations serving tool, zero-output-on-clean [ADR-016] {symbol: registerObligations, skip_test: true}
- [ ] PostToolUse edit-hook injection {symbol: cmdEditHook, reachable_from: runWithStdin, skip_test: true}
- [ ] Non-git expert module end-to-end (findings corpus)
