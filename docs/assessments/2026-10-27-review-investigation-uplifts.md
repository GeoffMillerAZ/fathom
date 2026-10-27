# Review, investigation and knowledge-search uplifts for an adopting platform team

Status: design review, 2026-10-27, read against fathom at `01ea58c6b`. It proposes plan rows and does not add them to any plan file.
The consumer is an adopting platform team that wants three things:
- organization-wide Well-Architected, operational-excellence and governance reviews;
- a readiness inventory fed by those reviews and by investigations;
- a review kit as the tool that judges.

That team's records stay private (ADR-011).

## Verdict

The design for this consumer already exists and is sound: ADR-069's platform amendment (`ADR.md:5058`), the PLT- plan and
the WAE- plan. The gap is not missing design. It is three things:

1. **Fathom is not yet reliable enough to be read during a review.** The adopter recorded abandoned `read_source` calls
   and `find_symbol` timeouts on its operational symbols during a readiness assessment. NP-P0's deadline-abandon work
   (`docs/assessments/2026-10-26-next-phase-plan.md:52`) is the highest-value uplift for these goals, ahead of any new
   output.
2. **The one output the consumer is built to read is not on a schedule that beats its own kill.**
   - The output is a fact receipt per subject with a basis.
   - The chain is WAE-4 → WAE-5 → EVX-9 → WAE-10 → PLT-7 (`docs/plans/well-architected-evidence.md:33-34,45`,
     `docs/plans/platform-engineering.md:40`).
   - WAE wave 2 is gated on language wave lf8 (`docs/plans/well-architected-evidence.md:12-13`).
   - PLT-M4's kill date is 2027-01-31 (`docs/plans/platform-engineering.md:53`), about 14 weeks out.
   - On the consumer side, every fathom-facts row is open. Its records still list two fathom fixes as blocking that
     shipped weeks ago: WAE-1 and WAE-2 (`docs/plans/well-architected-evidence.md:27-28`).
3. **Four honesty gaps would let a review consumer read "clean" where fathom saw nothing.** They are small, they need
   no table, and they are not in any plan (U3 to U5).

For governance, investigation and knowledge search, fathom already has most of what it needs to supply. What is
missing is a recipe, a skill (PLT-10, unbuilt), or a stale parameter that should be withdrawn. Nothing here needs a new
MCP tool (ADR-053, PLT-X8), a new table (the freeze until RT-M1 PASS), or a verdict.

## 1. Intent read before designing

- **Invariants.**
  - I38 (`docs/intent/invariants.md:68`): no conclusion the inputs could not earn.
  - I25 (`:22`): unreadable files are named.
  - I56 (`:85`): no fetch on fathom's own authority.
  - I58 (`:87`): nothing ranks people.
  - I62 (`:90`): a judgment is never a fact.
- **Decisions.**
  - ADR-069 decision 5 (`ADR.md:4940`): the review kit judges, the tierer tiers, the policy kit rules, and fathom has
    no readiness verb.
  - ADR-069's PLT amendment (`ADR.md:5058`): no question map, the inventory is a receipt per subject, and deltas are
    information only.
  - ADR-040: fathom is never the findings record.
  - ADR-053: no net tool growth.
  - ADR-078 D5 (`ADR.md:6018`): fathom never schedules, places or monitors work. This covers continuous review
    cadences.
- **Non-goals** (`docs/intent/scope/non-goals.md`):
  - no domain connectors (`:7`);
  - no web-scale search or embeddings-first chat (`:14`);
  - no hosted multi-tenant backend (`:17`);
  - no incident command center (`:21`);
  - no dispatch loop (`:27`).
- **Table freeze.** RT-M1 read FAIL a second time (`docs/assessments/2026-10-26-next-phase-plan.md:34`). Every row
  below adds no table, column, cache, memo file or background job. Where a row might need one, it says so and waits.
- **Triggers checked.**
  - Not fired: PLT-D3's packet directory (`docs/plans/platform-engineering.md:59`, no reviewer seen assembling one)
    and PLT-D9's question map (`:65`, the adopter runs a review kit).
  - PLT-D8 (`:64`) has not fired either: the adopter's review kit imports scanner SARIF itself.

## 2. Inventory by review type

What each review type has today, what is built but has no product entry point, what is planned, and the gap. "Served"
means a CLI verb, MCP tool, HTTP route or hook reaches it.

