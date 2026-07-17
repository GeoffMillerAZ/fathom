# adr-rules — eval results

Gate: docs/design/semantic-layers.md §4.6 — precision ≥ 0.90 on (claim, probe)
application pairs, recall ≥ 0.75 on labeled rules. Run via the shared harness
(`internal/semantic/eval_test.go`'s `runLayerEval`,
`TestADRRulesEval_Precision`), LIVE against the `claude-cli` subscription
lane (`FATHOM_LLM_PROVIDER=claude-cli`, no API key — the CLI's own default
model, no `--model` override).

## 2026-07-17 — PASS (2 consecutive runs)

```
FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=claude-cli \
  go test ./internal/semantic/... -run TestADRRulesEval_Precision -v
```

| Run | Claims emitted | Rejected | Precision | Recall | TP | FP | FN | Missed |
|-----|-----------------|----------|-----------|--------|----|----|----|--------|
| 1   | 3               | 0        | 1.00      | 1.00   | 3  | 0  | 0  | none   |
| 2   | 3               | 0        | 1.00      | 1.00   | 3  | 0  | 0  | none   |

**Result: PASS.** adr-rules ships in the embedded set.

## Iteration note

The first run (before this fixture was finalized) scored precision 0.50 /
recall 0.67 — both fixture bugs, not prompt/model quality: `labels.json`
`cite_substr` values spanned a Markdown line-wrap boundary in
`eval/decisions.md` (`\n`-joined cited text broke a substring check that
assumed single-line contiguity), and one Decision paragraph packed two
independent clauses the model reasonably split into two claims, one of
which then had no matching labeled rule. Fixed by keeping each Decision to
one governing clause and choosing `cite_substr` values that live entirely
within one source line — both are eval-harness authoring lessons for future
packs, not adr-rules prompt changes.
