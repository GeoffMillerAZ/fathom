# Decision models: what pays now, what is an experiment, and what waits

Owner-architect assessment, 2026-10-16, summarized into the repository the same day. Read at fathom `ebc3ce70` with
the readouts, mods and seam records on top. The pass (eight research agents, report only) and the owner's later
direction sit in gitignored workspace files and are not the record. The decision is ADR-073; the design is
`docs/design/decision-models.md`; this file is the reading that led to them.

## Verdict

Decision models pay now only as scaffolding that costs nothing: a kill switch and user consent (the pass found a
repository can make fathom spend the user's ambient key), disclosure of which answers a model made, a readout table
fitted offline, and the `fathom ask` spine at provider `none`. No judged site has passed a bar yet, and whether a fast
decision model reads code at all has never been measured, so the owner's direction is to skip provider calibration and
run small, stage-gated experiments that grade a model against truth fathom computes: at most $2 first, at most $8
more only if a template clears its gate, and an agent A/B only with the owner's separate approval. The port is
hexagonal: Jev, OpenAI's Decisions API and any LLM are adapters behind the same three kinds.

## What was read, graded

| Claim | Grade | Source |
|---|---|---|
| A repository's config can set the provider, and with no key named the adapter reads the transport's well-known variable while the default sites are on | measured (code read) | `internal/config/scope.go:80`, `internal/config/decidekey.go:68`, `internal/config/config.go:256` |
| Provider `none` returns before any key read, exec or dial | measured (code read) | `internal/decide/decide.go:195` |
| No kill switch exists beyond provider `none` | measured (grep): `FATHOM_DECIDE_LIVE` gates only the live smoke test | `internal/decide/live_smoke_test.go:219` |
| The port's yes or no kind is `predicate`, with `noul` a decode alias | measured (code read) | agentic-kit `foundations/decision/kind.go:27-33` |
| The kit's conformance kit runs against its three remote and LLM adapters | measured (code read) | `foundations/decision/{llm,typesafe,openai}/conformance_test.go` |
| Route was right 45.7% of the time at 0.9 or more, non-monotonic | measured | `docs/bench/readings/decision-port-calibration-2026-09-24.md` |
| The within-tier re-rank read -0.19 on governed edits | measured | `docs/bench/readings/decision-port-ab-2026-09-23.md` |
| Jev through OpenRouter answered 8 of 8 at 256 to 516 ms for $0.00017 | measured (a smoke, not a calibration) | `docs/bench/readings/decide/provider-smoke-2026-10-09.md` |
| OpenAI's Decisions API: public beta, `POST /v1/decisions`, one model `gpt-6-luna`, predicate, choice and score, $0.10 per million input tokens, no calibration claim | claimed by a vendor (its guide, read 2026-10-16) | the vendor's Decisions guide |
| Jev's latency 70 to 500 ms and price $0.042 per million input tokens | secondary (a public article) | not re-read |
| A fast decision model reads code well enough to label residue | unmeasured | Q31; J0 (JP-M1) has not run |
| `ask` saves context against reading files | hypothesis | E3 reads it at stage 0 |

This assessment does not claim that any provider is calibrated on any site, that `ask` saves tokens, or that the
experiments will pass.

## Decisions it drove, and where they are recorded

- The record: ADR-073, with dated amendment lines on ADR-042, ADR-056 and ADR-071; I14 and I60 reworded; I66 new.
- The design: `docs/design/decision-models.md` (the idea list DMP-1 to DMP-22 with value, gate, phase and status).
- Rows: `docs/plans/decision-port.md` KS-1 to KS-13 and DP-41; `docs/plans/agentic-decisions.md` (EXP-, AQ-);
  `docs/plans/readouts.md` RJ-1 to RJ-4, RJ-M1, RJ-M2, RJ-D1 and RO-32's key; `docs/plans/judgment-foundations.md`
  JG-1 to JG-5; `docs/plans/judged-predicates.md` JP-10 and JP-11 superseded, JP-12 and JP-14 re-triggered.
- The bet: goal G42 (G28 amended). The questions: Q37 (Q31 amended). Futures: F34 promoted in part, F35 extended,
  F80, F81. The budget: `docs/intent/autonomy.md`, the hold lifted in part. The sequence: roadmap section 9.9.
- Not done here: the kit's decision 0018 amendment (a kit change) and its level-10 typed-spec guidance in
  `docs/fast-judgment.md`.