| Review type | Served today | Built, not wired | Planned (rows) | Gap that matters to the consumer |
|---|---|---|---|---|
| Reliability pillar | `K8sFact` rows as symbols `<workload>#<key>` (`internal/extractor/k8sfacts.go:58`); observed-by in `symbol_context` (TL-15); `read_source ref` (WAE-3); `conserve --concern-deltas` (PLT-5, `cmd/fathom/conserve.go:92`) | none | WAE-4 to WAE-7, WAE-8/9/12, WAE-10, PLT-7, PLT-4 | No fact sheet with anchors and coverage, and no fact receipt (`docs/use-cases/platform-engineering.md:203-210`). Undecodable YAML documents, Helm templates included, are skipped silently (`internal/extractor/k8sextract.go:32,113-129`), which is U4 |
| Security pillar | IAM, Kyverno, Gatekeeper, Rego and Azure Policy dialect entities; findings loop for imported scanner SARIF (ADR-040) | none | WAE-14 security joins (`docs/plans/well-architected-evidence.md:44`), WAE-M6 | The adopter's security facts are typed in by hand. WAE-14 is the supply, and it waits on WAE-M2 |
| Operational excellence | GitHub Actions and other CI dialects; doc links; `localize`, `culprit`, `review --since` | none | WAE-6 alert annotations, WAE-7 rollout and shutdown facts | No join from an alert's runbook link to an indexed document. Deferred as RVU-D1 behind PLT-M0's ranking |
| Performance, cost, sustainability | resource requests and limits, replicas (`K8sFact`) | none | WAE-7; PLT-D7 resource attributes (deferred) | Mostly attestation-only or deployed state. PLT-M0's census (`docs/plans/platform-engineering.md:33`) is the honest instrument, and fathom should not widen here before it reads |
| Governance | `verify --format sarif`, `intent audit --format sarif`, `config audit --format sarif`, `obligations`, guards, platform modules, `trace export`/`trace verify` (`cmd/fathom/trace.go:66,118`), `certify gaps` | none | none for this consumer | No recipe names these as governance producers a review kit can import. The review kit already shells out to `verify` and `certify gaps`. U6 |
| Readiness inventory | none as a receipt; `list_services`, `fleet report` | `pkg/platform.CrossRepoBlastRadius` is dead since its tool was unregistered (SR-10); `fleet_drift` was unregistered for zero use (`internal/mcp/costclass.go:82`), so it is gone by decision, not unwired | PLT-7, PLT-4, PLT-6, PLT-15, PLT-12, PLT-17; SG-45 waits on RT-M1 | `version --json` has an empty `fact_kinds` (`cmd/fathom/version.go:87-90`), so a consumer cannot classify which facts fathom can supply. U5 |
| Investigations | `localize`, `staleness_report`, `as_of` on `obligations`/`review_diff`/`findings_near`, `culprit`, `retro`, `map` | `semantic.ObligationsAsOf`, `semantic.SupersessionChain` (dead) | PLT-10 `fathom-investigate` (`docs/plans/platform-engineering.md:46`); incident maps deferred | The skill has no `needs:` and is a cloud-lane document, yet it sits in P1. Findings import accepts only one anchor per event, so evidence and counterevidence collapse. U3 |
| Knowledge search | `search kind: docs`: BM25 with reciprocal rank fusion, one service per call (`internal/mcp/search_tool.go:266`) | the dependency-docs reader was removed (`ca9452f6c`), yet the tool schema still advertises `pkg` (`internal/mcp/search_tool.go:230`) | none | A parameter that always answers `not_indexed`. No cross-service docs search. Isolation between trust boundaries is undocumented as a recipe. U7 |

## 3. Ranked uplifts

The ranking weighs value to the consumer's goals, then feasibility, then fit with fathom's intent.

### U1. Serving reliability for review-class reads (highest value, already planned)

- **Evidence.**
  - The adopter's friction journal records `read_source` calls abandoned at the server deadline during a readiness
    assessment, and `find_symbol` timeouts on the operational symbols it reviews.
  - The same failure class is NP-P0's first item: "Deadline abandons inflate tokens and failed cells"
    (`docs/assessments/2026-10-26-next-phase-plan.md:52`).
  - A review consumer maps fathom exit 2 to missing, so every abandon becomes an `unknown` cell.
- **Uplift.** No new row. Land NP-P0's deadline-abandon work (O1014, O1446) first. Keep the adopter's recorded calls
  as a private acceptance replay in the operator's pack, and never commit them here.

### U2. The fact receipt on a schedule that beats its kill (planned, needs a ruling)

