EXP-M4 stage 1: CLOSED

# Stage 0's gate: does a fast decision model read code well enough to label the residue (EXP-M4)

Read 2026-10-17 from the three stage-0 readings, each run under `exp-preregistration.md` (EXP-1, fathom fd091c4f) and, for E2b and E3, its second page `exp-preregistration-2026-10-17.md` (7c688c49). Stage 1 opens only when at least one template QUALIFIES in E1 or E2; none did, so stage 1 is CLOSED and there are no winning templates. Filed as observation O1121.

## The verdicts

| Reading | Row | Verdict | What was measured |
|---|---|---|---|
| `exp-e1.md` | EXP-M1 | NONE | four templates with computed truth (errcheck, wrap, assert, lock), 200 spans each: the model was below the lexical baseline on errcheck, wrap and lock, and tied a perfect baseline on assert |
| `exp-e2.md` | EXP-M2 | NONE | E2a (does this caller depend on the changed behaviour, n 50) agreed 0.844 with frontier labels in its half-share band, under the 0.9 bar and equal to the majority label; E2b (what kind of failure is this test failure, n 50) agreed 0.629 there |
| `exp-e3.md` | EXP-M3 | STOP | the agentic query prototype on 5 real questions: the table was at least as correct as reading the files on 3 of 5 and used 1.39 times the reading arm's context |

## Spend

| Run | USD | Requests |
|---|---|---|
| E1 | 0.0176 | 17 |
| E2a | 0.0029 | 3 |
| E2b | 0.0021 | 2 |
| E3 | 0.0046 | 5 |
| Stage 0 | 0.0272 | 27 |

US$0.0272 of stage 0's US$2 cap and of the owner's US$10 for Jev on OpenRouter, every request priced from provider-reported usage. Stage 1's US$8 is not spent.

## In plain language

The bet was that a fast decision model reads code well enough that fathom can hand it the rows it cannot compute. Stage 0 says it does not, at least not yet and not on these questions. Where the truth was computable, a careful grep was as good or better on every template. Where it was not, the model agreed with a careful reviewer only about as often as always giving the most common answer. And on five real questions from recent waves, fathom's rows plus the model's labels got one right, against one right and two half right for an agent that simply read the files, while costing more context, not less. The money was never the problem: the whole stage cost under three cents.

So the stage gate closes. Stage 1 (`fathom ask`'s judged column, AQ-7, and its week of use, EXP-M5) and stage 2 (the agent A/B bench, JP-14) stop, as the plan's stage-0 Kill and ADR-073 say. The consent and kill rows (KS-1 to KS-7) were never part of the bet and stand. J0 (`judged-predicates.md` JP-M1) still gates everything else that would read code with a model.

What stays is the `fathom ask` spine at provider `none` (AQ-1 to AQ-6), where the roadmap put it, after readouts and mods, under its own kill: on 2027-01-31 a spine no wave session has run is removed. E3 is a warning for it, not a kill. Fathom's computed facts answered none of the 177 rows E3 selected, so at provider `none` the spine would have printed the selecting verbs' own rows. Before AQ-1 is built, the spine should show one question shape from real waves where a computed column answers rows a verb alone does not. E3 also found fathom-tool gaps that cost more correctness than the model did, and those are worth fixing whatever happens to the spine: a call inside a package-level function-literal variable shows up as every `init()` in its package (O1124), and a delete blast radius on a Go type misses the functions that take the type in a signature (`exp-e3.md`).

Two things this reading does not decide. F35's Drop if (agreement under 0.8 after one template revision) has not fired: E2b read 0.640 overall with no revision made. And no owner spot-check of the frontier labels was given (Q-DM5), so E2's figures are agreement with one reviewer, not accuracy.
