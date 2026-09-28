# Judgment foundations: every judgment optional per site, proof never replaced, instruments that say what they measure

Status: accepted 2026-09-29 (ADR-056). Vision: `docs/vision/40-judgment-and-exploration-maps.md`.
Plan: `docs/plans/judgment-foundations.md` (JF-). Goal G29. Builds on the decision port
(ADR-042 and its amendments, `docs/vision/32-decision-port.md`), the assess lanes (ADR-046), the
surface reckoning (ADR-053) and the governance-learning signal source (GL-10). Constraints carried
in: I14 (egress only from the daemon or an explicit CLI verb), I42 (provider `none` is byte
identical), I46 (judging spend never enters an A/B unit), ADR-044 decision 8 (0.9 right nine times
in ten before a proceed band is trusted), P11 (the advisor recommends, the gate decides), P12 (no
saving without a counterfactual), P14 (narrative is a lens).

## 1. Problem

Five things are wrong or missing before any new judgment can be added honestly.

1. A decision site added the usual way turns on in every repo that sets a provider. Today
   `DefaultDecide.Sites` (internal/config/config.go) lists every site and `TestSiteAllowed_Allowlist`
   (internal/decide/decide_test.go) asserts that every site in `decide.AllSites` is allowed by default.
   A new site such as `triage_failure` would switch on wherever `certify_suggest` already runs. "Jev is
   optional" is true per repo today and false per feature.
2. The `route` site failed its own calibration check (ADR-042 amendment 2026-09-24): 45.7 percent right
   at confidence 0.9 or more, and non-monotonic. It is still in the default site list. It is not live on
   this machine (0 route rows in `decisions.jsonl`, 12 prompt-hook answers in 4 days), so the cost of
   turning it off is nil and the cost of leaving it on is a wrong hedge in every adopter's prompt.
3. GL-10's rule-outcome events do not measure what their names say (section 5).
4. `certify_suggest` offers the model a strength level the evidence cannot support
   (`mutation-killed` without a mutation report), so a model answer can be wrong by construction.
5. Calibration is asserted per provider, not read. The owner committed to `fathom decide calibrate
   --provider X` re-grading every site; nothing yet produces a reading for a site, a template, a
   provider and a resolved model.

## 2. The decision-model bet (binding; principle P24)

Fast, cheap typed-judgment models (Jev is the first) will keep improving in accuracy, calibration,
price and latency, and fathom's architecture rides that curve without redesign. The boundary: judgment
never replaces proof for anything fathom can compute (callers, verify, conservation), and a model
answer is never a receipt a stranger can re-derive, however good the model becomes. The rules:

1. Every judgment is a typed decision-port site (a schema plus a provider: none, llm, typesafe, or a
   future one), never an ad hoc model call.
2. Every site owns an adjudicated calibration set that grows (sampled judged telemetry plus one-key
   human verdicts); `fathom decide calibrate --provider X` re-grades every site, so adopting a better
   model is a measurement, not a redesign.
3. Features improve monotonically: deterministic with provider none; judgment used only in the
   confidence bands where calibration has passed; bands widen as calibration improves. No feature is
   built whose value exists only if a future model arrives.
4. Judgments are cached keyed by (question template, content hash), stamped with provider and model
   version, and kept apart from proof; a better model re-grades cleanly.
5. No vendor lock-in: the port's primitives (choice, score, abstain) are generic and any fast model
   plugs in as Jev did.

## 3. Program rules R1 to R8 (every item obeys them)

| Rule | Text |
|---|---|
| R1 | Structural routing. Named operators or existing tools answer every computable question; only an explicit judgment question reaches the port. A model never decides whether a question is computable (route failed at exactly that). |
| R2 | New sites are opt-in and never enter `DefaultDecide.Sites`. |
| R3 | Bars use Wilson intervals. A calibrated hint needs 90 of 100 right at the shown confidence (ADR-044 decision 8, n at least 100). A judgment that prunes rows, gates anything or reorders a default list needs a Wilson 95 percent lower bound of at least 0.90 (about 96 of 100; the O113 lesson). A free-form question is shown to the asking agent labeled uncalibrated and never prunes or reorders. |
| R4 | One labeled template at a time: human labels go to one template until it passes or is dropped. |
| R5 | No table, durable memo, sampler queue or background judge exists before RT-M1 reads PASS (ADR-049 decision 2). |
| R6 | No new MCP tool (ADR-042 decision 5, ADR-053). Hooks and the CLI come first; MCP exposure only on call-log demand from at least 5 percent of sessions, folded into an existing extended tool. |
| R7 | A judgment never asserts a code fact, prunes silently, sets a status or blocks. A model never prunes on a security property. |
| R8 | Pin the model (jev-1.13.0 today), log the resolved model, and mark a site "recalibration due" on any model move. |

## 4. Per-site opt-in (A2)

**Mechanism.**

- `DefaultDecide.Sites` is frozen at today's list minus `route` (section 6). It never grows.
- `decide.OptInSites` lists `triage_failure`, `ask`, `explore`, `compact_advice` and the `judge_*`
  family. `SiteAllowed` refuses an opt-in site unless the repository's own `[decide] sites` names it,
  even when `provider` is set.
- `TestSiteAllowed_Allowlist` changes to assert that `AllSites` minus `OptInSites` are allowed by
  default; a new test asserts that each opt-in site is refused under the default.
- The degrade rule: a missing key, a timeout, a 429 or 529, refuse mode or safe mode each produce one
  disclosed line and the deterministic path. Nothing else changes.