- **Evidence.**
  - The chain is WAE-4 → WAE-5 → EVX-9 → WAE-10 → PLT-7 → PLT-4 (rows cited in the verdict).
  - WAE wave 2 rides lf8 (`docs/plans/well-architected-evidence.md:12-13`).
  - The consumer's facts lane exists only on paper, and it counts a result only when the basis is `exact`.
- **Uplift.**
  - A controller ruling to start WAE-4 and WAE-5 as soon as NP-P0 lands, rather than waiting on lf8. Neither needs a
    new extractor: `K8sFact` rows are already observations of their own file.
  - Then WAE-10 single-kind, then PLT-7 multi-kind.
  - Separately, tell the consumer that WAE-1 and WAE-2 have shipped. This is PLT-17's consumer-side-ask lane and
    costs nothing.
- **Fit.** WAE-4 stores facts as observations of their own file with no table, so the freeze holds.

### U3. Findings import schema v2: every anchor, at its revision (new)

- **Evidence.**
  - The importer refuses any `schemaVersion` other than 1 (`internal/findings/external/projection.go:226-227,248-249`).
  - The adopter's review kit already emits v2 events with an `anchors[]` list. It cuts them down to the primary anchor
    before calling `fathom findings import`, so that the import is accepted.
  - Investigations and governance findings cite several sites as evidence and counterevidence, and all but one are
    lost at the edit.
- **Uplift.**
  - Accept v2 and hash each anchor at `SourceRevision` through WAE-1's path (`AnchorLinks`).
  - Keep v1 unchanged.
  - Refuse an event whose anchors exceed a stated cap, naming the cap. Never truncate silently.
- **Freeze check.** Verify at row start that anchor links already allow many rows per finding. If they need a new
  column, the row waits for RT-M1 PASS.

### U4. Four honesty gaps a review consumer reads as "clean" (new, small)

1. **Vacuous passes in the change risk receipt.**
   - With no certificate touched, `certificatesFactor` says "every touched certificate is live"
     (`internal/review/receipt.go:326-352`). With no claimed row, `rowsProvenFactor` says "every claimed row is
     proven" (`:374-397`).
   - The use-case guide already calls these vacuous (`docs/use-cases/platform-engineering.md:44-46`). A reader of the
     receipt still gets a pass.
2. **Silent YAML skip.**
   - The Kubernetes extractor drops any document it cannot decode, Helm templates included, and reports no read
     failure (`internal/extractor/k8sextract.go:32,113-129`).
   - A workload declared in a template disappears from an inventory, and the denominator shrinks with no trace. That
     goes against I38's "no failure mode makes a repository look healthier".
3. **The `pkg` docs parameter.** It is advertised (`internal/mcp/search_tool.go:230`), but its reader was removed
   (`ca9452f6c`), so it always answers `not_indexed`. A knowledge-search consumer cannot tell "not indexed by design"
   from "not indexed yet".
4. **`fact_kinds` is empty.** This is U5. It is listed here because a consumer reading `{}` cannot tell "no facts"
   from "not listed".

### U5. A fact-kind manifest (new, small)

- **Evidence.**
  - `fact_kinds` is an empty object (`cmd/fathom/version.go:87-90`).
  - The adopter's delivery plan must classify every fact it needs as available and proven, available and unproven,
    missing, or owned elsewhere.
  - PLT-M0 hands an item-to-kind table to the review kit (`docs/plans/platform-engineering.md:33`). That table needs a
    machine-readable list of the kinds on the other side.
- **Uplift.** Fill `fact_kinds` from the extractor registry, with kind, version, dialect and how it is served today
  (`symbol`, `fact-sheet`, `receipt` or `none`). The manifest comes from the code, never from readings or claims.

### U6. Governance evidence: a recipe, not a feature (new, docs only)

- **Evidence.**
  - The review kit already shells out to `fathom verify` and `fathom certify gaps` and re-derives satisfaction itself.
  - Fathom has SARIF from `verify`, `intent audit`, `config audit` and `obligations`, and it has `trace export` and
    `trace verify`.
  - No guide names these as governance producers.
- **Uplift.** A section in the platform use-case guide covering:
  - which outputs a review kit may import as `external-tool` producer events, and at which evidence tier;
  - the rule that fathom never imports its own output back as a second witness (EVX-21);
  - the rule that governance verdicts (inherited floors, expiring exceptions, missing packs) stay the review kit's.

### U7. Investigations and knowledge search: ship the skill, withdraw the stale parameter, document isolation

