# Judged telemetry: the label economy, judgments as forecasts the index settles

Status: accepted 2026-09-29 (ADR-056) for the instrument fix and the calibration infrastructure; the
sampler, queue, background judge and judgments table are deferred behind RT-M1. Vision:
`docs/vision/40-judgment-and-exploration-maps.md`. Plan: `docs/plans/judged-telemetry.md` (JT-). Goal
G29. Builds on governance learning (`docs/design/governance-learning.md`, GL-10 to GL-13), the assess
lanes (ADR-046, `docs/design/assess-lanes.md`), the decision port (ADR-042, ADR-056) and the
responsible tenant (ADR-047). Constraints carried in: ADR-049 decision 7 (no model role in rule
learning, doc linking or progress), ADR-046 (judge on-disk work, never transcripts; assess itself is
frozen by ADR-053), ADR-041 decision 8 (nothing inferred from annotation text), I46, ADR-049 decision 2
(no new table, cache or job before RT-M1 reads PASS), P14.

## 1. Problem and the reframe

Fathom cannot tell whether what it serves works. Its only compliance signal cannot see compliance for
prose rules (`docs/design/judgment-foundations.md` section 5), every decision site lacks labels, and the
owner's lifetime output is 86 feature-map verdicts and 0 claim verdicts. The original idea, a judged
telemetry loop that scores hook events with a cheap model, mostly re-derives what structured events
already say. The durable value is different: **manufacturing calibration labels**. Tokens are not the
constraint; labels are. A full design would need about 700 to 1,000 human judgments, which the owner
does not supply spontaneously (0 claim verdicts) but does supply in batches when the labels feed a
decision they care about (86 verdicts).

## 2. The label economy

Get labels from three places, in order of cost.

1. **Oracle labels** where the index computes the truth (J0's properties). Free.
2. **Settled forecasts.** A judgment that is a forecast the index later settles grades itself:
   - `failure_kind`, settled by where the fix lands (test files only means outdated expectation; the next
     run passes with no change to reached symbols means flaky; a change to reached non-test code before
     green means code bug);
   - a `localize` candidate, settled by the files the fixing commit changed;
   - U6 preflight difficulty, settled by actual effort to proof (the session ledger joined to the verify
     ledger, about 3.4k sessions and 19.5k readings);
   - a review caller "depends on the change", weakly settled by that caller's tests failing or being edited
     in the same change;
   - a doc span "now wrong", weakly settled by that span being edited within N commits (absence proves
     nothing, so only positives count).
3. **Batched human adjudication** of the residue: `fathom adjudicate`, a batch of 10 items with one key each,
   recorded with provenance `human`, its sittings tied to decisions the owner already makes.

**R4:** human labels go to one template at a time. The one labeled template is `failure_kind` (the level 10
tier 2 question, `docs/design/test-run-hook.md` section 5), whose stream the test-run hook produces; J0's
residue set (about 50 to 80 labels, once) is the only other human labeling.

## 3. Accepted now

- **The instrument fix (A1).** No judgment is needed and it is the strongest item: the rule-outcome events
  are renamed to `still_served` and `top_changed` (`docs/design/judgment-foundations.md` section 5), which
  also removes the sharpest motivation for a judged compliance site until a checkable condition exists.
- **Calibration readings (A8).** The reading library and `fathom decide calibrate --provider X`
  (`docs/design/judgment-foundations.md` section 8). No table, so RT-M1 does not gate it.

## 4. Deferred behind RT-M1 (F40 and F39)

**Sampler, queue, background judge, judgments table (D-7).** Deterministic, seeded, stratified sampling
over hook and call events: a 1-in-N baseline, oversampling denials, reverts, failed verifies, a grep right
after a fathom answer and the same rule served to the same file repeatedly; a per-day cap on count and USD.
A queue feeds a daemon worker admitted by cost class, paused at shed, closed at refuse and in safe mode,
with its spend labeled and excluded from A/B units (I46). Units are read from disk and git, never from
transcripts: a rule text (at most 28 words) with the edit's hunk, a failure excerpt with candidate spans, a
doc span with a symbol diff. Answers go to a judgments table kept apart from human verdicts and computed
counts. A settlement job scores forecast-shaped judgments against later events and computes Brier score,
ECE and AUROC per template. **Cost** at the owner's volume (about 15k edit events a day, sampling capped at
500 a day): about $0.013 a day on Jev, about $0.42 a day on llm; unsampled about $0.38 a day and $12 a day.
**Trigger:** RT-M1 reads PASS and either tier 2 (F35) or `ask` (F34) needs more labels than settlement
supplies. **Host decision (owner, before D-7):** unfreeze the assess lanes (ADR-046) as the host of judged
telemetry, or supersede them. A parallel lane is refused because it would rebuild the consent capability
and the spend isolation.

