EXP-M2 verdict: NONE

# E2 reading: does a fast model agree with a careful reviewer (EXP-M2)

Run under the pre-registration `exp-preregistration.md` (EXP-1, fathom fd091c4f), 2026-10-17, through `decide.Open` (typesafe over OpenRouter). Labels were written by a Claude reviewer (provenance `frontier`) before any E2 call; no owner spot-check was given, so no `human` labels exist. This measures labelled agreement with frontier labels, never accuracy. Aggregates only; no span text, path or name is recorded here.

No question qualifies. E2a: the band holding at least half the answered items (0.5 to 0.9, 64 percent) agrees at 0.844, under the 0.9 bar, and equals the majority-label baseline there; its 0.9-and-above band is empty. E2b: the band holding at least half the answered items (0.5 to 0.9, 70 percent) agrees at 0.629, under the 0.9 bar; the 0.9-and-above band holds 8 items (16 percent), too few to be the half-share band.

## E2a: does this caller depend on the changed behaviour

n = 50. Frontier labels: 35 true, 15 false. Majority-label baseline: true.

| Band | n | Share | Agreement | Wilson 95% | Baseline | Lift |
|---|---|---|---|---|---|---|
| below 0.1 | 2 | 0.040 | 0.500 | 0.095 to 0.905 | 0.500 | +0.000 |
| 0.1 to 0.5 | 16 | 0.320 | 0.563 | 0.332 to 0.769 | 0.438 | +0.125 |
| 0.5 to 0.9 | 32 | 0.640 | 0.844 | 0.682 to 0.931 | 0.844 | +0.000 |
| 0.9 and above | 0 | 0.000 | n/a | n/a | n/a | n/a |

Overall agreement 0.740 against baseline 0.700 (lift +0.040). AUROC against the frontier labels 0.825. Invalid, declined, failed and unsent: 0. Model answers: 32 true, 18 false.

Cost US$0.0029 (3 calls, all provider-reported), US$0.0059 per 100 items. Latency p50 362 ms, p95 526 ms.

## E2b: what kind of failure is this test failure

Ran under the second page `exp-preregistration-2026-10-17.md` (commit 7c688c49, written before any E2b call), on the rebuilt item set: 50 items drawn from 360 candidate test-run logs (input sha256 c71bf39c). The earlier 7-item finding above (fix commits as the source) is superseded for E2b only; E2a is not re-run and its numbers stand. Labels by a Claude reviewer (provenance `frontier`), written before the call; no `human` labels.

n = 50. Frontier labels: code_bug 31, outdated_expectation 12, other 5, environment 2, timing 0. Majority-label baseline: code_bug (0.620). Model answers: code_bug 33, outdated_expectation 11, environment 3, other 2, timing 1. Invalid, declined, failed and unsent: 0.

| Band | n | Share | Agreement | Wilson 95% | Baseline | Lift |
|---|---|---|---|---|---|---|
| below 0.1 | 0 | 0.000 | n/a | n/a | n/a | n/a |
| 0.1 to 0.5 | 7 | 0.140 | 0.286 | 0.082 to 0.641 | 0.571 | -0.286 |
| 0.5 to 0.9 | 35 | 0.700 | 0.629 | 0.463 to 0.768 | 0.571 | +0.057 |
| 0.9 and above | 8 | 0.160 | 1.000 | 0.676 to 1.000 | 0.875 | +0.125 |

Overall agreement 0.640 (32 of 50) against baseline 0.620 (lift +0.020). AUROC is undefined for a choice question.

Per-option counts (the harness records marginals, not the joint confusion matrix, so per-cell counts are not available): label versus model answer, code_bug 31 / 33, outdated_expectation 12 / 11, other 5 / 2, environment 2 / 3, timing 0 / 1.

Cost US$0.0021 (2 calls, both provider-reported), US$0.0041 per 100 items. Latency p50 309 ms, p95 536 ms.

## Spend

E2a US$0.0029, E2b US$0.0021. Stage 0 so far (E1 US$0.0176, E2a US$0.0029, E2b US$0.0021): US$0.0226 of the US$2 cap.

## In plain language

On the caller question the fast model agreed with the careful reviewer on 37 of 50 items, only four better than always saying the caller depends on the change, and it never reached the 0.9 band. On the failure-kind question it agreed on 32 of 50, only one better than always saying code_bug. Its most confident answers (8 items at 0.9 and above) all matched the reviewer, which is a real signal but covers just 16 percent of items and the majority guess already gets 88 percent of them; in the middle band it matched 63 percent and in the low band 29 percent, below the guess. Neither question meets the bar of 0.9 agreement over at least half the items, so no question qualifies. The samples are small, so every band bound is wide, and these are agreements with one reviewer's labels, not accuracy.
