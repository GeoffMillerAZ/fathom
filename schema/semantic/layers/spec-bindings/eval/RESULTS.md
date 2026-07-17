# spec-bindings — eval results

Gate: docs/design/semantic-layers.md §4.6 — precision ≥ 0.90 on (claim, probe)
application pairs, recall ≥ 0.75 on labeled rules. Run via the shared harness
(`internal/semantic/eval_test.go`'s `runLayerEval`,
`TestSpecBindingsEval_Precision`).

## 2026-07-17 — PASS (production model, expanded fixtures; 2 consecutive runs)

The AUTHORITATIVE gate record. Fixture corpus: `plan-fixture.md`, expanded
to the obligations precedent's shape — 6 labeled rules across 6 packages
(spanning `[ ]`, `[x]`, and `[~]` checkbox states), plus 2 deliberate traps
that must produce no claim (`SB-5`, a `disposition: deferred` task with no
`package`; and an untracked idea line with no annotation block at all).

- **Lane:** `claude-cli` (subscription, no API key)
- **Model:** `claude-haiku-4-5-20251001` — pinned to the PRODUCTION compile
  default (`compileModel()`, cmd/fathom/compile.go), so these metrics
  transfer to the fleet-compile lane
- **Command:**

```
FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=claude-cli \
  go test ./internal/semantic -run TestSpecBindingsEval_Precision -v
```

| Run | Model | Claims emitted | Rejected | Precision | Recall | TP | FP | FN | Missed |
|-----|-------|-----------------|----------|-----------|--------|----|----|----|--------|
| 1   | claude-haiku-4-5-20251001 | 6 | 0 | 1.00 | 1.00 | 6 | 0 | 0 | none |
| 2   | claude-haiku-4-5-20251001 | 6 | 0 | 1.00 | 1.00 | 6 | 0 | 0 | none |

Exactly one claim per labeled rule in both runs — SB-1 through SB-4, SB-7,
SB-8 one-for-one; both trap lines correctly produced nothing.

**Result: PASS.** spec-bindings ships in the embedded set. No prompt
iteration was needed at any point.

## History

### 2026-07-17 (earlier, superseded) — PASS on the initial 4-rule fixture

Two consecutive 1.00/1.00 runs on the original 4-rule/2-trap fixture,
`claude-cli` lane with the CLI's own (unpinned) default model. Superseded by
the authoritative record above after the coordinator's critic asked for (a)
fixture counts matching the obligations precedent and (b) an explicitly
recorded model.
