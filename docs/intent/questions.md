# Open questions

Unresolved matters that change design. Each says what depends on the answer and who can
answer it. Delete when answered; the answer lands in the doc or record it affects.

**Q1. Which branch is the trunk?** `feat/mcp-scoped-surfaces` has carried 335 commits and
two release tags since 2026-06-17; local `main` is stale at that date while `origin/main`
is fast-forwarded by push. Rename the branch, realign `main`, or keep the split? Blocks:
`autonomy.md` branch model, the `CLAUDE.md`/`AGENTS.md` autonomy line. Answerable by: owner.

**Q2. The post-commit shell ingest against the shared store (INC-3).** It contradicts the
Index-first rule and is INC-2's failure path. Route it through the daemon's `ingest`
tool, keep it and land IT-14/15 first, or drop it (the daemon auto-refreshes on HEAD
drift)? Blocks: scope G1, `invariants.md` I17. Answerable by: owner, then a demonstration.

**Q3. Delete the 24 stale worktrees and 22 `worktree-*` branches (INC-1)?** Blocks:
INC-1 closure. Answerable by: owner (deletion is ask-first).

**Q4. Force-multiplier or product?** `docs/vision/07` Q4 ("who pays, and does anyone
need to?") has no written answer; it decides whether F2 (public release) is promoted or
dropped and what the A/B verdict (G2) is for. Answerable by: owner.

**Q5. The threat model.** The fleet's source is served unauthenticated on loopback to any
local process (`docs/vision/20` §7 Q2). Accept single-user in writing, or make law 9
hardening a goal before any second adopter? Blocks: F8, F15, `domain.md` assumption 2.
Answerable by: owner.

**Q6. Has XT-6b's trigger fired?** ADR-013 kept "the WASM path … if a pure-Go runtime
matures"; `tsengine` now runs four languages on wazero. Is the nine-language migration
wanted, and what is its Promote when (F1)? Answerable by: owner plus the calibration corpus.

**Q7. Two friction logs, colliding ids.** `docs/dogfood-observations.md` (O1–O29) and
`.fathom/observations.md` (O1–O19, `fathom observe`) both hold open observations under
the same numbering. Which is the home, and does `docs/CLAUDE.md`'s upkeep contract
change? Blocks: `incidents.md` pointer. Answerable by: owner.

**Q8. What is the active program?** `CLAUDE.md` names "semantic layers + expert modules";
the last six waves were ambient fathom, work frontier, instrument truth, verify spine.
Blocks: the router's strategy row; goal ordering in `scope.md`. Answerable by: owner.

**Q9. Fired clocks without a record.** (a) ADR-014's four group surfaces were kept
"pending the 30-day post-flip log" on 2026-07-16 and still exist with zero traffic;
(b) ADR-018's four-week pilot keep/kill was due ~2026-08-12 and the 2026-08-21 audit
reads as "keep" without saying so; (c) `docs/vision/01` §2 recommends tombstoning the
enrichment path (`WithEnricher`, zero production callers) and nothing decided it. Blocks:
decisions index status for ADR-014/018; a possible ADR-026. Answerable by: owner.

**Q10. `tmp/TRACKING.md`.** README and `docs/README.md` still name this gitignored file
(last touched 2026-07-08) as the roadmap. Retire the pointer in favor of `fathom plans
status` and `scope.md`? Answerable by: owner.

**Q12. Does ADR-006 still stand?** "Mandatory concern-scoped queries" is a parameter
check on one engine path; 55 of 58 tools never pass through it and the concern does not
filter unscoped stores. Reaffirm, re-scope, or supersede? Blocks: `invariants.md` I5,
the decisions index. Answerable by: owner.

**Q13. The obligations token ceiling.** ADR-016 promises ≤150 tokens per edit; the
serving path caps rules at 6 × 28 words and live payloads measure 210–350 tokens.
Enforce a serving-side cap (with the displaced-count line) or restate the contract?
Blocks: `invariants.md` I35; T3 in `principles.md`. Answerable by: owner plus a
measurement of injection precision at the lower cap.

**Q14. Thirty-six fully-proven plans cannot be archived.** `fathom plans archive` refuses them
because their row ids (`O-1..37`, `SS-1..9`, `CF-1..13`, …) were condensed out of `CHANGELOG.md`
and `docs/changelog/0.3.0.md` on 2026-08-24, and released blocks are frozen. Options: a
non-normative `docs/changelog/rows.md` index of archived-plan row ranges, an explicit
`--absorbed-by <path>` escape recorded in the tombstone, or leave them active (gate cost
O(43 plans) per push). Blocks: pre-push latency; `docs/plans/README.md`'s archival promise.
Answerable by: owner.