- **PLT-10 now.**
  - `fathom-investigate` has no `needs:` (`docs/plans/platform-engineering.md:46`).
  - Every verb it composes is served: `localize`, `read_source ref`, `blast_radius`, `review --since`, `culprit`.
  - Moving it from P1 to P0 costs one cloud-lane document, and it gives the consumer an investigation type it does not
    have.
  - PLT-16 and PLT-M5 still measure it before anything new is built.
- **The `pkg` parameter.** Withdraw it from the `search` schema, or have it describe itself as deferred under KR-11.
  This is RVU-3 below.
- **Isolation between trust boundaries.**
  - The adopter asks for authorization before search and aggregation. Fathom's documented answer is physical
    isolation: one loopback daemon and one store per trust boundary (ADR-069 decision 10), and no hosted multi-tenant
    backend (`docs/intent/scope/non-goals.md:17`).
  - That answer exists but is untested as a recipe. A recipe with a two-store fixture proving no cross-store answer is
    the honest uplift. An authorization layer is not.
- **Cross-service docs search.** Deferred as RVU-D2. A fan-out over every service loads the daemon, which is what
  RT-M1 measures.

### Ranking summary

| Rank | Uplift | Value | Feasibility | Fit | New rows |
|---|---|---|---|---|---|
| 1 | U1 serving reliability | blocks every reading | in flight (NP-P0) | P0 already | none |
| 2 | U2 fact receipt critical path | the only output the consumer reads | existing rows; PLT P1 is estimated at about 6 worker-days (design section 5) | ADR-069, freeze-safe | none (a ruling) |
| 3 | U4 honesty gaps | prevents false cleans | under 1 day each | I38 | RVU-1, RVU-2, RVU-3 |
| 4 | U3 findings v2 anchors | investigations and governance at the edit | about 1 to 2 days | ADR-040; freeze checked at start | RVU-4 |
| 5 | U5 fact-kind manifest | consumer-side classification and the census handoff | under 1 day | WAE-2's slot | RVU-5 |
| 6 | U7 PLT-10 now, isolation recipe | an investigation type; an honest answer to authorization | docs | ADR-069 d10, non-goals | RVU-6 (and a PLT-10 reorder) |
| 7 | U6 governance recipe | names producers that already exist | docs | ADR-040, EVX-21 | RVU-7 |

## 4. Plan rows (added to docs/plans/platform-engineering.md)

These use grammar v2 and are all unchecked. The `RVU-` prefix is free today. Symbols marked provisional must be
grounded with `find_symbol` when the row starts. Lanes follow the PLT- convention.

