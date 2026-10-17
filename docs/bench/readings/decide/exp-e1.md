EXP-M1 verdict: NONE

# E1 reading: does a fast model read code better than a careful grep (EXP-M1)

Run under the pre-registration `exp-preregistration.md` (EXP-1, fathom fd091c4f), 2026-10-16, seed 20261016, through `decide.Open` (typesafe over OpenRouter). The dry run's counts matched section 7 exactly (pool, draw, truth-true, baseline accuracy, too-long, requests) before any call. Aggregates only; no span text, path or name is recorded here.

No template qualifies. In no template does a band holding at least half the answered items both read 0.9 accuracy and strictly beat the lexical baseline on those items.

| Template | n | Model accuracy | Baseline accuracy | Lift | AUROC | p50 / p95 ms | Spend USD | USD per 100 spans |
|---|---|---|---|---|---|---|---|---|
| errcheck | 200 | 0.820 | 0.935 | -0.115 | 0.797 | 374 / 557 | 0.0044 | 0.0022 |
| wrap | 200 | 0.925 | 0.975 | -0.050 | 0.992 | 326 / 375 | 0.0036 | 0.0018 |
| assert | 200 | 1.000 | 1.000 | +0.000 | undefined | 339 / 350 | 0.0043 | 0.0021 |
| lock | 200 | 0.900 | 0.925 | -0.025 | 0.936 | 341 / 401 | 0.0053 | 0.0027 |

Invalid, declined, failed and unsent items: 0 in every template. 17 requests, all with provider-reported usage.

## errcheck

| Band | n | Share | Model accuracy | Wilson 95% | Baseline accuracy | Lift |
|---|---|---|---|---|---|---|
| below 0.1 | 6 | 0.030 | 1.000 | 0.610 to 1.000 | 1.000 | +0.000 |
| 0.1 to 0.5 | 9 | 0.045 | 1.000 | 0.701 to 1.000 | 0.889 | +0.111 |
| 0.5 to 0.9 | 88 | 0.440 | 0.705 | 0.602 to 0.790 | 0.864 | -0.159 |
| 0.9 and above | 97 | 0.485 | 0.897 | 0.821 to 0.943 | 1.000 | -0.103 |

## wrap

| Band | n | Share | Model accuracy | Wilson 95% | Baseline accuracy | Lift |
|---|---|---|---|---|---|---|
| below 0.1 | 17 | 0.085 | 1.000 | 0.816 to 1.000 | 0.882 | +0.118 |
| 0.1 to 0.5 | 16 | 0.080 | 0.125 | 0.035 to 0.360 | 0.875 | -0.750 |
| 0.5 to 0.9 | 57 | 0.285 | 0.982 | 0.907 to 0.997 | 0.982 | +0.000 |
| 0.9 and above | 110 | 0.550 | 1.000 | 0.966 to 1.000 | 1.000 | +0.000 |

## assert

| Band | n | Share | Model accuracy | Wilson 95% | Baseline accuracy | Lift |
|---|---|---|---|---|---|---|
| below 0.1 | 0 | 0.000 | n/a | n/a | n/a | n/a |
| 0.1 to 0.5 | 0 | 0.000 | n/a | n/a | n/a | n/a |
| 0.5 to 0.9 | 25 | 0.125 | 1.000 | 0.867 to 1.000 | 1.000 | +0.000 |
| 0.9 and above | 175 | 0.875 | 1.000 | 0.979 to 1.000 | 1.000 | +0.000 |

## lock

| Band | n | Share | Model accuracy | Wilson 95% | Baseline accuracy | Lift |
|---|---|---|---|---|---|---|
| below 0.1 | 10 | 0.050 | 1.000 | 0.722 to 1.000 | 1.000 | +0.000 |
| 0.1 to 0.5 | 34 | 0.170 | 0.882 | 0.734 to 0.953 | 0.971 | -0.088 |
| 0.5 to 0.9 | 82 | 0.410 | 0.817 | 0.720 to 0.886 | 0.841 | -0.024 |
| 0.9 and above | 74 | 0.370 | 0.986 | 0.927 to 0.998 | 0.986 | +0.000 |

## Spend

US$0.0176 provider-reported usage priced at US$0.042 per million input tokens (dry-run estimate was US$0.0189), against the US$2 cap; US$1.9824 remains for E2 and E3.

## In plain language

On these questions a fast model does not read code better than a careful grep. The greps are already strong (0.925 to 1.0 accurate), and the model is below them on errcheck (0.820 against 0.935), wrap (0.925 against 0.975) and lock (0.900 against 0.925); on assert both are perfect and the template could never qualify, since every drawn test asserts. Where the model is confident (the 0.9-and-above band) it is as accurate as grep on wrap, assert and lock and 0.103 below it on errcheck, so confidence earns no edge. Its middle bands are where it loses: errcheck 0.5 to 0.9 reads 0.705 against 0.864, and wrap 0.1 to 0.5 reads 0.125 (the model doubted statements that were true). It ranks well (AUROC 0.80 to 0.99) but ranking is not beating the lexical answer.

For the judged column this means the decision model should not be asked these four predicate questions in place of a computed or lexical answer: where truth is computable or a grep nearly is, the computed answer is cheaper and at least as right. The model was fast (p50 about 0.33 to 0.37 s per batched request) and cheap (about US$0.002 per 100 spans), so cost and latency are not the obstacle; accuracy against a strong baseline is. Whether it helps on questions with no computable truth is E2's question, and the agentic table-plus-residue design is E3's.
