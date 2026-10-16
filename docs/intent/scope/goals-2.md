# Scope: current goals, G26 to G37, G11, G12 and G7

Part of the scope router, `../scope.md` (the goal index, the compatibility surface and every section's member).

**G26. The feature map is measured against the owner's own scorecard (ADR-055).** Done when:
`docs/plans/feature-map.md`'s FM-M1 (evidence columns match the hand-built 86-feature
scorecard on at least 90% of cells) reads PASS. FM-M2 (verdict-proposal agreement at least 0.8 at
n at least 3) is deferred with FM-10 and FM-11 by the owner's decision of 2026-09-29: verdicts stay
human, and model proposals return only after J0 passes and a calibration against the 86 labeled
verdicts reaches the bar (F51).
{bet: feature-map, instrument: "fathom plans status", threshold: "FM-M1 at least 90% cell match against the 86-feature scorecard; verdicts stay human, FM-M2 deferred until J0 passes", review: 2027-01-31}

**G27. Level 10 through fathom: the test-run hook is measured against control and raw level 10 (ADR-056).**
Done when: `docs/plans/test-run-hook.md` TR-M1 reads PASS (arm B, the hook at tiers 0 and 1 with provider none,
uses at least 20 percent fewer tokens-to-green than control, the paired CI excludes 0, correctness does not drop)
or reads FAIL with the reading published and the hook left off, and TR-M2 (raw level 10 read against B) is
published either way.
{bet: test-run-hook, instrument: "fathom plans status", threshold: "TR-M1 arm B at least 20 percent fewer tokens-to-green than control with the paired CI excluding 0 and no correctness drop, or the FAIL reading published with the hook left off; TR-M2 raw level 10 read either way", review: 2027-01-31}

**G28. Does a typed judgment model read code? Decided before any site reads code with one (ADR-056).**
Done when: `docs/plans/judged-predicates.md` JP-M1 (the J0 kill screen) reads PASS or KILL. A KILL drops F34, F36
and F37; a PASS is followed by the 40-handlers cell JP-M3, read against arm A (arm C at least 40 percent fewer
tokens at recall and precision within 0.05, CI excluding 0). The showcase property is never a security property.
{bet: judged-predicates, instrument: "fathom plans status", threshold: "JP-M1 J0 screen reads PASS or KILL before any code-reading site is built; if PASS, JP-M3 arm C at least 40 percent fewer tokens than control at recall and precision within 0.05 with the CI excluding 0", review: 2027-02-15}

**G29. Judgments are calibrated, not assumed (ADR-056).** Done when: `docs/plans/judgment-foundations.md`
JF-1 to JF-12 and JF-23 are proven (the renamed rule-outcome events, per-site opt-in, `route` off by default, the
deterministic certify ladder, calibration readings, bands read from the latest reading), JF-M1 (a first calibration reading) exists, and
`docs/plans/judged-telemetry.md` JT-M1 reads PASS (`failure_kind` judged versus settled or human agreement at
least 0.8 on at least 100 adjudicated items across at least 3 sittings) or the template is dropped.
{bet: judged-calibration, instrument: "fathom plans status", threshold: "JF-1 to JF-12 and JF-23 proven; JF-M1 first calibration reading; JT-M1 judged versus human agreement at least 0.8 on at least 100 adjudicated items across at least 3 sittings, or the template dropped", review: 2027-03-15}

**G30. Exploration maps cut sub-agent tokens without losing accuracy, or are not built (ADR-056).**
Done when: `docs/plans/exploration-maps.md` XM-M1 records BUILD or KILL from the 14-day re-derivation reading, and
if BUILD, XM-M2 (the A/B on identical briefs) reads at least 25 percent fewer sub-agent tokens, no accuracy loss
and under 5 percent stale entries.
{bet: exploration-maps, instrument: "fathom plans status", threshold: "XM-M1 records BUILD or KILL; if BUILD, XM-M2 at least 25 percent fewer sub-agent tokens with no accuracy loss and under 5 percent stale entries", review: 2027-01-31}

**G31. Context survives compaction, measured, or the deliberate silence stands (ADR-056).** Done when:
`docs/plans/carry-across.md` CA-V1 verifies the hook, CA-M1 (Reading 0) records BUILD or STOP, and if BUILD,
CA-M2 reads re-reads of the pre-compaction set down at least 30 percent with the CI excluding 0 and tokens-to-proof
not worse.
{bet: carry-across, instrument: "fathom plans status", threshold: "CA-V1 verified; CA-M1 records BUILD or STOP; if BUILD, CA-M2 re-reads of the pre-compaction set down at least 30 percent with the CI excluding 0 and tokens-to-proof not worse", review: 2027-01-15}

