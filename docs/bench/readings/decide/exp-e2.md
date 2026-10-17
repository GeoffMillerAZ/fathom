EXP-M2 verdict: NONE

# E2 reading: does a fast model agree with a careful reviewer (EXP-M2)

Run under the pre-registration `exp-preregistration.md` (EXP-1, fathom fd091c4f), 2026-10-17, through `decide.Open` (typesafe over OpenRouter). Labels were written by a Claude reviewer (provenance `frontier`) before any E2 call; no owner spot-check was given, so no `human` labels exist. This measures labelled agreement with frontier labels, never accuracy. Aggregates only; no span text, path or name is recorded here.

No question qualifies. The band holding at least half the answered items (0.5 to 0.9, 64 percent) agrees at 0.844, under the 0.9 bar, and equals the majority-label baseline on those items; the 0.9-and-above band is empty.

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

Not run. The builder scanned all 45 candidate fix commits (fix-subject, non-merge, touching a .go file, among the 300 before fd091c4f) and found 7 fail-then-green items, short of the 50 needed, so 50 was unreachable by construction: fix commits add their regression test in the same commit, so it does not exist at the parent, and tests that do exist pass there. No E2b labels were written and no E2b call was made.

## Spend

E2 US$0.0029. Stage 0 so far (E1 US$0.0176 plus E2 US$0.0029): US$0.0205 of the US$2 cap.

## In plain language

On the caller question the fast model agreed with the careful reviewer on 37 of 50 items, only four better than always saying the caller depends on the change. It was never confident enough to reach the 0.9 band. Where it leaned yes with middling confidence (0.5 to 0.9) it matched the reviewer 84 percent of the time, exactly what the majority guess gets there. Where it doubted (0.1 to 0.5) it agreed only 56 percent,, though that is above the guess. It ranks reasonably (AUROC 0.82) but ranking is not agreement, and the sample is small, so every band bound is wide. The failure-kind question could not be tested at all.
