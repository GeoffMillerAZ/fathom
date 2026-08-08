# Vision 19 — The exploration ledger: measuring the 90x, and a dashboard a stranger can read

> Owner directive 2026-08-08: "one way I think it will save on tokens is in
> exploration… in some really early benchmarking I did long ago it was 90x
> less tokens on exploration and then each sub agent would repeat the same
> exploration so it would multiply the savings… really push for excellence
> on the observability and collecting and telling the story." Plus: the
> dashboard must be clear to a decent developer who does NOT already
> understand fathom, and usable to explain the value to management.
> docs/plans/exploration-ledger.md is the binding promotion.

## 1. The insight: both sides of the comparison live in the SAME transcript

The counterfactual arm (law 12) remains the causal instrument. But a
weaker-yet-honest observational instrument exists TODAY: every session
transcript records BOTH kinds of discovery —

- **raw exploration**: Grep / Glob / Read tool calls and the byte size of
  every result the model then had to carry as context;
- **indexed discovery**: fathom MCP calls and the byte size of THEIR
  results.

Same session, same units, same estimator (bytes/4 ≈ tokens, labeled an
estimate). That yields a per-question cost for each mode and their ratio —
the owner's 90x becomes a measured, per-session distribution instead of a
remembered benchmark. Labeled observational (the modes answer overlapping,
not identical, question classes); the arm pilot supplies the causal claim.

## 2. The subagent multiplier is directly countable

Transcript lines carry sidechain markers. A file Read by more than one
sidechain in one session is DUPLICATED exploration — each subagent paying
the same discovery cost fathom's shared index pays once. Count it: files
re-read across sidechains, and the estimated tokens spent on repeats. That
number is the multiplier made visible.

## 3. A dashboard a stranger can read

Every screen leads with one plain-English sentence about what fathom is
and what the page shows. Every jargon term carries its plain meaning
inline ("verify catches — work claimed done that the gate proved wasn't").
Every page has a no-JS "what am I looking at?" explainer (a details
element). The brief becomes a PAGE (/observatory/brief) a developer can
open, read, and forward to their manager — same honest numbers, framed
for someone who has never heard the word obligation.

## 4. Configuration observability (layers / packs / guards)

Per service: which semantic layers are installed (and whether eval-gated),
what the last compiles cost, how many guards watch which scopes, whether a
binding file exists, and how many rule-injections the repo actually
received. Configured-vs-inert becomes visible, and the panel names the
install command for repos with nothing configured.

## 5. Honesty upkeep in the same wave

The wiring grade currently reads "no project settings.json" as unwired —
but user-scope wiring is invisible from a repo walk, and reconcile proves
several such repos ARE current. Unknown must not grade as absent (law 3):
signals that cannot be seen from here leave the denominator.

## 6. Deferred, named

A reproducible `fathom bench exploration` harness (the 90x with receipts:
fixed question set, grep-sweep vs indexed answers over a public corpus,
published methodology) — deferred until an external-facing claim needs a
citable artifact; the in-session ledger carries the internal story first.