**G32. Work signals: stalls detected without a model, weights estimated with a published error (ADR-056).**
Done when: `docs/plans/work-signals.md` WS-M1 (candidate stall episodes checked against abandoned sessions and later
green runs), WS-M2 (the nudge A/B, keep or drop), WS-M3 (the estimated-weight error published) and WS-M4 (the
preflight model against a fitted baseline, keep or drop) are read.
{bet: work-signals, instrument: "fathom plans status", threshold: "WS-M1 episodes checked against abandoned sessions and green runs; WS-M2 nudge keep or drop recorded; WS-M3 estimated-weight error published; WS-M4 preflight model keep or drop recorded", review: 2027-02-28}

**G33. A lost pin is caught in the turn that lost it, or the line stays a record (ADR-057).**
Done when: `docs/plans/served-not-kept.md` PR-M0 (the $0 replay over the guards-pilot 6 and 8 diffs) reads PASS or
FAIL (a FAIL means the check is fixed before PR-3 is built, K1), PR-M1 (30 days of record mode on owner traffic,
every fire adjudicated by the owner) reads PASS or FAIL, and PR-M2 (the bench, about $76, the owner's approval
first) reads PASS (with the pin line on, at most half the line-off oracle violations on the pin tasks, the
interval excluding 0, tokens per clean success and tool calls at most 1.10 of the line-off arm) or FAIL with
the line left record-only. PR-8 (block a silent loss at pre-push, accepted 2026-09-29, built by PR-9) is not part of this bet.
{bet: served-not-kept, instrument: "fathom plans status", threshold: "PR-M2 part B: pin line on at most half the line-off oracle violations on the pin tasks with the interval excluding 0 and tokens per clean success and tool calls at most 1.10 of line-off (K5), else the line stays record-only; PR-M0 and PR-M1 read either way", review: 2027-01-31}

