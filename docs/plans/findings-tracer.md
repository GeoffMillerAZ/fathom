# Findings-as-claims tracer — D3, the cheapest test of the expert-system idea

docs/vision/03 §5 build-now list, sequenced after M1 (done): unlock
Kind:"finding" end-to-end on the existing claims stratum, wire the dead
claim_events triage machinery with the durability rule, add the
deterministic bridge and the remediation-drift serving flag. No new store,
no hosted anything — the reserved slot gets its first real consumer.

## The findings layer

- [x] A findings #Layer instance lands in the manifest (corpus: findings/**; kind finding; tier sme-stated) and compiles through the same engine — layer selection reaches the compiler so a non-obligations layer uses its own prompt and kind {id: FT-1, symbol: LoadLayer, package: internal/semantic, reachable_from: TestLoadLayer_Findings}
- [x] Kind-aware serving: rules payloads carry kind, and finding-kind claims served for a file are labeled as findings, not obligations {id: FT-2, symbol: ObligationsFor, package: internal/semantic, reachable_from: TestAdoptionServiceScoped}

## Triage (the dead machinery lives)

- [x] fathom claim refute and corroborate append ClaimEvents; refute flips status to refuted {id: FT-3, symbol: cmdClaim, package: cmd/fathom}
- [x] Refutation durability: a recompile that re-emits an unchanged rule under a refuted ClaimID retains refuted status — suppressed findings never resurrect silently {id: FT-4, symbol: PutClaims, package: internal/store, skip_test: true, skip: "PutClaims is exercised through the Store interface contract suite (claims_refute_durability subtest, DuckDB in-memory + on-disk + MemoryStore) — interface-dispatched multi-impl calls are the documented O9 residue test_reaches honestly refuses to resolve"}

## The deterministic bridge + drift

- [x] fathom report writes one analyzer's output as a markdown finding document into the corpus directory (start: security_rollup shape) so deterministic results flow through the normal compile path {id: FT-5, symbol: cmdReport, package: cmd/fathom}
- [x] Remediation drift: a second sweep pass flags finding claims whose scoped code changed since compile (serving annotation, never a status flip); scope-file fingerprints persisted at compile for non-git corpora {id: FT-6, symbol: Sweep, package: internal/semantic, reachable_from: TestAdoptionServiceScoped}

## Deferred

- [x] LLM-free compile path for deterministic findings (tier verified end-to-end) — trigger fired: the review-verb's C2-1 findings bridge (docs/plans/review-verb.md) is that first real review engagement, persisting review_diff findings directly as tier-`verified` claims (no LLM, no report/compile round trip) via `review.AttachClaims`, with refutation durability riding the store's existing PutClaims upsert {id: FT-7, symbol: AttachClaims, package: internal/analysis/review}
- [-] /rpc/expert surface registration {id: FT-8, disposition: deferred, reason: "spec-only until a non-code module is served to someone", trigger: "first external consumer of a findings store"} <!-- trigger FIRED 2026-09-18: the review kit (platform-engineering kits/review-kit) is that consumer; delivered as vision 30 / ADR-040 / docs/plans/findings-loop.md, as an import + serving loop rather than an /rpc/expert surface -->
