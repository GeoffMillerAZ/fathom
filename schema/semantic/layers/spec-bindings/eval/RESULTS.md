# spec-bindings — eval results

Gate: docs/design/semantic-layers.md §4.6 — precision ≥ 0.90 on (claim, probe)
application pairs, recall ≥ 0.75 on labeled rules. Run via the shared harness
(`internal/semantic/eval_test.go`'s `runLayerEval`,
`TestSpecBindingsEval_Precision`), LIVE against the `claude-cli`
subscription lane (`FATHOM_LLM_PROVIDER=claude-cli`, no API key — the CLI's
own default model, no `--model` override).

## 2026-07-17 — PASS (2 consecutive runs)

```
FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=claude-cli \
  go test ./internal/semantic/... -run TestSpecBindingsEval_Precision -v
```

| Run | Claims emitted | Rejected | Precision | Recall | TP | FP | FN | Missed |
|-----|-----------------|----------|-----------|--------|----|----|----|--------|
| 1   | 4               | 0        | 1.00      | 1.00   | 4  | 0  | 0  | none   |
| 2   | 4               | 0        | 1.00      | 1.00   | 4  | 0  | 0  | none   |

**Result: PASS on the first attempt** — no prompt iteration needed. The
fixture's two deliberate traps (`SB-5`, `disposition: deferred` with no
`package`; an untracked idea with no annotation block at all) were correctly
skipped in both runs: exactly 4 claims emitted, matching SB-1 through SB-4
one-for-one, zero false positives from either trap line. spec-bindings ships
in the embedded set.