**G34. Stewardship facts earn their place, or are cut (ADR-060).** Done when: `docs/plans/stewardship.md`
SV-M1 (the lines replay over the 2026-09-29 bundles), SV-M2 (the owner's adjudication of one leftovers list) and
SV-M3 (the used closure on fathom's own mirrors) each read KEEP or CUT, and a CUT removes what it killed in the same
change (vision 41 section 7). All three readings are $0; nothing starts before the owner lifts the 2026-09-30 wind-down.
{bet: stewardship, instrument: "fathom plans status", threshold: "SV-M1, SV-M2 and SV-M3 each read KEEP or CUT, a CUT removing what it killed in the same change", review: 2027-01-31}

**G35. The `.fathom/` zones and tech packs v1: one format, three authors (ADR-061, proposed; F5 promoted).** Owner
decisions D11 and K5 of 2026-10-09. Done when: `docs/plans/fathom-dir.md` waves 1 and 2 are proven (one resolver and one
keyed schema with existing loaders byte-identical, the local-state ignore, tech pack v1 served in this repository from
the pinned app-kit copy with the pasted copies gone) and FD-M1 (the generated-line reading over the rule-outcome log,
$0) reads KEEP or KILL, a KILL removing the serving path in the same change. Wave 3's merges are not part of this bet.
{bet: tech-packs, instrument: "fathom plans status", threshold: "docs/plans/fathom-dir.md waves 1 and 2 proven and FD-M1 reads KEEP or KILL, a KILL removing the generated-line serving in the same change", review: 2027-01-31}

**G36. Every file accounted for: the repository graph's facts earn their place, or are cut (ADR-067).**
Done when: `docs/plans/repository-facts.md` RF-M1 (the convention `test` role against each test runner's own
collection, unknown at most 5% of indexed files per subject), RF-M2 (`runs` edges against each tool's own dry
expansion) and RF-M3 (tsconfig membership against `tsc --listFilesOnly`) each read KEEP or CUT on three calibration
subjects, a CUT removing what it killed in the same change, and RF-M5 (the five-question probe) answers at least 27
of 30 files drawn by a seeded random sample, its seed and subject SHAs committed before the probe runs. Every reading
is $0. Agent outcomes are not part of this bet (the spend hold).
{bet: repository-facts, instrument: "fathom plans status", threshold: "RF-M1, RF-M2 and RF-M3 each read KEEP or CUT, a CUT removing what it killed in the same change, and RF-M5 answers at least 27 of 30 pre-registered seeded-sample files", review: 2027-01-31}

**G37. Concern facts earn a place in a Review Kit bundle, or are cut (ADR-069).** The evidence-plane program.
Done when: `docs/plans/well-architected-evidence.md` WAE-M2, WAE-M3 and WAE-M4 each read KEEP or CUT, a CUT
removing what it killed in the same change, and `docs/plans/index-snapshots.md` IXS-M0 is recorded. Kill: on
2027-01-31, a WAE-M4 that is unread or CUT drops WAE waves 3 to 4, not deferred. Index snapshots are not part of
this bet beyond IXS-M0; they get their own goal (provisionally G38) when the owner starts their phase 1. Every
reading is $0 in model spend; the owner's labelling and adjudication is the real cost; agent outcomes are not part
of this bet (the spend hold).
{bet: evidence-before-the-incident, instrument: "fathom plans status", threshold: "WAE-M2, WAE-M3 and WAE-M4 each read KEEP or CUT, a CUT removing what it killed; IXS-M0 recorded", review: 2027-01-31}

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
PASS or FAIL, and, after MOD-M3 PASS and the owner's Q-MOD-2 yes, MOD-M4 (the band) and MOD-M5 (the proof line) each
read, a CUT removing what it killed in the same change. Kill: MOD-M3 reading against M2's premise refuses M2; on
2027-01-31 an unread MOD-M2 drops phase 2, not deferred. Every reading is $0 in model spend.
{bet: claude-code-mods, instrument: "fathom plans status", threshold: "MOD-M1 reads KEEP or CUT and MOD-M2 and MOD-M3 PASS or FAIL; after MOD-M3 PASS, MOD-M4 and MOD-M5 each read, a CUT removing what it killed in the same change", review: 2027-01-31}

**G11. A reading, a hand-off and an answer carry their evidence (ADR-034).** The first
round's 74 rows were voided by an instrument defect (the wired arm never had the tools; O41
voided G10, ADR-033's ledger goal, on 2026-09-13 and it folds in here);
every cell now carries its server status and surface calls, published readings are plan
rows the gate verifies against the ledger, every tool answers under a budget with a
receipt, discoveries reach workers and the next session as fresh-resolved pointers, and
coverage is a code fact by layer. Done when: the reruns in `docs/plans/receipts.md` wave 5
carry readings with `surface_calls > 0` and the discoveries kill criteria are decided.
{bet: receipts-rerun, instrument: "fathom plans status", threshold: "docs/plans/receipts.md wave 5 proven with surface_calls > 0 on every reading; the discoveries kill criteria decided", review: 2026-10-25}

**G12. The surface leads with the answers grep cannot give, and every answer is falsifiable
(ADR-035).** The four impact verbs and a safe-delete check join the agent surface,
obligations names the governing rule, a layer rule compiles to a test, receipts carry a
re-derivable resolution tier, and fathom is measured on a corpus it did not write.
Done when: `docs/plans/verbs.md` waves A–D are proven and M10–M13 carry readings, honest
either way. **2026-09-17:** waves A–D proven (VB-1..36); M7's freshness kill fired (VB-18 —
freshness leaves positioning); M11/M11b and M8 held; M10 is on hold pending ADR-037's
raw-guide re-measure; M13's governance pilot and M15's historical-PR review stay open;
M12/M12b and M14 are a published null or stay advisory. Governance now bites at the edit
for checkable rules (ADR-037, ADR-038); fathom reads level with the best tools at lower
cost, not a token-savings win (`docs/bench/README.md` "Where fathom stands").
{bet: verbs-readings, instrument: "fathom plans status", threshold: "docs/plans/verbs.md M10 and M13 carry a reading (M11 and M12 are read), honest either way", review: 2026-10-25}

**G7. The evidence plane's waves 1–2 ship and its oracle gate is read (ADR-026).** Waves 1
and 2 shipped 2026-08-27: the verb, the ledger, the doctor rows, the clock, the shared
attention line, the capped primer, the guard proposal with provenance, U12/U13, the fleet
column, and K5's pair with K1's measured baseline (2 records, median 78 days). Done when:
the clock row records whether K0's demand signal (≥3 annotations across ≥2 repos) was met
by day 30 — either outcome closes the goal, and EP-16/17/18 are the only rows still waiting
on it.
{bet: evidence-plane-k0, instrument: "fathom intent audit", threshold: "the clock row records K0's demand signal (≥3 annotations across ≥2 repos) met or missed at day 30", review: 2026-09-27}