```text
- [ ] The change risk receipt never passes on nothing: `certificatesFactor` with no touched certificate and `rowsProvenFactor` with no claimed row each read "none declared" with weight none, never "every ... is live/proven"; `docs/use-cases/platform-engineering.md`'s vacuous-pass paragraph is rewritten to the new wording in the same change (lane: codex) {id: RVU-1, symbol: certificatesFactor, package: internal/review, reachable_from: TestRiskFactorsNameNothingDeclaredNeverPass}
- [ ] The Kubernetes extractor counts and names every YAML document it could not decode, per file and document index, Helm templates included; the ingest report prints them as skipped, and a readable file whose documents all failed never reads as "no workload" (I38); no store row is added: PLT-7's `skipped` coverage line re-derives the count by re-parsing at export (lane: codex) {id: RVU-2, symbol: k8sSkippedDocuments, package: internal/extractor, reachable_from: TestK8sUndecodableDocumentsNamedAsSkipped}
- [ ] `search`'s `pkg` parameter is withdrawn from the tool schema while KR-11 is deferred, or answers a named `deferred: KR-11` status instead of `not_indexed`; docs/mcp-tools.md updated in the same change (lane: codex) {id: RVU-3, symbol: searchPkgDocsKind, package: internal/mcp, reachable_from: TestSearchPkgNamesDeferredNotUnindexed}
- [ ] `fathom findings import` accepts projection `schemaVersion` 2 with `anchors[]`, hashes every anchor at the event's `SourceRevision` through `AnchorLinks` (WAE-1), refuses an event over a stated anchor cap naming it, and leaves v1 unchanged; if anchor links need a new column the row waits for RT-M1 PASS (lane: codex) {id: RVU-4, symbol: parseEventV2, package: internal/findings/external, reachable_from: TestImportV2HashesEveryAnchorAtSourceRevision}
- [ ] `fathom version --json` fills `fact_kinds` from the extractor registry: kind, version, dialect and served-by (`symbol`, `fact-sheet`, `receipt` or `none`), sorted; computed from code, never from readings (lane: codex) {id: RVU-5, symbol: factKindManifest, package: cmd/fathom, reachable_from: TestVersionJSONListsStoredFactKinds}
- [ ] The platform use-case guide gains `## Trust boundaries`: one loopback daemon and one store per boundary (ADR-069 decision 10), `strictSurfaces` named as no boundary, and a two-store fixture showing a query to one never returns the other's facts or counts (lane: cloud) {id: RVU-6, oracle: lexical, path: docs/use-cases/platform-engineering.md, pattern: "^## Trust boundaries$"}
- [ ] The platform use-case guide gains `## Governance evidence`: `verify`, `intent audit`, `config audit` and `obligations` SARIF, `trace export` and `trace verify` as producers a review kit may import as external-tool events, with their tier; fathom never imports its own output back (EVX-21); governance verdicts stay the review kit's (lane: cloud) {id: RVU-7, oracle: lexical, path: docs/use-cases/platform-engineering.md, pattern: "^## Governance evidence$"}
- [-] J4: an alert's runbook link resolves to an indexed document, computed at request time {id: RVU-D1, disposition: deferred, reason: "operational excellence is unmeasured; PLT-M0 has not ranked it", trigger: "PLT-M0 ranks a runbook item among its five most-needed code-answerable items and WAE-6 lands"}
- [-] `search kind: docs` across every service the daemon holds, ranked jointly {id: RVU-D2, disposition: deferred, reason: "a fleet fan-out is daemon load RT-M1 measures", trigger: "RT-M1 reads PASS and a named consumer asks in writing"}
```

Reorder rather than add: PLT-10 moved into P0 in `docs/plans/platform-engineering.md`.

## 5. What fathom should not do

| Refusal | Why |
|---|---|
| Supply the readiness roster or denominator | The expected roster must be approved independently of what producers return. Fathom's workload list is an observed set to reconcile against, never the denominator. Otherwise a silent skip (RVU-2) shrinks the denominator and nothing says so |
| A readiness verb, score, grade, chosen answer or pillar rollup | ADR-069 decision 5; PLT-X1 (`docs/plans/platform-engineering.md:68`); WAE-X2 (`docs/plans/well-architected-evidence.md:64`) |
| Run reviews on a cadence, or schedule continuous Well-Architected agents | ADR-078 D5; the dispatch-loop non-goal. A cadence belongs to the operator's CI or scheduler |
| An authorization layer, a hosted multi-tenant store or a shared org API | The non-goal at `docs/intent/scope/non-goals.md:17`; ADR-069 decision 10. Isolation is the answer (RVU-6) |
| Embeddings or a chat front end for knowledge search | The non-goal at `:14` |
| A question map, lens catalog or framework text | PLT-D9, PLT-X6, ADR-011 |
| Hold the findings record, or re-import its own output as a witness | ADR-040, WAE-X3, EVX-21 |
| Fetch deployed state, deploy events or incident data | I56, PLT-X5, the domain-connectors non-goal |
| Rank teams or people with any export | I58, PLT-X7 |
| A new MCP tool for any of this | ADR-053, PLT-X8 |

## Decision models

None applies to RVU-1 to RVU-7 (ADR-073):
- Each row is a deterministic factor wording, document count, schema decode, registry listing or document section.
- With provider `none`, every row runs unchanged. There is no opt-out, because there is no port call.
- Each row is measured by its own test or lexical oracle. Consumer value is measured by PLT-M4, which these rows feed.
- The one platform decision site stays PLT-M0's census classification (`ADR.md:5058`).

## Two horizons

- **Now.**
  - Reliability first: NP-P0.
  - The receipt chain next: WAE-4/5/10, then PLT-7.
  - Honesty fixes beside them: RVU-1 to RVU-5.
- **Three years out.** Each row leaves these open for free:
  - index snapshots and deployed-commit answers (PLT-D4);
  - the question map's corrected shape (design section 4.6);
  - cross-repository joins after RT-M1 PASS (WAE-D4, SG-2).

  RVU-2's skip count and RVU-5's manifest are exactly what a snapshot's cache key and a basis will later need.
- **Not pre-built.** The fleet docs search (RVU-D2) and the runbook join (RVU-D1) wait on their triggers.

**Specced, not implemented:** PLT-7, PLT-4, PLT-10, PLT-11, and WAE waves 2 to 4. **Planned, not specced:** RVU-1 to
RVU-7 (this document).
