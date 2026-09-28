# Work signals: stall detection, estimated weights, cost per proven weight point

Status: accepted 2026-09-29 (ADR-056): detection, estimated weights and the economics views are accepted;
the stall nudge and the preflight model site are experiments. Vision:
`docs/vision/40-judgment-and-exploration-maps.md`. Plan: `docs/plans/work-signals.md` (WS-). Goal G32.
Builds on work intelligence (`docs/design/work-intelligence.md`, WI rows, U1 to U6), progress and
economics (`docs/design/progress-and-economics.md`, PE rows) and the test-run hook
(`docs/design/test-run-hook.md`). Constraints carried in: the dispatch non-goal (fathom never switches
models, respawns or dispatches), I58 (rows keyed by agent identity, never a person), P11 (the advisor
recommends), P12, R2 to R7, ADR-049 decision 7 (no model role in progress).

## 1. Stall detection, "spinning wheels" (E3)

**Detection is deterministic first** and needs no model: the same test failing N times, the same tool call
repeated with identical arguments, edit-then-revert cycles, `verify` failing repeatedly on the same row, a
grep loop right after a fathom answer, tokens burned since the last proof progress (the last passing test,
the last proven row) above a threshold, frequent compactions. It is computed passively by the daemon as an
asynchronous fold over the hook and call logs, never by heavy work inside a hook.

**Constraints the critics added.** "The same test keeps failing" needs the test-run hook's tier 0 sensor
(`docs/design/test-run-hook.md`), and the TDD red phase looks exactly like stagnation. So before any line is
injected, candidate episodes are checked against abandoned sessions and later green runs (reading WS-M1).

**Reactions** are advisory and rate limited: at most one nudge per episode, opt-in; the next hook's
`additionalContext` names the pattern and offers proof tools (`localize`, `symbol_context`, `blast_radius`,
the failing test's reach) and, for orchestrators, suggests escalating the model. Fathom never switches
models, respawns or dispatches; it can write a stall signal an orchestrator may read.

**Reports:** stall episodes and "stall cost" per session, model, harness and zone; the cost-effectiveness
view shows when a cheaper model cost more per proven weight point than a stronger one (U5's model scorecard
gains it).

**Optional model role.** Classify ambiguous episodes as productive iteration versus stuck: sampled,
calibrated against the owner's labels through the one-key verdict, an opt-in site, never required for
detection, and not built until it has its own calibration (R3, R4).

**Experiment.** Nudges A/B by session hash. **Bar (the owner's):** episodes get shorter and cost per proven
weight point drops. **Kill:** keep detection and reports, drop the nudge. Cost: $0 for detection.

**Where it lands.** The rows are WS-1 to WS-7, WS-D1 (the deferred classifier) and WS-M1, WS-M2; the decision
record's "Lands: WI and PE" is satisfied by pointers in those plans and by PE-22, the page that shows stall cost
and cost per weight point, so nothing here is written twice.

## 2. Estimated weights and cost per weight point

**What exists on main.** The plan grammar's `weight:` (a positive integer, default 1, never gating, SI-7)
and `fathom plans status --weighted` rollups at fleet, project, plan and section level; cost per proven row
per plan and language (PE-16); the session-to-proof join (PE-15).

**The gap.** Almost every row is weight 1, so the weighted percentage equals the unweighted one.

**Proposed weights.** A deterministic estimator from row features: anchor type, planned files and the blast
radius of the anchor's package, rows it depends on, and the historical effort of similar proven rows from
the U6 preflight feature index. The estimate is shown as "estimated weight" beside any authored weight and
never overwrites it; the author accepts it into the plan with one command. The U6 model site (a difficulty
Score, WI-18) may refine the estimate only after it passes calibration (section 3); provider `none` uses the
deterministic estimate. Calibrate the estimates against actual effort (tokens, turns, wall time per proven
row) and publish the error (reading WS-M3).

**Economics by weight.** Cost per proven WEIGHT POINT, not only per row, on the Progress and Economics
pages, per model and harness (feeding U5's scorecard) and per zone (U2's interest). Keys are agent identity
only (I58).

## 3. The U6 preflight site against a fitted baseline (E4)

U6's features are numeric and a typed judgment model cannot count or compare numbers, so a held-out logistic
regression on the same features is the natural model and will probably win, which would kill the model site.
This also governs the estimator: a model may refine an estimated weight only after passing this test.

**Mechanism.** A 5-fold held-out logistic regression versus the model's score, both settled by effort to
proof (the session ledger joined to the verify ledger: about 3.4k sessions and 19.5k readings). Correction
2026-09-29: most of those readings sit under scratch paths (13,514 of 19,639), so the join reads the ledger
through the registered-roots filter (PE-24), and the corpus is counted again after filtering. **Cost:**
pennies. **Bar:** the model beats the fitted baseline on held-out AUROC by at least 0.05 with the CI
excluding 0. **Kill:** otherwise the U6 site uses the fitted model and the model is dropped there. **Depends
on** the features existing: WI-16 to WI-19 are still open rows (`docs/plans/work-intelligence.md`), so E4
runs when they land (reading WS-M4).

## 4. Privacy and cost

Detection and estimation read only fathom's own logs and plan files; nothing leaves the machine. A model
site here would send a structured feature row, never source text.

## 5. Readings

| Reading | Threshold |
|---|---|
| WS-M1 | candidate stall episodes checked against abandoned sessions and later green runs; the TDD red phase is not flagged |
| WS-M2 | the nudge A/B by session hash: episodes shorter and cost per proven weight point lower, or the nudge is dropped |
| WS-M3 | the estimated-weight error against actual effort, published |
| WS-M4 | E4: the model beats the fitted baseline by at least 0.05 held-out AUROC with the CI excluding 0, or the U6 site is dropped |
