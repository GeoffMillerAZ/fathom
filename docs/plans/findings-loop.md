# Findings loop — plan (vision 30, ADR-040)

Design: `docs/vision/30-findings-loop.md`. Decision: ADR-040. Counterpart plan:
platform-engineering `docs/plans/findings-projection.md` (RP-1 … RP-9).
Grammar: `docs/plans/README.md`. Rows are proven by `fathom verify`, never by
a checkbox. No benchmark is run by this plan; FL-22 commits the corpus only.

## A. Provenance substrate

- [x] Producer catalog: `schema/semantic/producers.cue` (`#Producer{id, kind, origin, tier_cap, requires}`) with the shipped producers (`fathom.review`, `reviewkit.system-review`, `reviewkit.security-review`, `reviewkit.review-import` origin fathom, `human.review`), loaded and validated at start; an unknown producer, a cap above `verified`, or a `fathom`-origin producer named for import is a named refusal {id: FL-1, symbol: LoadProducers, package: internal/semantic}
- [x] Served tier is `min(event tier, producer cap)`; tier never rises on import; pinned by a table test over every catalog kind {id: FL-2, symbol: CapTier, package: internal/semantic}
- [x] `ClaimEvent` verdict `annotated` (status-preserving; note stored, never served as rule text) added additively beside `refuted|corroborated|contradicted|re-anchored|superseded`; `ResolutionKind` gains `external-closure` and `revoked` {id: FL-3, symbol: Importer.ApplyEvent, package: internal/findings/external}

## B. P2 — the governed import

- [x] Projection decoder for the review kit's `#FathomProjectionOutbox` (schema_version 1) plus additive `anchor.start/end/symbol`; strict decode (unknown fields refuse); every validation failure names the event and the field {id: FL-4, symbol: ParseProjection, package: internal/findings/external}
- [x] Deterministic identity: `ClaimID("findings","finding","ext:<producer>/<org>/<repo>/<component>/<sourceId>", findingId)`, `PromptVersion "external-import/v1"`, `ReproKey = findingId`, `InputDigest = payloadDigest`; identical events reproduce identical IDs across runs {id: FL-5, symbol: ClaimFromEvent, package: internal/findings/external}
- [x] Lifecycle mapping: observation/fail → fresh; observation/pass and closure → superseded + `external-closure` event; revocation → superseded + `revoked`; annotation → `annotated` event; never `refuted` {id: FL-6, symbol: ApplyEvent, package: internal/findings/external, reachable_from: TestApplyEvent_DirectReach}
- [x] Per-claim cursor `external_findings(service_id, claim_id, producer, source_id, source_version, payload_digest, anchor_digest, imported_at)` — additive table on sqlite and MemoryStore; older version → stale, same version different digest → rejected `conflicting-replay`, same version same digest → duplicate {id: FL-7, symbol: PutExternalFinding, package: internal/store, skip_test: true, skip: "exercised through the Store interface contract suite (external_findings subtest, sqlite + MemoryStore) — interface-dispatched multi-impl calls are the documented O9 residue test_reaches refuses to resolve"}
- [x] Anchors: `scopes` path, `scope-fp` current store fingerprint, `cites` only when lines hash cleanly; an unreadable anchor file is recorded `unreadable` via the read manifest, never absent; `anchor_current` is recomputed at serve time from the current file digest {id: FL-8, symbol: AnchorLinks, package: internal/findings/external}
- [x] Subject binding: `[findings] subject` in `.fathom/config.toml` must equal the event subject; no mapping or a mismatch refuses before any read of the store {id: FL-9, symbol: BindSubject, package: internal/findings/external, reachable_from: TestBindSubject_DirectReach}
- [x] `fathom findings import --service --subject [--producer] --file [--dry-run] [--json]` emitting the receipt `{schema_version, delivered, duplicate, stale, rejected[], truncated, cursor}`; nothing written on a batch schema failure; `--dry-run` proves store-state equality {id: FL-10, symbol: cmdFindingsImport, package: cmd/fathom}
- [x] `pkg/client.ImportFindings` returns the identical receipt for the identical input the CLI takes {id: FL-11, symbol: ImportFindings, package: pkg/client}
- [x] Distinct reconciliation domains: native `StaleReviewClaims` never supersedes an `external-import/v1` claim and the importer never touches a `review-verb/v1` claim — one test with both populations in one store {id: FL-12, symbol: StaleReviewClaims, package: internal/analysis/review}
- [x] Isolation: a claim imported for service A is invisible to `ObligationsFor`, `findings near`, and both hooks for service B {id: FL-13, symbol: obligationsCore, package: internal/semantic}
- [x] Conformance fixtures `testdata/contracts/findings-import/*.json` (happy path, duplicate, out-of-order, conflicting replay, moved anchor, revocation, disagreement, fathom-origin refusal, unknown schema, unknown field) run through CLI and client by one test {id: FL-14, symbol: TestFindingsImportConformance, package: cmd/fathom, skip_test: true, skip: "the row's symbol IS the test"}