**Fitted recalibration and site auto-disable (D-6, F39).** A fitted map (Platt or temperature) waits for one
(site, provider, model) to hold at least 100 labels; auto-disable (a site whose rolling Brier score falls
below the base rate for 14 days disables itself and says so on the attention line) needs a minimum n per
window or it flaps.

**Durable memo.** A judgment memo saves under one cent per query on Jev, so it does not justify its
footprint there; it matters on the llm lane. **Trigger:** RT-M1 PASS plus use on llm. An in-process cache
decorator (the kit's hexagonal notes) is fine now.

## 5. Consumers, by verdict

| Consumer | Verdict | Reason |
|---|---|---|
| Judged rule compliance (feeding learning, or a served column) | Refused | contradicts governance-learning section 11 and ADR-049 decision 7; no labels; revisit through an ADR-049 decision 7 amendment backed by a template that passes R3 on owner-adjudicated hunks |
| Guard proposals from rule outcomes | Deferred (D-8, F41) | after A1 ships and 30 days of relabeled outcomes show a rule with a checkable condition repeatedly not met; proposals are deterministic, a human accepts, never sets `enforce: true` on its own, and model-judged hunks are never written into tests |
| Answer usefulness per tool and field | Deterministic only | a join over structured events (a field is used if a later call references a returned id or path; re-searched if a grep on the same term follows; check that Grep events exist before claiming "re-searched") |
| Config custodian proposals | Deterministic only | served-then-refuted layers, truncation followed by `read_source`, route decision versus the next tool actually used |
| U3 friction episode classification | Deferred (F49) | no consumer until U3 exists (P20) |
| Product feedback for fathom | Deferred (F49) | aggregated counts and taxonomy labels through the research bundle's closed allowlist, run by a human (P13, I14), once labels worth sharing exist |
| Weekly digest by a cheap model | Refused | narrative (P14); the model cannot write prose; the console renders counts |
| Self-calibration and auto-disable | Accepted as infrastructure | the reading library now; the fit and the auto-disable later (F39) |

## 6. Jev primitive, privacy, cost

Primitive: choice or noul over a unit read from disk, options from facts (the rule itself, the served
fields, a mistake taxonomy built from this repository's rules). Privacy: events never leave the machine;
only sampled units do (spans, rule text, redacted failure excerpts); default off, one site per consumer
(`judge_failure` for the first). Cost as in section 4.

## 7. Calibration set, bar, kill

- **Acceptance (per template):** judged versus human agreement at least 0.8 on at least 100 adjudicated
  items across at least 3 sittings (the FM-M2 shape); a "violated" style label at precision at least 0.9 at
  the shown confidence; a settled template's Brier score beats its base-rate forecast; proposals accepted at
  least 50 percent (the TC-M4 precedent).
- **Kill:** agreement below 0.8 after one template revision drops the template; no proposal accepted in 60
  days stops the loop; a rolling Brier score below the base rate for 14 days disables the site.

## 8. Refused (do not re-propose)

Judged rule compliance now; Jev for answer usefulness, config custodian proposals and product digests;
model-judged security properties as a served signal (measured in J0 only); a parallel lane beside assess.

## 9. Readings

| Reading | Threshold |
|---|---|
| JT-M1 | `failure_kind`: judged versus settled or human agreement at least 0.8 on at least 100 adjudicated items across at least 3 sittings, or the template is dropped after one revision |
| JT-M2 | a settled template's Brier score beats its base-rate forecast on the first settled batch |
