# HF-3 — the ADR-016 150-token ceiling, measured before it is enforced

Measured 2026-08-27 from `~/.fathom/hooks.jsonl`, window
2026-07-30 → 2026-08-26 (28 days, this machine, one adopter).
Unit: one `payload-cost` beat = one edit's injected governance payload.
`tokens_estimate` is bytes/4, labelled ESTIMATE everywhere (law 12).

## What was measured

| channel | n | p50 | p90 | p95 | max | over 150 |
|---|---|---|---|---|---|---|
| `post-edit` (obligations) | 1,918 | 131 | 315 | 329 | 378 | **897 (47%)** |
| `pre-edit-guard` (guards) | 370 | 185 | 198 | 222 | 380 | **308 (83%)** |

Rules per post-edit payload: p50 3, p95 6, max 6.

**Estimated displacement at a 150-token whole-payload cap** — rules that
would not fit, assuming each rule costs the payload's mean tokens-per-rule:

| channel | payloads that lose ≥1 rule | median lost | max lost |
|---|---|---|---|
| `post-edit` | 897 | 2 | 4 |
| `pre-edit-guard` | 306 | 1 | 3 |

So enforcing the ceiling today would silently drop **two of a typical
over-budget payload's rules**, on roughly half of all edits.

## What could NOT be measured, and why

**Precision — were the dropped rules ones the session went on to need?**
Not computable from what is recorded. The beat log stores each payload's
BYTES and RULE COUNT, never its text, so a historical payload cannot be
re-rendered under a smaller cap and scored against the edits that followed.

Making it computable means recording the governance text of every injected
payload — an always-on write of rule prose to a local log, on every edit, in
every adopting repo. That is a new standing cost and a new thing on disk to
justify; it is not obviously worth buying to answer one question.

The displacement figures above are therefore an UPPER BOUND on harm and say
nothing about its distribution. A cap that drops two rules is only bad if
those two mattered.

## What this says about HF-4

The ceiling is not close. It is missed on 47% of post-edit payloads and 83%
of pre-edit guards, at roughly 2.2× the limit at p95. Enforcing it as
written is not a tidy-up — it is a decision to withhold about two rules from
half of all edits, with no evidence about which two.

Three ways forward, and this is Q13's substance:

1. **Enforce at 150 and rank**, accepting the displacement. Needs a
   defensible rank (specificity? scope depth? recency?) and the dropped
   count named in the payload (law 3), so the loss is visible rather than
   silent.
2. **Raise the ceiling to the measured p90 (≈320)** and record the amendment
   in ADR-016. This is the honest option if the ceiling was set by intuition
   rather than by a measured budget — the number in the record has never
   been checked against what the channel actually costs until today.
3. **Buy the precision measurement first** (record payload text for a bounded
   window, behind a flag, in this repo only), then choose 1 or 2 with
   evidence about which rules go.

Recommendation: **(2) then (1)** — amend ADR-016 to a measured ceiling now,
so I35 stops being a permanently VIOLATED row nobody can satisfy, and treat
the drop-ranking as separate work with its own evidence. Owner's call: Q13.