## C. P0 / P1 — the review contract and the review key

- [x] `fathom review`: `--mode report|persist` (report never writes; store-state equality pinned including the error path), `--manifest FILE` (explicit `[{path, sha256}]`; disagreement with the tree is a named refusal), `--idempotency-key` (same key+payload → recorded response; same key, different payload → refuse) recorded in additive table `review_runs` {id: FL-15, symbol: runReviewBody, package: cmd/fathom}
- [x] Response envelope v1: `schema_version`, `producer{name,version,build}`, `inputs{manifest_digest, source_watermark, index_watermark, rules_digest, plans_digest, extractor_versions}`, per-dimension `checked`+`reason`, `coverage{complete, unknown}`, `truncated`, `stale_index`, `cancelled`; process subjects accepted with `unanchored: true` (delivered as a top-level `unanchored_paths` list — see review.UnanchoredPaths — rather than a per-finding flag) {id: FL-16, symbol: BuildResponse, package: internal/analysis/review}
- [x] `fathom review-key`: one sha256 over sorted components (manifest, typed transitive closure at the index watermark, rules, plans, tooling, index identity) with `complete` and `unknown[]`; a missing/unreadable path is an unknown entry never an empty hash; identical inputs → identical key {id: FL-17, symbol: Compute, package: internal/analysis/reviewkey}
- [x] `review-key --prior KEY.json` reports `changed_since_prior[]` by component; mutation matrix test: source-only, transitive dependency, untracked file, rule pack, plan, tool version, index rebuild each flip the right component {id: FL-18, symbol: Diff, package: internal/analysis/reviewkey}
- [x] `pkg/client.ReviewKey` mirrors the CLI; fixtures `testdata/contracts/review-key/*.json` {id: FL-19, symbol: ReviewKey, package: pkg/client}

## D. P3 — serving, sync, promotion

- [x] Pre-edit hook block "open findings on this file" after rules and the enforcement verdict: ≤3 findings, ≤1,200 bytes, `omitted` always rendered, drifted anchors flagged, superseded/refuted never shown; post-edit block only on cited-line overlap (`finding_touched` beat); rendered through the harness codec so Codex gets the same block {id: FL-20, symbol: renderFindingsBlock, package: cmd/fathom}
- [x] Prompt hook one-line count inside the ADR-036 deadline; session brief `open findings: N (oldest …)` {id: FL-21, symbol: findingsBriefLine, package: cmd/fathom}
- [x] Outbox sync: `[findings] outbox` → session hook imports a changed outbox once per session using a digest cursor under the state dir and reports the receipt or the failure reason in the brief {id: FL-22, symbol: syncFindingsOutbox, package: cmd/fathom}
- [x] `fathom findings candidates [--min-sites 3] [--json]`: rule candidates from `DerivePatterns` over native and external findings — class, normalized shape, member citations, producers, code-change resolutions, drafted rule sentence and the ADR-038 check family; never writes a rule; `findings status` gains `candidates` and `disagreements` {id: FL-23, symbol: RuleCandidates, package: internal/analysis/findings}

## E. Measurement and adoption

- [x] Scenario corpus `testdata/scenariobench/findings-loop.json` (≥4 tasks, arms full vs control, paired criteria `finding_addressed` / `finding_cited`, kill criteria) and `task bench-scenario-findings`; committed, NOT run {id: FL-24, symbol: TestFindingsLoopCorpusParses, package: internal/scenariobench, skip_test: true, skip: "the row's symbol IS the test"}
- [x] `fathom init` scaffolds a commented `[findings]` section; `docs/configuration.md` documents it {id: FL-25, symbol: cmdInit, package: cmd/fathom}
- [x] Skills: `fathom-onboard` step 7 (the loop), new `fathom-findings` skill (import, inspect, candidates, refute) installed by `fathom init --skills` and reconciled by `fleet reconcile` {id: FL-26, symbol: installSkills, package: cmd/fathom}
- [x] Docs: `docs/adoption/findings-loop.md` (CI and local variants), `docs/use-cases/change-review.md` loop section with a `## Maintained` update, `docs/mcp-tools.md` note that no agent-surface tool grew, `docs/layers.md` findings row (external producers, tier caps), `docs/usage.md` verbs, `docs/vision/README.md` row 30, intent scope/futures/decisions, CHANGELOG {id: FL-27, symbol: TestMaintainedLinesNameRegisteredToolsAndVerbs, package: cmd/fathom, skip_test: true, skip: "docs row; the named test guards the Maintained contract"}
