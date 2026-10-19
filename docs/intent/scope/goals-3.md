# Scope: current goals, G39 to G41

Part of the scope router, `../scope.md` (the goal index, the compatibility surface and every section's member).

**G39. Readouts earn their place, or are cut (ADR-071).** The next priority program after the RT-M1 reading (owner
sequence, 2026-10-16); the Claude Code mods program follows. Done when: `docs/plans/readouts.md` RO-M0 (the corpus
replay) reads KEEP or CUT, RO-M1 (14 days of observe) reads PASS or FAIL, and, when RO-M0 reads KEEP, RO-M3 (lean per
rule) reads for every rule turned on, each CUT or FAIL removing or demoting what it killed in the same change.
Kill: RO-M0 CUT stops the lossy track at lossless stages and rich enrichment. G38 stays the provisional id of the
index-snapshot goal (ADR-069). Every reading here is $0 in model spend; a saving claim and default-on wait on RO-M6
(instrument C, the budget hold), which is not part of this bet.
{bet: readouts, instrument: "fathom plans status", threshold: "RO-M0 reads KEEP or CUT and RO-M1 PASS or FAIL; on KEEP, RO-M3 reads for every rule turned on, a CUT removing or demoting what it killed in the same change", review: 2027-01-31}

**G40. Claude Code mods earn their place, or are cut (ADR-072).** The program after readouts and before the tenancy
program's next step (owner sequence, 2026-10-16; roadmap §9.7). Done when: `docs/plans/claude-code-mods.md` MOD-M1
(the scout) reads KEEP or CUT, MOD-M2 (the module on Claude Code's own test host) and MOD-M3 (one live session) read
PASS or FAIL, and, after MOD-M3 PASS with fathom-ui on in the owner-architect's wave sessions (Q-MOD-2), MOD-M4 (the
band) and MOD-M5 (the proof line) each read, a CUT removing what it killed in the same change. Kill: MOD-M3 reading
against M2's premise refuses M2; on 2027-01-31 an unread MOD-M2 drops phase 2, not deferred. Every reading is $0 in
model spend.
{bet: claude-code-mods, instrument: "fathom plans status", threshold: "MOD-M1 reads KEEP or CUT and MOD-M2 and MOD-M3 PASS or FAIL; after MOD-M3 PASS, MOD-M4 and MOD-M5 each read, a CUT removing what it killed in the same change", review: 2027-01-31}

**G41. Every interception seam rated Medium or higher works, or its own reading re-rates it (readouts design §20).**
The owner's standing ask of 2026-10-16: "keep working until all of the medium or higher value ones are implemented
and working". Working means `fathom verify` proves the seam's build rows and its seam reading reads PASS: at least 7
days of real sessions, its firing floor, and no false answer. Done when: each seam H1 to H12 has its seam reading at
PASS (H1: RO-M1 and RO-M3; `docs/plans/readouts.md` RO-M8, RO-M9, RO-M10, RO-M12, RO-M14, RO-M15, RO-M17, RO-M18 and
RO-M20; `docs/plans/claude-code-mods.md` MOD-M6; `docs/plans/index-snapshots.md` IXS-M11), or a sizing reading
(RO-M0 for H1, which read CUT on 2026-10-18 and re-rated it Low; RO-M11, RO-M13, RO-M16, RO-M19, IXS-M10) re-rated it below Medium and its build rows were dispositioned in the same
change. Kill: a seam reading's FAIL turns that seam off by default in the same change. Every reading is $0 in model
spend.
{bet: interception-seams, instrument: "fathom plans status", threshold: "each seam H1 to H12 reads its seam reading PASS, or a sizing reading re-rates it below Medium with its build rows dispositioned in the same change", review: 2027-01-31}

**G43. Parallel lines stop colliding at append points (ADR-075).** Promoted from F88 on 2026-10-19: the F82 replay classed
247 of 263 conflicted merges in 30 days as append-point conflicts against a bar of 4 a month. Done when:
`docs/plans/append-points.md` AP-M1 reads PASS or FAIL (the replay, same script and classifier, from a clone where
`task merge-drivers` ran, over the 30 days after the last build row). Kill: AP-M1 FAIL reopens ADR-075 at its Revisit
when; the deferred rows AP-D1 to AP-D4 build only on their triggers. Every reading is $0 in model spend.
{bet: append-points, instrument: "fathom plans status", threshold: "AP-M1 reads PASS (at most 61 append-point conflicted merges a month, 25% of 247) or FAIL (ADR-075 reopens)", review: 2027-01-31}
