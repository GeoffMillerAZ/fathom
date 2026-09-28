# Change and compliance: the risk receipt, the evidence pack, the migration tracker, the drift radar

Status: accepted 2026-09-27 (ADR-051 decision 4). Vision: `docs/vision/38-proof-layer-roadmap.md`
§4. Plan: `docs/plans/change-and-compliance.md` (CX-). Goal G25. Promotes futures F24 and the
computable subset of F9. Constraints carried in: `review_diff` and the review dimension
(ADR-035, ADR-043), conservation (ADR-044, I44, I45), the SARIF emitter (PC-21), the findings
loop's authority boundary (ADR-040: fathom never approves or closes), `internal/ownership`
(CODEOWNERS), ADR-011 (standard-agnostic, no org names), I57 (heuristic links never gate),
I58 (no ranking of people), the Moderne non-goal (no transformation engine), the dispatch
non-goal.

## 1. Problem

Review bots give opinions on a PR; compliance teams build traceability by hand and it rots;
migrations are tracked in spreadsheets; architecture decisions erode silently. Each needs a
join fathom already holds: impact, governance, certificates, conservation, proof history,
the intent corpus and git.

## 2. U7. The change risk receipt

For a diff (base..head), deterministic, every reason cited:

| Factor | Source | Weight in the band |
|---|---|---|
| blast radius | `blast_radius` over changed symbols, callers counted by depth | high when more than 50 transitive callers or any entrypoint |
| links touched | `system_impact` at `spec`/`typed` tiers (I57: lower tiers listed, never counted) | high on any provider change with a consumer in another service |
| certificates affected | certificates whose target or reach intersects the change, with state | high when a touched certificate is not live |
| conservation | `fathom conserve` planes | high on any unexcused loss |
| rows proven | plan rows the change claims, and their verdicts | high on a claimed-unproven row |
| rules touched | governing rules scoped to changed files, by level | medium on `warn`, high on `error` |
| zone interest | U2's rate for the changed zones | medium above 2.0 |
| uncertified contracts | T5: links at `spec`/`typed` tier without contract certificates | medium |

**The band** is `low`, `medium` or `high` by a published rule table (`.fathom/risk.cue`,
defaults in the binary): high if any high factor, medium if any medium, else low. No score is
summed; the band is the strongest reason, and every reason is listed with its evidence.

**Suggested reviewers** come from ownership of the impacted symbols: CODEOWNERS rules for the
changed and impacted files, then the most frequent committers to the impacted providers in 365
days, shown as review suggestions with the files that put them there, never ranked by
productivity (I58).

**Output.** `review_diff` gains a `receipt` section (additive); `fathom review --receipt
--format sarif|check` renders it through the shared emitter. Fathom never posts, approves or
blocks a PR on its own authority; CI decides what a `high` band does.

## 3. U8. The traceability matrix and the evidence pack

**The matrix** is a join, per requirement: requirement (a plan row, a foreign spec task, or
an `accept:` criterion) to code (its anchor) to test (`certified_by:` or reaching tests) to
certificate (state) to attested run (digest, commit) to decision (the ADR or intent record the
requirement cites). Each cell carries its live state and the reading that proved it.

**Standard mapping.** A repository-authored mapping file (`.fathom/trace/<standard>.cue`)
names a standard's clauses and which requirements satisfy each (by row id or pattern). Fathom
ships no standard's text or clause list (ADR-011); the mapping is configuration.

**The evidence pack** is exported only on request: `fathom trace export --standard <id>
--out <dir>` writes the matrix (JSON and a rendered Markdown), each proving artifact's
digest, the verify and certificate readings, the commit, and a manifest with sha256 of every
file. **Re-verification:** `fathom trace verify <dir>` recomputes the matrix at the pack's
commit and must produce byte-identical files (determinism is a contract); any difference is
named by file and cell.

## 4. U9. The migration tracker

A migration is declared in a plan (the refactor-conservation grammar already carries
`gone:`, `retires:` and `surface_change:`), or by `fathom refactor plan` which writes one. The
declaration names **from** and **to** for any of: symbols, imports, routes, config keys,
`pkg:` versions. Fathom computes, per repository in the fleet: remaining sites of `from`
(typed referents where available, lexical otherwise, each tier named), sites already on `to`,
blockers (sites in files governed by a rule forbidding the change, or in generated code), and
a burn-down from proof history. Completion is proven by conservation (`gone:` rows proven and
no unexcused loss). Fathom tracks and proves; it never rewrites a site (the Moderne non-goal).

## 5. U11. The architecture drift radar

1. **Decisions compiled to dependency rules (F24 promoted).** An ADR stating a dependency
   direction or layering ("X never imports Y", "handlers call services, never stores") is
   compiled, with its citation, into an archconformance rule (`.fathom/rules/`), refuted by
   the call graph on every ingest. Compilation is proposed; a human accepts the rule (ADR-016's
   compile discipline).
2. **Layer violations over time.** Each ingest records the violation count per rule; the
   radar shows the trend per rule and zone and the commits that added violations.
3. **Computable revisit triggers (F9's subset).** A `Revisit when` or `Promote when` condition
   that names a fathom instrument and a threshold in the bet-block shape (`{instrument,
   threshold}`) is evaluated on a schedule the operator runs (`fathom intent audit`), never
   by the daemon on its own (the dispatch non-goal). Prose triggers stay unevaluated and are
   listed as such.

Output: "decisions due for review" (fired triggers, with the reading) and "architecture eroding
here" (rules with a rising violation trend).

## 6. Tenancy and cost

The receipt reuses verbs already computed for a diff; its only new cost is the band table.
The matrix is a join over stored rows for the requirements in scope. The pack export reads
what the matrix names. Migration counts use typed referents from the index, lexical scans
bounded by the declared paths. The radar's violation counts are an ingest-time fact.

## 7. Measurement

| Reading | Threshold |
|---|---|
| CX-M1 | the receipt on every PR in at least 2 repositories for 30 days, with SARIF or check output consumed by CI |
| CX-M2 | an evidence pack exported and re-verified byte-identically at its commit |
| CX-M3 | one real migration tracked from declaration to proven completion |
| CX-M4 | the radar live: at least 3 compiled ADR rules, a violation trend, and the computable triggers evaluated |
| CX-M5 | the receipt's band against reviewer outcomes on 50 historical PRs: high-band PRs account for at least 2x the post-merge fixes of low-band PRs, or the table is revised |

## 8. Jev, weighed

- **Where it could help:** proposing the dependency rule an ADR's prose implies (U11 step 1),
  as a proposal a human accepts.
- **Where it cannot:** the band, reviewers, matrix states, migration counts; all deterministic.
- **Cost:** a call per ADR at compile time, only with `[decide]` configured.
- **The measurement:** at least 8 of 10 proposed rules accepted unchanged on this repository's
  ADRs, else the proposal path is removed and rules stay hand-authored.

## 9. Refused

- A summed risk score, or a model's opinion of risk.
- Posting to a forge, approving or blocking on fathom's authority.
- Shipping any standard's clause text (ADR-011).
- Transforming code to complete a migration.
- The daemon evaluating triggers on its own schedule.
- Ranking reviewers or authors (I58).