**Provider matrix (how Jev stays optional).**

| Provider | What works |
|---|---|
| `none` (default) | Nothing leaves the machine. Every feature works on its deterministic path: computable operators, plan-only requests, the test-run hook tiers 0 and 1, carry-across, maps. A judged question answers `unavailable (no provider)` with the redacted request plan. |
| `llm` | Every judged feature from the CLI and background work. Not inside hooks: it takes 1.5 to 2.8 s, 3 of 6 calls timed out at 3 s, and it costs 17 to 41 times Jev per call. It saves context, not dollars (40 rows at 2k tokens is about $0.09, the same as the frontier reading it replaces). |
| `typesafe` | Everything, including judgments delivered from a hook through a daemon endpoint under the `/route` deadline (measured 421 to 587 ms at about 22k tokens of state; 130 to 375 ms applies to small states only). |

Calibration is per (site, template@version, provider, resolved model): a template calibrated on Jev
reads as uncalibrated on llm. The I14 amendment: the port may open from the daemon, or from any
explicitly invoked CLI verb whose site is listed in `[decide] sites`; it never opens from a hook
process directly.

**Hexagonal hardening (accepted 2026-09-29, implemented in agentic-kit).** The port
(`foundations/decision`) is mostly hexagonal; the remaining lock-in leaks are closed there, then
fathom bumps its vendored mirror (`task sync-kit`, `task vendor-check`): a generic kind name with the
vendor term kept as a wire alias; adapter-declared limits instead of vendor limits in core validation;
adapters in subpackages so the core imports no vendor code; a `Capabilities()` declaration (kinds,
limits, batching, latency and cost class, confidence semantics); a conformance kit every adapter
passes; composition as decorators (cascade, fallback, cache, record and replay); and a calibration
mapping in the core from raw provider confidence to per-site calibrated bands. The record lives in
agentic-kit's decision-port plan; ADR-056 references it.

## 5. The rule-outcome instrument (A1, F1)

`cmd/fathom/ruleoutcome_track.go` (SR-61) and `askruleverdict.go` (SR-48) record `violated` when the
same claim topped the file's served set on the file's previous resolved turn, and `complied` when the
top claim changed. Rules are served by glob scope, so a prose rule with no checkable condition reads
`violated` on every repeat edit of the same file whether or not the edit complied, and `complied`
fires on a recompile or scope churn. The signal measures scope persistence and churn.

**Change.** Rename the events to `still_served` and `top_changed` in the tracker and in every reader
(the rule-outcome report, config propose, config audit and the config-change hook); keep `violated`
only for a rule with a checkable condition, if any exists; read existing ledger rows under their new
names and never drop them; re-label the GL-10 readings (a correction note in
`docs/design/governance-learning.md`); file the defect with `fathom observe`. Cost $0; it does not
depend on any provider.

**Bar.** A test in which a repeat edit to the same file under a prose rule emits `still_served` and
never `violated`. **Kill:** none; it is a correction.

## 6. Route off by default (A6)

Remove `route` from `DefaultDecide.Sites`. The code stays (a narrowing, not a deletion) and SR-44's
deterministic precheck stays. **Re-admission** needs at least 100 real prompts replayed through 4 to 6
decomposed nouls, weights fitted on half and tested on the other half, judged against R3. 41 held-out
prompts cannot establish 0.9: 38 of 41 has a Wilson lower bound near 0.80.

## 7. A deterministic certify_suggest ladder (A7)

`cmd/fathom/certify_suggest.go` offers the model `mutation-killed` without the report it would need.
Most of the strength ladder is computable from evidence: assertion calls present, a golden file read,
a property library imported, a mutation report present. Compute which levels the evidence permits;
if the site stays on, the model chooses only among those. The 43 of 43 `asserts` answers on Go table
tests may simply be correct. **Bar:** the model is never offered a level the evidence rules out.

## 8. Calibration readings, not fitted maps (A8)

A library in agentic-kit `foundations/decision` plus readings files. For each (site, template@version,
provider, resolved model) a reading reports n, accuracy by confidence band, Wilson bounds, Brier score,
AUROC, the label provenance mix (human, frontier, computed) and a "recalibration due" flag when the
model moves. There is no table, so RT-M1 does not gate it. A fitted map (Platt or temperature scaling)
is deferred (F39): it cannot repair route's non-monotonic failure, and a fit at n near 100 is noise.
`fathom decide calibrate --provider X` re-grades every site against its calibration set.

## 9. Doc corrections (A5)

Jev cannot generate text; at most it chooses among candidates that code or an llm produced. Two design
notes assumed otherwise and are corrected in this change: governance-learning section 11 ("proposing a
rewrite of an ignored rule") and knowledge-retrieval section 10 (query reformulation). The conservation
claim is stated at its true reach: conservation catches deleted, skipped, assertion-dropped and
golden-changed tests, and does not catch an expected constant edited to match buggy output, which
reads as `edited`, informational and never a loss on its own.

## 10. Cost, privacy, kill

Cost is $0 for every item here. Privacy: the changes send nothing; A2 and A6 narrow egress. Kill:
none of these is an experiment; each is a correction or a guard, proven by a test row in JF.

## 11. Readings

| Reading | Threshold |
|---|---|
| JF-M1 | a first calibration reading exists for `certify_suggest` (120 live rows on this machine): n, Wilson bounds, Brier and the label provenance mix, per (site, template, provider, resolved model) |
